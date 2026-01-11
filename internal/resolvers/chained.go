package resolvers

import (
	"context"
	"errors"

	"github.com/jroosing/hydradns/internal/dns"
)

// Chained combines multiple resolvers, trying each in order until one succeeds.
//
// Resolver Chain Semantics:
//
// This enables a resolution chain like:
//  1. FilteringResolver - block queries based on policy
//  2. CustomDNSResolver - check local custom DNS entries first
//  3. ForwardingResolver - forward to upstream if not found locally
//
// The first resolver to return a successful result (without error) wins.
// If a resolver blocks the query, that result is returned immediately.
// If all resolvers fail, the last error is returned.
//
// Context Handling:
//
// The chained resolver checks for context cancellation between resolver attempts.
// This allows graceful shutdown during concurrent query processing.
type Chained struct {
	Resolvers []Resolver
}

// Resolve tries each resolver in order until one succeeds.
// Respects context cancellation between resolver attempts.
func (c *Chained) Resolve(ctx context.Context, req dns.Packet, reqBytes []byte) (Result, error) {
	var lastErr error

	for _, r := range c.Resolvers {
		// Check for cancellation before each resolver
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}

		res, err := r.Resolve(ctx, req, reqBytes)
		if err == nil {
			return res, nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = errors.New("no resolver could answer")
	}
	return Result{}, lastErr
}

// Close releases resources from all child resolvers.
// Returns the last error encountered (all resolvers are closed regardless of errors).
func (c *Chained) Close() error {
	var lastErr error
	for _, r := range c.Resolvers {
		if err := r.Close(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}
