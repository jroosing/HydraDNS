package resolvers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTTLCache(t *testing.T) {
	cache := NewTTLCache[string, []byte](100)
	require.NotNil(t, cache)
	assert.Equal(t, 100, cache.maxEntries)

	// Test with zero/negative entries
	cache = NewTTLCache[string, []byte](0)
	assert.Equal(t, 1, cache.maxEntries, "expected minimum of 1")

	cache = NewTTLCache[string, []byte](-5)
	assert.Equal(t, 1, cache.maxEntries, "expected minimum of 1")
}

func TestCacheSetGet(t *testing.T) {
	cache := NewTTLCache[string, string](10)

	// Set and get
	cache.Set("key1", "value1", 1*time.Hour, CachePositive)
	val, found, entryType := cache.Get("key1")
	require.True(t, found)
	assert.Equal(t, "value1", val)
	assert.Equal(t, CachePositive, entryType)

	// Get non-existent key
	_, found, _ = cache.Get("nonexistent")
	assert.False(t, found)
}

func TestCacheExpiration(t *testing.T) {
	cache := NewTTLCache[string, string](10)

	// Set with very short TTL
	cache.Set("key1", "value1", 1*time.Millisecond, CachePositive)

	// Wait for expiration
	time.Sleep(5 * time.Millisecond)

	// Should not be found
	_, found, _ := cache.Get("key1")
	assert.False(t, found, "expected expired entry to not be found")
}

func TestCacheZeroTTL(t *testing.T) {
	cache := NewTTLCache[string, string](10)

	// Set with zero TTL should not store
	cache.Set("key1", "value1", 0, CachePositive)
	_, found, _ := cache.Get("key1")
	assert.False(t, found, "expected zero TTL entry to not be stored")

	// Negative TTL should not store
	cache.Set("key2", "value2", -1*time.Second, CachePositive)
	_, found, _ = cache.Get("key2")
	assert.False(t, found, "expected negative TTL entry to not be stored")
}

func TestCacheLRUEviction(t *testing.T) {
	cache := NewTTLCache[string, string](3)

	// Fill cache
	cache.Set("key1", "value1", 1*time.Hour, CachePositive)
	cache.Set("key2", "value2", 1*time.Hour, CachePositive)
	cache.Set("key3", "value3", 1*time.Hour, CachePositive)

	// Access key1 to make it recently used
	cache.Get("key1")

	// Add another entry - should evict key2 (oldest unused)
	cache.Set("key4", "value4", 1*time.Hour, CachePositive)

	// key1 should still exist
	_, found, _ := cache.Get("key1")
	assert.True(t, found, "expected key1 to still exist (recently used)")

	// key2 should be evicted
	_, found, _ = cache.Get("key2")
	assert.False(t, found, "expected key2 to be evicted")

	// key3 and key4 should exist
	_, found, _ = cache.Get("key3")
	assert.True(t, found, "expected key3 to exist")
	_, found, _ = cache.Get("key4")
	assert.True(t, found, "expected key4 to exist")
}

func TestCacheUpdate(t *testing.T) {
	cache := NewTTLCache[string, string](10)

	cache.Set("key1", "value1", 1*time.Hour, CachePositive)
	cache.Set("key1", "value2", 1*time.Hour, CachePositive)

	val, found, _ := cache.Get("key1")
	require.True(t, found)
	assert.Equal(t, "value2", val)
}

func TestCacheNegativeEntries(t *testing.T) {
	cache := NewTTLCache[string, string](10)

	// NXDOMAIN
	cache.Set("nxdomain", "nx", 5*time.Minute, CacheNXDOMAIN)
	_, found, entryType := cache.Get("nxdomain")
	require.True(t, found, "expected to find NXDOMAIN entry")
	assert.Equal(t, CacheNXDOMAIN, entryType)

	// NODATA
	cache.Set("nodata", "nd", 5*time.Minute, CacheNODATA)
	_, found, entryType = cache.Get("nodata")
	require.True(t, found, "expected to find NODATA entry")
	assert.Equal(t, CacheNODATA, entryType)

	// SERVFAIL
	cache.Set("servfail", "sf", 30*time.Second, CacheSERVFAIL)
	_, found, entryType = cache.Get("servfail")
	require.True(t, found, "expected to find SERVFAIL entry")
	assert.Equal(t, CacheSERVFAIL, entryType)
}

func TestCapTTL(t *testing.T) {
	cache := NewTTLCache[string, string](10)
	cache.maxTTL = 1 * time.Hour
	cache.maxNegativeTTL = 30 * time.Minute

	tests := []struct {
		name      string
		ttl       time.Duration
		entryType CacheEntryType
		wantMax   time.Duration
	}{
		{"positive under max", 30 * time.Minute, CachePositive, 30 * time.Minute},
		{"positive over max", 2 * time.Hour, CachePositive, 1 * time.Hour},
		{"nxdomain under max", 10 * time.Minute, CacheNXDOMAIN, 10 * time.Minute},
		{"nxdomain over max", 1 * time.Hour, CacheNXDOMAIN, 30 * time.Minute},
		{"servfail", 1 * time.Hour, CacheSERVFAIL, 30 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cache.capTTL(tt.ttl, tt.entryType)
			assert.LessOrEqual(t, got, tt.wantMax)
		})
	}
}

func TestCapTTLNegativeDisabled(t *testing.T) {
	cache := NewTTLCache[string, string](10)
	cache.negativeEnabled = false

	// Negative entries should return 0 TTL (don't cache)
	got := cache.capTTL(5*time.Minute, CacheNXDOMAIN)
	assert.Zero(t, got, "expected 0 TTL when negative caching disabled")

	got = cache.capTTL(5*time.Minute, CacheNODATA)
	assert.Zero(t, got, "expected 0 TTL for NODATA when disabled")

	got = cache.capTTL(30*time.Second, CacheSERVFAIL)
	assert.Zero(t, got, "expected 0 TTL for SERVFAIL when disabled")

	// Positive should still work
	got = cache.capTTL(30*time.Minute, CachePositive)
	assert.NotZero(t, got, "expected non-zero TTL for positive entry")
}

func TestCacheStatistics(t *testing.T) {
	cache := NewTTLCache[string, string](10)

	// Initial stats should be zero
	assert.Zero(t, cache.hits)
	assert.Zero(t, cache.misses)

	// Cache miss
	cache.Get("nonexistent")
	assert.Equal(t, 1, cache.misses)

	// Cache hit
	cache.Set("key1", "value1", 1*time.Hour, CachePositive)
	cache.Get("key1")
	assert.Equal(t, 1, cache.hits)

	// Negative hit
	cache.Set("nx", "value", 1*time.Hour, CacheNXDOMAIN)
	cache.Get("nx")
	assert.Equal(t, 1, cache.negativeHits)
}
