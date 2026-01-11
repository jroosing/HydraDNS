// Package dns provides DNS protocol parsing, encoding, and packet manipulation.
package dns

import (
	"fmt"
)

// DNS header flags and masks (RFC 1035 Section 4.1.1)
//
// The DNS header contains a 16-bit flags field with the following layout:
//
//	+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//	|QR|   Opcode  |AA|TC|RD|RA| Z|AD|CD|   RCODE   |
//	+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//	 15 14 13 12 11 10  9  8  7  6  5  4  3  2  1  0
//
// Bit positions (from MSB):
//   - Bit 15 (0x8000): QR - Query (0) or Response (1)
//   - Bits 14-11 (0x7800): OPCODE - Operation type (0=Query, 1=IQuery, 2=Status)
//   - Bit 10 (0x0400): AA - Authoritative Answer
//   - Bit 9 (0x0200): TC - Truncation (message was truncated)
//   - Bit 8 (0x0100): RD - Recursion Desired
//   - Bit 7 (0x0080): RA - Recursion Available
//   - Bit 6 (0x0040): Z - Reserved (must be zero)
//   - Bit 5 (0x0020): AD - Authenticated Data (DNSSEC)
//   - Bit 4 (0x0010): CD - Checking Disabled (DNSSEC)
//   - Bits 3-0 (0x000F): RCODE - Response code
const (
	QRFlag     uint16 = 0x8000 // Query/Response: 1 = response, 0 = query
	OpcodeMask uint16 = 0x7800 // Bits 14-11: operation type (use >> 11 to extract)
	AAFlag     uint16 = 0x0400 // Authoritative Answer
	TCFlag     uint16 = 0x0200 // Truncation: message was truncated
	RDFlag     uint16 = 0x0100 // Recursion Desired
	RAFlag     uint16 = 0x0080 // Recursion Available
	ZFlag      uint16 = 0x0040 // Reserved (must be zero in queries)
	ADFlag     uint16 = 0x0020 // Authenticated Data (DNSSEC)
	CDFlag     uint16 = 0x0010 // Checking Disabled (DNSSEC)
	RCodeMask  uint16 = 0x000F // Bits 3-0: response code
)

// RecordType represents DNS resource record types (RFC 1035, RFC 3596, RFC 6891).
type RecordType uint16

const (
	TypeA          RecordType = 1   // IPv4 address
	TypeNS         RecordType = 2   // Authoritative name server
	TypeCNAME      RecordType = 5   // Canonical name (alias)
	TypeSOA        RecordType = 6   // Start of Authority
	TypePTR        RecordType = 12  // Domain name pointer (reverse DNS)
	TypeMX         RecordType = 15  // Mail exchange
	TypeTXT        RecordType = 16  // Text strings
	TypeAAAA       RecordType = 28  // IPv6 address (RFC 3596)
	TypeSRV        RecordType = 33  // Service locator (RFC 2782)
	TypeOPT        RecordType = 41  // EDNS pseudo-record (RFC 6891)
	TypeDS         RecordType = 43  // Delegation Signer (DNSSEC, RFC 4034)
	TypeRRSIG      RecordType = 46  // DNSSEC signature (RFC 4034)
	TypeNSEC       RecordType = 47  // Next Secure record (DNSSEC, RFC 4034)
	TypeDNSKEY     RecordType = 48  // DNS Public Key (DNSSEC, RFC 4034)
	TypeNSEC3      RecordType = 50  // NSEC version 3 (DNSSEC, RFC 5155)
	TypeNSEC3PARAM RecordType = 51  // NSEC3 Parameters (DNSSEC, RFC 5155)
	TypeCAA        RecordType = 257 // Certification Authority Authorization (RFC 8659)
)

// RecordClass represents DNS resource record classes (RFC 1035).
type RecordClass uint16

const (
	ClassIN RecordClass = 1 // Internet class
)

// RCode represents DNS response codes (RFC 1035).
type RCode uint16

const (
	RCodeNoError  RCode = 0 // No error
	RCodeFormErr  RCode = 1 // Format error: query malformed
	RCodeServFail RCode = 2 // Server failure: internal error
	RCodeNXDomain RCode = 3 // Non-existent domain
	RCodeNotImp   RCode = 4 // Not implemented: unsupported query type
	RCodeRefused  RCode = 5 // Query refused by policy
)

// RCodeFromFlags extracts the response code from the DNS header flags.
// The RCODE occupies the low 4 bits of the flags field.
func RCodeFromFlags(flags uint16) RCode {
	return RCode(flags & RCodeMask)
}

// String returns the human-readable name of the record type.
func (rt RecordType) String() string {
	switch rt {
	case TypeA:
		return "A"
	case TypeNS:
		return "NS"
	case TypeCNAME:
		return "CNAME"
	case TypeSOA:
		return "SOA"
	case TypePTR:
		return "PTR"
	case TypeMX:
		return "MX"
	case TypeTXT:
		return "TXT"
	case TypeAAAA:
		return "AAAA"
	case TypeSRV:
		return "SRV"
	case TypeOPT:
		return "OPT"
	case TypeDS:
		return "DS"
	case TypeRRSIG:
		return "RRSIG"
	case TypeNSEC:
		return "NSEC"
	case TypeDNSKEY:
		return "DNSKEY"
	case TypeNSEC3:
		return "NSEC3"
	case TypeNSEC3PARAM:
		return "NSEC3PARAM"
	case TypeCAA:
		return "CAA"
	default:
		return fmt.Sprintf("TYPE%d", rt)
	}
}

// String returns the human-readable name of the record class.
func (rc RecordClass) String() string {
	switch rc {
	case ClassIN:
		return "IN"
	default:
		return fmt.Sprintf("CLASS%d", rc)
	}
}
