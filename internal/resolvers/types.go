// Package resolvers provides DNS resolution strategies for HydraDNS.
//
// Architecture:
//
// The resolver chain allows multiple resolution strategies to work together:
//
//   1. FilteringResolver - Filters queries by domain (whitelist/blacklist)
//   2. CustomDNSResolver - Answers local A/AAAA/CNAME records
//   3. ForwardingResolver - Queries upstream servers with caching
//   4. ChainedResolver - Tries resolvers in order, falling back as needed
//
// Caching Strategy:
//
// The ForwardingResolver includes a TTL-aware LRU cache that:
//   - Respects original record TTLs (capped at MaxCacheTTL)
//   - Caches negative responses (NXDOMAIN, NODATA) per RFC 2308
//   - Caches SERVFAIL responses temporarily to protect upstream
//   - Uses transaction ID stripping (txid=0) in cached responses
//
// Singleflight Deduplication:
//
// Multiple concurrent queries for the same domain share a single upstream request,
// preventing thundering herd amplification during cache misses.
//
// Type-Oriented Design:
//
// All record types (A, AAAA, CNAME, NS, etc.) are represented by explicit types
// rather than generic structs. This ensures type safety and makes DNS semantics clear.
package resolvers

import (
	"context"

	"github.com/jroosing/hydradns/internal/dns"
)

// Result holds the outcome of a DNS resolution.
type Result struct {
	ResponseBytes []byte // Wire-format DNS response
	Source        string // Where the answer came from (e.g., "custom-dns", "upstream-cache", "upstream")
}

// QuestionKey uniquely identifies a DNS question for caching purposes.
// DNS names are case-insensitive, so QName should be normalized to lowercase.
//
// This is the COMPLETE cache key. Transaction IDs are NOT part of the cache key
// and are not considered during cache lookups. Multiple clients querying the same
// QNAME+QTYPE+QCLASS will share the same cached response (with their txid patched).
type QuestionKey struct {
	QName  string // Lowercase domain name
	QType  uint16 // Query type (A, AAAA, MX, etc.)
	QClass uint16 // Query class (usually IN=1)
}

// Resolver is the interface for DNS resolution strategies.
// Implementations include CustomDNSResolver (simple local DNS), ForwardingResolver (upstream),
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
//   - Normalize upstream responses before caching (set to txid=0)
//   - Restore the original client txid when returning responses
//
// Cached responses contain txid=0 in the wire format, but this value is NEVER used
// for matching or lookups. The cache key (QNAME+QTYPE+QCLASS) determines cache hits.
// The stored TXID is purely a placeholder that gets overwritten before each response.
//
// Optimization: Early returns avoid unnecessary allocations when the txid already matches.
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
