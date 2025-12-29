package dns

import (
	"testing"
)

func TestHeaderMarshal(t *testing.T) {
	h := Header{
		ID:      0x1234,
		Flags:   0x8180, // Standard response, no error
		QDCount: 1,
		ANCount: 2,
		NSCount: 3,
		ARCount: 4,
	}

	b, err := h.Marshal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(b) != HeaderSize {
		t.Errorf("expected %d bytes, got %d", HeaderSize, len(b))
	}

	// Verify ID
	if b[0] != 0x12 || b[1] != 0x34 {
		t.Errorf("unexpected ID: %02x%02x", b[0], b[1])
	}

	// Verify Flags
	if b[2] != 0x81 || b[3] != 0x80 {
		t.Errorf("unexpected Flags: %02x%02x", b[2], b[3])
	}

	// Verify counts
	if b[4] != 0 || b[5] != 1 {
		t.Errorf("unexpected QDCount: %d", int(b[4])<<8|int(b[5]))
	}
	if b[6] != 0 || b[7] != 2 {
		t.Errorf("unexpected ANCount: %d", int(b[6])<<8|int(b[7]))
	}
	if b[8] != 0 || b[9] != 3 {
		t.Errorf("unexpected NSCount: %d", int(b[8])<<8|int(b[9]))
	}
	if b[10] != 0 || b[11] != 4 {
		t.Errorf("unexpected ARCount: %d", int(b[10])<<8|int(b[11]))
	}
}

func TestParseHeader(t *testing.T) {
	// Build a header
	msg := []byte{
		0x12, 0x34, // ID
		0x81, 0x80, // Flags (response, no error)
		0x00, 0x01, // QDCount
		0x00, 0x02, // ANCount
		0x00, 0x03, // NSCount
		0x00, 0x04, // ARCount
	}

	off := 0
	h, err := ParseHeader(msg, &off)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if h.ID != 0x1234 {
		t.Errorf("expected ID 0x1234, got 0x%04x", h.ID)
	}
	if h.Flags != 0x8180 {
		t.Errorf("expected Flags 0x8180, got 0x%04x", h.Flags)
	}
	if h.QDCount != 1 {
		t.Errorf("expected QDCount 1, got %d", h.QDCount)
	}
	if h.ANCount != 2 {
		t.Errorf("expected ANCount 2, got %d", h.ANCount)
	}
	if h.NSCount != 3 {
		t.Errorf("expected NSCount 3, got %d", h.NSCount)
	}
	if h.ARCount != 4 {
		t.Errorf("expected ARCount 4, got %d", h.ARCount)
	}
	if off != HeaderSize {
		t.Errorf("expected offset %d, got %d", HeaderSize, off)
	}
}

func TestParseHeaderTooShort(t *testing.T) {
	msg := []byte{0x12, 0x34, 0x81, 0x80} // Only 4 bytes

	off := 0
	_, err := ParseHeader(msg, &off)
	if err == nil {
		t.Error("expected error for too short message")
	}
}

func TestParseHeaderOffset(t *testing.T) {
	// Header at offset 5
	msg := make([]byte, 5+HeaderSize)
	msg[5] = 0xAB
	msg[6] = 0xCD

	off := 5
	h, err := ParseHeader(msg, &off)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.ID != 0xABCD {
		t.Errorf("expected ID 0xABCD, got 0x%04x", h.ID)
	}
	if off != 5+HeaderSize {
		t.Errorf("expected offset %d, got %d", 5+HeaderSize, off)
	}
}

func TestHeaderRoundTrip(t *testing.T) {
	original := Header{
		ID:      0xABCD,
		Flags:   0x0100, // Standard query
		QDCount: 1,
		ANCount: 0,
		NSCount: 0,
		ARCount: 0,
	}

	b, err := original.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	off := 0
	parsed, err := ParseHeader(b, &off)
	if err != nil {
		t.Fatalf("ParseHeader failed: %v", err)
	}

	if parsed != original {
		t.Errorf("round trip failed: got %+v, want %+v", parsed, original)
	}
}
