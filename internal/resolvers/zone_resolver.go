package resolvers

import (
	"context"
	"errors"
	"net"
	"sort"
	"strings"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/jroosing/hydradns/internal/zone"
)

// ZoneResolver answers DNS queries from locally configured zone files.
// It is authoritative for all configured zones.
type ZoneResolver struct {
	Zones []*zone.Zone
}

// NewZoneResolver creates a ZoneResolver for the given zones.
func NewZoneResolver(zones []*zone.Zone) *ZoneResolver {
	// Sort zones by origin length descending to ensure most specific match
	sorted := make([]*zone.Zone, len(zones))
	copy(sorted, zones)
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i].Origin) > len(sorted[j].Origin)
	})
	return &ZoneResolver{Zones: sorted}
}

// Close is a no-op for ZoneResolver (satisfies Resolver interface).
func (z *ZoneResolver) Close() error { return nil }

// Resolve answers a DNS query from local zone data.
// Returns an error if the query name is not within any configured zone.
func (z *ZoneResolver) Resolve(_ context.Context, req dns.Packet, _ []byte) (Result, error) {
	if len(z.Zones) == 0 {
		return Result{}, errors.New("no zones configured")
	}
	if len(req.Questions) == 0 {
		return Result{}, errors.New("no question")
	}

	q := req.Questions[0]
	match := z.findMatchingZone(q.Name)
	if match == nil {
		return Result{}, errors.New("name not in any configured zone")
	}

	return z.buildResponse(req, q, match)
}

// findMatchingZone finds the zone that contains the given name.
func (z *ZoneResolver) findMatchingZone(qname string) *zone.Zone {
	for _, cand := range z.Zones {
		if cand.ContainsName(qname) {
			return cand
		}
	}
	return nil
}

// buildResponse constructs a DNS response for the given question from zone data.
func (z *ZoneResolver) buildResponse(req dns.Packet, q dns.Question, match *zone.Zone) (Result, error) {
	answers := z.lookupRecords(match, q.Name, q.Type, q.Class)
	additionals := make([]dns.Record, 0)

	// Handle CNAME chasing for A/AAAA queries
	if len(answers) == 0 && isAddressQuery(q.Type) {
		answers, additionals = z.chaseCNAME(match, q)
	}

	flags := z.buildResponseFlags(req.Header.Flags, match, q, len(answers) > 0)
	authorities := z.buildAuthoritySection(match, q, len(answers) == 0)

	resp := dns.Packet{
		Header:      dns.Header{ID: req.Header.ID, Flags: flags},
		Questions:   []dns.Question{q},
		Answers:     answers,
		Authorities: authorities,
		Additionals: additionals,
	}

	b, err := resp.Marshal()
	if err != nil {
		return Result{}, err
	}
	return Result{ResponseBytes: b, Source: "zone"}, nil
}

// lookupRecords retrieves matching records from the zone.
func (z *ZoneResolver) lookupRecords(match *zone.Zone, qname string, qtype, qclass uint16) []dns.Record {
	answers := make([]dns.Record, 0)
	for _, rr := range match.Lookup(qname, qtype, qclass) {
		answers = append(answers, zoneRecordToDNSRecord(rr))
	}
	return answers
}

// isAddressQuery returns true for A or AAAA queries.
func isAddressQuery(qtype uint16) bool {
	return qtype == uint16(dns.TypeA) || qtype == uint16(dns.TypeAAAA)
}

// chaseCNAME follows CNAME records when no direct answer exists.
// If a CNAME exists, it returns the CNAME as the answer and looks up
// the target name for the additional section.
func (z *ZoneResolver) chaseCNAME(match *zone.Zone, q dns.Question) (answers, additionals []dns.Record) {
	cnames := match.Lookup(q.Name, uint16(dns.TypeCNAME), q.Class)
	if len(cnames) == 0 {
		return nil, nil
	}

	rr := cnames[0]
	target := rr.RData.(string)
	answers = append(answers, dns.Record{
		Name:  rr.Name,
		Type:  rr.Type,
		Class: rr.Class,
		TTL:   rr.TTL,
		Data:  target,
	})

	for _, a := range match.Lookup(target, q.Type, q.Class) {
		additionals = append(additionals, zoneRecordToDNSRecord(a))
	}
	return answers, additionals
}

// buildResponseFlags constructs the DNS header flags for the response.
//
// Flag construction for authoritative zone responses:
//   - QR (bit 15): Set to 1 (this is a response)
//   - AA (bit 10): Set to 1 (authoritative answer)
//   - RD (bit 8): Preserved from request (recursion desired)
//   - RCODE (bits 3-0): NOERROR or NXDOMAIN based on lookup result
func (z *ZoneResolver) buildResponseFlags(reqFlags uint16, match *zone.Zone, q dns.Question, hasAnswer bool) uint16 {
	// Start with request flags, then set response bits
	flags := reqFlags

	// Set QR (response) and AA (authoritative)
	flags |= dns.QRFlag | dns.AAFlag

	// Preserve RD if set in request
	flags |= (reqFlags & dns.RDFlag)

	// Determine RCODE
	if !hasAnswer {
		nameExists := match.NameExists(q.Name, q.Class)
		rcode := uint16(dns.RCodeNoError)
		if !nameExists {
			rcode = uint16(dns.RCodeNXDomain)
		}
		// Clear existing RCODE bits and set new value
		flags = (flags &^ dns.RCodeMask) | (rcode & dns.RCodeMask)
	}

	return flags
}

// buildAuthoritySection returns SOA record for negative responses.
func (z *ZoneResolver) buildAuthoritySection(match *zone.Zone, q dns.Question, isNegative bool) []dns.Record {
	if !isNegative {
		return nil
	}

	authorities := make([]dns.Record, 0)
	if soa := match.SOA(q.Class); soa != nil {
		b, _ := soa.RData.([]byte)
		authorities = append(authorities, dns.Record{
			Name:  soa.Name,
			Type:  soa.Type,
			Class: soa.Class,
			TTL:   soa.TTL,
			Data:  b,
		})
	}
	return authorities
}

// zoneRecordToDNSRecord converts a zone.Record to a dns.Record.
// It handles type-specific RDATA formatting (e.g., IP address parsing).
func zoneRecordToDNSRecord(rr zone.Record) dns.Record {
	switch dns.RecordType(rr.Type) {
	case dns.TypeA:
		return convertARecord(rr)
	case dns.TypeAAAA:
		return convertAAAARecord(rr)
	case dns.TypeMX:
		mx := rr.RData.(zone.MX)
		return dns.Record{
			Name:  rr.Name,
			Type:  rr.Type,
			Class: rr.Class,
			TTL:   rr.TTL,
			Data:  dns.MXData{Preference: mx.Preference, Exchange: mx.Exchange},
		}
	case dns.TypeSOA:
		return dns.Record{Name: rr.Name, Type: rr.Type, Class: rr.Class, TTL: rr.TTL, Data: rr.RData.([]byte)}
	default:
		return dns.Record{Name: rr.Name, Type: rr.Type, Class: rr.Class, TTL: rr.TTL, Data: rr.RData}
	}
}

// convertARecord converts an A record, parsing the IPv4 address string to bytes.
func convertARecord(rr zone.Record) dns.Record {
	ip := net.ParseIP(strings.TrimSpace(rr.RData.(string)))
	if ip == nil {
		return dns.Record{Name: rr.Name, Type: rr.Type, Class: rr.Class, TTL: rr.TTL, Data: []byte{0, 0, 0, 0}}
	}
	ipv4 := ip.To4()
	if ipv4 == nil {
		return dns.Record{Name: rr.Name, Type: rr.Type, Class: rr.Class, TTL: rr.TTL, Data: []byte{0, 0, 0, 0}}
	}
	return dns.Record{Name: rr.Name, Type: rr.Type, Class: rr.Class, TTL: rr.TTL, Data: []byte(ipv4)}
}

// convertAAAARecord converts an AAAA record, parsing the IPv6 address string to bytes.
func convertAAAARecord(rr zone.Record) dns.Record {
	ip := net.ParseIP(strings.TrimSpace(rr.RData.(string)))
	if ip == nil {
		return dns.Record{Name: rr.Name, Type: rr.Type, Class: rr.Class, TTL: rr.TTL, Data: make([]byte, 16)}
	}
	ipv6 := ip.To16()
	if ipv6 == nil {
		return dns.Record{Name: rr.Name, Type: rr.Type, Class: rr.Class, TTL: rr.TTL, Data: make([]byte, 16)}
	}
	return dns.Record{Name: rr.Name, Type: rr.Type, Class: rr.Class, TTL: rr.TTL, Data: []byte(ipv6)}
}
