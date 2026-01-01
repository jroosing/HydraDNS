package filtering

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDomainTrie_Add_Contains(t *testing.T) {
	tests := []struct {
		name       string
		addDomains []struct {
			domain   string
			wildcard bool
		}
		checkDomain string
		want        bool
	}{
		{
			name: "exact match",
			addDomains: []struct {
				domain   string
				wildcard bool
			}{
				{"example.com", false},
			},
			checkDomain: "example.com",
			want:        true,
		},
		{
			name: "exact match with different case",
			addDomains: []struct {
				domain   string
				wildcard bool
			}{
				{"Example.COM", false},
			},
			checkDomain: "example.com",
			want:        true,
		},
		{
			name: "subdomain without wildcard - should not match",
			addDomains: []struct {
				domain   string
				wildcard bool
			}{
				{"example.com", false},
			},
			checkDomain: "sub.example.com",
			want:        false,
		},
		{
			name: "subdomain with wildcard - should match",
			addDomains: []struct {
				domain   string
				wildcard bool
			}{
				{"example.com", true},
			},
			checkDomain: "sub.example.com",
			want:        true,
		},
		{
			name: "deep subdomain with wildcard",
			addDomains: []struct {
				domain   string
				wildcard bool
			}{
				{"example.com", true},
			},
			checkDomain: "a.b.c.example.com",
			want:        true,
		},
		{
			name: "parent domain should not match child entry",
			addDomains: []struct {
				domain   string
				wildcard bool
			}{
				{"sub.example.com", false},
			},
			checkDomain: "example.com",
			want:        false,
		},
		{
			name: "unrelated domain should not match",
			addDomains: []struct {
				domain   string
				wildcard bool
			}{
				{"example.com", true},
			},
			checkDomain: "other.org",
			want:        false,
		},
		{
			name: "similar domain should not match",
			addDomains: []struct {
				domain   string
				wildcard bool
			}{
				{"example.com", true},
			},
			checkDomain: "notexample.com",
			want:        false,
		},
		{
			name: "domain with trailing dot",
			addDomains: []struct {
				domain   string
				wildcard bool
			}{
				{"example.com.", false},
			},
			checkDomain: "example.com",
			want:        true,
		},
		{
			name: "check with trailing dot",
			addDomains: []struct {
				domain   string
				wildcard bool
			}{
				{"example.com", false},
			},
			checkDomain: "example.com.",
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trie := NewDomainTrie()
			for _, d := range tt.addDomains {
				trie.Add(d.domain, d.wildcard)
			}

			got := trie.Contains(tt.checkDomain)
			assert.Equal(t, tt.want, got, "Contains(%q)", tt.checkDomain)
		})
	}
}

func TestDomainTrie_Size(t *testing.T) {
	trie := NewDomainTrie()

	assert.Equal(t, 0, trie.Size(), "empty trie size")

	trie.Add("example.com", false)
	assert.Equal(t, 1, trie.Size(), "size after 1 add")

	// Adding same domain again should not increase size
	trie.Add("example.com", false)
	assert.Equal(t, 1, trie.Size(), "size after duplicate add")

	trie.Add("other.com", false)
	assert.Equal(t, 2, trie.Size(), "size after 2nd domain")
}

func TestDomainTrie_Clear(t *testing.T) {
	trie := NewDomainTrie()
	trie.Add("example.com", true)
	trie.Add("other.com", true)

	assert.Equal(t, 2, trie.Size(), "size before clear")

	trie.Clear()

	assert.Equal(t, 0, trie.Size(), "size after clear")
	assert.False(t, trie.Contains("example.com"), "Contains(example.com) after clear")
}

func TestDomainTrie_Merge(t *testing.T) {
	trie1 := NewDomainTrie()
	trie1.Add("example.com", true)

	trie2 := NewDomainTrie()
	trie2.Add("other.com", true)
	trie2.Add("another.org", false)

	trie1.Merge(trie2)

	assert.Equal(t, 3, trie1.Size(), "merged size")
	assert.True(t, trie1.Contains("example.com"), "merged trie should contain example.com")
	assert.True(t, trie1.Contains("other.com"), "merged trie should contain other.com")
	assert.True(t, trie1.Contains("another.org"), "merged trie should contain another.org")
}

func TestReversedLabels(t *testing.T) {
	tests := []struct {
		domain string
		want   []string
	}{
		{"example.com", []string{"com", "example"}},
		{"sub.example.com", []string{"com", "example", "sub"}},
		{"a.b.c.d.example.com", []string{"com", "example", "d", "c", "b", "a"}},
		{"com", []string{"com"}},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			got := reversedLabels(tt.domain)
			assert.Equal(t, len(tt.want), len(got), "length mismatch")
			for i, label := range got {
				assert.Equal(t, tt.want[i], label, "label[%d] mismatch", i)
			}
		})
	}
}

func TestNormalizeDomain(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Example.COM", "example.com"},
		{"example.com.", "example.com"},
		{"  example.com  ", "example.com"},
		{"EXAMPLE.COM.", "example.com"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeDomain(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDomainSet(t *testing.T) {
	set := NewDomainSet()

	set.Add("example.com")
	set.Add("Example.COM") // duplicate with different case

	assert.Equal(t, 1, set.Size(), "set size")
	assert.True(t, set.Contains("example.com"), "set should contain example.com")
	assert.True(t, set.Contains("EXAMPLE.COM"), "set should contain EXAMPLE.COM (case-insensitive)")
	assert.False(t, set.Contains("other.com"), "set should not contain other.com")

	set.Clear()
	assert.Equal(t, 0, set.Size(), "set size after clear")
}

// Benchmark tests

func BenchmarkDomainTrie_Add(b *testing.B) {
	trie := NewDomainTrie()
	domains := generateTestDomains(10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie.Add(domains[i%len(domains)], true)
	}
}

func BenchmarkDomainTrie_Contains(b *testing.B) {
	trie := NewDomainTrie()
	domains := generateTestDomains(10000)
	for _, d := range domains {
		trie.Add(d, true)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie.Contains(domains[i%len(domains)])
	}
}

func BenchmarkDomainTrie_Contains_Miss(b *testing.B) {
	trie := NewDomainTrie()
	domains := generateTestDomains(10000)
	for _, d := range domains {
		trie.Add(d, true)
	}

	// Generate non-matching domains
	missDomains := make([]string, 10000)
	for i := range missDomains {
		missDomains[i] = strings.ReplaceAll(domains[i], ".com", ".xyz")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie.Contains(missDomains[i%len(missDomains)])
	}
}

func BenchmarkDomainTrie_Contains_Subdomain(b *testing.B) {
	trie := NewDomainTrie()
	domains := generateTestDomains(10000)
	for _, d := range domains {
		trie.Add(d, true) // wildcard
	}

	// Generate subdomain queries
	subdomains := make([]string, 10000)
	for i := range subdomains {
		subdomains[i] = "sub." + domains[i]
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie.Contains(subdomains[i%len(subdomains)])
	}
}

func generateTestDomains(n int) []string {
	domains := make([]string, n)
	tlds := []string{"com", "org", "net", "io", "co"}
	for i := 0; i < n; i++ {
		domains[i] = strings.ToLower(strings.ReplaceAll(
			strings.ReplaceAll(
				strings.ReplaceAll(
					"domain"+string(rune('a'+i%26))+string(rune('a'+i/26%26))+"."+tlds[i%len(tlds)],
					" ", ""),
				"\n", ""),
			"\t", ""))
	}
	return domains
}
