package dns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEDNSOptionMarshal(t *testing.T) {
	opt := EDNSOption{
		Code: 10,
		Data: []byte{0x01, 0x02, 0x03},
	}
	b := opt.Marshal()
	// 2 bytes code + 2 bytes length + 3 bytes data = 7 bytes
	require.Len(t, b, 7)
	// Code = 10 (0x000A)
	assert.Equal(t, byte(0), b[0])
	assert.Equal(t, byte(10), b[1])
	// Length = 3
	assert.Equal(t, byte(0), b[2])
	assert.Equal(t, byte(3), b[3])
	// Data
	assert.Equal(t, []byte{1, 2, 3}, b[4:7])
}

func TestCreateOPT(t *testing.T) {
	tests := []struct {
		name    string
		size    int
		wantMin int
		wantMax int
	}{
		{"normal size", 4096, 4096, 4096},
		{"below minimum", 100, EDNSMinUDPPayloadSize, EDNSMinUDPPayloadSize},
		{"above maximum", 70000, 65535, 65535},
		{"at minimum", 512, 512, 512},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opt := CreateOPT(tt.size)
			assert.GreaterOrEqual(t, int(opt.UDPPayloadSize), tt.wantMin)
			assert.LessOrEqual(t, int(opt.UDPPayloadSize), tt.wantMax)
		})
	}
}

func TestClampInt(t *testing.T) {
	tests := []struct {
		name     string
		v        int
		min, max int
		want     int
	}{
		{"in range", 50, 0, 100, 50},
		{"below min", -10, 0, 100, 0},
		{"above max", 200, 0, 100, 100},
		{"at min", 0, 0, 100, 0},
		{"at max", 100, 0, 100, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampInt(tt.v, tt.min, tt.max)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOPTRecordMarshal(t *testing.T) {
	opt := OPTRecord{
		UDPPayloadSize: 4096,
		ExtendedRCode:  0,
		Version:        0,
		DNSSECOk:       true,
		Options:        nil,
	}

	b := opt.Marshal()

	// Should start with root name (0x00)
	assert.Equal(t, byte(0), b[0], "expected root name 0x00")

	// Type should be OPT (41)
	typeVal := int(b[1])<<8 | int(b[2])
	assert.Equal(t, int(TypeOPT), typeVal)

	// Class should be UDP payload size (4096)
	classVal := int(b[3])<<8 | int(b[4])
	assert.Equal(t, 4096, classVal)

	// TTL should have DO bit set (bit 15)
	// TTL is at bytes 5-8
	ttl := uint32(b[5])<<24 | uint32(b[6])<<16 | uint32(b[7])<<8 | uint32(b[8])
	doFlag := (ttl >> 15) & 1
	assert.Equal(t, uint32(1), doFlag, "expected DO flag set")
}

func TestPackOPTTTL(t *testing.T) {
	tests := []struct {
		name     string
		extRCode uint8
		version  uint8
		dnssecOk bool
	}{
		{"all zeros", 0, 0, false},
		{"DO flag set", 0, 0, true},
		{"extended rcode", 5, 0, false},
		{"version 1", 0, 1, false},
		{"all set", 3, 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ttl := packOPTTTL(tt.extRCode, tt.version, tt.dnssecOk)

			gotExtRCode := uint8(ttl >> 24)
			gotVersion := uint8(ttl >> 16)
			gotDO := ((ttl >> 15) & 1) == 1

			assert.Equal(t, tt.extRCode, gotExtRCode)
			assert.Equal(t, tt.version, gotVersion)
			assert.Equal(t, tt.dnssecOk, gotDO)
		})
	}
}

func TestExtractOPT(t *testing.T) {
	// Test with no OPT record
	additionals := []Record{
		{Name: "example.com", Type: uint16(TypeA), Class: 1, TTL: 300, Data: []byte{1, 2, 3, 4}},
	}
	opt := ExtractOPT(additionals)
	assert.Nil(t, opt, "expected nil for no OPT record")

	// Test with OPT record
	// UDP size = 4096, TTL packed with DO flag
	ttl := packOPTTTL(0, 0, true)
	additionals = []Record{
		{Name: "", Type: uint16(TypeOPT), Class: 4096, TTL: ttl, Data: []byte{}},
	}
	opt = ExtractOPT(additionals)
	require.NotNil(t, opt, "expected OPT record to be extracted")
	assert.Equal(t, uint16(4096), opt.UDPPayloadSize)
	assert.True(t, opt.DNSSECOk)
}

func TestClientMaxUDPSize(t *testing.T) {
	// No EDNS - should return default
	pkt := Packet{
		Header:      Header{ID: 1234},
		Additionals: nil,
	}
	size := ClientMaxUDPSize(pkt)
	assert.Equal(t, DefaultUDPPayloadSize, size)

	// With EDNS advertising 4096
	ttl := packOPTTTL(0, 0, false)
	pkt.Additionals = []Record{
		{Type: uint16(TypeOPT), Class: 4096, TTL: ttl, Data: []byte{}},
	}
	size = ClientMaxUDPSize(pkt)
	assert.Equal(t, 4096, size)

	// With EDNS advertising below minimum
	pkt.Additionals = []Record{
		{Type: uint16(TypeOPT), Class: 100, TTL: ttl, Data: []byte{}},
	}
	size = ClientMaxUDPSize(pkt)
	assert.Equal(t, DefaultUDPPayloadSize, size, "expected minimum")
}

func TestIsTruncated(t *testing.T) {
	tests := []struct {
		name     string
		response []byte
		want     bool
	}{
		{"too short", []byte{0, 1, 2}, false},
		{"not truncated", []byte{0, 0, 0x01, 0x00}, false}, // QR=0, no TC
		{"truncated", []byte{0, 0, 0x82, 0x00}, true},      // QR=1, TC=1
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsTruncated(tt.response)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAddEDNSToRequestBytes(t *testing.T) {
	// Build a simple DNS request without EDNS
	req := Packet{
		Header: Header{
			ID:      0x1234,
			Flags:   0x0100, // Standard query
			QDCount: 1,
		},
		Questions: []Question{
			{Name: "example.com", Type: uint16(TypeA), Class: 1},
		},
	}
	reqBytes, _ := req.Marshal()
	originalLen := len(reqBytes)

	// Add EDNS
	newBytes := AddEDNSToRequestBytes(req, reqBytes, 4096)
	assert.Greater(t, len(newBytes), originalLen, "expected longer message after adding EDNS")

	// Already has EDNS - should return unchanged
	req.Additionals = []Record{
		{Type: uint16(TypeOPT), Class: 4096, Data: []byte{}},
	}
	reqBytes2, _ := req.Marshal()
	newBytes2 := AddEDNSToRequestBytes(req, reqBytes2, 4096)
	assert.Len(t, newBytes2, len(reqBytes2), "should not add EDNS when already present")
}
