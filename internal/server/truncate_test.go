package server

import (
	"testing"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTruncateUDPResponse_SetsTCAndClearsCounts(t *testing.T) {
	resp := dns.Packet{
		Header:    dns.Header{ID: 1, Flags: uint16(dns.QRFlag)},
		Questions: []dns.Question{{Name: "example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)}},
		Answers:   []dns.Record{{Name: "example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN), TTL: 60, Data: []byte{1, 2, 3, 4}}},
	}
	b, err := resp.Marshal()
	require.NoError(t, err, "marshal failed")

	// Force truncation, but keep enough room for header+question.
	qEnd := findQuestionSectionEnd(b, 1)
	require.Greater(t, qEnd, 12, "unexpected question end")

	out := truncateUDPResponse(b, qEnd)
	require.LessOrEqual(t, len(out), qEnd, "expected <= %d bytes", qEnd)

	p, err := dns.ParsePacket(out)
	require.NoError(t, err, "parse failed")
	assert.NotZero(t, p.Header.Flags&uint16(dns.TCFlag), "TC flag not set")
	assert.Equal(t, uint16(0), p.Header.ANCount, "expected ANCount cleared")
	assert.Equal(t, uint16(0), p.Header.NSCount, "expected NSCount cleared")
	assert.Equal(t, uint16(0), p.Header.ARCount, "expected ARCount cleared")
	assert.Len(t, p.Questions, 1, "expected question preserved")
}

func TestTruncateUDPResponseSmallEnough(t *testing.T) {
	pkt := dns.Packet{
		Header: dns.Header{
			ID:      0x1234,
			Flags:   0x8180,
			QDCount: 1,
			ANCount: 1,
		},
		Questions: []dns.Question{
			{Name: "example.com", Type: uint16(dns.TypeA), Class: 1},
		},
		Answers: []dns.Record{
			{Name: "example.com", Type: uint16(dns.TypeA), Class: 1, TTL: 300, Data: []byte{1, 2, 3, 4}},
		},
	}

	respBytes, err := pkt.Marshal()
	require.NoError(t, err, "Marshal failed")

	truncated := truncateUDPResponse(respBytes, 4096)
	assert.Equal(t, len(respBytes), len(truncated), "expected unchanged response")
}

func TestTruncateUDPResponseZeroMaxSize(t *testing.T) {
	respBytes := make([]byte, 600)
	respBytes[0] = 0x12
	respBytes[1] = 0x34
	respBytes[2] = 0x81
	respBytes[3] = 0x80

	truncated := truncateUDPResponse(respBytes, 0)
	assert.LessOrEqual(t, len(truncated), dns.DefaultUDPPayloadSize, "expected truncation to default size")
}

func TestTruncateUDPResponseTooShort(t *testing.T) {
	shortResp := []byte{0x12, 0x34, 0x81, 0x80}
	result := truncateUDPResponse(shortResp, 512)
	assert.Equal(t, len(shortResp), len(result), "expected unchanged short response")
}

func TestExtractQuestionCount(t *testing.T) {
	msg := make([]byte, 12)
	msg[4] = 0x00
	msg[5] = 0x05

	count := extractQuestionCount(msg)
	assert.Equal(t, uint16(5), count)
}

func TestBuildTruncatedHeader(t *testing.T) {
	original := make([]byte, 12)
	original[0] = 0xAB
	original[1] = 0xCD
	original[2] = 0x81
	original[3] = 0x00
	original[4] = 0x00
	original[5] = 0x01
	original[6] = 0x00
	original[7] = 0x05

	header := buildTruncatedHeader(original, 1)

	require.Len(t, header, dns.HeaderSize)

	// Transaction ID preserved
	assert.Equal(t, byte(0xAB), header[0], "transaction ID byte 0 not preserved")
	assert.Equal(t, byte(0xCD), header[1], "transaction ID byte 1 not preserved")

	// TC flag set
	flags := uint16(header[2])<<8 | uint16(header[3])
	assert.NotZero(t, flags&dns.TCFlag, "expected TC flag to be set")

	// QDCOUNT preserved
	qdcount := uint16(header[4])<<8 | uint16(header[5])
	assert.Equal(t, uint16(1), qdcount, "expected QDCOUNT 1")

	// Other counts cleared
	assert.Equal(t, byte(0), header[6], "expected ANCOUNT high byte = 0")
	assert.Equal(t, byte(0), header[7], "expected ANCOUNT low byte = 0")
	assert.Equal(t, byte(0), header[8], "expected NSCOUNT high byte = 0")
	assert.Equal(t, byte(0), header[9], "expected NSCOUNT low byte = 0")
	assert.Equal(t, byte(0), header[10], "expected ARCOUNT high byte = 0")
	assert.Equal(t, byte(0), header[11], "expected ARCOUNT low byte = 0")
}

func TestSkipQNAME(t *testing.T) {
	tests := []struct {
		name     string
		msg      []byte
		startPos int
		wantPos  int
	}{
		{
			name:     "simple name",
			msg:      []byte{3, 'w', 'w', 'w', 7, 'e', 'x', 'a', 'm', 'p', 'l', 'e', 3, 'c', 'o', 'm', 0},
			startPos: 0,
			wantPos:  17,
		},
		{
			name:     "root name",
			msg:      []byte{0},
			startPos: 0,
			wantPos:  1,
		},
		{
			name:     "compression pointer",
			msg:      []byte{3, 'w', 'w', 'w', 0xC0, 0x0A},
			startPos: 0,
			wantPos:  6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := skipQNAME(tt.msg, tt.startPos)
			assert.Equal(t, tt.wantPos, got)
		})
	}
}

func TestFindQuestionSectionEnd(t *testing.T) {
	// Build a packet with one question
	pkt := dns.Packet{
		Header: dns.Header{
			ID:      0x1234,
			Flags:   0x0100,
			QDCount: 1,
		},
		Questions: []dns.Question{
			{Name: "example.com", Type: uint16(dns.TypeA), Class: 1},
		},
	}

	b, err := pkt.Marshal()
	require.NoError(t, err, "Marshal failed")

	end := findQuestionSectionEnd(b, 1)

	// End should be after header + encoded name + 4 bytes (type+class)
	assert.Greater(t, end, dns.HeaderSize, "expected end > HeaderSize")
	assert.LessOrEqual(t, end, len(b), "expected end <= message length")
}
