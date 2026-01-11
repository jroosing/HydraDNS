package dns

import (
	"encoding/binary"
	"fmt"
)

// Header represents a DNS message header (RFC 1035 Section 4.1.1).
//
// The header is always 12 bytes and contains:
//   - ID: 16-bit identifier for matching requests to responses
//   - Flags: 16-bit field containing QR, Opcode, AA, TC, RD, RA, Z, RCODE
//   - QDCount: Number of questions
//   - ANCount: Number of answer resource records
//   - NSCount: Number of authority resource records
//   - ARCount: Number of additional resource records
type Header struct {
	ID      uint16 // Transaction ID
	Flags   uint16 // See enums.go for flag definitions
	QDCount uint16 // Question count
	ANCount uint16 // Answer count
	NSCount uint16 // Authority (nameserver) count
	ARCount uint16 // Additional records count
}

// HeaderSize is the fixed size of a DNS header in bytes.
const HeaderSize = 12

// Marshal serializes the header to wire format (big-endian, 12 bytes).
func (h Header) Marshal() ([]byte, error) {
	b := make([]byte, HeaderSize)
	binary.BigEndian.PutUint16(b[0:2], h.ID)
	binary.BigEndian.PutUint16(b[2:4], h.Flags)
	binary.BigEndian.PutUint16(b[4:6], h.QDCount)
	binary.BigEndian.PutUint16(b[6:8], h.ANCount)
	binary.BigEndian.PutUint16(b[8:10], h.NSCount)
	binary.BigEndian.PutUint16(b[10:12], h.ARCount)
	return b, nil
}

// ParseHeader parses a DNS header from the message at the given offset.
// It advances *off by 12 bytes (the header size) on success.
func ParseHeader(msg []byte, off *int) (Header, error) {
	if *off+HeaderSize > len(msg) {
		return Header{}, fmt.Errorf("%w: unexpected EOF while reading DNS header", ErrDNSError)
	}
	h := Header{
		ID:      binary.BigEndian.Uint16(msg[*off : *off+2]),
		Flags:   binary.BigEndian.Uint16(msg[*off+2 : *off+4]),
		QDCount: binary.BigEndian.Uint16(msg[*off+4 : *off+6]),
		ANCount: binary.BigEndian.Uint16(msg[*off+6 : *off+8]),
		NSCount: binary.BigEndian.Uint16(msg[*off+8 : *off+10]),
		ARCount: binary.BigEndian.Uint16(msg[*off+10 : *off+12]),
	}
	*off += HeaderSize
	return h, nil
}

// --- DNSSEC-aware flag accessors (RFC 4035) ---

// AuthenticData returns true if the AD (Authenticated Data) flag is set.
// AD=1 indicates the resolver has cryptographically verified all RRSIGs in the answer.
func (h Header) AuthenticData() bool {
	return h.Flags&ADFlag != 0
}

// CheckingDisabled returns true if the CD (Checking Disabled) flag is set.
// CD=1 requests the server to not perform DNSSEC validation.
func (h Header) CheckingDisabled() bool {
	return h.Flags&CDFlag != 0
}

// SetAD sets or clears the AD (Authenticated Data) flag.
// Only authoritative or validating resolvers should set AD=1.
func (h *Header) SetAD(enabled bool) {
	if enabled {
		h.Flags |= ADFlag
	} else {
		h.Flags &^= ADFlag
	}
}

// SetCD sets or clears the CD (Checking Disabled) flag.
// Clients set CD=1 to disable server-side DNSSEC validation.
func (h *Header) SetCD(enabled bool) {
	if enabled {
		h.Flags |= CDFlag
	} else {
		h.Flags &^= CDFlag
	}
}

// RecursionDesired returns true if the RD (Recursion Desired) flag is set.
func (h Header) RecursionDesired() bool {
	return h.Flags&RDFlag != 0
}

// RecursionAvailable returns true if the RA (Recursion Available) flag is set.
func (h Header) RecursionAvailable() bool {
	return h.Flags&RAFlag != 0
}

// Authoritative returns true if the AA (Authoritative Answer) flag is set.
func (h Header) Authoritative() bool {
	return h.Flags&AAFlag != 0
}

// Truncated returns true if the TC (Truncated) flag is set.
func (h Header) Truncated() bool {
	return h.Flags&TCFlag != 0
}

// IsQuery returns true if this is a query (QR=0), false if it's a response (QR=1).
func (h Header) IsQuery() bool {
	return h.Flags&QRFlag == 0
}

// IsResponse returns true if this is a response (QR=1), false if it's a query (QR=0).
func (h Header) IsResponse() bool {
	return h.Flags&QRFlag != 0
}
