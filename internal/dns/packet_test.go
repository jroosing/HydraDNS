package dns

import (
	"testing"
)

func TestPacketMarshal(t *testing.T) {
	pkt := Packet{
		Header: Header{
			ID:      0x1234,
			Flags:   0x0100, // Standard query
			QDCount: 1,
			ANCount: 0,
			NSCount: 0,
			ARCount: 0,
		},
		Questions: []Question{
			{Name: "example.com", Type: uint16(TypeA), Class: 1},
		},
	}

	b, err := pkt.Marshal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Minimum: 12 (header) + encoded name + 4 (type/class)
	if len(b) < 12 {
		t.Errorf("packet too short: %d bytes", len(b))
	}

	// Verify header ID
	if b[0] != 0x12 || b[1] != 0x34 {
		t.Errorf("unexpected ID in wire format")
	}
}

func TestPacketMarshalWithAnswers(t *testing.T) {
	pkt := Packet{
		Header: Header{
			ID:      0x5678,
			Flags:   0x8180, // Response, no error
			QDCount: 1,
			ANCount: 1,
			NSCount: 0,
			ARCount: 0,
		},
		Questions: []Question{
			{Name: "example.com", Type: uint16(TypeA), Class: 1},
		},
		Answers: []Record{
			{
				Name:  "example.com",
				Type:  uint16(TypeA),
				Class: 1,
				TTL:   300,
				Data:  []byte{93, 184, 216, 34},
			},
		},
	}

	b, err := pkt.Marshal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(b) == 0 {
		t.Error("expected non-empty packet")
	}
}

func TestPacketMarshalWithAllSections(t *testing.T) {
	pkt := Packet{
		Header: Header{
			ID:      0xABCD,
			Flags:   0x8180,
			QDCount: 1,
			ANCount: 1,
			NSCount: 1,
			ARCount: 1,
		},
		Questions: []Question{
			{Name: "example.com", Type: uint16(TypeA), Class: 1},
		},
		Answers: []Record{
			{Name: "example.com", Type: uint16(TypeA), Class: 1, TTL: 300, Data: []byte{1, 2, 3, 4}},
		},
		Authorities: []Record{
			{Name: "example.com", Type: uint16(TypeNS), Class: 1, TTL: 86400, Data: "ns1.example.com"},
		},
		Additionals: []Record{
			{Name: "ns1.example.com", Type: uint16(TypeA), Class: 1, TTL: 86400, Data: []byte{5, 6, 7, 8}},
		},
	}

	b, err := pkt.Marshal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(b) == 0 {
		t.Error("expected non-empty packet")
	}
}

func TestPacketMarshalInvalidQuestion(t *testing.T) {
	// Question with invalid name (label too long)
	longLabel := make([]byte, 70)
	for i := range longLabel {
		longLabel[i] = 'a'
	}

	pkt := Packet{
		Header: Header{
			ID:      0x1234,
			Flags:   0x0100,
			QDCount: 1,
		},
		Questions: []Question{
			{Name: string(longLabel) + ".com", Type: uint16(TypeA), Class: 1},
		},
	}

	_, err := pkt.Marshal()
	if err == nil {
		t.Error("expected error for invalid question name")
	}
}

func TestParsePacket(t *testing.T) {
	// Build a simple query packet
	pkt := Packet{
		Header: Header{
			ID:      0x1234,
			Flags:   0x0100,
			QDCount: 1,
		},
		Questions: []Question{
			{Name: "example.com", Type: uint16(TypeA), Class: 1},
		},
	}

	b, err := pkt.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	parsed, err := ParsePacket(b)
	if err != nil {
		t.Fatalf("ParsePacket failed: %v", err)
	}

	if parsed.Header.ID != 0x1234 {
		t.Errorf("expected ID 0x1234, got 0x%04x", parsed.Header.ID)
	}
	if len(parsed.Questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(parsed.Questions))
	}
	if parsed.Questions[0].Name != "example.com" {
		t.Errorf("expected name example.com, got %s", parsed.Questions[0].Name)
	}
}

func TestParsePacketWithAnswers(t *testing.T) {
	// Build a response packet
	pkt := Packet{
		Header: Header{
			ID:      0x5678,
			Flags:   0x8180,
			QDCount: 1,
			ANCount: 1,
		},
		Questions: []Question{
			{Name: "example.com", Type: uint16(TypeA), Class: 1},
		},
		Answers: []Record{
			{Name: "example.com", Type: uint16(TypeA), Class: 1, TTL: 300, Data: []byte{1, 2, 3, 4}},
		},
	}

	b, err := pkt.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	parsed, err := ParsePacket(b)
	if err != nil {
		t.Fatalf("ParsePacket failed: %v", err)
	}

	if len(parsed.Answers) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(parsed.Answers))
	}
	if parsed.Answers[0].Name != "example.com" {
		t.Errorf("expected answer name example.com, got %s", parsed.Answers[0].Name)
	}
}

func TestParsePacketTooShort(t *testing.T) {
	_, err := ParsePacket([]byte{1, 2, 3}) // Too short for header
	if err == nil {
		t.Error("expected error for too short packet")
	}
}

func TestParsePacketTruncatedQuestion(t *testing.T) {
	// Valid header but truncated question
	msg := []byte{
		0x12, 0x34, // ID
		0x01, 0x00, // Flags
		0x00, 0x01, // QDCount = 1
		0x00, 0x00, // ANCount
		0x00, 0x00, // NSCount
		0x00, 0x00, // ARCount
		// Question starts but is truncated
		3, 'w', 'w', // Incomplete
	}

	_, err := ParsePacket(msg)
	if err == nil {
		t.Error("expected error for truncated question")
	}
}

func TestPacketRoundTrip(t *testing.T) {
	original := Packet{
		Header: Header{
			ID:      0xABCD,
			Flags:   0x8580, // Response with AA
			QDCount: 1,
			ANCount: 2,
			NSCount: 0,
			ARCount: 0,
		},
		Questions: []Question{
			{Name: "test.example.com", Type: uint16(TypeA), Class: 1},
		},
		Answers: []Record{
			{Name: "test.example.com", Type: uint16(TypeA), Class: 1, TTL: 300, Data: []byte{10, 0, 0, 1}},
			{Name: "test.example.com", Type: uint16(TypeA), Class: 1, TTL: 300, Data: []byte{10, 0, 0, 2}},
		},
	}

	b, err := original.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	parsed, err := ParsePacket(b)
	if err != nil {
		t.Fatalf("ParsePacket failed: %v", err)
	}

	if parsed.Header.ID != original.Header.ID {
		t.Errorf("ID mismatch: got %04x, want %04x", parsed.Header.ID, original.Header.ID)
	}
	if parsed.Header.Flags != original.Header.Flags {
		t.Errorf("Flags mismatch: got %04x, want %04x", parsed.Header.Flags, original.Header.Flags)
	}
	if len(parsed.Questions) != len(original.Questions) {
		t.Errorf("Question count mismatch: got %d, want %d", len(parsed.Questions), len(original.Questions))
	}
	if len(parsed.Answers) != len(original.Answers) {
		t.Errorf("Answer count mismatch: got %d, want %d", len(parsed.Answers), len(original.Answers))
	}
}
