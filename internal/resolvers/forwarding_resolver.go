package resolvers

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/jroosing/hydradns/internal/helpers"
)

// Forwarding resolver configuration constants.
const (
	maxUpstreams             = 3         // Maximum number of upstream servers to use
	upstreamRecoveryDuration = time.Hour // How long to wait before retrying a failed upstream

	// Default configuration values
	DefaultCacheMaxEntries = 20000 // Default maximum cached responses
	DefaultUDPPoolSize     = 256   // Default UDP connections per upstream
	DefaultUDPTimeout      = 3 * time.Second
	DefaultTCPTimeout      = 5 * time.Second
	DefaultMaxRetries      = 3 // Maximum query retries per upstream
)

// ForwardingResolver forwards DNS queries to upstream servers.
//
// Features:
//   - Response caching with TTL-aware expiration
//   - Singleflight deduplication (coalesces concurrent identical queries)
//   - UDP connection pooling for reduced latency
//   - TCP fallback when responses are truncated
//   - Upstream health tracking with automatic failover
//   - EDNS support for larger UDP responses
//   - DNSSEC-aware (preserves DO, AD, CD flags)
//   - Response validation (verifies response matches request)
//
// Caching Strategy:
//
// The cache stores responses with wire-format transaction ID set to 0.
// This allows multiple clients with different transaction IDs to share
// cached responses. The original client transaction ID is patched in
// before returning the response.
//
// Cached entry types:
//   - Positive: Successful responses with answers (respects record TTLs)
//   - NXDOMAIN: Non-existent domain (RFC 2308, 5 minute cache)
//   - NODATA: Name exists but no data for query type (RFC 2308, 5 minute cache)
//   - SERVFAIL: Server error (short cache, 30 seconds)
//
// Singleflight Deduplication:
//
// Multiple concurrent queries for the same question share a single upstream
// request. This prevents thundering herd problems and reduces upstream load
// during cache misses. The response is cached once and shared with all waiters.
//
// TCP Fallback:
//
// If a UDP response has the truncation (TC) bit set and TCP is enabled,
// the query is retried over TCP with the full response. This is essential
// for DNSSEC, large answer sets, and other data that doesn't fit in UDP.
//
// Upstream Health:
//
// Failed upstreams are marked as failed for 1 hour. After that, they are
// automatically tried again. Failover prioritizes upstreams in order.
type ForwardingResolver struct {
	upstreams []string // Upstream server IPs (port is always 53)

	udpTimeout  time.Duration // Timeout for UDP queries
	recvSize    int           // UDP receive buffer size
	tcpFallback bool          // Retry with TCP if UDP response is truncated
	tcpTimeout  time.Duration // Timeout for TCP queries
	maxRetries  int           // Maximum retries per upstream on timeout
	ednsUDPSize int           // Advertised EDNS UDP buffer size
	ednsEnabled bool          // Whether to add EDNS OPT record to queries

	cache *TTLCache[cacheKey, []byte] // Response cache

	// Singleflight: coalesce concurrent queries for the same question
	inflightMu sync.Mutex
	inflight   map[cacheKey]*inflightCall

	// Upstream health tracking
	healthMu         sync.Mutex
	upstreamFailedAt map[string]time.Time

	// UDP connection pool per upstream
	poolMu   sync.Mutex
	udpPools map[string]chan *net.UDPConn
	poolSize int
}

// cacheKey uniquely identifies a cached response.
type cacheKey struct {
	q  QuestionKey // The DNS question
	up string      // Upstream server (for cache isolation during failover)
}

// inflightCall tracks an in-progress query for singleflight deduplication.
type inflightCall struct {
	done chan struct{} // Closed when query completes
	resp []byte        // Response (if successful)
	err  error         // Error (if failed)
}

// NewForwardingResolver creates a ForwardingResolver with the given configuration.
//
// Parameters:
//   - upstreams: List of upstream DNS server IPs (max 3 used)
//   - poolSize: Number of UDP connections to pool per upstream
//   - cacheMaxEntries: Maximum number of cached responses
//   - tcpFallback: Whether to retry with TCP on truncated UDP responses
//   - udpTimeout: Timeout for each UDP query attempt
//   - tcpTimeout: Timeout for TCP queries
//   - maxRetries: Maximum retries per upstream on timeout
func NewForwardingResolver(
	upstreams []string,
	poolSize int,
	cacheMaxEntries int,
	tcpFallback bool,
	udpTimeout, tcpTimeout time.Duration,
	maxRetries int,
) *ForwardingResolver {
	if len(upstreams) == 0 {
		upstreams = []string{"8.8.8.8"}
	}
	if len(upstreams) > maxUpstreams {
		upstreams = upstreams[:maxUpstreams]
	}
	if poolSize <= 0 {
		poolSize = DefaultUDPPoolSize
	}
	if cacheMaxEntries <= 0 {
		cacheMaxEntries = DefaultCacheMaxEntries
	}
	if udpTimeout <= 0 {
		udpTimeout = DefaultUDPTimeout
	}
	if tcpTimeout <= 0 {
		tcpTimeout = DefaultTCPTimeout
	}
	if maxRetries <= 0 {
		maxRetries = DefaultMaxRetries
	}
	return &ForwardingResolver{
		upstreams:        upstreams,
		udpTimeout:       udpTimeout,
		recvSize:         4096,
		tcpFallback:      tcpFallback,
		tcpTimeout:       tcpTimeout,
		maxRetries:       maxRetries,
		ednsUDPSize:      dns.EDNSDefaultUDPPayloadSize,
		ednsEnabled:      true,
		cache:            NewTTLCache[cacheKey, []byte](cacheMaxEntries),
		inflight:         map[cacheKey]*inflightCall{},
		upstreamFailedAt: map[string]time.Time{},
		udpPools:         map[string]chan *net.UDPConn{},
		poolSize:         poolSize,
	}
}

// Close releases all pooled UDP connections.
func (f *ForwardingResolver) Close() error {
	f.poolMu.Lock()
	defer f.poolMu.Unlock()
	for _, ch := range f.udpPools {
		close(ch)
		for c := range ch {
			_ = c.Close()
		}
	}
	f.udpPools = map[string]chan *net.UDPConn{}
	return nil
}

// Resolve forwards a DNS query to an upstream server.
//
// Resolution strategy:
//  1. Check cache for existing response
//  2. Join existing inflight query if one exists (singleflight)
//  3. Query upstream servers with failover
//  4. Cache and return the response
//
// Goroutine lifecycle: No goroutines spawned by this method.
// All network I/O is synchronous and respects context cancellation.
func (f *ForwardingResolver) Resolve(ctx context.Context, req dns.Packet, reqBytes []byte) (Result, error) {
	txid := req.Header.ID
	up := f.selectUpstream()
	key := f.cacheKey(req, up)

	if v, age, ok, _ := f.cache.GetWithAge(key); ok {
		// Adjust TTLs in cached response to account for time spent in cache.
		// The cached bytes contain txid=0, which is irrelevant and gets overwritten
		// by PatchTransactionID to match the client's original txid.
		adjusted := adjustTTLs(v, age)
		return Result{ResponseBytes: PatchTransactionID(adjusted, txid), Source: "upstream-cache"}, nil
	}

	// Check context before starting network operations
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}

	// singleflight
	f.inflightMu.Lock()
	if call := f.inflight[key]; call != nil {
		f.inflightMu.Unlock()
		select {
		case <-call.done:
			if call.err != nil {
				return Result{}, call.err
			}
			return Result{ResponseBytes: PatchTransactionID(call.resp, txid), Source: "upstream-inflight"}, nil
		case <-ctx.Done():
			return Result{}, ctx.Err()
		}
	}
	call := &inflightCall{done: make(chan struct{})}
	f.inflight[key] = call
	f.inflightMu.Unlock()

	resp, err := f.queryAndCache(ctx, key, req, reqBytes)
	call.resp = resp
	call.err = err
	close(call.done)

	f.inflightMu.Lock()
	delete(f.inflight, key)
	f.inflightMu.Unlock()
	if err != nil {
		return Result{}, err
	}
	return Result{ResponseBytes: PatchTransactionID(resp, txid), Source: "upstream"}, nil
}

// queryAndCache queries upstream servers with failover and caches the result.
//
// The method tries each upstream in order, starting from the preferred one.
// On success, it validates the response to prevent cache poisoning, normalizes
// the transaction ID, and stores it in the cache.
func (f *ForwardingResolver) queryAndCache(
	ctx context.Context,
	key cacheKey,
	req dns.Packet,
	reqBytes []byte,
) ([]byte, error) {
	queryBytes := f.prepareQueryBytes(req, reqBytes)

	startIdx := f.findUpstreamIndex(key.up)
	lastErr := error(nil)

	for j := range len(f.upstreams) {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		i := (startIdx + j) % len(f.upstreams)
		u := f.upstreams[i]

		if !f.canTryUpstream(u) {
			continue
		}

		resp, err := f.queryOne(ctx, u, queryBytes)
		if err != nil {
			lastErr = err
			f.markFailed(u)
			continue
		}
		f.markHealthy(u)

		// Validate that the response matches our query to prevent cache poisoning
		if err := validateResponse(req, resp); err != nil {
			return nil, err
		}

		// Normalize transaction ID to 0 for cache storage
		// (actual txid is patched back when returning to client)
		norm := PatchTransactionID(resp, 0)
		f.storeInCache(key, norm)
		return norm, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("no upstream servers available")
}

// prepareQueryBytes normalizes the transaction ID to 0 for upstream reuse
// and ensures EDNS is present (preserving DO flag if client sent it).
// Zeroing the txid ensures cache hits are shared across clients while the
// original client txid is restored by PatchTransactionID before sending
// the response back.
func (f *ForwardingResolver) prepareQueryBytes(req dns.Packet, reqBytes []byte) []byte {
	// Ensure we have space for the txid
	if len(reqBytes) < 2 {
		return reqBytes
	}

	// Copy and zero the txid for cache normalization.
	// The cached response will contain txid=0 in its wire format, but this value
	// is NEVER used - it's a placeholder that gets patched to the client's original
	// txid before returning (via PatchTransactionID).
	out := make([]byte, len(reqBytes))
	copy(out, reqBytes)
	out[0], out[1] = 0, 0

	// Preserve EDNS from client (including DO flag) or add our own
	if f.ednsEnabled {
		// Check if client sent EDNS with DO flag
		clientOPT := dns.ExtractOPT(req.Additionals)
		if clientOPT != nil && clientOPT.DNSSECOk {
			// Client supports DNSSEC - preserve their OPT record as-is
			return out
		}
		// Add EDNS without DO flag (client didn't request DNSSEC)
		return dns.AddEDNSToRequestBytes(req, out, f.ednsUDPSize)
	}

	return out
}

// findUpstreamIndex returns the index of the given upstream server.
func (f *ForwardingResolver) findUpstreamIndex(upstream string) int {
	for i, u := range f.upstreams {
		if u == upstream {
			return i
		}
	}
	return 0
}

// cacheKey generates a cache key from a request and upstream.
// The question is normalized to lowercase for case-insensitive matching.
func (f *ForwardingResolver) cacheKey(req dns.Packet, upstream string) cacheKey {
	q := normalizeQuestionKey(req)
	// Use first upstream for cache key to share cache across failover
	up := f.upstreams[0]
	if upstream != "" {
		up = upstream
	}
	return cacheKey{q: q, up: up}
}

// normalizeQuestionKey extracts a DNS question for caching.
// The question name is already normalized (lowercase) during DNS parsing.
func normalizeQuestionKey(req dns.Packet) QuestionKey {
	if len(req.Questions) == 0 {
		return QuestionKey{}
	}
	return QuestionKey{
		QName:  req.Questions[0].Name,
		QType:  req.Questions[0].Type,
		QClass: req.Questions[0].Class,
	}
}

// canTryUpstream checks if an upstream is healthy or has recovered.
// An upstream is considered recovered after upstreamRecoveryDuration.
func (f *ForwardingResolver) canTryUpstream(up string) bool {
	f.healthMu.Lock()
	defer f.healthMu.Unlock()

	failedAt, ok := f.upstreamFailedAt[up]
	if !ok {
		return true // never failed
	}
	if time.Since(failedAt) >= upstreamRecoveryDuration {
		delete(f.upstreamFailedAt, up)
		return true // recovered
	}
	return false // still in cooldown
}

// selectUpstream returns the best upstream server to use.
// Prefers healthy upstreams in order; if all have failed, clears the failure
// state and returns the first upstream.
func (f *ForwardingResolver) selectUpstream() string {
	for _, u := range f.upstreams {
		if f.canTryUpstream(u) {
			return u
		}
	}

	// All upstreams have failed - clear state and retry from first
	f.healthMu.Lock()
	f.upstreamFailedAt = map[string]time.Time{}
	f.healthMu.Unlock()
	return f.upstreams[0]
}

// markFailed records the current time as the failure timestamp for an upstream.
// Only marks failure once; subsequent failures don't update the timestamp.
func (f *ForwardingResolver) markFailed(up string) {
	f.healthMu.Lock()
	defer f.healthMu.Unlock()
	if _, ok := f.upstreamFailedAt[up]; !ok {
		f.upstreamFailedAt[up] = time.Now()
	}
}

// markHealthy clears the failure state for an upstream.
func (f *ForwardingResolver) markHealthy(up string) {
	f.healthMu.Lock()
	defer f.healthMu.Unlock()
	delete(f.upstreamFailedAt, up)
}

// ensurePool returns or creates the UDP connection pool for an upstream.
// Connections are pre-dialed and stored in a buffered channel.
func (f *ForwardingResolver) ensurePool(up string) (chan *net.UDPConn, error) {
	f.poolMu.Lock()
	if ch, ok := f.udpPools[up]; ok {
		f.poolMu.Unlock()
		return ch, nil
	}
	ch := make(chan *net.UDPConn, f.poolSize)
	f.udpPools[up] = ch
	f.poolMu.Unlock()

	// Pre-dial connections for the pool
	addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(up, "53"))
	if err != nil {
		return nil, err
	}
	for range f.poolSize {
		c, _ := net.DialUDP("udp", nil, addr)
		if c == nil {
			break // partial pool is acceptable
		}
		ch <- c
	}
	return ch, nil
}

// queryOne sends a DNS query to a single upstream with retries.
//
// Connection handling:
//  1. Try to get a pooled connection
//  2. Fall back to creating a transient connection if pool is empty
//  3. Return healthy connections to pool; discard broken ones
//
// If the UDP response is truncated and tcpFallback is enabled,
// automatically retries with TCP. On timeout errors, retries up to
// maxRetries times before giving up.
func (f *ForwardingResolver) queryOne(ctx context.Context, up string, req []byte) ([]byte, error) {
	pool, err := f.ensurePool(up)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for range f.maxRetries {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		resp, err := f.queryOneAttempt(ctx, pool, up, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		// Only retry on timeout errors
		if !isTimeoutError(err) {
			return nil, err
		}
	}
	return nil, lastErr
}

// isTimeoutError checks if an error is a timeout error worth retrying.
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

// queryOneAttempt sends a single query attempt to an upstream server.
func (f *ForwardingResolver) queryOneAttempt(
	ctx context.Context,
	pool chan *net.UDPConn,
	up string,
	req []byte,
) ([]byte, error) {
	c, fromPool, err := f.acquireConnection(ctx, pool, up)
	if err != nil {
		return nil, err
	}

	connOK := true
	defer func() {
		f.releaseConnection(c, pool, fromPool, connOK)
	}()

	// Set deadline from timeout or context, whichever is sooner
	deadline := time.Now().Add(f.udpTimeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = c.SetDeadline(deadline)

	// Send query
	if _, writeErr := c.Write(req); writeErr != nil {
		connOK = false
		return nil, writeErr
	}

	// Receive response with fixed buffer size
	buf := make([]byte, f.recvSize)
	n, err := c.Read(buf)
	if err != nil {
		connOK = false
		return nil, err
	}
	resp := buf[:n:n] // Limit capacity to prevent reuse of buffer tail

	// Retry with TCP if response is truncated
	if f.tcpFallback && dns.IsTruncated(resp) {
		return queryUpstreamTCP(ctx, req, up, f.tcpTimeout)
	}
	return resp, nil
}

// acquireConnection gets a connection from the pool or creates a transient one.
func (f *ForwardingResolver) acquireConnection(
	ctx context.Context,
	pool chan *net.UDPConn,
	up string,
) (*net.UDPConn, bool, error) {
	select {
	case c := <-pool:
		return c, true, nil // pooled connection
	case <-ctx.Done():
		return nil, false, ctx.Err()
	default:
		// Pool empty - create transient connection
		addr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(up, "53"))
		if err != nil {
			return nil, false, err
		}
		c, err := net.DialUDP("udp", nil, addr)
		if err != nil {
			return nil, false, err
		}
		return c, false, nil
	}
}

// releaseConnection returns a connection to the pool or closes it.
func (f *ForwardingResolver) releaseConnection(c *net.UDPConn, pool chan *net.UDPConn, fromPool, connOK bool) {
	if !connOK {
		_ = c.Close()
		return
	}
	if !fromPool {
		_ = c.Close() // transient connections are always closed
		return
	}
	// Best-effort return to pool
	select {
	case pool <- c:
	default:
		_ = c.Close() // pool full
	}
}

// queryUpstreamTCP sends a DNS query over TCP with length-prefix framing.
//
// TCP DNS message format (RFC 1035 section 4.2.2):
//
//	+--+--+
//	|Length| 2 bytes, big-endian message length
//	+--+--+
//	|      |
//	| DNS  | Variable length DNS message
//	|      |
//	+------+
func queryUpstreamTCP(ctx context.Context, req []byte, host string, timeout time.Duration) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, "53"))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Send 2-byte length prefix followed by request
	// Use two writes to avoid allocation from append(prefix, req...)
	var prefix [2]byte
	binary.BigEndian.PutUint16(prefix[:], helpers.ClampIntToUint16(len(req)))
	if _, err := conn.Write(prefix[:]); err != nil {
		return nil, err
	}
	if _, err := conn.Write(req); err != nil {
		return nil, err
	}

	// Read response length
	if _, err := io.ReadFull(conn, prefix[:]); err != nil {
		return nil, err
	}
	respLen := int(binary.BigEndian.Uint16(prefix[:]))
	if respLen <= 0 || respLen > 65535 {
		return nil, fmt.Errorf("TCP response length invalid: %d", respLen)
	}

	// Read response body
	resp := make([]byte, respLen)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// validateResponse checks that the response matches the original request.
// This helps mitigate cache poisoning attacks by verifying:
//   - Response contains a question section
//   - QNAME matches (case-insensitive)
//   - QTYPE matches
//   - QCLASS matches
func validateResponse(req dns.Packet, respBytes []byte) error {
	resp, err := dns.ParsePacket(respBytes)
	if err != nil {
		return fmt.Errorf("failed to parse upstream response: %w", err)
	}
	if len(resp.Questions) == 0 {
		return errors.New("response has no question section")
	}

	reqQ := req.Questions[0]
	resQ := resp.Questions[0]

	// Compare QNAME (case-insensitive, ignore trailing dot)
	if !equalDNSNames(reqQ.Name, resQ.Name) {
		return fmt.Errorf("QNAME mismatch: expected %s, got %s", reqQ.Name, resQ.Name)
	}
	if reqQ.Type != resQ.Type {
		return fmt.Errorf("QTYPE mismatch: expected %d, got %d", reqQ.Type, resQ.Type)
	}
	if reqQ.Class != resQ.Class {
		return fmt.Errorf("QCLASS mismatch: expected %d, got %d", reqQ.Class, resQ.Class)
	}
	return nil
}

// equalDNSNames compares two DNS names case-insensitively, ignoring trailing dots.
func equalDNSNames(a, b string) bool {
	a = strings.TrimSuffix(a, ".")
	b = strings.TrimSuffix(b, ".")
	return strings.EqualFold(a, b)
}

// storeInCache analyzes a response and caches it with appropriate TTL.
// Different response types (positive, NXDOMAIN, NODATA, SERVFAIL) are
// cached with different TTLs based on RFC 2308 guidance.
func (f *ForwardingResolver) storeInCache(key cacheKey, resp []byte) {
	decision := analyzeCacheDecision(resp)

	// Only cache if we have a valid TTL
	if decision.ttlSeconds <= 0 {
		return
	}

	f.cache.Set(key, resp, time.Duration(decision.ttlSeconds)*time.Second, decision.entryType)
}

// cacheDecision contains the result of analyzing a response for caching.
type cacheDecision struct {
	ttlSeconds int            // How long to cache the response
	entryType  CacheEntryType // Type of cache entry (positive, negative, etc.)
}

// analyzeCacheDecision determines caching parameters from a DNS response.
//
// Caching rules (based on RFC 2308):
//   - SERVFAIL: Cache for 30 seconds
//   - NXDOMAIN: Use SOA MINIMUM field, or 300 seconds if no SOA
//   - NODATA (no answers): Use SOA MINIMUM field, or 300 seconds if no SOA
//   - Success: Use minimum TTL from answer records
func analyzeCacheDecision(respBytes []byte) cacheDecision {
	resp, err := dns.ParsePacket(respBytes)
	if err != nil {
		return cacheDecision{ttlSeconds: 0, entryType: CachePositive}
	}

	rcode := dns.RCodeFromFlags(resp.Header.Flags)

	// Handle error responses
	if rcode == dns.RCodeServFail {
		return cacheDecision{ttlSeconds: 30, entryType: CacheSERVFAIL}
	}

	if rcode == dns.RCodeNXDomain {
		ttl := extractSOAMinimum(resp)
		if ttl <= 0 {
			ttl = 300 // default negative cache TTL
		}
		return cacheDecision{ttlSeconds: ttl, entryType: CacheNXDOMAIN}
	}

	if rcode != dns.RCodeNoError {
		return cacheDecision{ttlSeconds: 0, entryType: CachePositive}
	}

	// NODATA: success but no answers
	if len(resp.Answers) == 0 {
		ttl := extractSOAMinimum(resp)
		if ttl <= 0 {
			ttl = 300 // default negative cache TTL
		}
		return cacheDecision{ttlSeconds: ttl, entryType: CacheNODATA}
	}

	// Positive response: use minimum TTL from answers
	minTTL := findMinimumTTL(resp.Answers)
	return cacheDecision{ttlSeconds: minTTL, entryType: CachePositive}
}

// findMinimumTTL returns the smallest non-zero TTL from a list of records.
// Returns 0 if no valid TTLs are found.
func findMinimumTTL(answers []dns.Record) int {
	minTTL := math.MaxInt
	found := false

	for _, a := range answers {
		ttl := a.Header().TTL
		if ttl == 0 {
			continue
		}
		if int(ttl) < minTTL {
			minTTL = int(ttl)
			found = true
		}
	}

	if !found {
		return 0
	}
	return minTTL
}

// extractSOAMinimum extracts the MINIMUM field from a SOA record in the
// authority section. This is used for negative caching (RFC 2308).
//
// SOA RDATA format:
//
//	+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//	/                     MNAME                     /  Primary nameserver
//	+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//	/                     RNAME                     /  Responsible person's mailbox
//	+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//	|                    SERIAL                     |  4 bytes
//	|                    REFRESH                    |  4 bytes
//	|                     RETRY                     |  4 bytes
//	|                    EXPIRE                     |  4 bytes
//	|                   MINIMUM                     |  4 bytes (offset +16 from SERIAL)
//	+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
//
// Returns 0 if no SOA record is found or parsing fails.
func extractSOAMinimum(resp dns.Packet) int {
	for _, r := range resp.Authorities {
		if r.Type() != dns.TypeSOA {
			continue
		}
		opaque, ok := r.(*dns.OpaqueRecord)
		if !ok {
			continue
		}
		b, ok := opaque.Data.([]byte)
		if !ok {
			continue
		}

		// Skip MNAME (primary nameserver name)
		off := 0
		_, err := dns.DecodeName(b, &off)
		if err != nil {
			break
		}

		// Skip RNAME (responsible person's mailbox name)
		_, err = dns.DecodeName(b, &off)
		if err != nil {
			break
		}

		// MINIMUM is at offset +16 from the start of the numeric fields
		// (after SERIAL, REFRESH, RETRY, EXPIRE, each 4 bytes)
		if off+20 <= len(b) {
			minimum := binary.BigEndian.Uint32(b[off+16 : off+20])
			return int(minimum)
		}

		// Fallback: try reading last 4 bytes as MINIMUM
		if len(b) >= 4 {
			minimum := binary.BigEndian.Uint32(b[len(b)-4:])
			return int(minimum)
		}
	}
	return 0
}

// adjustTTLs decrements TTLs in a cached DNS response to reflect time spent in cache.
// Returns a new byte slice with adjusted TTLs. If age >= original TTL, sets TTL to 1.
// Walks the wire format directly without full packet parsing.
func adjustTTLs(respBytes []byte, age time.Duration) []byte {
	if len(respBytes) < dns.HeaderSize || age <= 0 {
		return respBytes
	}

	ageSeconds := uint32(age.Seconds())
	if ageSeconds == 0 {
		return respBytes
	}

	// Copy response bytes for in-place modification
	adjusted := make([]byte, len(respBytes))
	copy(adjusted, respBytes)

	// Read header counts
	qdcount := binary.BigEndian.Uint16(adjusted[4:6])
	ancount := binary.BigEndian.Uint16(adjusted[6:8])
	nscount := binary.BigEndian.Uint16(adjusted[8:10])
	arcount := binary.BigEndian.Uint16(adjusted[10:12])

	off := dns.HeaderSize

	// Skip questions
	for range qdcount {
		_, err := dns.DecodeName(adjusted, &off)
		if err != nil || off+4 > len(adjusted) {
			return respBytes
		}
		off += 4 // QTYPE + QCLASS
	}

	// Adjust TTLs in answers, authorities, and additionals
	totalRecords := int(ancount) + int(nscount) + int(arcount)
	for range totalRecords {
		// Skip NAME
		_, err := dns.DecodeName(adjusted, &off)
		if err != nil || off+10 > len(adjusted) {
			return respBytes
		}

		// Read TYPE
		recordType := binary.BigEndian.Uint16(adjusted[off : off+2])
		off += 4 // TYPE + CLASS

		// Adjust TTL (unless it's an OPT pseudo-record)
		if recordType != uint16(dns.TypeOPT) {
			oldTTL := binary.BigEndian.Uint32(adjusted[off : off+4])
			newTTL := max(uint32(1), oldTTL-ageSeconds)
			binary.BigEndian.PutUint32(adjusted[off:off+4], newTTL)
		}
		off += 4 // TTL

		// Skip RDLENGTH and RDATA
		if off+2 > len(adjusted) {
			return respBytes
		}
		rdlen := int(binary.BigEndian.Uint16(adjusted[off : off+2]))
		off += 2
		if off+rdlen > len(adjusted) {
			return respBytes
		}
		off += rdlen
	}

	return adjusted
}
