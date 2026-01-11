package dns

import "fmt"

// OpaqueRecord represents a DNS record with an unknown or unsupported type.
type OpaqueRecord struct {
	H    RRHeader
	T    RecordType
	Data any // typically []byte
}

// NewOpaqueRecord creates a new opaque record for unknown/unsupported types.
func NewOpaqueRecord(h RRHeader, rt RecordType, data []byte) *OpaqueRecord {
	return &OpaqueRecord{H: h, T: rt, Data: data}
}

// Type returns the record type.
func (r *OpaqueRecord) Type() RecordType { return r.T }

// Header returns the record header.
func (r *OpaqueRecord) Header() RRHeader { return r.H }

// SetHeader sets the record header.
func (r *OpaqueRecord) SetHeader(h RRHeader) { r.H = h }

// MarshalRData marshals the opaque data to wire format.
func (r *OpaqueRecord) MarshalRData() ([]byte, error) {
	if r.Data == nil {
		return nil, nil
	}
	b, ok := r.Data.([]byte)
	if !ok {
		return nil, fmt.Errorf("%w: opaque record data must be raw bytes", ErrDNSError)
	}
	return b, nil
}

// ParseOpaqueRData parses raw opaque RDATA (for TXT, OPT, and unknown types).
func ParseOpaqueRData(msg []byte, off *int, rdlen int, rt RecordType) (*OpaqueRecord, error) {
	b := make([]byte, rdlen)
	copy(b, msg[*off:*off+rdlen])
	*off += rdlen
	return &OpaqueRecord{T: rt, Data: b}, nil
}
