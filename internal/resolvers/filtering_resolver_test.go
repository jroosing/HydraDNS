package resolvers

import (
	"context"
	"testing"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/jroosing/hydradns/internal/filtering"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// filteringMockResolver is a test resolver for filtering tests.
type filteringMockResolver struct {
	result Result
	err    error
	called bool
}

func (m *filteringMockResolver) Resolve(ctx context.Context, req dns.Packet, reqBytes []byte) (Result, error) {
	m.called = true
	if m.err != nil {
		return Result{}, m.err
	}
	return m.result, nil
}

func (m *filteringMockResolver) Close() error {
	return nil
}

func TestFilteringResolver_BlockedDomain(t *testing.T) {
	policy := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{
		Enabled:          true,
		BlockAction:      filtering.ActionBlock,
		BlacklistDomains: []string{"blocked.example.com"},
	})
	defer policy.Close()

	mock := &filteringMockResolver{
		result: Result{ResponseBytes: []byte("success"), Source: "mock"},
	}

	fr := NewFilteringResolver(policy, mock)
	defer fr.Close()

	// Create a request for blocked domain
	req := dns.Packet{
		Header: dns.Header{ID: 1234},
		Questions: []dns.Question{
			{Name: "blocked.example.com", Type: 1, Class: 1},
		},
	}

	result, err := fr.Resolve(context.Background(), req, nil)
	require.NoError(t, err)

	// Should be blocked, not passed to mock
	assert.False(t, mock.called, "Mock resolver should not have been called for blocked domain")
	assert.Equal(t, "filtered-blocked", result.Source)

	// Response should be valid NXDOMAIN
	assert.NotEmpty(t, result.ResponseBytes, "Expected non-empty response bytes")
}

func TestFilteringResolver_AllowedDomain(t *testing.T) {
	policy := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{
		Enabled:          true,
		BlockAction:      filtering.ActionBlock,
		BlacklistDomains: []string{"blocked.example.com"},
	})
	defer policy.Close()

	mock := &filteringMockResolver{
		result: Result{ResponseBytes: []byte("success"), Source: "mock"},
	}

	fr := NewFilteringResolver(policy, mock)
	defer fr.Close()

	// Create a request for allowed domain
	req := dns.Packet{
		Header: dns.Header{ID: 1234},
		Questions: []dns.Question{
			{Name: "allowed.example.com", Type: 1, Class: 1},
		},
	}

	result, err := fr.Resolve(context.Background(), req, nil)
	require.NoError(t, err)

	// Should be passed to mock
	assert.True(t, mock.called, "Mock resolver should have been called for allowed domain")
	assert.Equal(t, "mock", result.Source)
}

func TestFilteringResolver_WhitelistPriority(t *testing.T) {
	policy := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{
		Enabled:          true,
		BlockAction:      filtering.ActionBlock,
		WhitelistDomains: []string{"safe.example.com"},
		BlacklistDomains: []string{"safe.example.com"}, // Also on blacklist
	})
	defer policy.Close()

	mock := &filteringMockResolver{
		result: Result{ResponseBytes: []byte("success"), Source: "mock"},
	}

	fr := NewFilteringResolver(policy, mock)
	defer fr.Close()

	// Create a request for domain on both lists
	req := dns.Packet{
		Header: dns.Header{ID: 1234},
		Questions: []dns.Question{
			{Name: "safe.example.com", Type: 1, Class: 1},
		},
	}

	result, err := fr.Resolve(context.Background(), req, nil)
	require.NoError(t, err)

	// Whitelist should take priority
	assert.True(t, mock.called, "Mock resolver should have been called (whitelist takes priority)")
	assert.Equal(t, "mock", result.Source)
}

func TestFilteringResolver_DisabledFiltering(t *testing.T) {
	policy := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{
		Enabled:          false, // Disabled
		BlockAction:      filtering.ActionBlock,
		BlacklistDomains: []string{"blocked.example.com"},
	})
	defer policy.Close()

	mock := &filteringMockResolver{
		result: Result{ResponseBytes: []byte("success"), Source: "mock"},
	}

	fr := NewFilteringResolver(policy, mock)
	defer fr.Close()

	// Create a request for "blocked" domain (but filtering is disabled)
	req := dns.Packet{
		Header: dns.Header{ID: 1234},
		Questions: []dns.Question{
			{Name: "blocked.example.com", Type: 1, Class: 1},
		},
	}

	result, err := fr.Resolve(context.Background(), req, nil)
	require.NoError(t, err)

	// Should be passed to mock (filtering disabled)
	assert.True(t, mock.called, "Mock resolver should have been called (filtering disabled)")
	assert.Equal(t, "mock", result.Source)
}

func TestFilteringResolver_NoQuestions(t *testing.T) {
	policy := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{
		Enabled:          true,
		BlockAction:      filtering.ActionBlock,
		BlacklistDomains: []string{"blocked.example.com"},
	})
	defer policy.Close()

	mock := &filteringMockResolver{
		result: Result{ResponseBytes: []byte("success"), Source: "mock"},
	}

	fr := NewFilteringResolver(policy, mock)
	defer fr.Close()

	// Create a request with no questions
	req := dns.Packet{
		Header:    dns.Header{ID: 1234},
		Questions: nil,
	}

	result, err := fr.Resolve(context.Background(), req, nil)
	require.NoError(t, err)

	// Should be passed to mock (no questions to filter)
	assert.True(t, mock.called, "Mock resolver should have been called (no questions)")
	assert.Equal(t, "mock", result.Source)
}

func TestFilteringResolver_SubdomainBlocking(t *testing.T) {
	policy := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{
		Enabled:          true,
		BlockAction:      filtering.ActionBlock,
		BlacklistDomains: []string{"ads.example.com"}, // Wildcards subdomains
	})
	defer policy.Close()

	mock := &filteringMockResolver{
		result: Result{ResponseBytes: []byte("success"), Source: "mock"},
	}

	fr := NewFilteringResolver(policy, mock)
	defer fr.Close()

	// Create a request for subdomain of blocked domain
	req := dns.Packet{
		Header: dns.Header{ID: 1234},
		Questions: []dns.Question{
			{Name: "tracker.ads.example.com", Type: 1, Class: 1},
		},
	}

	result, err := fr.Resolve(context.Background(), req, nil)
	require.NoError(t, err)

	// Should be blocked (subdomain of blocked domain)
	assert.False(t, mock.called, "Mock resolver should not have been called for subdomain of blocked domain")
	assert.Equal(t, "filtered-blocked", result.Source)
}
