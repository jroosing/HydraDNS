package dns

import "fmt"

// NameRecord represents DNS records that contain a single domain name (CNAME, NS, PTR).
type NameRecord struct {
	H      RRHeader
	T      RecordType
	Target string
}

// NewNameRecord creates a new name-based record (CNAME, NS, or PTR).
func NewNameRecord(h RRHeader, rt RecordType, target string) *NameRecord {
	return &NameRecord{H: h, T: rt, Target: target}
}

// NewCNAMERecord creates a new CNAME record.
func NewCNAMERecord(h RRHeader, target string) *NameRecord {
	return NewNameRecord(h, TypeCNAME, target)
}

// NewNSRecord creates a new NS record.
func NewNSRecord(h RRHeader, target string) *NameRecord {
	return NewNameRecord(h, TypeNS, target)
}

// NewPTRRecord creates a new PTR record.
func NewPTRRecord(h RRHeader, target string) *NameRecord {
	return NewNameRecord(h, TypePTR, target)
}

// Type returns the record type (CNAME, NS, or PTR).
func (r *NameRecord) Type() RecordType { return r.T }

// Header returns the record header.
func (r *NameRecord) Header() RRHeader { return r.H }

// SetHeader sets the record header.
func (r *NameRecord) SetHeader(h RRHeader) { r.H = h }

// MarshalRData marshals the target name to wire format.
func (r *NameRecord) MarshalRData() ([]byte, error) {
	return EncodeName(r.Target)
}

// ParseNameRData parses CNAME, NS, or PTR record RDATA from wire format.
func ParseNameRData(msg []byte, off *int, start, rdlen int, rt RecordType) (*NameRecord, error) {
	n, err := DecodeName(msg, off)
	if err != nil {
		return nil, err
	}
	if *off-start != rdlen {
		return nil, fmt.Errorf("%w: name record RDATA length mismatch (RFC 1035 §3.3)", ErrDNSError)
	}
	return &NameRecord{Target: n, T: rt}, nil
}
