// Package dns provides DNS protocol parsing, encoding, and packet manipulation.
//
// Standards Compliance:
//
// This package implements DNS protocol features from the following RFCs:
//
//   - RFC 1035: Domain Names - Implementation and Specification (core DNS protocol)
//   - RFC 1034: Domain Names - Concepts and Facilities (DNS concepts)
//   - RFC 2308: Negative Caching of DNS Queries (NXDOMAIN, NODATA caching)
//   - RFC 3596: DNS Extensions to Support IPv6 (AAAA records)
//   - RFC 4034: DNSSEC Resource Records (DNSSEC records: RRSIG, DNSKEY, etc.)
//   - RFC 4035: DNSSEC Protocol Extensions (AD, CD flags)
//   - RFC 6891: Extension Mechanisms for DNS (EDNS, OPT records)
//
// Type-Oriented Design:
//
// Each DNS record type is represented by an explicit type (IPRecord, NameRecord, etc.)
// rather than a generic struct. This ensures type safety and makes DNS semantics clear.
//
// Error Handling:
//
// All errors are wrapped with context using fmt.Errorf("...: %w", err).
// This preserves error chains while adding operational context.
package dns

import "errors"

var (
	// ErrDNSError is a sentinel error type for DNS protocol violations.
	// Wrap this with fmt.Errorf("context: %w", ErrDNSError) to add context.
	ErrDNSError = errors.New("dns wire error")
)
