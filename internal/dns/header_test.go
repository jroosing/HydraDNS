package dns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)

	assert.Len(t, b, HeaderSize)

	// Verify ID
	assert.Equal(t, byte(0x12), b[0])
	assert.Equal(t, byte(0x34), b[1])

	// Verify Flags
	assert.Equal(t, byte(0x81), b[2])
	assert.Equal(t, byte(0x80), b[3])

	// Verify counts
	assert.Equal(t, []byte{0, 1}, b[4:6], "unexpected QDCount")
	assert.Equal(t, []byte{0, 2}, b[6:8], "unexpected ANCount")
	assert.Equal(t, []byte{0, 3}, b[8:10], "unexpected NSCount")
	assert.Equal(t, []byte{0, 4}, b[10:12], "unexpected ARCount")
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
	require.NoError(t, err)

	assert.Equal(t, uint16(0x1234), h.ID)
	assert.Equal(t, uint16(0x8180), h.Flags)
	assert.Equal(t, uint16(1), h.QDCount)
	assert.Equal(t, uint16(2), h.ANCount)
	assert.Equal(t, uint16(3), h.NSCount)
	assert.Equal(t, uint16(4), h.ARCount)
	assert.Equal(t, HeaderSize, off)
}

func TestParseHeaderTooShort(t *testing.T) {
	msg := []byte{0x12, 0x34, 0x81, 0x80} // Only 4 bytes

	off := 0
	_, err := ParseHeader(msg, &off)
	assert.Error(t, err, "expected error for too short message")
}

func TestParseHeaderOffset(t *testing.T) {
	// Header at offset 5
	msg := make([]byte, 5+HeaderSize)
	msg[5] = 0xAB
	msg[6] = 0xCD

	off := 5
	h, err := ParseHeader(msg, &off)
	require.NoError(t, err)
	assert.Equal(t, uint16(0xABCD), h.ID)
	assert.Equal(t, 5+HeaderSize, off)
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
	require.NoError(t, err, "Marshal failed")

	off := 0
	parsed, err := ParseHeader(b, &off)
	require.NoError(t, err, "ParseHeader failed")

	assert.Equal(t, original, parsed, "round trip failed")
}
