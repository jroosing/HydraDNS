package resolvers

import (
	"context"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/jroosing/hydradns/internal/filtering"
)

// FilteringResolver applies domain filtering before passing queries to the next resolver.
// Blocked domains receive an NXDOMAIN response immediately.
//
// Filtering Decision Flow:
//
// 1. Check whitelist (allowed domains) → pass through immediately
// 2. Check blacklist/blocklists (blocked domains) → return NXDOMAIN
// 3. Default allow → pass to next resolver
//
// Blocking Response:
//
// Blocked queries return an NXDOMAIN response (RCODE=3). The response:
//   - Echoes the original question
//   - Sets the QR (response), RA (recursive), and AA flags
//   - Contains no answer, authority, or additional sections
//
// This resolver MUST be placed first in the resolver chain to ensure
// all queries pass through the filter before any other resolution.
type FilteringResolver struct {
	policy *filtering.PolicyEngine
	next   Resolver
}

// NewFilteringResolver creates a filtering resolver with the given policy engine.
// The next resolver is called for domains that are not blocked.
func NewFilteringResolver(policy *filtering.PolicyEngine, next Resolver) *FilteringResolver {
	return &FilteringResolver{
		policy: policy,
		next:   next,
	}
}

// Resolve checks the domain against the filtering policy.
// Blocked domains return NXDOMAIN immediately; allowed domains pass through to the next resolver.
func (f *FilteringResolver) Resolve(ctx context.Context, req dns.Packet, reqBytes []byte) (Result, error) {
	// Extract the query name
	if len(req.Questions) == 0 {
		// No question, pass through
		return f.next.Resolve(ctx, req, reqBytes)
	}

	qname := req.Questions[0].Name

	// Evaluate against policy
	result := f.policy.Evaluate(qname)

	switch result.Action {
	case filtering.ActionBlock:
		// Return NXDOMAIN for blocked domains
		resp := buildBlockedResponse(req)
		respBytes, err := resp.Marshal()
		if err != nil {
			return Result{}, err
		}
		return Result{
			ResponseBytes: respBytes,
			Source:        "filtered-blocked",
		}, nil

	case filtering.ActionLog:
		// Log action allows the query but it was logged by the policy engine
		// Fall through to next resolver
		fallthrough

	case filtering.ActionAllow:
		// Pass through to next resolver
		return f.next.Resolve(ctx, req, reqBytes)

	default:
		// Unknown action, allow by default
		return f.next.Resolve(ctx, req, reqBytes)
	}
}

// Close releases resources.
func (f *FilteringResolver) Close() error {
	var err error
	if f.policy != nil {
		err = f.policy.Close()
	}
	if f.next != nil {
		if nextErr := f.next.Close(); nextErr != nil && err == nil {
			err = nextErr
		}
	}
	return err
}

// Policy returns the underlying policy engine for stats/management.
func (f *FilteringResolver) Policy() *filtering.PolicyEngine {
	return f.policy
}

// buildBlockedResponse creates an NXDOMAIN response for a blocked domain.
func buildBlockedResponse(req dns.Packet) dns.Packet {
	return dns.Packet{
		Header: dns.Header{
			ID:    req.Header.ID,
			Flags: buildBlockedFlags(req.Header.Flags),
		},
		Questions: req.Questions,
		Answers:   nil,
	}
}

// buildBlockedFlags creates response flags for NXDOMAIN.
func buildBlockedFlags(reqFlags uint16) uint16 {
	// Set QR (response), copy opcode, set RA (recursion available)
	// Set RCODE to NXDOMAIN (3)
	flags := uint16(1 << 15)   // QR = 1 (response)
	flags |= reqFlags & 0x7800 // Copy opcode (bits 11-14)
	if reqFlags&(1<<8) != 0 {  // RD bit was set
		flags |= 1 << 8 // RD = 1
		flags |= 1 << 7 // RA = 1 (recursion available)
	}
	flags |= uint16(dns.RCodeNXDomain) // RCODE = NXDOMAIN (3)
	return flags
}
