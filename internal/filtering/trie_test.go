package filtering

import (
	"strings"
	"testing"
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
			if got != tt.want {
				t.Errorf("Contains(%q) = %v, want %v", tt.checkDomain, got, tt.want)
			}
		})
	}
}

func TestDomainTrie_Size(t *testing.T) {
	trie := NewDomainTrie()

	if trie.Size() != 0 {
		t.Errorf("empty trie size = %d, want 0", trie.Size())
	}

	trie.Add("example.com", false)
	if trie.Size() != 1 {
		t.Errorf("size after 1 add = %d, want 1", trie.Size())
	}

	// Adding same domain again should not increase size
	trie.Add("example.com", false)
	if trie.Size() != 1 {
		t.Errorf("size after duplicate add = %d, want 1", trie.Size())
	}

	trie.Add("other.com", false)
	if trie.Size() != 2 {
		t.Errorf("size after 2nd domain = %d, want 2", trie.Size())
	}
}

func TestDomainTrie_Clear(t *testing.T) {
	trie := NewDomainTrie()
	trie.Add("example.com", true)
	trie.Add("other.com", true)

	if trie.Size() != 2 {
		t.Errorf("size before clear = %d, want 2", trie.Size())
	}

	trie.Clear()

	if trie.Size() != 0 {
		t.Errorf("size after clear = %d, want 0", trie.Size())
	}

	if trie.Contains("example.com") {
		t.Error("Contains(example.com) after clear = true, want false")
	}
}

func TestDomainTrie_Merge(t *testing.T) {
	trie1 := NewDomainTrie()
	trie1.Add("example.com", true)

	trie2 := NewDomainTrie()
	trie2.Add("other.com", true)
	trie2.Add("another.org", false)

	trie1.Merge(trie2)

	if trie1.Size() != 3 {
		t.Errorf("merged size = %d, want 3", trie1.Size())
	}

	if !trie1.Contains("example.com") {
		t.Error("merged trie should contain example.com")
	}
	if !trie1.Contains("other.com") {
		t.Error("merged trie should contain other.com")
	}
	if !trie1.Contains("another.org") {
		t.Error("merged trie should contain another.org")
	}
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
			if len(got) != len(tt.want) {
				t.Errorf("reversedLabels(%q) length = %d, want %d", tt.domain, len(got), len(tt.want))
				return
			}
			for i, label := range got {
				if label != tt.want[i] {
					t.Errorf("reversedLabels(%q)[%d] = %q, want %q", tt.domain, i, label, tt.want[i])
				}
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
			if got != tt.want {
				t.Errorf("normalizeDomain(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDomainSet(t *testing.T) {
	set := NewDomainSet()

	set.Add("example.com")
	set.Add("Example.COM") // duplicate with different case

	if set.Size() != 1 {
		t.Errorf("set size = %d, want 1", set.Size())
	}

	if !set.Contains("example.com") {
		t.Error("set should contain example.com")
	}
	if !set.Contains("EXAMPLE.COM") {
		t.Error("set should contain EXAMPLE.COM (case-insensitive)")
	}
	if set.Contains("other.com") {
		t.Error("set should not contain other.com")
	}

	set.Clear()
	if set.Size() != 0 {
		t.Errorf("set size after clear = %d, want 0", set.Size())
	}
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
