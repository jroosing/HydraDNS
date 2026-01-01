package server

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/jroosing/hydradns/internal/resolvers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockResolver implements resolvers.Resolver for testing.
type mockResolver struct {
	response  []byte
	err       error
	delay     time.Duration
	callCount int
}

func (m *mockResolver) Resolve(ctx context.Context, req dns.Packet, reqBytes []byte) (resolvers.Result, error) {
	m.callCount++
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return resolvers.Result{}, ctx.Err()
		}
	}
	if m.err != nil {
		return resolvers.Result{}, m.err
	}
	return resolvers.Result{ResponseBytes: m.response, Source: "mock"}, nil
}

func (m *mockResolver) Close() error { return nil }

// buildTestQuery creates a valid DNS query for testing.
func buildTestQuery(t *testing.T, qname string, qtype dns.RecordType) []byte {
	t.Helper()
	p := dns.Packet{
		Header: dns.Header{ID: 1234, Flags: dns.RDFlag, QDCount: 1},
		Questions: []dns.Question{
			{Name: qname, Type: uint16(qtype), Class: uint16(dns.ClassIN)},
		},
	}
	b, err := p.Marshal()
	require.NoError(t, err, "failed to marshal test query")
	return b
}

// buildTestResponse creates a valid DNS response for testing.
func buildTestResponse(t *testing.T, qname string, qtype dns.RecordType) []byte {
	t.Helper()
	p := dns.Packet{
		Header: dns.Header{ID: 1234, Flags: dns.QRFlag | dns.RDFlag | dns.RAFlag, QDCount: 1, ANCount: 1},
		Questions: []dns.Question{
			{Name: qname, Type: uint16(qtype), Class: uint16(dns.ClassIN)},
		},
		Answers: []dns.Record{
			{Name: qname, Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN), TTL: 300, Data: []byte{192, 0, 2, 1}},
		},
	}
	b, err := p.Marshal()
	require.NoError(t, err, "failed to marshal test response")
	return b
}

func TestQueryHandler_Handle_Success(t *testing.T) {
	qname := "example.com"
	queryBytes := buildTestQuery(t, qname, dns.TypeA)
	responseBytes := buildTestResponse(t, qname, dns.TypeA)

	resolver := &mockResolver{response: responseBytes}
	handler := &QueryHandler{
		Resolver: resolver,
		Timeout:  5 * time.Second,
	}

	result := handler.Handle(context.Background(), "udp", "192.168.1.1:12345", queryBytes)

	assert.True(t, result.ParsedOK, "expected ParsedOK = true")
	assert.Equal(t, "mock", result.Source)
	assert.NotEmpty(t, result.ResponseBytes, "expected non-empty response")
	assert.Equal(t, 1, resolver.callCount, "expected resolver to be called once")
}

func TestQueryHandler_Handle_ParseError(t *testing.T) {
	resolver := &mockResolver{}
	handler := &QueryHandler{
		Resolver: resolver,
		Timeout:  5 * time.Second,
	}

	// Invalid DNS request (too short)
	result := handler.Handle(context.Background(), "udp", "192.168.1.1:12345", []byte{0x00, 0x01})

	assert.False(t, result.ParsedOK, "expected ParsedOK = false for invalid request")
	// Should return parse-error or formerr
	assert.True(t, result.Source == "parse-error" || result.Source == "formerr",
		"expected source 'parse-error' or 'formerr', got %q", result.Source)
	assert.Equal(t, 0, resolver.callCount, "resolver should not be called on parse error")
}

func TestQueryHandler_Handle_ResolverError(t *testing.T) {
	queryBytes := buildTestQuery(t, "example.com", dns.TypeA)

	resolver := &mockResolver{err: errors.New("upstream failure")}
	handler := &QueryHandler{
		Resolver: resolver,
		Timeout:  5 * time.Second,
	}

	result := handler.Handle(context.Background(), "udp", "192.168.1.1:12345", queryBytes)

	assert.True(t, result.ParsedOK, "expected ParsedOK = true (parsing succeeded)")
	assert.Equal(t, "servfail", result.Source)
	// Response should be SERVFAIL
	assert.NotEmpty(t, result.ResponseBytes, "expected SERVFAIL response")
}

func TestQueryHandler_Handle_Timeout(t *testing.T) {
	queryBytes := buildTestQuery(t, "example.com", dns.TypeA)

	resolver := &mockResolver{delay: 500 * time.Millisecond}
	handler := &QueryHandler{
		Resolver: resolver,
		Timeout:  50 * time.Millisecond, // Very short timeout
	}

	result := handler.Handle(context.Background(), "udp", "192.168.1.1:12345", queryBytes)

	assert.True(t, result.ParsedOK, "expected ParsedOK = true")
	assert.Equal(t, "timeout", result.Source)
}

func TestQueryHandler_Handle_ContextCancelled(t *testing.T) {
	queryBytes := buildTestQuery(t, "example.com", dns.TypeA)

	resolver := &mockResolver{delay: 500 * time.Millisecond}
	handler := &QueryHandler{
		Resolver: resolver,
		Timeout:  5 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately
	cancel()

	result := handler.Handle(ctx, "udp", "192.168.1.1:12345", queryBytes)

	assert.Equal(t, "shutdown", result.Source)
}

func TestQueryHandler_Handle_WithLogger(t *testing.T) {
	queryBytes := buildTestQuery(t, "example.com", dns.TypeA)
	responseBytes := buildTestResponse(t, "example.com", dns.TypeA)

	resolver := &mockResolver{response: responseBytes}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	handler := &QueryHandler{
		Logger:   logger,
		Resolver: resolver,
		Timeout:  5 * time.Second,
	}

	result := handler.Handle(context.Background(), "tcp", "10.0.0.1:54321", queryBytes)

	assert.True(t, result.ParsedOK, "expected ParsedOK = true")
}

func TestQueryHandler_Handle_DefaultTimeout(t *testing.T) {
	queryBytes := buildTestQuery(t, "example.com", dns.TypeA)
	responseBytes := buildTestResponse(t, "example.com", dns.TypeA)

	resolver := &mockResolver{response: responseBytes}
	handler := &QueryHandler{
		Resolver: resolver,
		Timeout:  0, // Should default to 4s
	}

	start := time.Now()
	result := handler.Handle(context.Background(), "udp", "192.168.1.1:12345", queryBytes)
	elapsed := time.Since(start)

	assert.True(t, result.ParsedOK, "expected ParsedOK = true")
	// Should complete quickly (mock has no delay)
	assert.Less(t, elapsed, 100*time.Millisecond, "expected quick response")
}

func TestTryBuildErrorFromRaw_ValidHeader(t *testing.T) {
	// Build a valid request with header and question
	queryBytes := buildTestQuery(&testing.T{}, "example.com", dns.TypeA)

	resp := tryBuildErrorFromRaw(queryBytes, uint16(dns.RCodeFormErr))

	require.NotNil(t, resp, "expected non-nil response")
	// Parse and verify it's a FORMERR response
	parsed, err := dns.ParsePacket(resp)
	require.NoError(t, err, "failed to parse error response")

	rcode := parsed.Header.Flags & dns.RCodeMask
	assert.Equal(t, uint16(dns.RCodeFormErr), rcode, "expected RCODE FORMERR")
}

func TestTryBuildErrorFromRaw_TooShort(t *testing.T) {
	// Too short to parse header
	resp := tryBuildErrorFromRaw([]byte{0x00}, uint16(dns.RCodeFormErr))
	assert.Nil(t, resp, "expected nil response for too-short request")
}

func TestTryBuildErrorFromRaw_HeaderOnlyNoQuestion(t *testing.T) {
	// Valid 12-byte header with QDCount=0
	header := []byte{
		0x12, 0x34, // ID
		0x00, 0x00, // Flags
		0x00, 0x00, // QDCount = 0
		0x00, 0x00, // ANCount
		0x00, 0x00, // NSCount
		0x00, 0x00, // ARCount
	}

	resp := tryBuildErrorFromRaw(header, uint16(dns.RCodeServFail))
	require.NotNil(t, resp, "expected non-nil response")
}

func TestExtractQuestionInfo(t *testing.T) {
	tests := []struct {
		name      string
		packet    dns.Packet
		wantQName string
		wantQType int
	}{
		{
			name: "with question",
			packet: dns.Packet{
				Questions: []dns.Question{
					{Name: "test.example.com", Type: uint16(dns.TypeAAAA), Class: uint16(dns.ClassIN)},
				},
			},
			wantQName: "test.example.com",
			wantQType: int(dns.TypeAAAA),
		},
		{
			name:      "no question",
			packet:    dns.Packet{},
			wantQName: "<no-question>",
			wantQType: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qname, qtype := extractQuestionInfo(tt.packet)
			assert.Equal(t, tt.wantQName, qname)
			assert.Equal(t, tt.wantQType, qtype)
		})
	}
}

func TestMustMarshal(t *testing.T) {
	t.Run("valid packet", func(t *testing.T) {
		p := dns.Packet{
			Header: dns.Header{ID: 1234, Flags: dns.QRFlag},
		}
		b := mustMarshal(p)
		assert.NotNil(t, b, "expected non-nil result for valid packet")
	})
}
