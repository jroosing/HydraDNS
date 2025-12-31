package resolvers

import (
	"context"
	"testing"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/jroosing/hydradns/internal/filtering"
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
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should be blocked, not passed to mock
	if mock.called {
		t.Error("Mock resolver should not have been called for blocked domain")
	}

	if result.Source != "filtered-blocked" {
		t.Errorf("Expected source 'filtered-blocked', got %q", result.Source)
	}

	// Response should be valid NXDOMAIN
	if len(result.ResponseBytes) == 0 {
		t.Error("Expected non-empty response bytes")
	}
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
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should be passed to mock
	if !mock.called {
		t.Error("Mock resolver should have been called for allowed domain")
	}

	if result.Source != "mock" {
		t.Errorf("Expected source 'mock', got %q", result.Source)
	}
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
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Whitelist should take priority
	if !mock.called {
		t.Error("Mock resolver should have been called (whitelist takes priority)")
	}

	if result.Source != "mock" {
		t.Errorf("Expected source 'mock', got %q", result.Source)
	}
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
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should be passed to mock (filtering disabled)
	if !mock.called {
		t.Error("Mock resolver should have been called (filtering disabled)")
	}

	if result.Source != "mock" {
		t.Errorf("Expected source 'mock', got %q", result.Source)
	}
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
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should be passed to mock (no questions to filter)
	if !mock.called {
		t.Error("Mock resolver should have been called (no questions)")
	}

	if result.Source != "mock" {
		t.Errorf("Expected source 'mock', got %q", result.Source)
	}
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
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should be blocked (subdomain of blocked domain)
	if mock.called {
		t.Error("Mock resolver should not have been called for subdomain of blocked domain")
	}

	if result.Source != "filtered-blocked" {
		t.Errorf("Expected source 'filtered-blocked', got %q", result.Source)
	}
}
