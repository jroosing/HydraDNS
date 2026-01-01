package resolvers

import (
	"context"
	"testing"
	"time"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeCacheDecision_PositiveMinTTL(t *testing.T) {
	resp := dns.Packet{
		Header:    dns.Header{ID: 0, Flags: uint16(dns.QRFlag)},
		Questions: []dns.Question{{Name: "example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)}},
		Answers: []dns.Record{
			{
				Name:  "example.com",
				Type:  uint16(dns.TypeA),
				Class: uint16(dns.ClassIN),
				TTL:   10,
				Data:  []byte{1, 2, 3, 4},
			},
		},
	}
	b, err := resp.Marshal()
	require.NoError(t, err)
	d := analyzeCacheDecision(b)
	assert.Equal(t, 10, d.ttlSeconds)
	assert.Equal(t, CachePositive, d.entryType)
}

func TestAnalyzeCacheDecision_MultipleAnswers(t *testing.T) {
	resp := dns.Packet{
		Header:    dns.Header{ID: 0, Flags: uint16(dns.QRFlag)},
		Questions: []dns.Question{{Name: "example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)}},
		Answers: []dns.Record{
			{
				Name:  "example.com",
				Type:  uint16(dns.TypeA),
				Class: uint16(dns.ClassIN),
				TTL:   300,
				Data:  []byte{1, 2, 3, 4},
			},
			{
				Name:  "example.com",
				Type:  uint16(dns.TypeA),
				Class: uint16(dns.ClassIN),
				TTL:   100,
				Data:  []byte{5, 6, 7, 8},
			}, // smallest
			{
				Name:  "example.com",
				Type:  uint16(dns.TypeA),
				Class: uint16(dns.ClassIN),
				TTL:   200,
				Data:  []byte{9, 10, 11, 12},
			},
		},
	}
	b, err := resp.Marshal()
	require.NoError(t, err)
	d := analyzeCacheDecision(b)
	assert.Equal(t, 100, d.ttlSeconds)
	assert.Equal(t, CachePositive, d.entryType)
}

func TestAnalyzeCacheDecision_NXDomain(t *testing.T) {
	nxdomainFlags := uint16(dns.QRFlag) | uint16(dns.RCodeNXDomain)
	resp := dns.Packet{
		Header: dns.Header{ID: 0, Flags: nxdomainFlags},
		Questions: []dns.Question{
			{Name: "nonexistent.example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)},
		},
	}
	b, err := resp.Marshal()
	require.NoError(t, err)
	d := analyzeCacheDecision(b)
	assert.Equal(t, CacheNXDOMAIN, d.entryType)
	assert.Equal(t, 300, d.ttlSeconds, "expected default TTL when no SOA")
}

func TestAnalyzeCacheDecision_ServFail(t *testing.T) {
	servfailFlags := uint16(dns.QRFlag) | uint16(dns.RCodeServFail)
	resp := dns.Packet{
		Header:    dns.Header{ID: 0, Flags: servfailFlags},
		Questions: []dns.Question{{Name: "example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)}},
	}
	b, err := resp.Marshal()
	require.NoError(t, err)
	d := analyzeCacheDecision(b)
	assert.Equal(t, CacheSERVFAIL, d.entryType)
	assert.Equal(t, 30, d.ttlSeconds)
}

func TestAnalyzeCacheDecision_NoData(t *testing.T) {
	// NODATA: success response (RCODE=0) but no answers
	resp := dns.Packet{
		Header:    dns.Header{ID: 0, Flags: uint16(dns.QRFlag)},
		Questions: []dns.Question{{Name: "example.com", Type: uint16(dns.TypeAAAA), Class: uint16(dns.ClassIN)}},
		Answers:   nil, // no answers
	}
	b, err := resp.Marshal()
	require.NoError(t, err)
	d := analyzeCacheDecision(b)
	assert.Equal(t, CacheNODATA, d.entryType)
	assert.Equal(t, 300, d.ttlSeconds, "expected default TTL when no SOA")
}

func TestAnalyzeCacheDecision_InvalidResponse(t *testing.T) {
	d := analyzeCacheDecision([]byte{0, 1, 2}) // too short to parse
	assert.Zero(t, d.ttlSeconds, "expected TTL=0 for invalid response")
}

func TestFindMinimumTTL(t *testing.T) {
	tests := []struct {
		name     string
		answers  []dns.Record
		expected int
	}{
		{
			name:     "empty answers",
			answers:  nil,
			expected: 0,
		},
		{
			name: "single record",
			answers: []dns.Record{
				{TTL: 120},
			},
			expected: 120,
		},
		{
			name: "multiple records find minimum",
			answers: []dns.Record{
				{TTL: 300},
				{TTL: 60},
				{TTL: 120},
			},
			expected: 60,
		},
		{
			name: "skip zero TTL",
			answers: []dns.Record{
				{TTL: 0},
				{TTL: 100},
			},
			expected: 100,
		},
		{
			name: "all zero TTL",
			answers: []dns.Record{
				{TTL: 0},
				{TTL: 0},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findMinimumTTL(tt.answers)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestNewForwardingResolver_Defaults(t *testing.T) {
	// Empty upstreams should default to 8.8.8.8
	fr := NewForwardingResolver(nil, 0, 0, false, 0, 0, 0)
	defer fr.Close()

	assert.Len(t, fr.upstreams, 1)
	assert.Equal(t, "8.8.8.8", fr.upstreams[0])
	assert.Equal(t, 256, fr.poolSize)
	assert.Equal(t, 3, fr.maxRetries)
	assert.Equal(t, 3*time.Second, fr.udpTimeout)
	assert.Equal(t, 5*time.Second, fr.tcpTimeout)
}

func TestNewForwardingResolver_MaxUpstreams(t *testing.T) {
	// More than 3 upstreams should be capped
	upstreams := []string{"1.1.1.1", "8.8.8.8", "9.9.9.9", "208.67.222.222", "208.67.220.220"}
	fr := NewForwardingResolver(upstreams, 10, 100, true, time.Second, time.Second, 2)
	defer fr.Close()

	assert.Len(t, fr.upstreams, 3, "expected max 3 upstreams")
}

func TestNewForwardingResolver_CustomValues(t *testing.T) {
	fr := NewForwardingResolver(
		[]string{"1.1.1.1"},
		128,            // poolSize
		5000,           // cacheMaxEntries
		true,           // tcpFallback
		2*time.Second,  // udpTimeout
		10*time.Second, // tcpTimeout
		5,              // maxRetries
	)
	defer fr.Close()

	assert.Equal(t, 128, fr.poolSize)
	assert.Equal(t, 2*time.Second, fr.udpTimeout)
	assert.Equal(t, 10*time.Second, fr.tcpTimeout)
	assert.Equal(t, 5, fr.maxRetries)
	assert.True(t, fr.tcpFallback)
}

func TestForwardingResolver_Close(t *testing.T) {
	fr := NewForwardingResolver([]string{"1.1.1.1"}, 2, 100, false, time.Second, time.Second, 1)

	// Trigger pool creation by calling ensurePool
	_, err := fr.ensurePool("1.1.1.1")
	require.NoError(t, err)

	// Close should clean up pools
	require.NoError(t, fr.Close())

	// Verify pools are empty
	fr.poolMu.Lock()
	poolLen := len(fr.udpPools)
	fr.poolMu.Unlock()

	assert.Zero(t, poolLen, "expected empty pools after Close")
}

func TestForwardingResolver_CacheKey(t *testing.T) {
	fr := NewForwardingResolver([]string{"1.1.1.1", "8.8.8.8"}, 1, 100, false, time.Second, time.Second, 1)
	defer fr.Close()

	req := dns.Packet{
		Questions: []dns.Question{
			{Name: "Example.COM", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)},
		},
	}

	key := fr.cacheKey(req, "1.1.1.1")

	// Name should be lowercased
	assert.Equal(t, "example.com", key.q.QName, "expected lowercased qname")
	assert.Equal(t, uint16(dns.TypeA), key.q.QType)
}

func TestForwardingResolver_CacheKeyEmptyQuestions(t *testing.T) {
	fr := NewForwardingResolver([]string{"1.1.1.1"}, 1, 100, false, time.Second, time.Second, 1)
	defer fr.Close()

	req := dns.Packet{
		Questions: nil,
	}

	key := fr.cacheKey(req, "")

	assert.Empty(t, key.q.QName, "expected empty qname for no questions")
}

func TestSelectUpstream_AllHealthy(t *testing.T) {
	fr := NewForwardingResolver([]string{"1.1.1.1", "8.8.8.8"}, 1, 100, false, time.Second, time.Second, 1)
	defer fr.Close()

	// When all are healthy, should return first
	selected := fr.selectUpstream()
	assert.Equal(t, "1.1.1.1", selected)
}

func TestSelectUpstream_FirstFailed(t *testing.T) {
	fr := NewForwardingResolver([]string{"1.1.1.1", "8.8.8.8"}, 1, 100, false, time.Second, time.Second, 1)
	defer fr.Close()

	// Mark first upstream as failed
	fr.markFailed("1.1.1.1")

	// Should select second upstream
	selected := fr.selectUpstream()
	assert.Equal(t, "8.8.8.8", selected, "expected second upstream after first failed")
}

func TestSelectUpstream_AllFailed(t *testing.T) {
	fr := NewForwardingResolver([]string{"1.1.1.1", "8.8.8.8"}, 1, 100, false, time.Second, time.Second, 1)
	defer fr.Close()

	// Mark all upstreams as failed
	fr.markFailed("1.1.1.1")
	fr.markFailed("8.8.8.8")

	// When all failed, should reset and return first
	selected := fr.selectUpstream()
	assert.Equal(t, "1.1.1.1", selected, "expected first upstream after all failed reset")
}

func TestCanTryUpstream_NeverFailed(t *testing.T) {
	fr := NewForwardingResolver([]string{"1.1.1.1"}, 1, 100, false, time.Second, time.Second, 1)
	defer fr.Close()

	assert.True(t, fr.canTryUpstream("1.1.1.1"), "expected canTryUpstream true for never-failed upstream")
}

func TestCanTryUpstream_RecentlyFailed(t *testing.T) {
	fr := NewForwardingResolver([]string{"1.1.1.1"}, 1, 100, false, time.Second, time.Second, 1)
	defer fr.Close()

	fr.markFailed("1.1.1.1")

	assert.False(t, fr.canTryUpstream("1.1.1.1"), "expected canTryUpstream false for recently-failed upstream")
}

func TestMarkHealthy_ClearsFailure(t *testing.T) {
	fr := NewForwardingResolver([]string{"1.1.1.1"}, 1, 100, false, time.Second, time.Second, 1)
	defer fr.Close()

	fr.markFailed("1.1.1.1")
	assert.False(t, fr.canTryUpstream("1.1.1.1"), "should be marked as failed")

	fr.markHealthy("1.1.1.1")
	assert.True(t, fr.canTryUpstream("1.1.1.1"), "should be healthy after markHealthy")
}

func TestFindUpstreamIndex(t *testing.T) {
	fr := NewForwardingResolver([]string{"1.1.1.1", "8.8.8.8", "9.9.9.9"}, 1, 100, false, time.Second, time.Second, 1)
	defer fr.Close()

	assert.Equal(t, 1, fr.findUpstreamIndex("8.8.8.8"))
	assert.Equal(t, 0, fr.findUpstreamIndex("unknown"), "expected index 0 for unknown")
}

func TestValidateResponse(t *testing.T) {
	req := dns.Packet{
		Questions: []dns.Question{
			{Name: "example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)},
		},
	}

	tests := []struct {
		name        string
		resp        dns.Packet
		expectError bool
	}{
		{
			name: "matching response",
			resp: dns.Packet{
				Header:    dns.Header{Flags: uint16(dns.QRFlag)},
				Questions: []dns.Question{{Name: "example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)}},
				Answers: []dns.Record{
					{Name: "example.com", Type: uint16(dns.TypeA), TTL: 300, Data: []byte{1, 2, 3, 4}},
				},
			},
			expectError: false,
		},
		{
			name: "case insensitive match",
			resp: dns.Packet{
				Header:    dns.Header{Flags: uint16(dns.QRFlag)},
				Questions: []dns.Question{{Name: "EXAMPLE.COM", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)}},
			},
			expectError: false,
		},
		{
			name: "trailing dot ignored",
			resp: dns.Packet{
				Header:    dns.Header{Flags: uint16(dns.QRFlag)},
				Questions: []dns.Question{{Name: "example.com.", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)}},
			},
			expectError: false,
		},
		{
			name: "qname mismatch",
			resp: dns.Packet{
				Header:    dns.Header{Flags: uint16(dns.QRFlag)},
				Questions: []dns.Question{{Name: "other.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)}},
			},
			expectError: true,
		},
		{
			name: "qtype mismatch",
			resp: dns.Packet{
				Header: dns.Header{Flags: uint16(dns.QRFlag)},
				Questions: []dns.Question{
					{Name: "example.com", Type: uint16(dns.TypeAAAA), Class: uint16(dns.ClassIN)},
				},
			},
			expectError: true,
		},
		{
			name: "qclass mismatch",
			resp: dns.Packet{
				Header:    dns.Header{Flags: uint16(dns.QRFlag)},
				Questions: []dns.Question{{Name: "example.com", Type: uint16(dns.TypeA), Class: 3}}, // CH class
			},
			expectError: true,
		},
		{
			name: "no question in response",
			resp: dns.Packet{
				Header:    dns.Header{Flags: uint16(dns.QRFlag)},
				Questions: nil,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			respBytes, err := tt.resp.Marshal()
			require.NoError(t, err, "failed to marshal response")

			err = validateResponse(req, respBytes)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateResponse_NoRequestQuestions(t *testing.T) {
	req := dns.Packet{
		Questions: nil, // no questions in request
	}
	resp := dns.Packet{
		Header:    dns.Header{Flags: uint16(dns.QRFlag)},
		Questions: []dns.Question{{Name: "example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)}},
	}
	respBytes, _ := resp.Marshal()

	// Should pass validation when request has no questions
	err := validateResponse(req, respBytes)
	assert.NoError(t, err, "expected no error when request has no questions")
}

func TestEqualDNSNames(t *testing.T) {
	tests := []struct {
		a, b   string
		expect bool
	}{
		{"example.com", "example.com", true},
		{"example.com", "EXAMPLE.COM", true},
		{"example.com.", "example.com", true},
		{"example.com.", "example.com.", true},
		{"example.com", "other.com", false},
		{"", "", true},
		{"a", "b", false},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := equalDNSNames(tt.a, tt.b)
			assert.Equal(t, tt.expect, got)
		})
	}
}

func TestPatchTransactionID(t *testing.T) {
	original := []byte{0x12, 0x34, 0x00, 0x00, 0x00, 0x01} // ID=0x1234

	patched := PatchTransactionID(original, 0xABCD)

	// Original should be unchanged
	assert.Equal(t, byte(0x12), original[0])
	assert.Equal(t, byte(0x34), original[1])

	// Patched should have new ID
	assert.Equal(t, byte(0xAB), patched[0])
	assert.Equal(t, byte(0xCD), patched[1])

	// Rest should be same
	assert.Equal(t, byte(0x00), patched[2])
	assert.Equal(t, byte(0x00), patched[3])
}

func TestPatchTransactionID_ShortPacket(t *testing.T) {
	short := []byte{0x12} // too short
	patched := PatchTransactionID(short, 0xABCD)

	// Should return as-is
	assert.Len(t, patched, 1)
	assert.Equal(t, byte(0x12), patched[0])
}

func TestForwardingResolver_ResolveContextCancelled(t *testing.T) {
	fr := NewForwardingResolver([]string{"1.1.1.1"}, 1, 100, false, time.Second, time.Second, 1)
	defer fr.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	req := dns.Packet{
		Header: dns.Header{ID: 1234},
		Questions: []dns.Question{
			{Name: "example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)},
		},
	}

	_, err := fr.Resolve(ctx, req, nil)
	assert.Equal(t, context.Canceled, err)
}

func TestPrepareQueryBytes_EDNSEnabled(t *testing.T) {
	fr := NewForwardingResolver([]string{"1.1.1.1"}, 1, 100, false, time.Second, time.Second, 1)
	defer fr.Close()

	// EDNS is enabled by default
	require.True(t, fr.ednsEnabled, "EDNS should be enabled by default")

	req := dns.Packet{
		Header: dns.Header{ID: 1234},
		Questions: []dns.Question{
			{Name: "example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)},
		},
	}
	reqBytes, _ := req.Marshal()

	prepared := fr.prepareQueryBytes(req, reqBytes)

	// Prepared should be longer (EDNS OPT record added)
	assert.Greater(t, len(prepared), len(reqBytes), "expected EDNS to add bytes to query")
}

func TestIsTimeoutError(t *testing.T) {
	assert.False(t, isTimeoutError(nil), "nil error should not be timeout")

	// Create a mock timeout error
	type mockTimeoutErr struct {
		error
	}
	m := mockTimeoutErr{}
	// Regular errors are not timeouts
	assert.False(t, isTimeoutError(m), "regular error should not be timeout")
}
