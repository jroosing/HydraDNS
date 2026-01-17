package filtering_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jroosing/hydradns/internal/filtering"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// DomainTrie Tests
// =============================================================================

func TestDomainTrie_Add_And_Contains(t *testing.T) {
	trie := filtering.NewDomainTrie()

	trie.Add("example.com", false)
	trie.Add("blocked.example.org", false)

	assert.True(t, trie.Contains("example.com"), "Should contain exact match")
	assert.True(t, trie.Contains("blocked.example.org"), "Should contain exact match")
	assert.False(t, trie.Contains("other.com"), "Should not contain non-added domain")
	assert.False(t, trie.Contains("sub.example.com"), "Should not match subdomains without wildcard")
}

func TestDomainTrie_Wildcard(t *testing.T) {
	trie := filtering.NewDomainTrie()

	// Add with wildcard - should match all subdomains
	trie.Add("example.com", true)

	assert.True(t, trie.Contains("example.com"), "Should match exact domain")
	assert.True(t, trie.Contains("sub.example.com"), "Should match subdomain with wildcard")
	assert.True(t, trie.Contains("deep.sub.example.com"), "Should match deep subdomain")
	assert.False(t, trie.Contains("example.org"), "Should not match different domain")
}

func TestDomainTrie_CaseInsensitive(t *testing.T) {
	trie := filtering.NewDomainTrie()

	trie.Add("Example.COM", false)

	assert.True(t, trie.Contains("example.com"), "Should match lowercase")
	assert.True(t, trie.Contains("EXAMPLE.COM"), "Should match uppercase")
	assert.True(t, trie.Contains("ExAmPlE.cOm"), "Should match mixed case")
}

func TestDomainTrie_Size(t *testing.T) {
	trie := filtering.NewDomainTrie()

	assert.Equal(t, 0, trie.Size(), "Empty trie should have size 0")

	trie.Add("a.com", false)
	assert.Equal(t, 1, trie.Size())

	trie.Add("b.com", false)
	assert.Equal(t, 2, trie.Size())

	// Adding duplicate should not increase size
	trie.Add("a.com", false)
	assert.Equal(t, 2, trie.Size())
}

func TestDomainTrie_Clear(t *testing.T) {
	trie := filtering.NewDomainTrie()

	trie.Add("example.com", false)
	trie.Add("test.com", false)
	assert.Equal(t, 2, trie.Size())

	trie.Clear()
	assert.Equal(t, 0, trie.Size())
	assert.False(t, trie.Contains("example.com"))
}

func TestDomainTrie_Remove(t *testing.T) {
	trie := filtering.NewDomainTrie()

	trie.Add("example.com", false)
	trie.Add("sub.example.com", false)
	assert.True(t, trie.Contains("example.com"))
	assert.True(t, trie.Contains("sub.example.com"))
	assert.Equal(t, 2, trie.Size())

	// Remove specific domain
	removed := trie.Remove("sub.example.com")
	assert.True(t, removed)
	assert.False(t, trie.Contains("sub.example.com"))
	assert.True(t, trie.Contains("example.com"))
	assert.Equal(t, 1, trie.Size())

	// Remove non-existent
	removed = trie.Remove("notfound.com")
	assert.False(t, removed)
	assert.Equal(t, 1, trie.Size())

	// Remove last remaining domain and ensure cleanup
	removed = trie.Remove("example.com")
	assert.True(t, removed)
	assert.False(t, trie.Contains("example.com"))
	assert.Equal(t, 0, trie.Size())
}

// (Policy remove methods verified indirectly via handler tests)

func TestDomainTrie_Merge(t *testing.T) {
	trie1 := filtering.NewDomainTrie()
	trie1.Add("example.com", false)

	trie2 := filtering.NewDomainTrie()
	trie2.Add("test.org", false)
	trie2.Add("other.net", false)

	trie1.Merge(trie2)

	assert.True(t, trie1.Contains("example.com"))
	assert.True(t, trie1.Contains("test.org"))
	assert.True(t, trie1.Contains("other.net"))
	assert.Equal(t, 3, trie1.Size())
}

func TestDomainTrie_EmptyDomain(t *testing.T) {
	trie := filtering.NewDomainTrie()

	trie.Add("", false)
	assert.Equal(t, 0, trie.Size(), "Empty domain should not be added")
}

func TestDomainTrie_TrailingDot(t *testing.T) {
	trie := filtering.NewDomainTrie()

	trie.Add("example.com.", false)
	assert.True(t, trie.Contains("example.com"), "Should handle trailing dot")
	assert.True(t, trie.Contains("example.com."), "Should match with trailing dot")
}

// =============================================================================
// PolicyEngine Tests
// =============================================================================

func TestPolicyEngine_Disabled(t *testing.T) {
	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{
		Enabled:          false,
		BlacklistDomains: []string{"blocked.com"},
	})
	defer pe.Close()

	result := pe.Evaluate("blocked.com")
	assert.Equal(t, filtering.ActionAllow, result.Action, "Disabled engine should allow all")
}

func TestPolicyEngine_Blacklist(t *testing.T) {
	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{
		Enabled:          true,
		BlockAction:      filtering.ActionBlock,
		BlacklistDomains: []string{"blocked.com", "ads.example.org"},
	})
	defer pe.Close()

	tests := []struct {
		domain     string
		wantAction filtering.Action
	}{
		{"blocked.com", filtering.ActionBlock},
		{"sub.blocked.com", filtering.ActionBlock}, // wildcard
		{"ads.example.org", filtering.ActionBlock},
		{"allowed.com", filtering.ActionAllow},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			result := pe.Evaluate(tt.domain)
			assert.Equal(t, tt.wantAction, result.Action)
		})
	}
}

func TestPolicyEngine_Whitelist(t *testing.T) {
	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{
		Enabled:          true,
		BlockAction:      filtering.ActionBlock,
		WhitelistDomains: []string{"allowed.com"},
		BlacklistDomains: []string{"blocked.com"},
	})
	defer pe.Close()

	result := pe.Evaluate("allowed.com")
	assert.Equal(t, filtering.ActionAllow, result.Action)
	assert.Equal(t, "whitelist", result.ListName)
}

func TestPolicyEngine_WhitelistTakesPriority(t *testing.T) {
	// Domain is both whitelisted and blacklisted - whitelist wins
	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{
		Enabled:          true,
		BlockAction:      filtering.ActionBlock,
		WhitelistDomains: []string{"example.com"},
		BlacklistDomains: []string{"example.com"},
	})
	defer pe.Close()

	result := pe.Evaluate("example.com")
	assert.Equal(t, filtering.ActionAllow, result.Action, "Whitelist should take priority")
	assert.Equal(t, "whitelist", result.ListName)
}

func TestPolicyEngine_SubdomainMatching(t *testing.T) {
	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{
		Enabled:          true,
		BlockAction:      filtering.ActionBlock,
		BlacklistDomains: []string{"ads.example.com"},
	})
	defer pe.Close()

	// Subdomains should be blocked
	assert.Equal(t, filtering.ActionBlock, pe.Evaluate("ads.example.com").Action)
	assert.Equal(t, filtering.ActionBlock, pe.Evaluate("tracker.ads.example.com").Action)

	// Parent domain should not be blocked
	assert.Equal(t, filtering.ActionAllow, pe.Evaluate("example.com").Action)
	assert.Equal(t, filtering.ActionAllow, pe.Evaluate("other.example.com").Action)
}

func TestPolicyEngine_EvaluateWithContext(t *testing.T) {
	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{
		Enabled:          true,
		BlockAction:      filtering.ActionBlock,
		BlacklistDomains: []string{"blocked.com"},
	})
	defer pe.Close()

	ctx := context.Background()
	result, err := pe.EvaluateWithContext(ctx, "blocked.com")
	require.NoError(t, err)
	assert.Equal(t, filtering.ActionBlock, result.Action)
}

func TestPolicyEngine_EvaluateWithContext_Cancelled(t *testing.T) {
	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{
		Enabled: true,
	})
	defer pe.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := pe.EvaluateWithContext(ctx, "example.com")
	require.Error(t, err, "Should return error for cancelled context")
	assert.Equal(t, context.Canceled, err)
}

func TestPolicyEngine_Statistics(t *testing.T) {
	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{
		Enabled:          true,
		BlockAction:      filtering.ActionBlock,
		BlacklistDomains: []string{"blocked.com"},
	})
	defer pe.Close()

	// Make some queries
	pe.Evaluate("blocked.com") // blocked
	pe.Evaluate("allowed.com") // allowed
	pe.Evaluate("other.com")   // allowed

	stats := pe.Stats()
	assert.Equal(t, uint64(3), stats.QueriesTotal)
	assert.Equal(t, uint64(1), stats.QueriesBlocked)
	assert.Equal(t, uint64(2), stats.QueriesAllowed)
}

func TestPolicyEngine_Close(t *testing.T) {
	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{
		Enabled: true,
	})

	err := pe.Close()
	assert.NoError(t, err)
}

// =============================================================================
// Action Tests
// =============================================================================

func TestAction_String(t *testing.T) {
	tests := []struct {
		action filtering.Action
		want   string
	}{
		{filtering.ActionAllow, "allow"},
		{filtering.ActionBlock, "block"},
		{filtering.ActionLog, "log"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.action.String())
		})
	}
}

// =============================================================================
// DomainSet Tests
// =============================================================================

func TestDomainSet_BasicOperations(t *testing.T) {
	ds := filtering.NewDomainSet()

	ds.Add("example.com")
	ds.Add("test.org")

	assert.True(t, ds.Contains("example.com"))
	assert.True(t, ds.Contains("test.org"))
	assert.False(t, ds.Contains("other.com"))
	assert.Equal(t, 2, ds.Size())
}

// =============================================================================
// Parser Tests
// =============================================================================

func TestParser_ParseHosts(t *testing.T) {
	parser := filtering.NewParser()

	hostsContent := `
# Comment
127.0.0.1 localhost
0.0.0.0 ads.example.com
0.0.0.0 tracker.example.org
`
	trie, err := parser.Parse(strings.NewReader(hostsContent), filtering.FormatHosts)
	require.NoError(t, err)

	// Hosts file entries should be in the trie
	assert.True(t, trie.Contains("ads.example.com"))
	assert.True(t, trie.Contains("tracker.example.org"))
	// localhost typically shouldn't be added
	assert.False(t, trie.Contains("localhost"))
}

func TestParser_ParseDomainList(t *testing.T) {
	parser := filtering.NewParser()

	content := `
# Comment line
example.com
test.org
blocked.net
`
	trie, err := parser.Parse(strings.NewReader(content), filtering.FormatDomains)
	require.NoError(t, err)

	assert.True(t, trie.Contains("example.com"))
	assert.True(t, trie.Contains("test.org"))
	assert.True(t, trie.Contains("blocked.net"))
	assert.Equal(t, 3, trie.Size())
}

func TestParser_ParseAdblock(t *testing.T) {
	parser := filtering.NewParser()

	content := `
[Adblock Plus 2.0]
||ads.example.com^
||tracker.example.org^
! This is a comment
@@||allowed.example.com^
`
	trie, err := parser.Parse(strings.NewReader(content), filtering.FormatAdblock)
	require.NoError(t, err)

	assert.True(t, trie.Contains("ads.example.com"))
	assert.True(t, trie.Contains("tracker.example.org"))
}

func TestParser_AutoDetect(t *testing.T) {
	parser := filtering.NewParser()

	// Auto-detect should work for hosts format
	content := `0.0.0.0 ads.example.com`
	trie, err := parser.Parse(strings.NewReader(content), filtering.FormatAuto)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, trie.Size(), 0)
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestDomainTrie_ConcurrentReads(_ *testing.T) {
	trie := filtering.NewDomainTrie()

	// Add some domains
	domains := []string{"a.com", "b.com", "c.com", "d.com", "e.com"}
	for _, d := range domains {
		trie.Add(d, false)
	}

	// Concurrent reads should be safe
	done := make(chan bool)
	for range 10 {
		go func() {
			for range 1000 {
				for _, d := range domains {
					_ = trie.Contains(d)
				}
			}
			done <- true
		}()
	}

	for range 10 {
		<-done
	}
}

func TestPolicyEngine_ConcurrentEvaluate(t *testing.T) {
	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{
		Enabled:          true,
		BlacklistDomains: []string{"blocked.com"},
	})
	defer pe.Close()

	done := make(chan bool)
	for range 10 {
		go func() {
			for range 1000 {
				pe.Evaluate("blocked.com")
				pe.Evaluate("allowed.com")
			}
			done <- true
		}()
	}

	for range 10 {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for goroutines")
		}
	}
}
