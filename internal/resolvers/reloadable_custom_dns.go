package resolvers

import (
	"context"
	"errors"
	"sync"

	"github.com/jroosing/hydradns/internal/dns"
)

// ErrNoCustomDNS is returned when no custom DNS resolver is configured.
var ErrNoCustomDNS = errors.New("no custom DNS resolver configured")

// ReloadableCustomDNSResolver wraps a CustomDNSResolver and allows atomic replacement.
// This enables runtime updates to custom DNS configuration without server restart.
//
// Thread-safety: All methods are safe for concurrent use.
type ReloadableCustomDNSResolver struct {
	mu       sync.RWMutex
	resolver *CustomDNSResolver
}

// NewReloadableCustomDNSResolver creates a new reloadable wrapper around a CustomDNSResolver.
// If resolver is nil, the wrapper starts empty (no custom DNS configured).
func NewReloadableCustomDNSResolver(resolver *CustomDNSResolver) *ReloadableCustomDNSResolver {
	return &ReloadableCustomDNSResolver{
		resolver: resolver,
	}
}

// Resolve delegates to the current CustomDNSResolver instance.
func (r *ReloadableCustomDNSResolver) Resolve(ctx context.Context, req dns.Packet, reqBytes []byte) (Result, error) {
	r.mu.RLock()
	resolver := r.resolver
	r.mu.RUnlock()

	if resolver == nil || resolver.IsEmpty() {
		// No custom DNS configured - let the next resolver in chain handle it
		return Result{}, ErrNoCustomDNS
	}

	return resolver.Resolve(ctx, req, reqBytes)
}

// Reload atomically replaces the current CustomDNSResolver with a new one built from config.
// The old resolver is closed after replacement. Pass nil to disable custom DNS.
func (r *ReloadableCustomDNSResolver) Reload(newResolver *CustomDNSResolver) error {
	r.mu.Lock()
	old := r.resolver
	r.resolver = newResolver
	r.mu.Unlock()

	// Close old resolver (if any)
	if old != nil {
		return old.Close()
	}
	return nil
}

// Close closes the current CustomDNSResolver.
func (r *ReloadableCustomDNSResolver) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.resolver != nil {
		return r.resolver.Close()
	}
	return nil
}

// IsEmpty returns true if no custom DNS resolver is configured or it has no entries.
func (r *ReloadableCustomDNSResolver) IsEmpty() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.resolver == nil {
		return true
	}
	return r.resolver.IsEmpty()
}

// ContainsDomain checks if a domain is configured in custom DNS.
func (r *ReloadableCustomDNSResolver) ContainsDomain(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.resolver == nil {
		return false
	}
	return r.resolver.ContainsDomain(name)
}
