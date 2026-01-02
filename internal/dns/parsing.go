package dns

import (
	"errors"
	"fmt"

	"github.com/jroosing/hydradns/internal/helpers"
)

// Limits for incoming DNS messages to prevent resource exhaustion attacks.
const (
	MaxIncomingDNSMessageSize = 4096 // Maximum size of incoming DNS message
	MaxQuestions              = 4    // Maximum questions per query (RFC allows 1 typically)
	MaxRRPerSection           = 100  // Maximum resource records per section
	MaxTotalRR                = 200  // Maximum total resource records
)

// ParseRequestBounded parses a DNS request with security bounds checking.
// It validates that the message is a standard query (not a response),
// uses opcode 0 (QUERY), and doesn't exceed resource limits.
//
// Returns an error if:
//   - Message exceeds MaxIncomingDNSMessageSize
//   - QR flag is set (packet is a response, not a query)
//   - Opcode is not 0 (only standard queries are supported)
//   - Question or RR counts exceed limits
func ParseRequestBounded(msg []byte) (Packet, error) {
	if len(msg) > MaxIncomingDNSMessageSize {
		return Packet{}, errors.New("dns message too large")
	}
	p, err := ParsePacket(msg)
	if err != nil {
		return Packet{}, err
	}

	// Validate QR flag: must be 0 for queries
	// QR is bit 15 of flags (0x8000)
	if isResponse(p.Header.Flags) {
		return Packet{}, errors.New("invalid packet: QR flag set (response packet received)")
	}

	// Extract and validate opcode (bits 14-11)
	if opcode := extractOpcode(p.Header.Flags); opcode != 0 {
		return Packet{}, fmt.Errorf("unsupported OpCode: %d", opcode)
	}

	// Validate section counts
	if err := validateSectionCounts(p.Header); err != nil {
		return Packet{}, err
	}

	return p, nil
}

// isResponse checks if the QR flag is set (indicating a response packet).
func isResponse(flags uint16) bool {
	return (flags & QRFlag) != 0
}

// extractOpcode extracts the 4-bit opcode from the flags field.
// Opcode occupies bits 14-11, so we mask with 0x7800 and shift right by 11.
func extractOpcode(flags uint16) uint16 {
	return (flags & OpcodeMask) >> 11
}

// validateSectionCounts checks that section counts don't exceed limits.
func validateSectionCounts(h Header) error {
	qd := int(h.QDCount)
	an := int(h.ANCount)
	ns := int(h.NSCount)
	ar := int(h.ARCount)

	if qd > MaxQuestions {
		return errors.New("too many questions")
	}
	if qd != 1 {
		return errors.New("unsupported question count")
	}
	if an > MaxRRPerSection || ns > MaxRRPerSection || ar > MaxRRPerSection {
		return errors.New("too many resource records")
	}
	if (an + ns + ar) > MaxTotalRR {
		return errors.New("too many total resource records")
	}
	return nil
}

// BuildErrorResponse constructs a DNS error response packet.
// It preserves the transaction ID and RD flag from the request,
// sets the QR flag (response), and applies the given response code.
//
// The response includes the original question section but no answer records.
func BuildErrorResponse(req Packet, rcode uint16) Packet {
	flags := buildResponseFlags(req.Header.Flags, rcode)

	h := Header{
		ID:      req.Header.ID,
		Flags:   flags,
		QDCount: helpers.ClampIntToUint16(len(req.Questions)),
		ANCount: 0,
		NSCount: 0,
		ARCount: 0,
	}
	return Packet{Header: h, Questions: req.Questions}
}

// buildResponseFlags constructs the flags field for an error response.
//
// Flag construction:
//  1. Set QR flag (bit 15) to mark as response
//  2. Preserve RD flag (bit 8) from request if set
//  3. Clear existing RCODE and set new rcode in bits 3-0
func buildResponseFlags(reqFlags uint16, rcode uint16) uint16 {
	// Start with QR flag set (this is a response)
	flags := QRFlag

	// Preserve RD (Recursion Desired) from the request
	flags |= (reqFlags & RDFlag)

	// Clear RCODE bits and set new response code (low 4 bits)
	rcode &= RCodeMask
	flags = (flags &^ RCodeMask) | rcode

	return flags
}
