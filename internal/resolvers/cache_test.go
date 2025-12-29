package resolvers

import (
	"testing"
	"time"
)

func TestNewTTLCache(t *testing.T) {
	cache := NewTTLCache[string, []byte](100)
	if cache == nil {
		t.Fatal("expected non-nil cache")
	}
	if cache.maxEntries != 100 {
		t.Errorf("expected maxEntries 100, got %d", cache.maxEntries)
	}

	// Test with zero/negative entries
	cache = NewTTLCache[string, []byte](0)
	if cache.maxEntries != 1 {
		t.Errorf("expected maxEntries 1 (minimum), got %d", cache.maxEntries)
	}

	cache = NewTTLCache[string, []byte](-5)
	if cache.maxEntries != 1 {
		t.Errorf("expected maxEntries 1 (minimum), got %d", cache.maxEntries)
	}
}

func TestCacheSetGet(t *testing.T) {
	cache := NewTTLCache[string, string](10)

	// Set and get
	cache.Set("key1", "value1", 1*time.Hour, CachePositive)
	val, found, entryType := cache.Get("key1")
	if !found {
		t.Fatal("expected to find key1")
	}
	if val != "value1" {
		t.Errorf("expected value1, got %s", val)
	}
	if entryType != CachePositive {
		t.Errorf("expected CachePositive, got %d", entryType)
	}

	// Get non-existent key
	_, found, _ = cache.Get("nonexistent")
	if found {
		t.Error("expected not found for nonexistent key")
	}
}

func TestCacheExpiration(t *testing.T) {
	cache := NewTTLCache[string, string](10)

	// Set with very short TTL
	cache.Set("key1", "value1", 1*time.Millisecond, CachePositive)

	// Wait for expiration
	time.Sleep(5 * time.Millisecond)

	// Should not be found
	_, found, _ := cache.Get("key1")
	if found {
		t.Error("expected expired entry to not be found")
	}
}

func TestCacheZeroTTL(t *testing.T) {
	cache := NewTTLCache[string, string](10)

	// Set with zero TTL should not store
	cache.Set("key1", "value1", 0, CachePositive)

	_, found, _ := cache.Get("key1")
	if found {
		t.Error("expected zero TTL entry to not be stored")
	}

	// Negative TTL should not store
	cache.Set("key2", "value2", -1*time.Second, CachePositive)
	_, found, _ = cache.Get("key2")
	if found {
		t.Error("expected negative TTL entry to not be stored")
	}
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
	if !found {
		t.Error("expected key1 to still exist (recently used)")
	}

	// key2 should be evicted
	_, found, _ = cache.Get("key2")
	if found {
		t.Error("expected key2 to be evicted")
	}

	// key3 and key4 should exist
	_, found, _ = cache.Get("key3")
	if !found {
		t.Error("expected key3 to exist")
	}
	_, found, _ = cache.Get("key4")
	if !found {
		t.Error("expected key4 to exist")
	}
}

func TestCacheUpdate(t *testing.T) {
	cache := NewTTLCache[string, string](10)

	cache.Set("key1", "value1", 1*time.Hour, CachePositive)
	cache.Set("key1", "value2", 1*time.Hour, CachePositive)

	val, found, _ := cache.Get("key1")
	if !found {
		t.Fatal("expected to find key1")
	}
	if val != "value2" {
		t.Errorf("expected updated value2, got %s", val)
	}
}

func TestCacheNegativeEntries(t *testing.T) {
	cache := NewTTLCache[string, string](10)

	// NXDOMAIN
	cache.Set("nxdomain", "nx", 5*time.Minute, CacheNXDOMAIN)
	_, found, entryType := cache.Get("nxdomain")
	if !found {
		t.Error("expected to find NXDOMAIN entry")
	}
	if entryType != CacheNXDOMAIN {
		t.Errorf("expected CacheNXDOMAIN, got %d", entryType)
	}

	// NODATA
	cache.Set("nodata", "nd", 5*time.Minute, CacheNODATA)
	_, found, entryType = cache.Get("nodata")
	if !found {
		t.Error("expected to find NODATA entry")
	}
	if entryType != CacheNODATA {
		t.Errorf("expected CacheNODATA, got %d", entryType)
	}

	// SERVFAIL
	cache.Set("servfail", "sf", 30*time.Second, CacheSERVFAIL)
	_, found, entryType = cache.Get("servfail")
	if !found {
		t.Error("expected to find SERVFAIL entry")
	}
	if entryType != CacheSERVFAIL {
		t.Errorf("expected CacheSERVFAIL, got %d", entryType)
	}
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
			if got > tt.wantMax {
				t.Errorf("capTTL = %v, want max %v", got, tt.wantMax)
			}
		})
	}
}

func TestCapTTLNegativeDisabled(t *testing.T) {
	cache := NewTTLCache[string, string](10)
	cache.negativeEnabled = false

	// Negative entries should return 0 TTL (don't cache)
	got := cache.capTTL(5*time.Minute, CacheNXDOMAIN)
	if got != 0 {
		t.Errorf("expected 0 TTL when negative caching disabled, got %v", got)
	}

	got = cache.capTTL(5*time.Minute, CacheNODATA)
	if got != 0 {
		t.Errorf("expected 0 TTL for NODATA when disabled, got %v", got)
	}

	got = cache.capTTL(30*time.Second, CacheSERVFAIL)
	if got != 0 {
		t.Errorf("expected 0 TTL for SERVFAIL when disabled, got %v", got)
	}

	// Positive should still work
	got = cache.capTTL(30*time.Minute, CachePositive)
	if got == 0 {
		t.Error("expected non-zero TTL for positive entry")
	}
}

func TestCacheStatistics(t *testing.T) {
	cache := NewTTLCache[string, string](10)

	// Initial stats should be zero
	if cache.hits != 0 || cache.misses != 0 {
		t.Error("expected initial stats to be zero")
	}

	// Cache miss
	cache.Get("nonexistent")
	if cache.misses != 1 {
		t.Errorf("expected 1 miss, got %d", cache.misses)
	}

	// Cache hit
	cache.Set("key1", "value1", 1*time.Hour, CachePositive)
	cache.Get("key1")
	if cache.hits != 1 {
		t.Errorf("expected 1 hit, got %d", cache.hits)
	}

	// Negative hit
	cache.Set("nx", "value", 1*time.Hour, CacheNXDOMAIN)
	cache.Get("nx")
	if cache.negativeHits != 1 {
		t.Errorf("expected 1 negative hit, got %d", cache.negativeHits)
	}
}
