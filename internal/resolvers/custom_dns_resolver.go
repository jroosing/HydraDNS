package resolvers

import (
	"context"
	"errors"
	"net/netip"
	"strings"

	"github.com/jroosing/hydradns/internal/dns"
)

// CustomDNSResolver provides simple A/AAAA/CNAME resolution from configuration.
// It's designed for homelab environments with straightforward DNS needs.
//
// Supported record types:
//   - A records (IPv4 addresses)
//   - AAAA records (IPv6 addresses)
//   - CNAME records (aliases)
//
// Responses are marked as authoritative (AA flag set) for configured domains.
// Name normalization converts domains to lowercase without trailing dots,
// making lookups case-insensitive per RFC 1035.
type CustomDNSResolver struct {
	hosts  map[string][]netip.Addr // normalized name -> IP addresses
	cnames map[string]string       // normalized alias -> normalized canonical name
}

// NewCustomDNSResolver creates a CustomDNSResolver from host and CNAME mappings.
//
// Hosts map domain names to IP addresses (IPv4 or IPv6).
// CNAMEs map alias names to canonical names.
//
// Domain names are normalized to lowercase without trailing dots.
func NewCustomDNSResolver(hosts map[string][]string, cnames map[string]string) (*CustomDNSResolver, error) {
	r := &CustomDNSResolver{
		hosts:  make(map[string][]netip.Addr),
		cnames: make(map[string]string),
	}

	// Parse and normalize hosts
	for name, ips := range hosts {
		normalized := normalizeName(name)
		var addrs []netip.Addr
		for _, ip := range ips {
			addr, err := netip.ParseAddr(strings.TrimSpace(ip))
			if err != nil {
				return nil, errors.New("invalid IP address for " + name + ": " + ip)
			}
			addrs = append(addrs, addr)
		}
		if len(addrs) > 0 {
			r.hosts[normalized] = addrs
		}
	}

	// Normalize CNAMEs
	for alias, target := range cnames {
		r.cnames[normalizeName(alias)] = normalizeName(target)
	}

	return r, nil
}

// Close is a no-op (implements Resolver interface).
func (r *CustomDNSResolver) Close() error {
	return nil
}

// Resolve answers DNS queries from configured hosts and CNAMEs.
func (r *CustomDNSResolver) Resolve(_ context.Context, req dns.Packet, _ []byte) (Result, error) {
	if len(req.Questions) == 0 {
		return Result{}, errors.New("no question in request")
	}

	q := req.Questions[0]
	qname := normalizeName(q.Name)

	// Check for CNAME first
	if target, ok := r.cnames[qname]; ok {
		return r.buildCNAMEResponse(req, q, target)
	}

	// Check for A/AAAA records
	if addrs, ok := r.hosts[qname]; ok {
		return r.buildAddressResponse(req, q, addrs)
	}

	// Name not found
	return Result{}, errors.New("name not in custom DNS configuration")
}

// buildCNAMEResponse constructs a CNAME response.
func (r *CustomDNSResolver) buildCNAMEResponse(req dns.Packet, q dns.Question, target string) (Result, error) {
	// Create CNAME record
	header := dns.NewRRHeader(q.Name, dns.RecordClass(q.Class), 3600)
	cname := dns.NewNameRecord(header, dns.TypeCNAME, target)

	// Build response with CNAME
	resp := dns.Packet{
		Header: dns.Header{
			ID:    req.Header.ID,
			Flags: buildCustomDNSFlags(req.Header.Flags),
		},
		Questions: []dns.Question{q},
		Answers:   []dns.Record{cname},
	}

	// If querying for A/AAAA, try to resolve the target
	if q.Type == uint16(dns.TypeA) || q.Type == uint16(dns.TypeAAAA) {
		targetNorm := normalizeName(target)
		if addrs, ok := r.hosts[targetNorm]; ok {
			for _, addr := range addrs {
				if matchesQueryType(addr, q.Type) {
					h := dns.NewRRHeader(target, dns.RecordClass(q.Class), 3600)
					resp.Additionals = append(resp.Additionals, dns.NewIPRecord(h, addr.AsSlice()))
				}
			}
		}
	}

	b, err := resp.Marshal()
	if err != nil {
		return Result{}, err
	}
	return Result{ResponseBytes: b, Source: "custom-dns"}, nil
}

// buildAddressResponse constructs an A or AAAA response.
func (r *CustomDNSResolver) buildAddressResponse(req dns.Packet, q dns.Question, addrs []netip.Addr) (Result, error) {
	var answers []dns.Record

	for _, addr := range addrs {
		if matchesQueryType(addr, q.Type) {
			header := dns.NewRRHeader(q.Name, dns.RecordClass(q.Class), 3600)
			answers = append(answers, dns.NewIPRecord(header, addr.AsSlice()))
		}
	}

	// No matching addresses found
	if len(answers) == 0 {
		return Result{}, errors.New("no matching address records")
	}

	resp := dns.Packet{
		Header: dns.Header{
			ID:    req.Header.ID,
			Flags: buildCustomDNSFlags(req.Header.Flags),
		},
		Questions: []dns.Question{q},
		Answers:   answers,
	}

	b, err := resp.Marshal()
	if err != nil {
		return Result{}, err
	}
	return Result{ResponseBytes: b, Source: "custom-dns"}, nil
}

// matchesQueryType checks if an IP address matches the query type (A or AAAA).
func matchesQueryType(addr netip.Addr, qtype uint16) bool {
	if qtype == uint16(dns.TypeA) {
		return addr.Is4()
	}
	if qtype == uint16(dns.TypeAAAA) {
		return addr.Is6()
	}
	return false
}

// buildCustomDNSFlags constructs response flags for custom DNS responses.
// Sets QR (response) and AA (authoritative). Does not set RA or preserve RD
// since these are authoritative responses, not recursive lookups.
func buildCustomDNSFlags(_ uint16) uint16 {
	return dns.QRFlag | dns.AAFlag
}

// normalizeName converts a domain name to lowercase and removes trailing dot.
func normalizeName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.TrimSuffix(name, ".")
}

// ContainsDomain checks if a domain is configured in custom DNS.
func (r *CustomDNSResolver) ContainsDomain(name string) bool {
	normalized := normalizeName(name)
	_, hasHost := r.hosts[normalized]
	_, hasCNAME := r.cnames[normalized]
	return hasHost || hasCNAME
}

// IsEmpty returns true if no custom DNS entries are configured.
func (r *CustomDNSResolver) IsEmpty() bool {
	return len(r.hosts) == 0 && len(r.cnames) == 0
}
