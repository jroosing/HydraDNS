package server

import (
	"testing"

	"github.com/jroosing/hydradns/internal/dns"
)

func TestTruncateUDPResponse_SetsTCAndClearsCounts(t *testing.T) {
	resp := dns.Packet{
		Header:    dns.Header{ID: 1, Flags: uint16(dns.QRFlag)},
		Questions: []dns.Question{{Name: "example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)}},
		Answers:   []dns.Record{{Name: "example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN), TTL: 60, Data: []byte{1, 2, 3, 4}}},
	}
	b, err := resp.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Force truncation, but keep enough room for header+question.
	qEnd := findQuestionSectionEnd(b, 1)
	if qEnd <= 12 {
		t.Fatalf("unexpected question end: %d", qEnd)
	}
	out := truncateUDPResponse(b, qEnd)
	if len(out) > qEnd {
		t.Fatalf("expected <= %d bytes, got %d", qEnd, len(out))
	}

	p, err := dns.ParsePacket(out)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if (p.Header.Flags & uint16(dns.TCFlag)) == 0 {
		t.Fatalf("TC flag not set")
	}
	if p.Header.ANCount != 0 || p.Header.NSCount != 0 || p.Header.ARCount != 0 {
		t.Fatalf("expected counts cleared, got an=%d ns=%d ar=%d", p.Header.ANCount, p.Header.NSCount, p.Header.ARCount)
	}
	if len(p.Questions) != 1 {
		t.Fatalf("expected question preserved")
	}
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
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	truncated := truncateUDPResponse(respBytes, 4096)

	if len(truncated) != len(respBytes) {
		t.Errorf("expected unchanged response, got %d bytes (original %d)", len(truncated), len(respBytes))
	}
}

func TestTruncateUDPResponseZeroMaxSize(t *testing.T) {
	respBytes := make([]byte, 600)
	respBytes[0] = 0x12
	respBytes[1] = 0x34
	respBytes[2] = 0x81
	respBytes[3] = 0x80

	truncated := truncateUDPResponse(respBytes, 0)

	if len(truncated) > dns.DefaultUDPPayloadSize {
		t.Errorf("expected truncation to default size, got %d", len(truncated))
	}
}

func TestTruncateUDPResponseTooShort(t *testing.T) {
	shortResp := []byte{0x12, 0x34, 0x81, 0x80}
	result := truncateUDPResponse(shortResp, 512)

	if len(result) != len(shortResp) {
		t.Errorf("expected unchanged short response, got %d bytes", len(result))
	}
}

func TestExtractQuestionCount(t *testing.T) {
	msg := make([]byte, 12)
	msg[4] = 0x00
	msg[5] = 0x05

	count := extractQuestionCount(msg)
	if count != 5 {
		t.Errorf("expected 5, got %d", count)
	}
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

	if len(header) != dns.HeaderSize {
		t.Errorf("expected %d bytes, got %d", dns.HeaderSize, len(header))
	}

	if header[0] != 0xAB || header[1] != 0xCD {
		t.Error("transaction ID not preserved")
	}

	flags := uint16(header[2])<<8 | uint16(header[3])
	if (flags & dns.TCFlag) == 0 {
		t.Error("expected TC flag to be set")
	}

	qdcount := uint16(header[4])<<8 | uint16(header[5])
	if qdcount != 1 {
		t.Errorf("expected QDCOUNT 1, got %d", qdcount)
	}

	if header[6] != 0 || header[7] != 0 {
		t.Error("expected ANCOUNT = 0")
	}
	if header[8] != 0 || header[9] != 0 {
		t.Error("expected NSCOUNT = 0")
	}
	if header[10] != 0 || header[11] != 0 {
		t.Error("expected ARCOUNT = 0")
	}
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
			if got != tt.wantPos {
				t.Errorf("skipQNAME() = %d, want %d", got, tt.wantPos)
			}
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
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	end := findQuestionSectionEnd(b, 1)

	// End should be after header + encoded name + 4 bytes (type+class)
	if end <= dns.HeaderSize {
		t.Errorf("expected end > %d, got %d", dns.HeaderSize, end)
	}
	if end > len(b) {
		t.Errorf("expected end <= %d, got %d", len(b), end)
	}
}
