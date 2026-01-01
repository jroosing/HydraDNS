package dns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRequestBoundedRejectsResponse(t *testing.T) {
	// header with QR=1
	msg := make([]byte, 12)
	msg[2] = 0x80
	msg[5] = 1 // qdcount=1
	_, err := ParseRequestBounded(msg)
	assert.Error(t, err)
}

func TestParseRequestBounded_TooLarge(t *testing.T) {
	msg := make([]byte, MaxIncomingDNSMessageSize+1)
	_, err := ParseRequestBounded(msg)
	require.Error(t, err, "expected error for oversized message")
	assert.Contains(t, err.Error(), "too large")
}

func TestParseRequestBounded_UnsupportedOpcode(t *testing.T) {
	// Build a header with opcode 1 (IQUERY) - bits 14-11
	// Opcode 1 = 0001 in bits 14-11 = 0x0800
	msg := buildValidQueryHeader()
	msg[2] = 0x08 // Set opcode to 1 (bits 14-11)

	_, err := ParseRequestBounded(msg)
	require.Error(t, err, "expected error for unsupported opcode")
	assert.Contains(t, err.Error(), "OpCode")
}

func TestParseRequestBounded_TooManyQuestions(t *testing.T) {
	msg := buildValidQueryHeader()
	// Set QDCount to MaxQuestions + 1
	msg[4] = 0
	msg[5] = byte(MaxQuestions + 1)

	_, err := ParseRequestBounded(msg)
	assert.Error(t, err, "expected error for too many questions")
}

func TestParseRequestBounded_WrongQuestionCount(t *testing.T) {
	msg := buildValidQueryHeader()
	// Set QDCount to 0 (must be exactly 1)
	msg[4] = 0
	msg[5] = 0

	_, err := ParseRequestBounded(msg)
	assert.Error(t, err, "expected error for wrong question count")
}

func TestParseRequestBounded_TooManyAnswerRecords(t *testing.T) {
	msg := buildValidQueryHeader()
	// Set ANCount to MaxRRPerSection + 1
	msg[6] = byte((MaxRRPerSection + 1) >> 8)
	msg[7] = byte(MaxRRPerSection + 1)

	_, err := ParseRequestBounded(msg)
	assert.Error(t, err, "expected error for too many answer records")
}

func TestParseRequestBounded_ValidQuery(t *testing.T) {
	// Build a complete valid query for "example.com" A record
	p := Packet{
		Header: Header{ID: 0x1234, Flags: RDFlag, QDCount: 1},
		Questions: []Question{
			{Name: "example.com", Type: uint16(TypeA), Class: uint16(ClassIN)},
		},
	}
	msg, err := p.Marshal()
	require.NoError(t, err, "failed to marshal")

	result, err := ParseRequestBounded(msg)
	require.NoError(t, err)
	assert.Equal(t, uint16(0x1234), result.Header.ID)
	assert.Len(t, result.Questions, 1)
}

func TestBuildErrorResponse(t *testing.T) {
	tests := []struct {
		name      string
		rcode     RCode
		wantRCode uint16
	}{
		{"SERVFAIL", RCodeServFail, uint16(RCodeServFail)},
		{"FORMERR", RCodeFormErr, uint16(RCodeFormErr)},
		{"NXDOMAIN", RCodeNXDomain, uint16(RCodeNXDomain)},
		{"REFUSED", RCodeRefused, uint16(RCodeRefused)},
		{"NOERROR", RCodeNoError, uint16(RCodeNoError)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := Packet{
				Header: Header{ID: 0xABCD, Flags: RDFlag, QDCount: 1},
				Questions: []Question{
					{Name: "test.com", Type: uint16(TypeA), Class: uint16(ClassIN)},
				},
			}

			resp := BuildErrorResponse(req, tt.wantRCode)

			// Check ID preserved
			assert.Equal(t, uint16(0xABCD), resp.Header.ID)

			// Check QR flag set
			assert.NotZero(t, resp.Header.Flags&QRFlag, "QR flag should be set")

			// Check RD flag preserved
			assert.NotZero(t, resp.Header.Flags&RDFlag, "RD flag should be preserved")

			// Check RCODE
			gotRCode := resp.Header.Flags & RCodeMask
			assert.Equal(t, tt.wantRCode, gotRCode)

			// Check question preserved
			assert.Len(t, resp.Questions, 1)

			// Check no answer records
			assert.Zero(t, resp.Header.ANCount, "ANCount should be 0")
		})
	}
}

func TestIsResponse(t *testing.T) {
	assert.False(t, isResponse(0x0000), "0x0000 should not be a response")
	assert.True(t, isResponse(0x8000), "0x8000 should be a response")
	assert.True(t, isResponse(0x8100), "0x8100 should be a response")
}

func TestExtractOpcode(t *testing.T) {
	tests := []struct {
		flags      uint16
		wantOpcode uint16
	}{
		{0x0000, 0}, // Standard query
		{0x0800, 1}, // IQUERY
		{0x1000, 2}, // STATUS
		{0x7800, 15}, // Max opcode
	}

	for _, tt := range tests {
		got := extractOpcode(tt.flags)
		assert.Equal(t, tt.wantOpcode, got)
	}
}

func TestRCodeFromFlags(t *testing.T) {
	tests := []struct {
		flags uint16
		want  RCode
	}{
		{0x0000, RCodeNoError},
		{0x0001, RCodeFormErr},
		{0x0002, RCodeServFail},
		{0x0003, RCodeNXDomain},
		{0x0005, RCodeRefused},
		{0x8003, RCodeNXDomain}, // With QR flag set
	}

	for _, tt := range tests {
		got := RCodeFromFlags(tt.flags)
		assert.Equal(t, tt.want, got)
	}
}

// buildValidQueryHeader creates a minimal valid DNS query header
func buildValidQueryHeader() []byte {
	// Standard query with QDCount=1
	return []byte{
		0x12, 0x34, // ID
		0x01, 0x00, // Flags: RD=1, everything else 0
		0x00, 0x01, // QDCount = 1
		0x00, 0x00, // ANCount = 0
		0x00, 0x00, // NSCount = 0
		0x00, 0x00, // ARCount = 0
		// Question section for "." (root)
		0x00,       // empty label (root)
		0x00, 0x01, // QTYPE = A
		0x00, 0x01, // QCLASS = IN
	}
}
