package dns

import (
	"fmt"
	"net"
)

// IPRecord represents a DNS A or AAAA record containing an IP address.
// The Type is determined by the IP address version (IPv4 → TypeA, IPv6 → TypeAAAA).
type IPRecord struct {
	H    RRHeader
	Addr net.IP
}

// NewIPRecord creates a new IP record (A or AAAA based on address type).
func NewIPRecord(h RRHeader, addr net.IP) *IPRecord {
	return &IPRecord{H: h, Addr: addr}
}

// Type returns TypeA for IPv4 addresses, TypeAAAA for IPv6.
func (r *IPRecord) Type() RecordType {
	if r.Addr.To4() != nil {
		return TypeA
	}
	return TypeAAAA
}

// Header returns the record header.
func (r *IPRecord) Header() RRHeader { return r.H }

// SetHeader sets the record header.
func (r *IPRecord) SetHeader(h RRHeader) { r.H = h }

// MarshalRData marshals the IP address to wire format.
func (r *IPRecord) MarshalRData() ([]byte, error) {
	if ip4 := r.Addr.To4(); ip4 != nil {
		return []byte(ip4), nil
	}
	if ip6 := r.Addr.To16(); ip6 != nil {
		return []byte(ip6), nil
	}
	return nil, fmt.Errorf("%w: invalid IP address", ErrDNSError)
}

// ParseIPRData parses A or AAAA record RDATA from wire format.
func ParseIPRData(msg []byte, off *int, rdlen int) (*IPRecord, error) {
	if rdlen != 4 && rdlen != 16 {
		return nil, fmt.Errorf("%w: A/AAAA record must be 4/16 bytes (RFC 1035 §3.4.1), got %d", ErrDNSError, rdlen)
	}
	if *off+rdlen > len(msg) {
		return nil, fmt.Errorf("%w: unexpected EOF reading IP record (RFC 1035 §3.4.1)", ErrDNSError)
	}
	b := make([]byte, rdlen)
	copy(b, msg[*off:*off+rdlen])
	*off += rdlen
	return &IPRecord{Addr: net.IP(b)}, nil
}
