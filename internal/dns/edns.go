package dns

import (
	"encoding/binary"
)

// EDNS (Extension Mechanisms for DNS) constants per RFC 6891.
const (
	DefaultUDPPayloadSize     = 512  // Traditional DNS UDP limit (RFC 1035)
	EDNSDefaultUDPPayloadSize = 1232 // Safe EDNS size avoiding fragmentation
	EDNSMaxUDPPayloadSize     = 4096 // Maximum practical EDNS UDP size
	EDNSMinUDPPayloadSize     = 512  // Minimum EDNS UDP payload size
)

// EDNSOption represents an EDNS option in the OPT record's RDATA.
type EDNSOption struct {
	Code uint16 // Option code
	Data []byte // Option data
}

// Marshal serializes an EDNS option to wire format.
func (o EDNSOption) Marshal() []byte {
	b := make([]byte, 4+len(o.Data))
	binary.BigEndian.PutUint16(b[0:2], o.Code)
	binary.BigEndian.PutUint16(b[2:4], uint16(len(o.Data)))
	copy(b[4:], o.Data)
	return b
}

// OPTRecord represents an EDNS OPT pseudo-record (RFC 6891).
//
// The OPT record uses a non-standard encoding:
//   - NAME: Must be root (0x00)
//   - TYPE: 41 (OPT)
//   - CLASS: Sender's UDP payload size (not a class!)
//   - TTL: Extended RCODE, version, and flags (packed into 32 bits)
//   - RDATA: Zero or more EDNS options
//
// TTL field layout (32 bits):
//
//	+---+---+---+---+---+---+---+---+---+---+---+---+---+---+---+---+
//	|         EXTENDED-RCODE        |            VERSION            |
//	+---+---+---+---+---+---+---+---+---+---+---+---+---+---+---+---+
//	| DO|                    Z (reserved)                           |
//	+---+---+---+---+---+---+---+---+---+---+---+---+---+---+---+---+
//
// Bits 31-24: Extended RCODE (upper 8 bits)
// Bits 23-16: EDNS version
// Bit 15: DO (DNSSEC OK) flag
// Bits 14-0: Reserved (must be zero)
type OPTRecord struct {
	UDPPayloadSize uint16       // Sender's maximum UDP payload size
	ExtendedRCode  uint8        // Upper 8 bits of RCODE
	Version        uint8        // EDNS version (must be 0)
	DNSSECOk       bool         // DO flag: client supports DNSSEC
	Options        []EDNSOption // EDNS options
}

// CreateOPT creates an OPT record advertising the given UDP payload size.
func CreateOPT(udpPayloadSize int) OPTRecord {
	sz := clampInt(udpPayloadSize, EDNSMinUDPPayloadSize, 65535)
	return OPTRecord{UDPPayloadSize: uint16(sz)}
}

// clampInt restricts a value to the range [min, max].
func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// Marshal serializes the OPT record to DNS wire format.
func (o OPTRecord) Marshal() []byte {
	// Build TTL field: pack extended RCODE, version, and flags
	ttl := packOPTTTL(o.ExtendedRCode, o.Version, o.DNSSECOk)

	// Serialize RDATA (EDNS options)
	rdata := make([]byte, 0)
	for _, opt := range o.Options {
		rdata = append(rdata, opt.Marshal()...)
	}

	// Build complete record: root name + type + class + TTL + rdlength + rdata
	b := make([]byte, 0, 11+len(rdata))
	b = append(b, 0) // Root name (single zero byte)

	fixed := make([]byte, 10)
	binary.BigEndian.PutUint16(fixed[0:2], uint16(TypeOPT))
	binary.BigEndian.PutUint16(fixed[2:4], o.UDPPayloadSize) // CLASS field = UDP size
	binary.BigEndian.PutUint32(fixed[4:8], ttl)
	binary.BigEndian.PutUint16(fixed[8:10], uint16(len(rdata)))
	b = append(b, fixed...)
	b = append(b, rdata...)
	return b
}

// packOPTTTL constructs the 32-bit TTL field for an OPT record.
//
// Layout:
//   - Bits 31-24: Extended RCODE
//   - Bits 23-16: Version
//   - Bit 15: DO (DNSSEC OK) flag
//   - Bits 14-0: Reserved (zero)
func packOPTTTL(extRCode, version uint8, dnssecOk bool) uint32 {
	ttl := uint32(extRCode)<<24 | uint32(version)<<16
	if dnssecOk {
		ttl |= 1 << 15 // Set DO flag (bit 15)
	}
	return ttl
}

// ExtractOPT finds and parses an OPT record from the additionals section.
// Returns nil if no OPT record is present.
func ExtractOPT(additionals []Record) *OPTRecord {
	for _, rr := range additionals {
		if RecordType(rr.Type) != TypeOPT {
			continue
		}
		raw, ok := rr.Data.([]byte)
		if !ok {
			return nil
		}
		o := unpackOPT(rr)
		_ = raw // Options parsing omitted (not used by server)
		return &o
	}
	return nil
}

// unpackOPT extracts OPT record fields from a parsed Record.
// The CLASS field contains UDP payload size, and TTL contains packed flags.
func unpackOPT(rr Record) OPTRecord {
	return OPTRecord{
		UDPPayloadSize: rr.Class,
		ExtendedRCode:  uint8(rr.TTL >> 24),          // Bits 31-24
		Version:        uint8(rr.TTL >> 16),          // Bits 23-16
		DNSSECOk:       ((rr.TTL >> 15) & 0x1) == 1,  // Bit 15
		Options:        nil,
	}
}

// ClientMaxUDPSize determines the maximum UDP response size for a client.
// It checks for an EDNS OPT record and returns the advertised size,
// or DefaultUDPPayloadSize (512) if no EDNS is present.
func ClientMaxUDPSize(req Packet) int {
	opt := ExtractOPT(req.Additionals)
	if opt != nil {
		if opt.UDPPayloadSize < DefaultUDPPayloadSize {
			return DefaultUDPPayloadSize
		}
		return int(opt.UDPPayloadSize)
	}
	return DefaultUDPPayloadSize
}

// IsTruncated checks if a DNS response has the TC (Truncation) flag set.
// This indicates the message was truncated and should be retried over TCP.
func IsTruncated(responseBytes []byte) bool {
	if len(responseBytes) < 4 {
		return false
	}
	flags := binary.BigEndian.Uint16(responseBytes[2:4])
	return (flags & TCFlag) != 0
}

// AddEDNSToRequestBytes adds an OPT record to a DNS request if one isn't present.
// This advertises the client's EDNS capabilities and desired UDP buffer size.
func AddEDNSToRequestBytes(req Packet, reqBytes []byte, udpSize int) []byte {
	// Don't add if EDNS already present
	if ExtractOPT(req.Additionals) != nil {
		return reqBytes
	}

	opt := CreateOPT(udpSize)
	optBytes := opt.Marshal()

	if len(reqBytes) < HeaderSize {
		return reqBytes
	}

	// Increment ARCOUNT (additional record count) in header
	ar := binary.BigEndian.Uint16(reqBytes[10:12])
	if ar < 65535 {
		ar++
	}

	// Build new message with OPT record appended
	out := make([]byte, 0, len(reqBytes)+len(optBytes))
	out = append(out, reqBytes...)
	binary.BigEndian.PutUint16(out[10:12], ar)
	out = append(out, optBytes...)
	return out
}
