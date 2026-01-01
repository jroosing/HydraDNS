package server

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrefixKey(t *testing.T) {
	assert.Equal(t, "v4:203.0.113.0/24", prefixKey("203.0.113.9"))
	assert.Equal(t, "v6:2001:db8::/64", prefixKey("2001:db8::1"))
}

func TestPrefixKey_InvalidIP(t *testing.T) {
	// Invalid IPs return "ip:" + ip fallback
	got := prefixKey("not-an-ip")
	assert.Equal(t, "ip:not-an-ip", got)
}

func TestPrefixKeyFromAddr(t *testing.T) {
	tests := []struct {
		ip       string
		expected string
	}{
		{"192.168.1.100", "192.168.1.0/24"},
		{"10.0.0.1", "10.0.0.0/24"},
		{"2001:db8::1", "2001:db8::/64"},
		{"::1", "::/64"}, // IPv6 loopback masked to /64 prefix
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			addr := netip.MustParseAddr(tt.ip)
			got := prefixKeyFromAddr(addr)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestNewRateLimiter(t *testing.T) {
	settings := RateLimitSettings{
		CleanupSeconds:   30,
		MaxIPEntries:     1000,
		MaxPrefixEntries: 500,
		GlobalQPS:        10000,
		GlobalBurst:      20000,
		PrefixQPS:        1000,
		PrefixBurst:      2000,
		IPQPS:            100,
		IPBurst:          200,
	}

	rl := NewRateLimiter(settings)
	require.NotNil(t, rl, "NewRateLimiter returned nil")
	assert.NotNil(t, rl.global, "global limiter is nil")
	assert.NotNil(t, rl.prefix, "prefix limiter is nil")
	assert.NotNil(t, rl.ip, "ip limiter is nil")
}

func TestNewRateLimiter_ZeroCleanup(t *testing.T) {
	settings := RateLimitSettings{
		CleanupSeconds: 0, // Should default to 60 seconds
		GlobalQPS:      100,
		GlobalBurst:    100,
	}

	rl := NewRateLimiter(settings)
	require.NotNil(t, rl, "NewRateLimiter returned nil")
}

func TestRateLimiter_Allow(t *testing.T) {
	settings := RateLimitSettings{
		CleanupSeconds:   60,
		MaxIPEntries:     100,
		MaxPrefixEntries: 100,
		GlobalQPS:        1000,
		GlobalBurst:      1000,
		PrefixQPS:        500,
		PrefixBurst:      500,
		IPQPS:            10,
		IPBurst:          10,
	}

	rl := NewRateLimiter(settings)

	// First 10 requests should be allowed (burst)
	for i := range 10 {
		assert.True(t, rl.Allow("192.168.1.100"), "request %d should be allowed (within burst)", i)
	}

	// 11th request might be denied depending on timing
	// Just check it doesn't panic
	_ = rl.Allow("192.168.1.100")
}

func TestRateLimiter_AllowNil(t *testing.T) {
	var rl *RateLimiter
	assert.True(t, rl.Allow("192.168.1.1"), "nil RateLimiter should always allow")
}

func TestRateLimiter_AllowAddr(t *testing.T) {
	settings := RateLimitSettings{
		CleanupSeconds:   60,
		MaxIPEntries:     100,
		MaxPrefixEntries: 100,
		GlobalQPS:        1000,
		GlobalBurst:      1000,
		PrefixQPS:        500,
		PrefixBurst:      500,
		IPQPS:            5,
		IPBurst:          5,
	}

	rl := NewRateLimiter(settings)
	addr := netip.MustParseAddr("10.0.0.1")

	// First 5 requests should be allowed
	for i := range 5 {
		assert.True(t, rl.AllowAddr(addr), "request %d should be allowed (within burst)", i)
	}
}

func TestRateLimiter_AllowAddrNil(t *testing.T) {
	var rl *RateLimiter
	addr := netip.MustParseAddr("10.0.0.1")
	assert.True(t, rl.AllowAddr(addr), "nil RateLimiter should always allow")
}

func TestTokenBucketRateLimiter_Allow(t *testing.T) {
	limiter := NewTokenBucketRateLimiter(TokenBucketConfig{
		Rate:            10,
		Burst:           5,
		CleanupInterval: time.Minute,
		MaxEntries:      100,
	})

	// Burst of 5 should be allowed
	for i := range 5 {
		assert.True(t, limiter.Allow("key1"), "request %d should be allowed", i)
	}

	// 6th request should be denied (burst exhausted)
	assert.False(t, limiter.Allow("key1"), "6th request should be denied (burst exhausted)")
}

func TestTokenBucketRateLimiter_DisabledRate(t *testing.T) {
	limiter := NewTokenBucketRateLimiter(TokenBucketConfig{
		Rate:  0, // Disabled
		Burst: 10,
	})

	// Should always allow when rate is 0
	for range 100 {
		assert.True(t, limiter.Allow("key"), "should always allow when rate is 0")
	}
}

func TestTokenBucketRateLimiter_DisabledBurst(t *testing.T) {
	limiter := NewTokenBucketRateLimiter(TokenBucketConfig{
		Rate:  10,
		Burst: 0, // Disabled
	})

	// Should always allow when burst is 0
	for range 100 {
		assert.True(t, limiter.Allow("key"), "should always allow when burst is 0")
	}
}

func TestTokenBucketRateLimiter_Nil(t *testing.T) {
	var limiter *TokenBucketRateLimiter
	assert.True(t, limiter.Allow("key"), "nil limiter should always allow")
}

func TestTokenBucketRateLimiter_MultipleKeys(t *testing.T) {
	limiter := NewTokenBucketRateLimiter(TokenBucketConfig{
		Rate:            10,
		Burst:           2,
		CleanupInterval: time.Minute,
		MaxEntries:      100,
	})

	// Each key gets its own bucket
	assert.True(t, limiter.Allow("key1"), "key1 first request should be allowed")
	assert.True(t, limiter.Allow("key1"), "key1 second request should be allowed")

	// key1 exhausted
	assert.False(t, limiter.Allow("key1"), "key1 third request should be denied")

	// key2 should still have tokens
	assert.True(t, limiter.Allow("key2"), "key2 first request should be allowed")
	assert.True(t, limiter.Allow("key2"), "key2 second request should be allowed")
}

func TestTokenBucketRateLimiter_TokenReplenishment(t *testing.T) {
	limiter := NewTokenBucketRateLimiter(TokenBucketConfig{
		Rate:            100, // 100 tokens/second = 1 token per 10ms
		Burst:           1,
		CleanupInterval: time.Minute,
		MaxEntries:      100,
	})

	// Use the one token
	assert.True(t, limiter.Allow("key"), "first request should be allowed")

	// Wait for token replenishment
	time.Sleep(15 * time.Millisecond)

	// Should have replenished at least 1 token
	assert.True(t, limiter.Allow("key"), "request after replenishment should be allowed")
}

func TestFormatRateLimitsLog(t *testing.T) {
	settings := RateLimitSettings{
		CleanupSeconds:   60,
		MaxIPEntries:     10000,
		MaxPrefixEntries: 5000,
		GlobalQPS:        50000,
		GlobalBurst:      100000,
		PrefixQPS:        5000,
		PrefixBurst:      10000,
		IPQPS:            100,
		IPBurst:          200,
	}

	log := FormatRateLimitsLog(settings)

	assert.NotEmpty(t, log)
	// Just verify it contains expected parts
	assert.Contains(t, log, "global=")
	assert.Contains(t, log, "prefix=")
	assert.Contains(t, log, "ip=")
}

func TestFormatRateLimitsLog_Disabled(t *testing.T) {
	settings := RateLimitSettings{
		GlobalQPS:   0, // Disabled
		GlobalBurst: 0,
		PrefixQPS:   0,
		PrefixBurst: 0,
		IPQPS:       0,
		IPBurst:     0,
	}

	log := FormatRateLimitsLog(settings)
	assert.Contains(t, log, "disabled")
}
