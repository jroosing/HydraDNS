// Package resolvers provides DNS resolution strategies including
// forwarding, caching, zone-based resolution, and resolver chaining.
package resolvers

import (
	"context"

	"github.com/jroosing/hydradns/internal/dns"
)

// Result holds the outcome of a DNS resolution.
type Result struct {
	ResponseBytes []byte // Wire-format DNS response
	Source        string // Where the answer came from (e.g., "zone", "upstream-cache", "upstream")
}

// QuestionKey uniquely identifies a DNS question for caching purposes.
// DNS names are case-insensitive, so QName should be normalized to lowercase.
type QuestionKey struct {
	QName  string // Lowercase domain name
	QType  uint16 // Query type (A, AAAA, MX, etc.)
	QClass uint16 // Query class (usually IN=1)
}

// Resolver is the interface for DNS resolution strategies.
// Implementations include ZoneResolver (local zones), ForwardingResolver (upstream),
// and Chained (combining multiple resolvers).
type Resolver interface {
	// Resolve processes a DNS query and returns a response.
	// The context can be used for cancellation and timeouts.
	Resolve(ctx context.Context, req dns.Packet, reqBytes []byte) (Result, error)

	// Close releases any resources held by the resolver (e.g., connection pools).
	Close() error
}

// PatchTransactionID replaces the transaction ID in a DNS message.
//
// The transaction ID occupies the first 2 bytes of every DNS message (big-endian).
// This function is used to:
//   - Normalize cached responses (store with txid=0)
//   - Restore the original txid when returning cached responses
//
// It returns a new slice; the input is not modified.
func PatchTransactionID(msg []byte, txid uint16) []byte {
	if len(msg) < 2 {
		return msg
	}
	// Check if already has the desired txid (avoid allocation)
	if msg[0] == byte(txid>>8) && msg[1] == byte(txid) {
		return msg
	}
	out := make([]byte, len(msg))
	copy(out, msg)
	out[0] = byte(txid >> 8) // High byte
	out[1] = byte(txid)      // Low byte
	return out
}
