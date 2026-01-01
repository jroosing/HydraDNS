// Package resolvers_test provides behavior tests for the resolvers package.
package resolvers_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/jroosing/hydradns/internal/resolvers"
	"github.com/jroosing/hydradns/internal/zone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// PatchTransactionID Tests
// ============================================================================

func TestPatchTransactionID_UpdatesID(t *testing.T) {
	msg := []byte{0x12, 0x34, 0x00, 0x01, 0x00, 0x01}

	result := resolvers.PatchTransactionID(msg, 0xABCD)

	assert.Equal(t, uint16(0xABCD), uint16(result[0])<<8|uint16(result[1]))
	// Original should be unchanged
	assert.Equal(t, byte(0x12), msg[0])
	assert.Equal(t, byte(0x34), msg[1])
}

func TestPatchTransactionID_SameID_NoAllocation(t *testing.T) {
	msg := []byte{0x12, 0x34, 0x00, 0x01}

	result := resolvers.PatchTransactionID(msg, 0x1234)

	// Should return the same slice when ID already matches
	assert.Equal(t, msg, result)
}

func TestPatchTransactionID_ShortMessage(t *testing.T) {
	msg := []byte{0x12} // Too short

	result := resolvers.PatchTransactionID(msg, 0xABCD)

	// Should return original for short messages
	assert.Equal(t, msg, result)
}

func TestPatchTransactionID_EmptyMessage(t *testing.T) {
	msg := []byte{}

	result := resolvers.PatchTransactionID(msg, 0xABCD)

	assert.Equal(t, msg, result)
}

// ============================================================================
// TTLCache Tests
// ============================================================================

func TestTTLCache_SetAndGet(t *testing.T) {
	cache := resolvers.NewTTLCache[string, []byte](100)

	cache.Set("key1", []byte("value1"), time.Minute, resolvers.CachePositive)

	val, found, entryType := cache.Get("key1")
	assert.True(t, found)
	assert.Equal(t, []byte("value1"), val)
	assert.Equal(t, resolvers.CachePositive, entryType)
}

func TestTTLCache_Miss(t *testing.T) {
	cache := resolvers.NewTTLCache[string, []byte](100)

	val, found, _ := cache.Get("nonexistent")
	assert.False(t, found)
	assert.Nil(t, val)
}

func TestTTLCache_Expiration(t *testing.T) {
	cache := resolvers.NewTTLCache[string, []byte](100)

	// Set with very short TTL
	cache.Set("key1", []byte("value1"), time.Millisecond, resolvers.CachePositive)

	// Wait for expiration
	time.Sleep(5 * time.Millisecond)

	_, found, _ := cache.Get("key1")
	assert.False(t, found, "Entry should be expired")
}

func TestTTLCache_LRUEviction(t *testing.T) {
	cache := resolvers.NewTTLCache[int, []byte](3)

	cache.Set(1, []byte("one"), time.Minute, resolvers.CachePositive)
	cache.Set(2, []byte("two"), time.Minute, resolvers.CachePositive)
	cache.Set(3, []byte("three"), time.Minute, resolvers.CachePositive)

	// Access key 1 to make it recently used
	cache.Get(1)

	// Add key 4, should evict key 2 (oldest not recently accessed)
	cache.Set(4, []byte("four"), time.Minute, resolvers.CachePositive)

	_, found1, _ := cache.Get(1)
	_, found2, _ := cache.Get(2)
	_, found3, _ := cache.Get(3)
	_, found4, _ := cache.Get(4)

	assert.True(t, found1, "Key 1 should still exist (recently accessed)")
	assert.False(t, found2, "Key 2 should be evicted (oldest)")
	assert.True(t, found3, "Key 3 should still exist")
	assert.True(t, found4, "Key 4 should exist")
}

func TestTTLCache_NegativeEntries(t *testing.T) {
	cache := resolvers.NewTTLCache[string, []byte](100)

	cache.Set("nxdomain", []byte("nx"), time.Minute, resolvers.CacheNXDOMAIN)
	cache.Set("nodata", []byte("nd"), time.Minute, resolvers.CacheNODATA)
	cache.Set("servfail", []byte("sf"), time.Minute, resolvers.CacheSERVFAIL)

	val, found, entryType := cache.Get("nxdomain")
	assert.True(t, found)
	assert.Equal(t, resolvers.CacheNXDOMAIN, entryType)
	assert.Equal(t, []byte("nx"), val)

	_, found2, entryType2 := cache.Get("nodata")
	assert.True(t, found2)
	assert.Equal(t, resolvers.CacheNODATA, entryType2)

	_, found3, entryType3 := cache.Get("servfail")
	assert.True(t, found3)
	assert.Equal(t, resolvers.CacheSERVFAIL, entryType3)
}

func TestTTLCache_Update(t *testing.T) {
	cache := resolvers.NewTTLCache[string, []byte](100)

	cache.Set("key", []byte("value1"), time.Minute, resolvers.CachePositive)
	cache.Set("key", []byte("value2"), time.Minute, resolvers.CachePositive)

	val, found, _ := cache.Get("key")
	assert.True(t, found)
	assert.Equal(t, []byte("value2"), val)
}

func TestTTLCache_ZeroTTL_NotStored(t *testing.T) {
	cache := resolvers.NewTTLCache[string, []byte](100)

	cache.Set("key", []byte("value"), 0, resolvers.CachePositive)

	_, found, _ := cache.Get("key")
	assert.False(t, found, "Entry with TTL=0 should not be stored")
}

// ============================================================================
// QuestionKey Tests
// ============================================================================

func TestQuestionKey_Equality(t *testing.T) {
	key1 := resolvers.QuestionKey{QName: "example.com", QType: 1, QClass: 1}
	key2 := resolvers.QuestionKey{QName: "example.com", QType: 1, QClass: 1}
	key3 := resolvers.QuestionKey{QName: "other.com", QType: 1, QClass: 1}

	assert.Equal(t, key1, key2)
	assert.NotEqual(t, key1, key3)
}

func TestQuestionKey_DifferentTypes(t *testing.T) {
	keyA := resolvers.QuestionKey{QName: "example.com", QType: uint16(dns.TypeA), QClass: 1}
	keyAAAA := resolvers.QuestionKey{QName: "example.com", QType: uint16(dns.TypeAAAA), QClass: 1}

	assert.NotEqual(t, keyA, keyAAAA)
}

// ============================================================================
// Chained Resolver Tests
// ============================================================================

type mockResolver struct {
	resolveFunc func(ctx context.Context, req dns.Packet, reqBytes []byte) (resolvers.Result, error)
	closeFunc   func() error
}

func (m *mockResolver) Resolve(ctx context.Context, req dns.Packet, reqBytes []byte) (resolvers.Result, error) {
	if m.resolveFunc != nil {
		return m.resolveFunc(ctx, req, reqBytes)
	}
	return resolvers.Result{}, errors.New("not implemented")
}

func (m *mockResolver) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func TestChained_FirstResolverSucceeds(t *testing.T) {
	first := &mockResolver{
		resolveFunc: func(_ context.Context, _ dns.Packet, _ []byte) (resolvers.Result, error) {
			return resolvers.Result{ResponseBytes: []byte("first"), Source: "first"}, nil
		},
	}
	second := &mockResolver{
		resolveFunc: func(_ context.Context, _ dns.Packet, _ []byte) (resolvers.Result, error) {
			return resolvers.Result{ResponseBytes: []byte("second"), Source: "second"}, nil
		},
	}

	chained := &resolvers.Chained{Resolvers: []resolvers.Resolver{first, second}}

	result, err := chained.Resolve(context.Background(), dns.Packet{}, nil)

	require.NoError(t, err)
	assert.Equal(t, []byte("first"), result.ResponseBytes)
	assert.Equal(t, "first", result.Source)
}

func TestChained_FallsBackToSecond(t *testing.T) {
	first := &mockResolver{
		resolveFunc: func(_ context.Context, _ dns.Packet, _ []byte) (resolvers.Result, error) {
			return resolvers.Result{}, errors.New("first failed")
		},
	}
	second := &mockResolver{
		resolveFunc: func(_ context.Context, _ dns.Packet, _ []byte) (resolvers.Result, error) {
			return resolvers.Result{ResponseBytes: []byte("second"), Source: "second"}, nil
		},
	}

	chained := &resolvers.Chained{Resolvers: []resolvers.Resolver{first, second}}

	result, err := chained.Resolve(context.Background(), dns.Packet{}, nil)

	require.NoError(t, err)
	assert.Equal(t, []byte("second"), result.ResponseBytes)
}

func TestChained_AllFail(t *testing.T) {
	first := &mockResolver{
		resolveFunc: func(_ context.Context, _ dns.Packet, _ []byte) (resolvers.Result, error) {
			return resolvers.Result{}, errors.New("first failed")
		},
	}
	second := &mockResolver{
		resolveFunc: func(_ context.Context, _ dns.Packet, _ []byte) (resolvers.Result, error) {
			return resolvers.Result{}, errors.New("second failed")
		},
	}

	chained := &resolvers.Chained{Resolvers: []resolvers.Resolver{first, second}}

	_, err := chained.Resolve(context.Background(), dns.Packet{}, nil)

	assert.Error(t, err)
	assert.Equal(t, "second failed", err.Error())
}

func TestChained_ContextCancellation(t *testing.T) {
	first := &mockResolver{
		resolveFunc: func(_ context.Context, _ dns.Packet, _ []byte) (resolvers.Result, error) {
			return resolvers.Result{}, errors.New("first failed")
		},
	}
	second := &mockResolver{}

	chained := &resolvers.Chained{Resolvers: []resolvers.Resolver{first, second}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := chained.Resolve(ctx, dns.Packet{}, nil)

	assert.ErrorIs(t, err, context.Canceled)
}

func TestChained_Close(t *testing.T) {
	closed := []string{}
	first := &mockResolver{closeFunc: func() error { closed = append(closed, "first"); return nil }}
	second := &mockResolver{closeFunc: func() error { closed = append(closed, "second"); return nil }}

	chained := &resolvers.Chained{Resolvers: []resolvers.Resolver{first, second}}

	err := chained.Close()

	require.NoError(t, err)
	assert.Equal(t, []string{"first", "second"}, closed)
}

func TestChained_Close_ReturnsLastError(t *testing.T) {
	first := &mockResolver{closeFunc: func() error { return errors.New("first error") }}
	second := &mockResolver{closeFunc: func() error { return errors.New("second error") }}

	chained := &resolvers.Chained{Resolvers: []resolvers.Resolver{first, second}}

	err := chained.Close()

	assert.Error(t, err)
	assert.Equal(t, "second error", err.Error())
}

func TestChained_EmptyResolvers(t *testing.T) {
	chained := &resolvers.Chained{Resolvers: []resolvers.Resolver{}}

	_, err := chained.Resolve(context.Background(), dns.Packet{}, nil)

	assert.Error(t, err)
}

// ============================================================================
// ZoneResolver Tests
// ============================================================================

func createTestZone(t *testing.T) *zone.Zone {
	zoneText := `
$ORIGIN example.com.
$TTL 3600
@       IN  SOA   ns1.example.com. admin.example.com. 2024010101 3600 1800 604800 86400
@       IN  NS    ns1.example.com.
@       IN  A     192.0.2.1
www     IN  A     192.0.2.2
mail    IN  A     192.0.2.3
mail    IN  MX    10 mail.example.com.
`
	z, err := zone.ParseText(zoneText)
	require.NoError(t, err)
	return z
}

func TestZoneResolver_AnswersFromZone(t *testing.T) {
	z := createTestZone(t)
	resolver := resolvers.NewZoneResolver([]*zone.Zone{z})

	req := dns.Packet{
		Header: dns.Header{ID: 0x1234},
		Questions: []dns.Question{
			{Name: "www.example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)},
		},
	}

	result, err := resolver.Resolve(context.Background(), req, nil)

	require.NoError(t, err)
	assert.Equal(t, "zone", result.Source)
	assert.NotEmpty(t, result.ResponseBytes)

	// Parse response
	resp, err := dns.ParsePacket(result.ResponseBytes)
	require.NoError(t, err)
	assert.Equal(t, uint16(0x1234), resp.Header.ID)
	assert.True(t, resp.Header.Flags&dns.QRFlag != 0, "Should be a response")
	assert.True(t, resp.Header.Flags&dns.AAFlag != 0, "Should be authoritative")
}

func TestZoneResolver_NXDOMAIN(t *testing.T) {
	z := createTestZone(t)
	resolver := resolvers.NewZoneResolver([]*zone.Zone{z})

	req := dns.Packet{
		Header: dns.Header{ID: 0x5678},
		Questions: []dns.Question{
			{Name: "nonexistent.example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)},
		},
	}

	result, err := resolver.Resolve(context.Background(), req, nil)

	require.NoError(t, err)

	resp, err := dns.ParsePacket(result.ResponseBytes)
	require.NoError(t, err)

	rcode := resp.Header.Flags & dns.RCodeMask
	assert.Equal(t, uint16(dns.RCodeNXDomain), rcode, "Should return NXDOMAIN")
	assert.True(t, resp.Header.Flags&dns.AAFlag != 0, "Should be authoritative")
}

func TestZoneResolver_NameNotInZone(t *testing.T) {
	z := createTestZone(t)
	resolver := resolvers.NewZoneResolver([]*zone.Zone{z})

	req := dns.Packet{
		Header: dns.Header{ID: 0x1234},
		Questions: []dns.Question{
			{Name: "www.other.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)},
		},
	}

	_, err := resolver.Resolve(context.Background(), req, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not in any configured zone")
}

func TestZoneResolver_NoZones(t *testing.T) {
	resolver := resolvers.NewZoneResolver([]*zone.Zone{})

	req := dns.Packet{
		Header: dns.Header{ID: 0x1234},
		Questions: []dns.Question{
			{Name: "example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)},
		},
	}

	_, err := resolver.Resolve(context.Background(), req, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no zones configured")
}

func TestZoneResolver_NoQuestion(t *testing.T) {
	z := createTestZone(t)
	resolver := resolvers.NewZoneResolver([]*zone.Zone{z})

	req := dns.Packet{
		Header:    dns.Header{ID: 0x1234},
		Questions: []dns.Question{},
	}

	_, err := resolver.Resolve(context.Background(), req, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no question")
}

func TestZoneResolver_Close(t *testing.T) {
	z := createTestZone(t)
	resolver := resolvers.NewZoneResolver([]*zone.Zone{z})

	err := resolver.Close()

	assert.NoError(t, err)
}

func TestZoneResolver_MultipleZones(t *testing.T) {
	zone1Text := `
$ORIGIN example.com.
$TTL 3600
@   IN  SOA  ns1.example.com. admin.example.com. 1 3600 1800 604800 86400
@   IN  A    192.0.2.1
`
	zone2Text := `
$ORIGIN other.com.
$TTL 3600
@   IN  SOA  ns1.other.com. admin.other.com. 1 3600 1800 604800 86400
@   IN  A    192.0.2.2
`
	z1, err := zone.ParseText(zone1Text)
	require.NoError(t, err)
	z2, err := zone.ParseText(zone2Text)
	require.NoError(t, err)

	resolver := resolvers.NewZoneResolver([]*zone.Zone{z1, z2})

	// Query first zone
	req1 := dns.Packet{
		Header:    dns.Header{ID: 0x1111},
		Questions: []dns.Question{{Name: "example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)}},
	}
	result1, err := resolver.Resolve(context.Background(), req1, nil)
	require.NoError(t, err)
	assert.Equal(t, "zone", result1.Source)

	// Query second zone
	req2 := dns.Packet{
		Header:    dns.Header{ID: 0x2222},
		Questions: []dns.Question{{Name: "other.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)}},
	}
	result2, err := resolver.Resolve(context.Background(), req2, nil)
	require.NoError(t, err)
	assert.Equal(t, "zone", result2.Source)
}

// ============================================================================
// ForwardingResolver Tests
// ============================================================================

func TestNewForwardingResolver_Defaults(t *testing.T) {
	resolver := resolvers.NewForwardingResolver(
		nil,  // upstreams
		0,    // poolSize
		0,    // cacheMaxEntries
		true, // tcpFallback
		0,    // udpTimeout
		0,    // tcpTimeout
		0,    // maxRetries
	)
	defer resolver.Close()

	// Just verify it was created with defaults
	assert.NotNil(t, resolver)
}

func TestNewForwardingResolver_CustomConfig(t *testing.T) {
	resolver := resolvers.NewForwardingResolver(
		[]string{"1.1.1.1", "8.8.8.8"},
		128,
		10000,
		false,
		2*time.Second,
		4*time.Second,
		2,
	)
	defer resolver.Close()

	assert.NotNil(t, resolver)
}

func TestNewForwardingResolver_TruncatesUpstreams(t *testing.T) {
	resolver := resolvers.NewForwardingResolver(
		[]string{"1.1.1.1", "8.8.8.8", "9.9.9.9", "208.67.222.222", "208.67.220.220"},
		10,
		100,
		true,
		time.Second,
		time.Second,
		1,
	)
	defer resolver.Close()

	assert.NotNil(t, resolver)
}

func TestForwardingResolver_Close(t *testing.T) {
	resolver := resolvers.NewForwardingResolver(
		[]string{"8.8.8.8"},
		10,
		100,
		true,
		time.Second,
		time.Second,
		1,
	)

	err := resolver.Close()
	assert.NoError(t, err)

	// Close again should be safe
	err = resolver.Close()
	assert.NoError(t, err)
}

// ============================================================================
// CacheEntryType Tests
// ============================================================================

func TestCacheEntryType_Values(t *testing.T) {
	assert.Equal(t, resolvers.CacheEntryType(0), resolvers.CachePositive)
	assert.Equal(t, resolvers.CacheEntryType(1), resolvers.CacheNXDOMAIN)
	assert.Equal(t, resolvers.CacheEntryType(2), resolvers.CacheNODATA)
	assert.Equal(t, resolvers.CacheEntryType(3), resolvers.CacheSERVFAIL)
}

// ============================================================================
// Result Tests
// ============================================================================

func TestResult_Fields(t *testing.T) {
	result := resolvers.Result{
		ResponseBytes: []byte{0x12, 0x34},
		Source:        "test",
	}

	assert.Equal(t, []byte{0x12, 0x34}, result.ResponseBytes)
	assert.Equal(t, "test", result.Source)
}

// ============================================================================
// Integration-style Tests
// ============================================================================

func TestZoneResolver_PreservesRDFlag(t *testing.T) {
	z := createTestZone(t)
	resolver := resolvers.NewZoneResolver([]*zone.Zone{z})

	// Request with RD flag set
	req := dns.Packet{
		Header: dns.Header{
			ID:    0x1234,
			Flags: dns.RDFlag,
		},
		Questions: []dns.Question{
			{Name: "www.example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)},
		},
	}

	result, err := resolver.Resolve(context.Background(), req, nil)
	require.NoError(t, err)

	resp, err := dns.ParsePacket(result.ResponseBytes)
	require.NoError(t, err)

	// RD flag should be preserved
	assert.True(t, resp.Header.Flags&dns.RDFlag != 0, "RD flag should be preserved")
}

func TestChained_ZoneThenForwarding(t *testing.T) {
	z := createTestZone(t)
	zoneResolver := resolvers.NewZoneResolver([]*zone.Zone{z})

	// ForwardingResolver that we won't actually use (zone should answer first)
	forwardingResolver := resolvers.NewForwardingResolver(
		[]string{"8.8.8.8"},
		10,
		100,
		true,
		time.Second,
		time.Second,
		1,
	)

	chained := &resolvers.Chained{
		Resolvers: []resolvers.Resolver{zoneResolver, forwardingResolver},
	}
	defer chained.Close()

	// Query for something in the zone
	req := dns.Packet{
		Header: dns.Header{ID: 0x1234},
		Questions: []dns.Question{
			{Name: "www.example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)},
		},
	}

	result, err := chained.Resolve(context.Background(), req, nil)

	require.NoError(t, err)
	assert.Equal(t, "zone", result.Source, "Should be answered by zone resolver")
}
