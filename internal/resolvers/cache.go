package resolvers

import (
	"container/list"
	"sync"
	"time"
)

// CacheEntryType categorizes cached DNS responses for different TTL handling.
type CacheEntryType int

const (
	CachePositive CacheEntryType = iota // Successful response with answers
	CacheNXDOMAIN                       // Non-existent domain (RCODE=3)
	CacheNODATA                         // Name exists but no data for query type
	CacheSERVFAIL                       // Server failure (RCODE=2)
)

// cacheEntry holds a cached value with expiration and LRU tracking.
type cacheEntry[V any] struct {
	key       any
	value     V
	expiresAt time.Time
	type_     CacheEntryType
	elem      *list.Element // Position in LRU list
}

// TTLCache is a thread-safe, TTL-aware LRU cache for DNS responses.
//
// Features:
//   - Per-entry TTL based on DNS record TTLs
//   - LRU eviction when capacity is reached
//   - Negative caching for NXDOMAIN, NODATA, SERVFAIL
//   - Hit/miss statistics
type TTLCache[K comparable, V any] struct {
	mu sync.Mutex

	defaultTTL      time.Duration // Default TTL for entries without explicit TTL
	maxTTL          time.Duration // Maximum TTL cap for positive entries
	maxEntries      int           // Maximum number of cache entries
	negativeEnabled bool          // Whether to cache negative responses
	negativeTTL     time.Duration // Default TTL for negative entries
	servfailTTL     time.Duration // TTL for SERVFAIL responses
	maxNegativeTTL  time.Duration // Maximum TTL cap for negative entries

	lru  *list.List           // LRU list (front = oldest, back = newest)
	data map[K]*cacheEntry[V] // Key -> entry mapping

	hits         int // Cache hit count
	misses       int // Cache miss count
	negativeHits int // Negative cache hit count
}

// NewTTLCache creates a new TTL cache with the specified maximum entries.
func NewTTLCache[K comparable, V any](maxEntries int) *TTLCache[K, V] {
	if maxEntries <= 0 {
		maxEntries = 1
	}
	return &TTLCache[K, V]{
		defaultTTL:      60 * time.Second,
		maxTTL:          24 * time.Hour,
		maxEntries:      maxEntries,
		negativeEnabled: true,
		negativeTTL:     5 * time.Minute,
		servfailTTL:     30 * time.Second,
		maxNegativeTTL:  1 * time.Hour,
		lru:             list.New(),
		data:            map[K]*cacheEntry[V]{},
	}
}

// Get retrieves a value from the cache.
// Returns (value, found, entryType). Expired entries are removed and count as misses.
func (c *TTLCache[K, V]) Get(key K) (V, bool, CacheEntryType) {
	var zero V
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	e := c.data[key]
	if e == nil {
		c.misses++
		return zero, false, CachePositive
	}

	// Check expiration
	if !e.expiresAt.After(now) {
		c.lru.Remove(e.elem)
		delete(c.data, key)
		c.misses++
		return zero, false, CachePositive
	}

	// Move to back of LRU (most recently used)
	c.lru.MoveToBack(e.elem)
	c.hits++
	if e.type_ != CachePositive {
		c.negativeHits++
	}
	return e.value, true, e.type_
}

// Set stores a value in the cache with the specified TTL and entry type.
// TTL is capped based on entry type. Entries with TTL <= 0 are not stored.
func (c *TTLCache[K, V]) Set(key K, val V, ttl time.Duration, entryType CacheEntryType) {
	if ttl <= 0 {
		return
	}

	// Apply TTL caps based on entry type
	ttl = c.capTTL(ttl, entryType)
	if ttl <= 0 {
		return
	}

	expires := time.Now().Add(ttl)

	c.mu.Lock()
	defer c.mu.Unlock()

	// Update existing entry
	if existing := c.data[key]; existing != nil {
		existing.value = val
		existing.expiresAt = expires
		existing.type_ = entryType
		c.lru.MoveToBack(existing.elem)
		return
	}

	// Create new entry
	e := &cacheEntry[V]{value: val, expiresAt: expires, type_: entryType}
	e.elem = c.lru.PushBack(key)
	c.data[key] = e

	// Evict oldest entries if over capacity
	c.evictOldest()
}

// capTTL applies TTL caps based on entry type.
// Returns 0 if the entry should not be cached (negative caching disabled).
func (c *TTLCache[K, V]) capTTL(ttl time.Duration, entryType CacheEntryType) time.Duration {
	switch entryType {
	case CacheSERVFAIL:
		if !c.negativeEnabled {
			return 0
		}
		if ttl > c.maxNegativeTTL {
			return c.maxNegativeTTL
		}
	case CacheNXDOMAIN, CacheNODATA:
		if !c.negativeEnabled {
			return 0
		}
		if ttl > c.maxNegativeTTL {
			return c.maxNegativeTTL
		}
	default: // CachePositive
		if ttl > c.maxTTL {
			return c.maxTTL
		}
	}
	return ttl
}

// evictOldest removes the oldest entries until under capacity.
func (c *TTLCache[K, V]) evictOldest() {
	for len(c.data) > c.maxEntries {
		front := c.lru.Front()
		if front == nil {
			break
		}
		k := front.Value.(K)
		c.lru.Remove(front)
		delete(c.data, k)
	}
}
