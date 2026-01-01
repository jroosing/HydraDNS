package filtering

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyEngine_Evaluate(t *testing.T) {
	cfg := PolicyEngineConfig{
		Enabled:          true,
		BlockAction:      ActionBlock,
		LogBlocked:       false,
		LogAllowed:       false,
		WhitelistDomains: []string{"safe.example.com"},
		BlacklistDomains: []string{"ads.example.com", "tracker.example.org"},
	}

	pe := NewPolicyEngine(cfg)
	defer pe.Close()

	tests := []struct {
		name           string
		domain         string
		expectedAction Action
	}{
		{
			name:           "blocked domain",
			domain:         "ads.example.com",
			expectedAction: ActionBlock,
		},
		{
			name:           "another blocked domain",
			domain:         "tracker.example.org",
			expectedAction: ActionBlock,
		},
		{
			name:           "whitelisted domain",
			domain:         "safe.example.com",
			expectedAction: ActionAllow,
		},
		{
			name:           "unblocked domain",
			domain:         "google.com",
			expectedAction: ActionAllow,
		},
		{
			name:           "subdomain of blocked (wildcard)",
			domain:         "sub.ads.example.com",
			expectedAction: ActionBlock,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pe.Evaluate(tt.domain)
			assert.Equal(t, tt.expectedAction, result.Action, "Evaluate(%q).Action", tt.domain)
		})
	}
}

func TestPolicyEngine_WhitelistPriority(t *testing.T) {
	// Domain is on both whitelist and blacklist
	// Whitelist should take priority
	cfg := PolicyEngineConfig{
		Enabled:          true,
		BlockAction:      ActionBlock,
		WhitelistDomains: []string{"example.com"},
		BlacklistDomains: []string{"example.com"},
	}

	pe := NewPolicyEngine(cfg)
	defer pe.Close()

	result := pe.Evaluate("example.com")
	assert.Equal(t, ActionAllow, result.Action, "Whitelist should take priority")
	assert.Equal(t, "whitelist", result.ListName)
}

func TestPolicyEngine_Disabled(t *testing.T) {
	cfg := PolicyEngineConfig{
		Enabled:          false,
		BlockAction:      ActionBlock,
		BlacklistDomains: []string{"ads.example.com"},
	}

	pe := NewPolicyEngine(cfg)
	defer pe.Close()

	result := pe.Evaluate("ads.example.com")
	assert.Equal(t, ActionAllow, result.Action, "Disabled engine should allow everything")
}

func TestPolicyEngine_Stats(t *testing.T) {
	cfg := PolicyEngineConfig{
		Enabled:          true,
		BlockAction:      ActionBlock,
		WhitelistDomains: []string{"safe.com"},
		BlacklistDomains: []string{"ads.com"},
	}

	pe := NewPolicyEngine(cfg)
	defer pe.Close()

	// Make some queries
	pe.Evaluate("ads.com")       // blocked
	pe.Evaluate("ads.com")       // blocked
	pe.Evaluate("safe.com")      // allowed (whitelist)
	pe.Evaluate("unblocked.com") // allowed (not in lists)

	stats := pe.Stats()

	assert.Equal(t, uint64(4), stats.QueriesTotal)
	assert.Equal(t, uint64(2), stats.QueriesBlocked)
	assert.Equal(t, uint64(2), stats.QueriesAllowed)
	assert.Equal(t, 1, stats.WhitelistSize)
	assert.Equal(t, 1, stats.BlacklistSize)
}

func TestPolicyEngine_AddToLists(t *testing.T) {
	cfg := PolicyEngineConfig{
		Enabled:     true,
		BlockAction: ActionBlock,
	}

	pe := NewPolicyEngine(cfg)
	defer pe.Close()

	// Initially should be allowed
	result := pe.Evaluate("ads.example.com")
	assert.Equal(t, ActionAllow, result.Action, "Initial query should be allowed")

	// Add to blacklist
	pe.AddToBlacklist("ads.example.com")

	result = pe.Evaluate("ads.example.com")
	assert.Equal(t, ActionBlock, result.Action, "After blacklist add should be blocked")

	// Add to whitelist (should override blacklist)
	pe.AddToWhitelist("ads.example.com")

	result = pe.Evaluate("ads.example.com")
	assert.Equal(t, ActionAllow, result.Action, "Whitelist should override blacklist")
}

func TestPolicyEngine_SetEnabled(t *testing.T) {
	cfg := PolicyEngineConfig{
		Enabled:          true,
		BlockAction:      ActionBlock,
		BlacklistDomains: []string{"ads.example.com"},
	}

	pe := NewPolicyEngine(cfg)
	defer pe.Close()

	// Should be blocked while enabled
	result := pe.Evaluate("ads.example.com")
	assert.Equal(t, ActionBlock, result.Action)

	// Disable filtering
	pe.SetEnabled(false)

	result = pe.Evaluate("ads.example.com")
	assert.Equal(t, ActionAllow, result.Action, "Expected allow when disabled")

	// Re-enable
	pe.SetEnabled(true)

	result = pe.Evaluate("ads.example.com")
	assert.Equal(t, ActionBlock, result.Action, "Expected block after re-enable")
}

func TestAction_String(t *testing.T) {
	tests := []struct {
		action   Action
		expected string
	}{
		{ActionAllow, "allow"},
		{ActionBlock, "block"},
		{ActionLog, "log"},
		{Action(99), "unknown"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.action.String())
	}
}

func TestPolicyEngine_EvaluateWithContext(t *testing.T) {
	cfg := PolicyEngineConfig{
		Enabled:          true,
		BlockAction:      ActionBlock,
		BlacklistDomains: []string{"blocked.com"},
	}

	pe := NewPolicyEngine(cfg)
	defer pe.Close()

	t.Run("allowed domain", func(t *testing.T) {
		result, err := pe.EvaluateWithContext(context.Background(), "allowed.com")
		require.NoError(t, err)
		assert.Equal(t, ActionAllow, result.Action)
	})

	t.Run("blocked domain", func(t *testing.T) {
		result, err := pe.EvaluateWithContext(context.Background(), "blocked.com")
		require.NoError(t, err)
		assert.Equal(t, ActionBlock, result.Action)
	})

	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately

		_, err := pe.EvaluateWithContext(ctx, "anysite.com")
		assert.Equal(t, context.Canceled, err)
	})
}

func TestPolicyEngine_ListInfo(t *testing.T) {
	cfg := PolicyEngineConfig{
		Enabled:          true,
		WhitelistDomains: []string{"safe.com"},
		BlacklistDomains: []string{"blocked.com"},
	}

	pe := NewPolicyEngine(cfg)
	defer pe.Close()

	// ListInfo returns info about blocklists (URLs)
	// With only static domains (no URLs), this should return empty
	info := pe.ListInfo()
	assert.NotNil(t, info, "ListInfo should not return nil")
	// For a policy engine with only static domains, there's no URL-based list info
}

func TestPolicyEngine_String(t *testing.T) {
	cfg := PolicyEngineConfig{
		Enabled:          true,
		WhitelistDomains: []string{"a.com"},
		BlacklistDomains: []string{"b.com", "c.com"},
	}

	pe := NewPolicyEngine(cfg)
	defer pe.Close()

	s := pe.String()
	assert.NotEmpty(t, s, "String() returned empty string")
	// Just check it contains expected parts
	assert.Contains(t, s, "PolicyEngine")
	assert.Contains(t, s, "enabled=true")
	assert.Contains(t, s, "whitelist=1")
	assert.Contains(t, s, "blacklist=2")
}

func BenchmarkPolicyEngine_Evaluate(b *testing.B) {
	cfg := PolicyEngineConfig{
		Enabled:          true,
		BlockAction:      ActionBlock,
		BlacklistDomains: make([]string, 10000),
	}

	// Generate 10k blocked domains
	for i := range 10000 {
		cfg.BlacklistDomains[i] = fmt.Sprintf("blocked%d.example.com", i)
	}

	pe := NewPolicyEngine(cfg)
	defer pe.Close()

	domains := []string{
		"blocked5000.example.com", // hit in middle
		"safe.example.com",        // miss
		"blocked1.example.com",    // hit at start
		"blocked9999.example.com", // hit at end
	}

	for i := 0; b.Loop(); i++ {
		pe.Evaluate(domains[i%len(domains)])
	}
}

func BenchmarkPolicyEngine_Evaluate_Parallel(b *testing.B) {
	cfg := PolicyEngineConfig{
		Enabled:          true,
		BlockAction:      ActionBlock,
		BlacklistDomains: make([]string, 10000),
	}

	for i := range 10000 {
		cfg.BlacklistDomains[i] = fmt.Sprintf("blocked%d.example.com", i)
	}

	pe := NewPolicyEngine(cfg)
	defer pe.Close()

	domains := []string{
		"blocked5000.example.com",
		"safe.example.com",
		"blocked1.example.com",
		"blocked9999.example.com",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			pe.Evaluate(domains[i%len(domains)])
			i++
		}
	})
}
