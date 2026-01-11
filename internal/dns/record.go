package dns

import (
	"encoding/binary"
	"fmt"

	"github.com/jroosing/hydradns/internal/helpers"
)

// RRHeader contains common metadata for DNS resource records.
// This is distinct from Header which is the DNS message header.
type RRHeader struct {
	Name  string
	Class uint16
	TTL   uint32
}

// NewRRHeader creates a new resource record header.
func NewRRHeader(name string, class RecordClass, ttl uint32) RRHeader {
	return RRHeader{Name: name, Class: uint16(class), TTL: ttl}
}

// Record is the interface for DNS resource records.
// All DNS records implement this interface for type-safe handling.
type Record interface {
	// Type returns the DNS record type.
	Type() RecordType

	// Header returns the record's metadata.
	Header() RRHeader

	// SetHeader sets the record's metadata.
	SetHeader(h RRHeader)

	// MarshalRData marshals the record-specific data (RDATA) to wire format.
	MarshalRData() ([]byte, error)
}

// ParseRecord parses a resource record from wire format.
// It advances *off past the parsed record on success.
func ParseRecord(msg []byte, off *int) (Record, error) {
	name, err := DecodeName(msg, off)
	if err != nil {
		return nil, err
	}
	if *off+10 > len(msg) {
		return nil, fmt.Errorf("%w: unexpected EOF while reading DNS record", ErrDNSError)
	}
	rrType := binary.BigEndian.Uint16(msg[*off : *off+2])
	rrClass := binary.BigEndian.Uint16(msg[*off+2 : *off+4])
	ttl := binary.BigEndian.Uint32(msg[*off+4 : *off+8])
	rdlen := binary.BigEndian.Uint16(msg[*off+8 : *off+10])
	*off += 10
	start := *off
	if start+int(rdlen) > len(msg) {
		return nil, fmt.Errorf("%w: unexpected EOF while reading DNS record rdata", ErrDNSError)
	}

	tr, err := parseRData(RecordType(rrType), msg, off, start, int(rdlen))
	if err != nil {
		return nil, err
	}
	tr.SetHeader(RRHeader{Name: name, Class: rrClass, TTL: ttl})

	return tr, nil
}

// parseRData parses RDATA into a Record based on record type.
//
// For a forwarding DNS server, we only parse record types needed for:
//   - Custom DNS construction: A, AAAA, CNAME, NS, PTR
//   - Everything else uses OpaqueRecord for transparent forwarding
func parseRData(rt RecordType, msg []byte, off *int, start, rdlen int) (Record, error) {
	switch rt {
	case TypeA, TypeAAAA:
		return ParseIPRData(msg, off, rdlen)
	case TypeCNAME, TypeNS, TypePTR:
		return ParseNameRData(msg, off, start, rdlen, rt)
	default:
		// All other record types (MX, SRV, CAA, TXT, OPT, DNSSEC, etc.)
		// are passed through opaquely for forwarding
		return ParseOpaqueRData(msg, off, rdlen, rt)
	}
}

// MarshalRecord converts a Record to wire-format bytes.
func MarshalRecord(r Record) ([]byte, error) {
	rdata, err := r.MarshalRData()
	if err != nil {
		return nil, err
	}
	h := r.Header()
	return marshalRecordWithRData(h, r.Type(), rdata)
}

// marshalRecordWithRData marshals a Record using pre-computed RDATA.
func marshalRecordWithRData(h RRHeader, rt RecordType, rdata []byte) ([]byte, error) {
	nameWire := []byte{0}
	if rt != TypeOPT {
		b, err := EncodeName(h.Name)
		if err != nil {
			return nil, err
		}
		nameWire = b
	}

	out := make([]byte, 0, len(nameWire)+10+len(rdata))
	if len(rdata) > 65535 {
		return nil, fmt.Errorf("rdata too large: %d bytes (max 65535)", len(rdata))
	}
	out = append(out, nameWire...)
	fixed := make([]byte, 10)
	binary.BigEndian.PutUint16(fixed[0:2], uint16(rt))
	binary.BigEndian.PutUint16(fixed[2:4], h.Class)
	binary.BigEndian.PutUint32(fixed[4:8], h.TTL)
	binary.BigEndian.PutUint16(fixed[8:10], helpers.ClampIntToUint16(len(rdata)))
	out = append(out, fixed...)
	out = append(out, rdata...)
	return out, nil
}
