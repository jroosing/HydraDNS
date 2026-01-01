package server

import (
	"encoding/binary"

	"github.com/jroosing/hydradns/internal/dns"
)

// truncateUDPResponse truncates a DNS response to fit within the UDP size limit.
//
// When a DNS response exceeds maxSize, this function:
//  1. Sets the TC (Truncation) flag to signal the client should retry over TCP
//  2. Preserves only the header and question section
//  3. Removes all answer, authority, and additional records
//
// The client is expected to retry the query over TCP upon seeing the TC flag.
func truncateUDPResponse(respBytes []byte, maxSize int) []byte {
	if maxSize <= 0 {
		maxSize = dns.DefaultUDPPayloadSize
	}
	if len(respBytes) <= maxSize {
		return respBytes
	}
	if len(respBytes) < dns.HeaderSize {
		return respBytes
	}

	qdcount := extractQuestionCount(respBytes)
	header := buildTruncatedHeader(respBytes, qdcount)

	if qdcount == 0 {
		return header
	}

	questionEnd := findQuestionSectionEnd(respBytes, int(qdcount))
	if questionEnd <= dns.HeaderSize {
		return header
	}
	if questionEnd > maxSize {
		return header
	}

	// Combine header + question section only
	out := make([]byte, 0, questionEnd)
	out = append(out, header...)
	out = append(out, respBytes[dns.HeaderSize:questionEnd]...)
	return out
}

// extractQuestionCount reads the QDCOUNT from a DNS message header.
// QDCOUNT is at bytes 4-5 (big-endian).
func extractQuestionCount(msg []byte) uint16 {
	return binary.BigEndian.Uint16(msg[4:6])
}

// buildTruncatedHeader creates a new DNS header with the TC flag set.
//
// The new header:
//   - Preserves the original transaction ID
//   - Preserves most flags but sets TC (Truncation) flag
//   - Keeps the original question count
//   - Sets answer, authority, and additional counts to 0
func buildTruncatedHeader(respBytes []byte, qdcount uint16) []byte {
	// Read original flags and set TC flag (bit 9)
	flags := binary.BigEndian.Uint16(respBytes[2:4])
	newFlags := flags | dns.TCFlag

	h := make([]byte, dns.HeaderSize)
	copy(h[0:2], respBytes[0:2])                 // Transaction ID
	binary.BigEndian.PutUint16(h[2:4], newFlags) // Flags with TC set
	binary.BigEndian.PutUint16(h[4:6], qdcount)  // Question count (preserved)
	binary.BigEndian.PutUint16(h[6:8], 0)        // Answer count = 0
	binary.BigEndian.PutUint16(h[8:10], 0)       // Authority count = 0
	binary.BigEndian.PutUint16(h[10:12], 0)      // Additional count = 0
	return h
}

// findQuestionSectionEnd finds the byte offset where the question section ends.
//
// Each question in the DNS message consists of:
//   - QNAME: A sequence of labels (length-prefixed strings) ending with 0x00,
//     or a compression pointer (0xC0 + offset)
//   - QTYPE: 2 bytes
//   - QCLASS: 2 bytes
//
// This function parses the QNAME carefully to handle both regular labels
// and compression pointers.
func findQuestionSectionEnd(msg []byte, qdcount int) int {
	pos := dns.HeaderSize // Start after 12-byte header

	for range qdcount {
		pos = skipQNAME(msg, pos)
		if pos > len(msg) {
			return len(msg)
		}

		// Skip QTYPE (2 bytes) and QCLASS (2 bytes)
		if pos+4 > len(msg) {
			return len(msg)
		}
		pos += 4
	}
	return pos
}

// skipQNAME advances past a DNS name in wire format.
//
// DNS names are encoded as:
//   - Regular label: 1-byte length (0-63) followed by that many bytes
//   - Compression pointer: 2 bytes starting with 0xC0 (11xxxxxx pattern)
//   - End: Single 0x00 byte (zero-length label)
func skipQNAME(msg []byte, pos int) int {
	for pos < len(msg) {
		labelLen := msg[pos]

		// Zero-length label marks end of name
		if labelLen == 0 {
			return pos + 1
		}

		// Compression pointer (high 2 bits = 11)
		// Pointer is 2 bytes total, and the name ends after it
		if labelLen >= 0xC0 {
			if pos+2 > len(msg) {
				return len(msg)
			}
			return pos + 2
		}

		// Regular label: skip length byte + label bytes
		pos++
		if pos+int(labelLen) > len(msg) {
			return len(msg)
		}
		pos += int(labelLen)
	}
	return pos
}
