// Package filtering provides high-performance DNS domain filtering
// with whitelist/blacklist support and multiple blocklist format parsing.
package filtering

import (
	"slices"
	"strings"
	"sync"
)

// DomainTrie is a high-performance trie for domain name matching.
//
// Data Structure:
//
// Domains are stored with labels in reverse order for efficient suffix matching.
// Example: "ads.example.com" is stored as ["com", "example", "ads"].
//
// This allows matching "*.example.com" (all subdomains of example.com) by:
//   1. Lookup fails at "ads" level
//   2. But succeeds at "example.com" level with wildcard flag
//   3. All queries for *.example.com are blocked
//
// Memory Efficiency:
//
// Each node uses a map for children (most nodes have few children).
// Root node points to top-level TLD nodes (com, org, edu, etc.).
// This sparse structure avoids allocating unused nodes.
//
// Performance:
//
// Lookup is O(k) where k is the number of labels in the domain.
// Typical domains have 2-4 labels, making lookups very fast (10s of nanoseconds).
//
// Thread Safety:
//
// The trie is thread-safe for concurrent reads after building using RWMutex.
// For runtime updates (adding/removing domains), the caller must synchronize.
type DomainTrie struct {
	root *trieNode
	mu   sync.RWMutex
	size int // number of domains stored
}

// trieNode represents a node in the domain trie.
// Memory-optimized: uses a map for sparse children (most nodes have few children).
type trieNode struct {
	children map[string]*trieNode
	isEnd    bool // marks end of a complete domain
	isWild   bool // marks wildcard match (blocks all subdomains)
}

// NewDomainTrie creates an empty domain trie.
func NewDomainTrie() *DomainTrie {
	return &DomainTrie{
		root: newTrieNode(),
	}
}

func newTrieNode() *trieNode {
	return &trieNode{
		children: make(map[string]*trieNode, 4), // small initial capacity
	}
}

// Add inserts a domain into the trie.
// The domain should be in standard format (e.g., "ads.example.com").
// If wildcard is true, all subdomains will also match.
func (t *DomainTrie) Add(domain string, wildcard bool) {
	domain = normalizeDomain(domain)
	if domain == "" {
		return
	}

	labels := reversedLabels(domain)
	if len(labels) == 0 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	node := t.root
	for _, label := range labels {
		child, exists := node.children[label]
		if !exists {
			child = newTrieNode()
			node.children[label] = child
		}
		node = child
	}

	if !node.isEnd {
		t.size++
	}
	node.isEnd = true
	if wildcard {
		node.isWild = true
	}
}

// Contains checks if a domain matches any entry in the trie.
// Returns true if the domain itself or any parent domain with wildcard flag matches.
//
// For example, if "example.com" is added with wildcard=true:
//   - Contains("example.com") -> true
//   - Contains("ads.example.com") -> true
//   - Contains("sub.ads.example.com") -> true
//
// If "ads.example.com" is added with wildcard=false:
//   - Contains("ads.example.com") -> true
//   - Contains("sub.ads.example.com") -> false (unless wildcard was set)
func (t *DomainTrie) Contains(domain string) bool {
	domain = normalizeDomain(domain)
	if domain == "" {
		return false
	}

	labels := reversedLabels(domain)
	if len(labels) == 0 {
		return false
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	node := t.root
	for i, label := range labels {
		child, exists := node.children[label]
		if !exists {
			return false
		}
		node = child

		// Check for wildcard match at this level
		// A wildcard means all subdomains match, so if we're not at the end
		// of the input domain, a wildcard here means it matches
		if node.isWild && i < len(labels)-1 {
			return true
		}
	}

	// Exact match at the end
	return node.isEnd
}

// Size returns the number of domains in the trie.
func (t *DomainTrie) Size() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.size
}

// Clear removes all entries from the trie.
func (t *DomainTrie) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.root = newTrieNode()
	t.size = 0
}

// Merge adds all domains from another trie into this one.
func (t *DomainTrie) Merge(other *DomainTrie) {
	if other == nil {
		return
	}

	other.mu.RLock()
	defer other.mu.RUnlock()

	t.mu.Lock()
	defer t.mu.Unlock()

	t.mergeNode(t.root, other.root, nil)
}

func (t *DomainTrie) mergeNode(dst, src *trieNode, path []string) {
	for label, srcChild := range src.children {
		dstChild, exists := dst.children[label]
		if !exists {
			dstChild = newTrieNode()
			dst.children[label] = dstChild
		}

		newPath := append(slices.Clone(path), label)

		if srcChild.isEnd && !dstChild.isEnd {
			dstChild.isEnd = true
			t.size++
		}
		if srcChild.isWild {
			dstChild.isWild = true
		}

		t.mergeNode(dstChild, srcChild, newPath)
	}
}

// normalizeDomain converts a domain to lowercase and removes trailing dots.
func normalizeDomain(domain string) string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	domain = strings.TrimSuffix(domain, ".")
	return domain
}

// reversedLabels splits a domain into labels in reverse order.
// "ads.example.com" -> ["com", "example", "ads"].
func reversedLabels(domain string) []string {
	labels := strings.Split(domain, ".")
	n := len(labels)
	for i := range n / 2 {
		labels[i], labels[n-1-i] = labels[n-1-i], labels[i]
	}
	return labels
}

// DomainSet is a simple hash set for exact domain matching.
// Use this for small sets or when no subdomain matching is needed.
type DomainSet struct {
	domains map[string]struct{}
	mu      sync.RWMutex
}

// NewDomainSet creates an empty domain set.
func NewDomainSet() *DomainSet {
	return &DomainSet{
		domains: make(map[string]struct{}),
	}
}

// Add inserts a domain into the set.
func (s *DomainSet) Add(domain string) {
	domain = normalizeDomain(domain)
	if domain == "" {
		return
	}

	s.mu.Lock()
	s.domains[domain] = struct{}{}
	s.mu.Unlock()
}

// Contains checks if an exact domain is in the set.
func (s *DomainSet) Contains(domain string) bool {
	domain = normalizeDomain(domain)
	if domain == "" {
		return false
	}

	s.mu.RLock()
	_, exists := s.domains[domain]
	s.mu.RUnlock()
	return exists
}

// Size returns the number of domains in the set.
func (s *DomainSet) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.domains)
}

// Clear removes all domains from the set.
func (s *DomainSet) Clear() {
	s.mu.Lock()
	s.domains = make(map[string]struct{})
	s.mu.Unlock()
}
