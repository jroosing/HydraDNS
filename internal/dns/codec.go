package dns

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// NormalizeName returns a lowercase DNS name without trailing dots.
// This is useful for case-insensitive DNS name comparisons per RFC 4343.
// NormalizeName converts a domain name to lowercase for case-insensitive comparison.
// DNS domain names are case-insensitive per RFC 1035 Section 3.1.
func NormalizeName(name string) string {
	return strings.ToLower(strings.TrimSuffix(name, "."))
}

// EncodeName encodes a domain name to DNS wire format (RFC 1035 Section 3.1).
//
// DNS names are encoded as a sequence of labels, where each label is:
//   - 1 byte: length (0-63)
//   - N bytes: label characters
//
// The name is terminated by a zero-length label (single 0x00 byte).
//
// Example: "www.example.com" encodes as:
//
//	[3]www[7]example[3]com[0]
//	0x03 'w' 'w' 'w' 0x07 'e' 'x' 'a' 'm' 'p' 'l' 'e' 0x03 'c' 'o' 'm' 0x00
//
// Constraints:
//   - Each label max 63 bytes
//   - Total encoded name max 255 bytes
//   - ASCII only (no IDN/punycode handled here)
//
// EncodeName encodes a domain name to DNS wire format with compression support (RFC 1035 Section 4.1.4).
//
// DNS names are encoded as a sequence of labels, each preceded by a length byte.
// The sequence terminates with a zero-length label (root).
//
// Example: "example.com" → [7]"example"[3]"com"[0]
//
// This implementation does NOT perform message compression (pointers) since that
// requires knowledge of previously-encoded names in the message. Callers needing
// compression should use the Packet.Marshal() method instead.
func EncodeName(domain string) ([]byte, error) {
	if domain == "" {
		return nil, fmt.Errorf("%w: domain_name must be non-empty", ErrDNSError)
	}
	domain = trimDot(domain)
	if domain == "" {
		return []byte{0}, nil // Root domain
	}

	out := make([]byte, 0, len(domain)+2)
	labelStart := 0
	for i := 0; i <= len(domain); i++ {
		if i == len(domain) || domain[i] == '.' {
			if i == labelStart {
				return nil, fmt.Errorf("%w: invalid domain name (empty label): %q", ErrDNSError, domain)
			}
			label := domain[labelStart:i]

			// Validate ASCII
			for j := range len(label) {
				if label[j] > 0x7F {
					return nil, fmt.Errorf("%w: domain_name must be ASCII", ErrDNSError)
				}
			}

			// Check label length (max 63 per RFC 1035)
			if len(label) > 63 {
				return nil, fmt.Errorf("%w: DNS label too long (%d > 63): %q", ErrDNSError, len(label), label)
			}

			out = append(out, byte(len(label)))
			out = append(out, label...)
			labelStart = i + 1
		}
	}
	out = append(out, 0) // Terminating zero-length label

	if len(out) > 255 {
		return nil, fmt.Errorf("%w: encoded domain name too long (%d > 255)", ErrDNSError, len(out))
	}
	return out, nil
}

// DecodeName decodes a possibly-compressed DNS name from wire format.
//
// DNS name compression (RFC 1035 Section 4.1.4) uses pointers to reduce
// message size. A compression pointer is identified by the two high bits
// of a label length byte being set (11xxxxxx pattern = 0xC0).
//
// The pointer value is a 14-bit offset from the start of the message:
//
//	+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//	| 1  1|                OFFSET                   |
//	+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//
// This function reads from msg starting at *off, advancing *off past the
// encoded name (including any compression pointer bytes).
//
// Returns an ASCII, dot-separated name without a trailing dot.
// DecodeName decodes a domain name from DNS wire format with compression support (RFC 1035 Section 4.1.4).
//
// Compression pointers (high 2 bits = 11) indicate an offset to a previously-encoded name.
// This is used to reduce message size when the same names appear multiple times.
//
// Returns the decoded domain name or an error if the message is truncated or malformed.
func DecodeName(msg []byte, off *int) (string, error) {
	name, err := decodeName(msg, off, 0, map[int]struct{}{})
	if err != nil {
		return "", err
	}
	return name, nil
}

// decodeName is the recursive implementation of DecodeName.
// It tracks recursion depth and visited offsets to detect compression loops.
func decodeName(msg []byte, off *int, depth int, visited map[int]struct{}) (string, error) {
	const maxCompressionDepth = 20

	if depth > maxCompressionDepth {
		return "", fmt.Errorf("%w: too many DNS compression pointer indirections", ErrDNSError)
	}
	if *off < 0 || *off >= len(msg) {
		return "", fmt.Errorf("%w: unexpected EOF while decoding DNS name", ErrDNSError)
	}

	// Pre-allocate for typical domain depth (e.g., www.example.com = 3 labels)
	labels := make([]string, 0, 6)
	for {
		if *off >= len(msg) {
			return "", fmt.Errorf("%w: unexpected EOF while decoding DNS name", ErrDNSError)
		}
		labelLen := msg[*off]
		*off++

		// Zero-length label marks end of name
		if labelLen == 0 {
			break
		}

		// Check for compression pointer (high 2 bits = 11)
		if isCompressionPointer(labelLen) {
			rest, err := followCompressionPointer(msg, off, labelLen, depth, visited)
			if err != nil {
				return "", err
			}
			if rest != "" {
				labels = append(labels, rest)
			}
			break
		}

		// Check for reserved label type (high 2 bits = 01 or 10)
		if hasReservedBits(labelLen) {
			return "", fmt.Errorf("%w: invalid DNS label length (reserved high bits set)", ErrDNSError)
		}

		// Regular label
		label, err := readLabel(msg, off, int(labelLen))
		if err != nil {
			return "", err
		}
		labels = append(labels, label)
	}

	return joinLabels(labels), nil
}

// isCompressionPointer checks if the label length byte indicates a compression pointer.
// Compression pointers have the two high bits set (11xxxxxx = 0xC0 mask).
func isCompressionPointer(b byte) bool {
	return (b & 0xC0) == 0xC0
}

// hasReservedBits checks if the label uses reserved encoding (01xxxxxx or 10xxxxxx).
// These patterns are reserved for future use per RFC 1035.
func hasReservedBits(b byte) bool {
	return (b & 0xC0) != 0
}

// followCompressionPointer follows a DNS compression pointer and returns the name at that offset.
// The pointer is a 14-bit value: the first byte's low 6 bits + the next byte.
func followCompressionPointer(
	msg []byte,
	off *int,
	firstByte byte,
	depth int,
	visited map[int]struct{},
) (string, error) {
	if *off >= len(msg) {
		return "", fmt.Errorf("%w: unexpected EOF while decoding compression pointer", ErrDNSError)
	}

	// Extract 14-bit pointer: mask off high 2 bits of first byte, combine with second byte
	ptr := int(binary.BigEndian.Uint16([]byte{firstByte & 0x3F, msg[*off]}))
	*off++

	if ptr >= len(msg) {
		return "", fmt.Errorf("%w: DNS compression pointer out of bounds", ErrDNSError)
	}
	if _, ok := visited[ptr]; ok {
		return "", fmt.Errorf("%w: DNS compression pointer loop detected", ErrDNSError)
	}
	visited[ptr] = struct{}{}

	ptrOff := ptr
	return decodeName(msg, &ptrOff, depth+1, visited)
}

// readLabel reads a single DNS label of the given length.
func readLabel(msg []byte, off *int, length int) (string, error) {
	if *off+length > len(msg) {
		return "", fmt.Errorf("%w: unexpected EOF while reading DNS label", ErrDNSError)
	}
	label := msg[*off : *off+length]
	*off += length

	// Validate ASCII
	for _, b := range label {
		if b > 0x7F {
			return "", fmt.Errorf("%w: decoded DNS name was not ASCII", ErrDNSError)
		}
	}
	return string(label), nil
}

// trimDot removes all trailing dots from a string.
func trimDot(s string) string {
	for len(s) > 0 && s[len(s)-1] == '.' {
		s = s[:len(s)-1]
	}
	return s
}

// joinLabels concatenates DNS labels with dots.
// Uses strings.Builder with size pre-allocation for efficiency.
func joinLabels(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	if len(labels) == 1 {
		return labels[0]
	}
	// Pre-calculate size to minimize Builder allocations
	totalSize := len(labels) - 1 // dots
	for _, label := range labels {
		totalSize += len(label)
	}
	var b strings.Builder
	b.Grow(totalSize)
	b.WriteString(labels[0])
	for i := 1; i < len(labels); i++ {
		b.WriteByte('.')
		b.WriteString(labels[i])
	}
	return b.String()
}
