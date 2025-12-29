package dns

import (
	"encoding/binary"
	"fmt"
	"net"
)

type Record struct {
	Name  string
	Type  uint16
	Class uint16
	TTL   uint32
	// Data is type-specific:
	// - A/AAAA/OPT/SOA: []byte
	// - CNAME/NS/PTR: string
	// - MX: MXData
	// - TXT: either string, []string, or []byte (raw)
	Data any
}

type MXData struct {
	Preference uint16
	Exchange   string
}

func ParseRecord(msg []byte, off *int) (Record, error) {
	name, err := DecodeName(msg, off)
	if err != nil {
		return Record{}, err
	}
	if *off+10 > len(msg) {
		return Record{}, fmt.Errorf("%w: unexpected EOF while reading DNS record", ErrDNSError)
	}
	rrType := binary.BigEndian.Uint16(msg[*off : *off+2])
	rrClass := binary.BigEndian.Uint16(msg[*off+2 : *off+4])
	ttl := binary.BigEndian.Uint32(msg[*off+4 : *off+8])
	rdlen := binary.BigEndian.Uint16(msg[*off+8 : *off+10])
	*off += 10
	start := *off
	if start+int(rdlen) > len(msg) {
		return Record{}, fmt.Errorf("%w: unexpected EOF while reading DNS record rdata", ErrDNSError)
	}

	var data any
	switch RecordType(rrType) {
	case TypeCNAME, TypeNS, TypePTR:
		n, err := DecodeName(msg, off)
		if err != nil {
			return Record{}, err
		}
		if *off-start != int(rdlen) {
			return Record{}, fmt.Errorf("%w: invalid DNS record rdata length for name-based type", ErrDNSError)
		}
		data = n
	case TypeMX:
		if *off+2 > len(msg) {
			return Record{}, fmt.Errorf("%w: unexpected EOF while reading MX preference", ErrDNSError)
		}
		pref := binary.BigEndian.Uint16(msg[*off : *off+2])
		*off += 2
		ex, err := DecodeName(msg, off)
		if err != nil {
			return Record{}, err
		}
		if *off-start != int(rdlen) {
			return Record{}, fmt.Errorf("%w: invalid DNS record rdata length for MX", ErrDNSError)
		}
		data = MXData{Preference: pref, Exchange: ex}
	default:
		b := make([]byte, rdlen)
		copy(b, msg[*off:*off+int(rdlen)])
		*off += int(rdlen)
		data = b
	}

	return Record{Name: name, Type: rrType, Class: rrClass, TTL: ttl, Data: data}, nil
}

func (rr Record) Marshal() ([]byte, error) {
	nameWire := []byte{0}
	if rr.Type != uint16(TypeOPT) {
		b, err := EncodeName(rr.Name)
		if err != nil {
			return nil, err
		}
		nameWire = b
	}

	rdata, err := rr.marshalRData()
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(nameWire)+10+len(rdata))
	out = append(out, nameWire...)
	fixed := make([]byte, 10)
	binary.BigEndian.PutUint16(fixed[0:2], rr.Type)
	binary.BigEndian.PutUint16(fixed[2:4], rr.Class)
	binary.BigEndian.PutUint32(fixed[4:8], rr.TTL)
	binary.BigEndian.PutUint16(fixed[8:10], uint16(len(rdata)))
	out = append(out, fixed...)
	out = append(out, rdata...)
	return out, nil
}

func (rr Record) marshalRData() ([]byte, error) {
	switch RecordType(rr.Type) {
	case TypeA:
		b, ok := rr.Data.([]byte)
		if !ok || len(b) != 4 {
			return nil, fmt.Errorf("%w: A record data must be 4 bytes", ErrDNSError)
		}
		return b, nil
	case TypeAAAA:
		b, ok := rr.Data.([]byte)
		if !ok || len(b) != 16 {
			return nil, fmt.Errorf("%w: AAAA record data must be 16 bytes", ErrDNSError)
		}
		return b, nil
	case TypeMX:
		mx, ok := rr.Data.(MXData)
		if !ok {
			return nil, fmt.Errorf("%w: MX record data must be MXData", ErrDNSError)
		}
		ex, err := EncodeName(mx.Exchange)
		if err != nil {
			return nil, err
		}
		out := make([]byte, 2+len(ex))
		binary.BigEndian.PutUint16(out[0:2], mx.Preference)
		copy(out[2:], ex)
		return out, nil
	case TypeCNAME, TypeNS, TypePTR:
		s, ok := rr.Data.(string)
		if !ok || s == "" {
			return nil, fmt.Errorf("%w: name-based record data must be a non-empty string", ErrDNSError)
		}
		return EncodeName(s)
	case TypeTXT:
		return marshalTXT(rr.Data)
	case TypeOPT:
		if rr.Data == nil {
			return nil, nil
		}
		b, ok := rr.Data.([]byte)
		if ok {
			return b, nil
		}
		return nil, fmt.Errorf("%w: OPT record data must be raw bytes", ErrDNSError)
	default:
		if b, ok := rr.Data.([]byte); ok {
			return b, nil
		}
		return nil, fmt.Errorf("%w: unsupported RR type for serialization: %d", ErrDNSError, rr.Type)
	}
}

func marshalTXT(v any) ([]byte, error) {
	switch t := v.(type) {
	case string:
		return marshalTXTString(t), nil
	case []string:
		// Pre-calculate total size to avoid reallocations
		totalLen := 0
		for _, s := range t {
			totalLen += 1 + len(s) // length byte + string data
		}
		out := make([]byte, 0, totalLen)
		for _, s := range t {
			b := []byte(s)
			if len(b) > 255 {
				return nil, fmt.Errorf("%w: TXT character-string cannot exceed 255 bytes", ErrDNSError)
			}
			out = append(out, byte(len(b)))
			out = append(out, b...)
		}
		return out, nil
	case []byte:
		return t, nil
	default:
		return nil, fmt.Errorf("%w: TXT record data must be string, []string, or []byte", ErrDNSError)
	}
}

func marshalTXTString(s string) []byte {
	b := []byte(s)
	if len(b) <= 255 {
		out := make([]byte, 1+len(b))
		out[0] = byte(len(b))
		copy(out[1:], b)
		return out
	}
	// Long string: split into 255-byte chunks
	// Calculate total size: len(b) data bytes + (len(b)/255 + 1) length bytes
	numChunks := (len(b) + 254) / 255
	out := make([]byte, 0, len(b)+numChunks)
	for i := 0; i < len(b); i += 255 {
		chunk := b[i:]
		if len(chunk) > 255 {
			chunk = chunk[:255]
		}
		out = append(out, byte(len(chunk)))
		out = append(out, chunk...)
	}
	return out
}

func (rr Record) IPv4() (string, bool) {
	if RecordType(rr.Type) != TypeA {
		return "", false
	}
	b, ok := rr.Data.([]byte)
	if !ok || len(b) != 4 {
		return "", false
	}
	return net.IPv4(b[0], b[1], b[2], b[3]).String(), true
}

func (rr Record) IPv6() (string, bool) {
	if RecordType(rr.Type) != TypeAAAA {
		return "", false
	}
	b, ok := rr.Data.([]byte)
	if !ok || len(b) != 16 {
		return "", false
	}
	return net.IP(b).String(), true
}
