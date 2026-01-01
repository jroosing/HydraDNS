package dns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)

	// Minimum: 12 (header) + encoded name + 4 (type/class)
	assert.GreaterOrEqual(t, len(b), 12, "packet too short")

	// Verify header ID
	assert.Equal(t, byte(0x12), b[0])
	assert.Equal(t, byte(0x34), b[1])
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
	require.NoError(t, err)
	assert.NotEmpty(t, b)
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
	require.NoError(t, err)
	assert.NotEmpty(t, b)
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
	assert.Error(t, err, "expected error for invalid question name")
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
	require.NoError(t, err, "Marshal failed")

	parsed, err := ParsePacket(b)
	require.NoError(t, err, "ParsePacket failed")

	assert.Equal(t, uint16(0x1234), parsed.Header.ID)
	require.Len(t, parsed.Questions, 1)
	assert.Equal(t, "example.com", parsed.Questions[0].Name)
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
	require.NoError(t, err, "Marshal failed")

	parsed, err := ParsePacket(b)
	require.NoError(t, err, "ParsePacket failed")

	require.Len(t, parsed.Answers, 1)
	assert.Equal(t, "example.com", parsed.Answers[0].Name)
}

func TestParsePacketTooShort(t *testing.T) {
	_, err := ParsePacket([]byte{1, 2, 3}) // Too short for header
	assert.Error(t, err, "expected error for too short packet")
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
	assert.Error(t, err, "expected error for truncated question")
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
	require.NoError(t, err, "Marshal failed")

	parsed, err := ParsePacket(b)
	require.NoError(t, err, "ParsePacket failed")

	assert.Equal(t, original.Header.ID, parsed.Header.ID, "ID mismatch")
	assert.Equal(t, original.Header.Flags, parsed.Header.Flags, "Flags mismatch")
	assert.Len(t, parsed.Questions, len(original.Questions), "Question count mismatch")
	assert.Len(t, parsed.Answers, len(original.Answers), "Answer count mismatch")
}
