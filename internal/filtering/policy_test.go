package filtering

import (
	"fmt"
	"testing"
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
			if result.Action != tt.expectedAction {
				t.Errorf("Evaluate(%q).Action = %v, want %v",
					tt.domain, result.Action, tt.expectedAction)
			}
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
	if result.Action != ActionAllow {
		t.Errorf("Whitelist should take priority, got action=%v", result.Action)
	}
	if result.ListName != "whitelist" {
		t.Errorf("Expected ListName='whitelist', got %q", result.ListName)
	}
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
	if result.Action != ActionAllow {
		t.Errorf("Disabled engine should allow everything, got action=%v", result.Action)
	}
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

	if stats.QueriesTotal != 4 {
		t.Errorf("QueriesTotal = %d, want 4", stats.QueriesTotal)
	}
	if stats.QueriesBlocked != 2 {
		t.Errorf("QueriesBlocked = %d, want 2", stats.QueriesBlocked)
	}
	if stats.QueriesAllowed != 2 {
		t.Errorf("QueriesAllowed = %d, want 2", stats.QueriesAllowed)
	}
	if stats.WhitelistSize != 1 {
		t.Errorf("WhitelistSize = %d, want 1", stats.WhitelistSize)
	}
	if stats.BlacklistSize != 1 {
		t.Errorf("BlacklistSize = %d, want 1", stats.BlacklistSize)
	}
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
	if result.Action != ActionAllow {
		t.Errorf("Initial query should be allowed, got %v", result.Action)
	}

	// Add to blacklist
	pe.AddToBlacklist("ads.example.com")

	result = pe.Evaluate("ads.example.com")
	if result.Action != ActionBlock {
		t.Errorf("After blacklist add should be blocked, got %v", result.Action)
	}

	// Add to whitelist (should override blacklist)
	pe.AddToWhitelist("ads.example.com")

	result = pe.Evaluate("ads.example.com")
	if result.Action != ActionAllow {
		t.Errorf("Whitelist should override blacklist, got %v", result.Action)
	}
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
	if result.Action != ActionBlock {
		t.Errorf("Expected block, got %v", result.Action)
	}

	// Disable filtering
	pe.SetEnabled(false)

	result = pe.Evaluate("ads.example.com")
	if result.Action != ActionAllow {
		t.Errorf("Expected allow when disabled, got %v", result.Action)
	}

	// Re-enable
	pe.SetEnabled(true)

	result = pe.Evaluate("ads.example.com")
	if result.Action != ActionBlock {
		t.Errorf("Expected block after re-enable, got %v", result.Action)
	}
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
		if got := tt.action.String(); got != tt.expected {
			t.Errorf("Action(%d).String() = %q, want %q", tt.action, got, tt.expected)
		}
	}
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
	if s == "" {
		t.Error("String() returned empty string")
	}
	// Just check it contains expected parts
	if !containsAll(s, "PolicyEngine", "enabled=true", "whitelist=1", "blacklist=2") {
		t.Errorf("String() = %q, missing expected parts", s)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func BenchmarkPolicyEngine_Evaluate(b *testing.B) {
	cfg := PolicyEngineConfig{
		Enabled:          true,
		BlockAction:      ActionBlock,
		BlacklistDomains: make([]string, 10000),
	}

	// Generate 10k blocked domains
	for i := 0; i < 10000; i++ {
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

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pe.Evaluate(domains[i%len(domains)])
	}
}

func BenchmarkPolicyEngine_Evaluate_Parallel(b *testing.B) {
	cfg := PolicyEngineConfig{
		Enabled:          true,
		BlockAction:      ActionBlock,
		BlacklistDomains: make([]string, 10000),
	}

	for i := 0; i < 10000; i++ {
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
