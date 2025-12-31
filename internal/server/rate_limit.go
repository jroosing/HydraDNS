package server

import (
	"fmt"
	"math"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// This file implements pre-parse admission control using token bucket rate limiting.
//
// Rate limiting is applied at three levels:
//   - Global: Overall server-wide query rate limit
//   - Prefix: Per-network prefix limit (/24 for IPv4, /64 for IPv6)
//   - IP: Per source IP limit
//
// All limits use the token bucket algorithm, which allows short bursts
// while enforcing an average rate over time.

// RateLimiter combines global, prefix, and per-IP rate limiters.
// A request must pass all three levels to be allowed.
type RateLimiter struct {
	global *TokenBucketRateLimiter // Server-wide rate limit
	prefix *TokenBucketRateLimiter // Per network prefix rate limit
	ip     *TokenBucketRateLimiter // Per source IP rate limit
}

// NewRateLimiterFromEnv creates a RateLimiter configured via environment variables.
//
// Environment variables:
//   - HYDRADNS_RL_CLEANUP_SECONDS: Stale entry cleanup interval (default: 60)
//   - HYDRADNS_RL_MAX_IP_ENTRIES: Max tracked IPs (default: 65536)
//   - HYDRADNS_RL_MAX_PREFIX_ENTRIES: Max tracked prefixes (default: 16384)
//   - HYDRADNS_RL_GLOBAL_QPS: Global queries per second (default: 100000)
//   - HYDRADNS_RL_GLOBAL_BURST: Global burst size (default: 100000)
//   - HYDRADNS_RL_PREFIX_QPS: Per-prefix QPS (default: 10000)
//   - HYDRADNS_RL_PREFIX_BURST: Per-prefix burst (default: 20000)
//   - HYDRADNS_RL_IP_QPS: Per-IP QPS (default: 3000)
//   - HYDRADNS_RL_IP_BURST: Per-IP burst (default: 6000)
func NewRateLimiterFromEnv() *RateLimiter {
	cleanupSeconds := envFloat("HYDRADNS_RL_CLEANUP_SECONDS", 60.0)
	maxIP := envInt("HYDRADNS_RL_MAX_IP_ENTRIES", 65_536)
	maxPrefix := envInt("HYDRADNS_RL_MAX_PREFIX_ENTRIES", 16_384)

	globalQPS := envFloat("HYDRADNS_RL_GLOBAL_QPS", 100_000.0)
	globalBurst := envInt("HYDRADNS_RL_GLOBAL_BURST", 100_000)
	prefixQPS := envFloat("HYDRADNS_RL_PREFIX_QPS", 10_000.0)
	prefixBurst := envInt("HYDRADNS_RL_PREFIX_BURST", 20_000)
	ipQPS := envFloat("HYDRADNS_RL_IP_QPS", 3_000)
	ipBurst := envInt("HYDRADNS_RL_IP_BURST", 6_000)

	cleanupInterval := time.Duration(math.Max(0.0, cleanupSeconds) * float64(time.Second))
	if cleanupInterval <= 0 {
		cleanupInterval = 60 * time.Second
	}

	return &RateLimiter{
		global: NewTokenBucketRateLimiter(TokenBucketConfig{Rate: globalQPS, Burst: globalBurst, CleanupInterval: cleanupInterval, MaxEntries: 1}),
		prefix: NewTokenBucketRateLimiter(TokenBucketConfig{Rate: prefixQPS, Burst: prefixBurst, CleanupInterval: cleanupInterval, MaxEntries: maxPrefix}),
		ip:     NewTokenBucketRateLimiter(TokenBucketConfig{Rate: ipQPS, Burst: ipBurst, CleanupInterval: cleanupInterval, MaxEntries: maxIP}),
	}
}

// Allow checks if a request from srcIP should be allowed.
// Returns true if the request passes all rate limit levels.
func (r *RateLimiter) Allow(srcIP string) bool {
	if r == nil {
		return true
	}
	// Check in order: global -> prefix -> IP
	// Fail fast: if global limit is exceeded, don't check others
	if !r.global.Allow("*") {
		return false
	}
	if !r.prefix.Allow(prefixKey(srcIP)) {
		return false
	}
	if !r.ip.Allow(srcIP) {
		return false
	}
	return true
}

// AllowAddr checks if a request from the given netip.Addr should be allowed.
// This is a faster path that avoids string allocation for the IP address.
func (r *RateLimiter) AllowAddr(ip netip.Addr) bool {
	if r == nil {
		return true
	}
	// Check in order: global -> prefix -> IP
	if !r.global.Allow("*") {
		return false
	}
	// For prefix, extract the prefix key without string allocation
	prefixKey := prefixKeyFromAddr(ip)
	if !r.prefix.Allow(prefixKey) {
		return false
	}
	// For IP, use the string representation (unavoidable for map key)
	ipKey := ip.String()
	if !r.ip.Allow(ipKey) {
		return false
	}
	return true
}

// prefixKeyFromAddr returns the prefix key for a netip.Addr.
// Uses /24 for IPv4 and /64 for IPv6.
func prefixKeyFromAddr(ip netip.Addr) string {
	if ip.Is4() {
		prefix, _ := ip.Prefix(24)
		return prefix.String()
	}
	prefix, _ := ip.Prefix(64)
	return prefix.String()
}

// RateLimitsStartupLog returns a human-readable summary of rate limit configuration.
func RateLimitsStartupLog() string {
	cleanupSeconds := envFloat("HYDRADNS_RL_CLEANUP_SECONDS", 60.0)
	maxIP := envInt("HYDRADNS_RL_MAX_IP_ENTRIES", 65_536)
	maxPrefix := envInt("HYDRADNS_RL_MAX_PREFIX_ENTRIES", 16_384)

	globalQPS := envFloat("HYDRADNS_RL_GLOBAL_QPS", 100_000.0)
	globalBurst := envInt("HYDRADNS_RL_GLOBAL_BURST", 100_000)
	prefixQPS := envFloat("HYDRADNS_RL_PREFIX_QPS", 10_000.0)
	prefixBurst := envInt("HYDRADNS_RL_PREFIX_BURST", 20_000)
	ipQPS := envFloat("HYDRADNS_RL_IP_QPS", 3_000.0)
	ipBurst := envInt("HYDRADNS_RL_IP_BURST", 6_000)

	fmtLimiter := func(name string, rate float64, burst int) string {
		if rate <= 0.0 || burst <= 0 {
			return name + "=disabled"
		}
		return fmt.Sprintf("%s=%gqps/%d", name, rate, burst)
	}

	return fmt.Sprintf(
		"%s %s %s cleanup_s=%g max_ip=%d max_prefix=%d",
		fmtLimiter("global", globalQPS, globalBurst),
		fmtLimiter("prefix", prefixQPS, prefixBurst),
		fmtLimiter("ip", ipQPS, ipBurst),
		cleanupSeconds,
		maxIP,
		maxPrefix,
	)
}

// TokenBucketConfig configures a token bucket rate limiter.
type TokenBucketConfig struct {
	Rate            float64       // Tokens replenished per second (queries per second)
	Burst           int           // Maximum tokens (burst capacity)
	CleanupInterval time.Duration // How often to clean up stale entries
	MaxEntries      int           // Maximum tracked keys (prevents memory exhaustion)
}

// TokenBucketRateLimiter implements the token bucket algorithm for rate limiting.
//
// Token bucket algorithm:
//   - Each key (IP, prefix, etc.) has a bucket of tokens
//   - Tokens are replenished at a constant rate (Rate tokens/second)
//   - Each request consumes 1 token
//   - Bucket has a maximum capacity (Burst)
//   - Request is allowed if tokens >= 1, denied otherwise
//
// This allows short bursts up to Burst requests, while limiting
// the long-term average to Rate requests per second.
type TokenBucketRateLimiter struct {
	rate            float64       // Tokens added per second
	burst           float64       // Maximum tokens in bucket
	cleanupInterval time.Duration // Time between stale entry cleanup
	maxEntries      int           // Maximum tracked keys

	mu          sync.Mutex           // Protects all fields below
	lastCleanup time.Time            // When cleanup was last run
	lastUpdate  map[string]time.Time // Last access time per key
	tokens      map[string]float64   // Current token count per key
}

// NewTokenBucketRateLimiter creates a new rate limiter with the given configuration.
func NewTokenBucketRateLimiter(cfg TokenBucketConfig) *TokenBucketRateLimiter {
	maxEntries := cfg.MaxEntries
	if maxEntries <= 0 {
		maxEntries = 1
	}
	ci := cfg.CleanupInterval
	if ci <= 0 {
		ci = 60 * time.Second
	}
	return &TokenBucketRateLimiter{
		rate:            cfg.Rate,
		burst:           float64(cfg.Burst),
		cleanupInterval: ci,
		maxEntries:      maxEntries,
		lastCleanup:     time.Now(),
		lastUpdate:      map[string]time.Time{},
		tokens:          map[string]float64{},
	}
}

// Allow checks if a request for the given key should be allowed.
// Returns true and consumes a token if allowed, false otherwise.
//
// Rate limiting is disabled if rate or burst is <= 0.
func (l *TokenBucketRateLimiter) Allow(key string) bool {
	// Allow disabling by setting rate/burst <= 0
	if l == nil || l.rate <= 0.0 || l.burst <= 0.0 {
		return true
	}

	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	// Periodic cleanup of stale entries
	if now.Sub(l.lastCleanup) > l.cleanupInterval {
		l.cleanupLocked(now)
	}

	// Check if this is a new key
	last, exists := l.lastUpdate[key]
	if !exists {
		// Ensure we don't exceed max entries
		if len(l.lastUpdate) >= l.maxEntries {
			l.cleanupLocked(now)
			if len(l.lastUpdate) >= l.maxEntries {
				// Still at capacity - deny new entries
				if _, ok := l.lastUpdate[key]; !ok {
					return false
				}
			}
		}
		// Initialize new key with full bucket minus 1 token
		l.lastUpdate[key] = now
		l.tokens[key] = l.burst - 1.0
		return true
	}

	// Replenish tokens based on elapsed time
	elapsed := now.Sub(last).Seconds()
	l.lastUpdate[key] = now

	tokens := l.tokens[key]
	if elapsed > 0 {
		// Add tokens for elapsed time, capped at burst
		tokens = math.Min(l.burst, tokens+(elapsed*l.rate))
	}

	// Check if we have tokens available
	if tokens >= 1.0 {
		l.tokens[key] = tokens - 1.0
		return true
	}

	l.tokens[key] = tokens
	return false
}

// cleanupLocked removes entries that haven't been accessed recently.
// Must be called with l.mu held.
func (l *TokenBucketRateLimiter) cleanupLocked(now time.Time) {
	staleBefore := now.Add(-l.cleanupInterval)
	for k, last := range l.lastUpdate {
		if !last.After(staleBefore) {
			delete(l.lastUpdate, k)
			delete(l.tokens, k)
		}
	}
	l.lastCleanup = now
}

// prefixKey converts an IP address to a network prefix key.
// IPv4 addresses are converted to /24 prefixes.
// IPv6 addresses are converted to /64 prefixes.
func prefixKey(ip string) string {
	// Scan once to determine IP type and find dot positions
	var dotPositions [3]int
	dotCount := 0
	hasColon := false

	for i := 0; i < len(ip); i++ {
		switch ip[i] {
		case '.':
			if dotCount < 3 {
				dotPositions[dotCount] = i
				dotCount++
			}
		case ':':
			hasColon = true
		}
	}

	// Fast path for IPv4 (has dots, no colons)
	if dotCount >= 3 && !hasColon {
		// Extract first 3 octets without allocation via Split
		return "v4:" + ip[:dotPositions[2]] + ".0/24"
	}

	// IPv6 handling
	if hasColon {
		addr, err := netip.ParseAddr(ip)
		if err == nil {
			pfx, err := addr.Prefix(64)
			if err == nil {
				return "v6:" + pfx.Masked().Addr().String() + "/64"
			}
		}
		return "v6:" + ip
	}

	// Unknown format
	return "ip:" + ip
}

// envFloat reads a float64 from an environment variable with a default value.
func envFloat(name string, def float64) float64 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return def
	}
	return v
}

// envInt reads an int from an environment variable with a default value.
func envInt(name string, def int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}
