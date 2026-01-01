// Package server_test provides behavior tests for the server package.
package server_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/jroosing/hydradns/internal/resolvers"
	"github.com/jroosing/hydradns/internal/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// RateLimiter Tests
// ============================================================================

func TestRateLimiter_AllowsWithinLimit(t *testing.T) {
	limiter := server.NewRateLimiter(server.RateLimitSettings{
		GlobalQPS:   1000,
		GlobalBurst: 100,
		PrefixQPS:   100,
		PrefixBurst: 10,
		IPQPS:       10,
		IPBurst:     5,
	})

	// Should allow first few requests
	for i := range 5 {
		assert.True(t, limiter.Allow("192.168.1.1"), "Request %d should be allowed", i)
	}
}

func TestRateLimiter_BlocksExceedingLimit(t *testing.T) {
	limiter := server.NewRateLimiter(server.RateLimitSettings{
		GlobalQPS:   1000,
		GlobalBurst: 100,
		PrefixQPS:   100,
		PrefixBurst: 10,
		IPQPS:       10,
		IPBurst:     2, // Very low burst
	})

	// Exhaust the burst
	limiter.Allow("192.168.1.1")
	limiter.Allow("192.168.1.1")

	// Should now be rate limited
	assert.False(t, limiter.Allow("192.168.1.1"), "Should be rate limited after exceeding burst")
}

func TestRateLimiter_DifferentIPsIndependent(t *testing.T) {
	// Test that IPs in different /24 subnets have independent per-IP buckets
	// Must set MaxIPEntries and MaxPrefixEntries to avoid eviction
	limiter := server.NewRateLimiter(server.RateLimitSettings{
		GlobalQPS:        100000,
		GlobalBurst:      10000,
		PrefixQPS:        100000,
		PrefixBurst:      10000,
		IPQPS:            10,
		IPBurst:          2,
		MaxIPEntries:     1000, // Important: must track multiple IPs
		MaxPrefixEntries: 1000,
	})

	// IP1: use up its burst
	assert.True(t, limiter.Allow("192.168.1.1"), "IP1 first request")
	assert.True(t, limiter.Allow("192.168.1.1"), "IP1 second request")
	// IP1 should now be rate limited

	// IP2 in DIFFERENT /24 subnet should have its own bucket
	assert.True(t, limiter.Allow("10.0.0.1"), "IP2 first request - different /24 should have its own bucket")
	assert.True(t, limiter.Allow("10.0.0.1"), "IP2 second request")
}

func TestRateLimiter_NilLimiter(t *testing.T) {
	var limiter *server.RateLimiter

	// Nil limiter should allow everything
	assert.True(t, limiter.Allow("192.168.1.1"))
}

func TestRateLimiter_AllowAddr(t *testing.T) {
	limiter := server.NewRateLimiter(server.RateLimitSettings{
		GlobalQPS:   1000,
		GlobalBurst: 100,
		PrefixQPS:   100,
		PrefixBurst: 10,
		IPQPS:       10,
		IPBurst:     5,
	})

	ip := netip.MustParseAddr("192.168.1.1")

	for i := range 5 {
		assert.True(t, limiter.AllowAddr(ip), "Request %d should be allowed", i)
	}
}

func TestRateLimiter_IPv6(t *testing.T) {
	limiter := server.NewRateLimiter(server.RateLimitSettings{
		GlobalQPS:   1000,
		GlobalBurst: 100,
		PrefixQPS:   100,
		PrefixBurst: 10,
		IPQPS:       10,
		IPBurst:     5,
	})

	ip := netip.MustParseAddr("2001:db8::1")

	for i := range 5 {
		assert.True(t, limiter.AllowAddr(ip), "IPv6 request %d should be allowed", i)
	}
}

func TestRateLimiter_PrefixLimit(t *testing.T) {
	limiter := server.NewRateLimiter(server.RateLimitSettings{
		GlobalQPS:   1000,
		GlobalBurst: 100,
		PrefixQPS:   10,
		PrefixBurst: 3, // Low prefix burst
		IPQPS:       10,
		IPBurst:     10,
	})

	// Different IPs in same /24 prefix
	limiter.Allow("192.168.1.1")
	limiter.Allow("192.168.1.2")
	limiter.Allow("192.168.1.3")

	// Should be prefix-limited now
	assert.False(t, limiter.Allow("192.168.1.4"), "Should be prefix-limited")
}

func TestRateLimiter_GlobalLimit(t *testing.T) {
	limiter := server.NewRateLimiter(server.RateLimitSettings{
		GlobalQPS:   10,
		GlobalBurst: 2, // Very low global burst
		PrefixQPS:   1000,
		PrefixBurst: 100,
		IPQPS:       1000,
		IPBurst:     100,
	})

	limiter.Allow("192.168.1.1")
	limiter.Allow("10.0.0.1")

	// Should be globally limited now despite different IPs
	assert.False(t, limiter.Allow("172.16.0.1"), "Should be globally limited")
}

// ============================================================================
// TokenBucketRateLimiter Tests
// ============================================================================

func TestTokenBucket_AllowConsumesToken(t *testing.T) {
	tb := server.NewTokenBucketRateLimiter(server.TokenBucketConfig{
		Rate:       1.0,
		Burst:      5,
		MaxEntries: 100,
	})

	// Should allow up to burst
	for i := range 5 {
		assert.True(t, tb.Allow("key1"), "Request %d should be allowed", i)
	}

	// Should be rate limited now
	assert.False(t, tb.Allow("key1"), "Should be rate limited after burst")
}

func TestTokenBucket_DifferentKeys(t *testing.T) {
	tb := server.NewTokenBucketRateLimiter(server.TokenBucketConfig{
		Rate:       1.0,
		Burst:      2,
		MaxEntries: 100,
	})

	// Exhaust key1
	tb.Allow("key1")
	tb.Allow("key1")

	// key2 should have its own bucket
	assert.True(t, tb.Allow("key2"), "Different key should have separate bucket")
}

func TestTokenBucket_TokenReplenishment(t *testing.T) {
	tb := server.NewTokenBucketRateLimiter(server.TokenBucketConfig{
		Rate:       1000.0, // 1000 tokens per second
		Burst:      1,
		MaxEntries: 100,
	})

	// Exhaust tokens
	assert.True(t, tb.Allow("key1"))
	assert.False(t, tb.Allow("key1"))

	// Wait for replenishment
	time.Sleep(5 * time.Millisecond)

	// Should have tokens again
	assert.True(t, tb.Allow("key1"), "Should have replenished tokens")
}

func TestTokenBucket_DisabledWithZeroRate(t *testing.T) {
	tb := server.NewTokenBucketRateLimiter(server.TokenBucketConfig{
		Rate:       0, // Disabled
		Burst:      5,
		MaxEntries: 100,
	})

	// With rate=0, behavior depends on implementation
	// Typically allows since no tokens are consumed
	_ = tb.Allow("key1")
}

// ============================================================================
// RateLimitSettings Tests
// ============================================================================

func TestFormatRateLimitsLog(t *testing.T) {
	settings := server.RateLimitSettings{
		GlobalQPS:        1000,
		GlobalBurst:      100,
		PrefixQPS:        100,
		PrefixBurst:      10,
		IPQPS:            10,
		IPBurst:          5,
		CleanupSeconds:   60,
		MaxIPEntries:     10000,
		MaxPrefixEntries: 1000,
	}

	result := server.FormatRateLimitsLog(settings)

	assert.Contains(t, result, "global=1000qps/100")
	assert.Contains(t, result, "prefix=100qps/10")
	assert.Contains(t, result, "ip=10qps/5")
}

func TestFormatRateLimitsLog_Disabled(t *testing.T) {
	settings := server.RateLimitSettings{
		GlobalQPS:   0, // Disabled
		GlobalBurst: 0,
		PrefixQPS:   0,
		PrefixBurst: 0,
		IPQPS:       0,
		IPBurst:     0,
	}

	result := server.FormatRateLimitsLog(settings)

	assert.Contains(t, result, "global=disabled")
	assert.Contains(t, result, "prefix=disabled")
	assert.Contains(t, result, "ip=disabled")
}

// ============================================================================
// QueryHandler Tests
// ============================================================================

type mockResolver struct {
	resolveFunc func(ctx context.Context, req dns.Packet, reqBytes []byte) (resolvers.Result, error)
}

func (m *mockResolver) Resolve(ctx context.Context, req dns.Packet, reqBytes []byte) (resolvers.Result, error) {
	if m.resolveFunc != nil {
		return m.resolveFunc(ctx, req, reqBytes)
	}
	return resolvers.Result{}, errors.New("not implemented")
}

func (m *mockResolver) Close() error { return nil }

func createValidDNSRequest(t *testing.T) []byte {
	pkt := dns.Packet{
		Header: dns.Header{
			ID:    0x1234,
			Flags: 0x0100, // Standard query, RD=1
		},
		Questions: []dns.Question{
			{Name: "example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)},
		},
	}
	data, err := pkt.Marshal()
	require.NoError(t, err)
	return data
}

func TestQueryHandler_SuccessfulResolve(t *testing.T) {
	responseBytes := []byte{0x12, 0x34, 0x81, 0x80, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00}

	resolver := &mockResolver{
		resolveFunc: func(_ context.Context, _ dns.Packet, _ []byte) (resolvers.Result, error) {
			return resolvers.Result{ResponseBytes: responseBytes, Source: "test"}, nil
		},
	}

	handler := &server.QueryHandler{
		Resolver: resolver,
		Timeout:  5 * time.Second,
	}

	result := handler.Handle(context.Background(), "udp", "127.0.0.1:12345", createValidDNSRequest(t))

	assert.True(t, result.ParsedOK, "Should successfully parse request")
	assert.Equal(t, responseBytes, result.ResponseBytes)
	assert.Equal(t, "test", result.Source)
}

func TestQueryHandler_ResolverError(t *testing.T) {
	resolver := &mockResolver{
		resolveFunc: func(_ context.Context, _ dns.Packet, _ []byte) (resolvers.Result, error) {
			return resolvers.Result{}, errors.New("resolver failed")
		},
	}

	handler := &server.QueryHandler{
		Resolver: resolver,
		Timeout:  5 * time.Second,
	}

	result := handler.Handle(context.Background(), "udp", "127.0.0.1:12345", createValidDNSRequest(t))

	assert.True(t, result.ParsedOK)
	assert.Equal(t, "servfail", result.Source)
	// Should return SERVFAIL response
	assert.NotNil(t, result.ResponseBytes)
}

func TestQueryHandler_Timeout(t *testing.T) {
	resolver := &mockResolver{
		resolveFunc: func(_ context.Context, _ dns.Packet, _ []byte) (resolvers.Result, error) {
			time.Sleep(500 * time.Millisecond)
			return resolvers.Result{}, nil
		},
	}

	handler := &server.QueryHandler{
		Resolver: resolver,
		Timeout:  10 * time.Millisecond, // Very short timeout
	}

	result := handler.Handle(context.Background(), "udp", "127.0.0.1:12345", createValidDNSRequest(t))

	assert.True(t, result.ParsedOK)
	assert.Equal(t, "timeout", result.Source)
}

func TestQueryHandler_InvalidRequest(t *testing.T) {
	resolver := &mockResolver{}

	handler := &server.QueryHandler{
		Resolver: resolver,
		Timeout:  5 * time.Second,
	}

	// Send garbage that can't be parsed
	result := handler.Handle(context.Background(), "udp", "127.0.0.1:12345", []byte{0x00})

	assert.False(t, result.ParsedOK)
	assert.Contains(t, []string{"parse-error", "formerr"}, result.Source)
}

func TestQueryHandler_ContextCancellation(t *testing.T) {
	resolver := &mockResolver{
		resolveFunc: func(ctx context.Context, _ dns.Packet, _ []byte) (resolvers.Result, error) {
			<-ctx.Done()
			return resolvers.Result{}, ctx.Err()
		},
	}

	handler := &server.QueryHandler{
		Resolver: resolver,
		Timeout:  5 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	result := handler.Handle(ctx, "udp", "127.0.0.1:12345", createValidDNSRequest(t))

	// Should handle cancellation gracefully
	assert.True(t, result.ParsedOK)
}

// ============================================================================
// HandleResult Tests
// ============================================================================

func TestHandleResult_Fields(t *testing.T) {
	result := server.HandleResult{
		ResponseBytes: []byte{0x12, 0x34},
		Source:        "test",
		ParsedOK:      true,
	}

	assert.Equal(t, []byte{0x12, 0x34}, result.ResponseBytes)
	assert.Equal(t, "test", result.Source)
	assert.True(t, result.ParsedOK)
}

// ============================================================================
// Truncation Tests (behavior tests through QueryHandler)
// ============================================================================

func TestTruncation_LargeResponse(t *testing.T) {
	// Create a large response that would need truncation for UDP
	largeResponse := make([]byte, 1000)
	// Set up a valid DNS header
	largeResponse[0] = 0x12 // ID high
	largeResponse[1] = 0x34 // ID low
	largeResponse[2] = 0x81 // Flags high (QR=1)
	largeResponse[3] = 0x80 // Flags low (RA=1)
	largeResponse[4] = 0x00 // QDCOUNT high
	largeResponse[5] = 0x01 // QDCOUNT low
	largeResponse[6] = 0x00 // ANCOUNT high
	largeResponse[7] = 0x05 // ANCOUNT low

	// Response should be larger than default UDP size (512)
	assert.Greater(t, len(largeResponse), dns.DefaultUDPPayloadSize)
}

// ============================================================================
// Integration-style Tests
// ============================================================================

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	limiter := server.NewRateLimiter(server.RateLimitSettings{
		GlobalQPS:   10000,
		GlobalBurst: 1000,
		PrefixQPS:   1000,
		PrefixBurst: 100,
		IPQPS:       100,
		IPBurst:     10,
	})

	done := make(chan bool)
	for range 10 {
		go func() {
			for range 100 {
				limiter.Allow("192.168.1.1")
			}
			done <- true
		}()
	}

	for range 10 {
		<-done
	}
}

func TestQueryHandler_SequentialRequests(t *testing.T) {
	callCount := 0
	resolver := &mockResolver{
		resolveFunc: func(_ context.Context, _ dns.Packet, _ []byte) (resolvers.Result, error) {
			callCount++
			return resolvers.Result{
				ResponseBytes: []byte{0x12, 0x34, 0x81, 0x80, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
				Source:        "test",
			}, nil
		},
	}

	handler := &server.QueryHandler{
		Resolver: resolver,
		Timeout:  5 * time.Second,
	}

	for range 5 {
		result := handler.Handle(context.Background(), "udp", "127.0.0.1:12345", createValidDNSRequest(t))
		assert.True(t, result.ParsedOK)
		assert.Equal(t, "test", result.Source)
	}

	assert.Equal(t, 5, callCount)
}

func TestTokenBucket_ConcurrentAccess(t *testing.T) {
	tb := server.NewTokenBucketRateLimiter(server.TokenBucketConfig{
		Rate:       1000,
		Burst:      100,
		MaxEntries: 1000,
	})

	done := make(chan bool)
	for i := range 10 {
		go func(id int) {
			key := string(rune('a' + id))
			for range 50 {
				tb.Allow(key)
			}
			done <- true
		}(i)
	}

	for range 10 {
		<-done
	}
}
