package dns

import (
	"testing"
)

func TestEDNSOptionMarshal(t *testing.T) {
	opt := EDNSOption{
		Code: 10,
		Data: []byte{0x01, 0x02, 0x03},
	}
	b := opt.Marshal()
	// 2 bytes code + 2 bytes length + 3 bytes data = 7 bytes
	if len(b) != 7 {
		t.Fatalf("expected 7 bytes, got %d", len(b))
	}
	// Code = 10 (0x000A)
	if b[0] != 0 || b[1] != 10 {
		t.Errorf("expected code 10, got %d", int(b[0])<<8|int(b[1]))
	}
	// Length = 3
	if b[2] != 0 || b[3] != 3 {
		t.Errorf("expected length 3, got %d", int(b[2])<<8|int(b[3]))
	}
	// Data
	if b[4] != 1 || b[5] != 2 || b[6] != 3 {
		t.Errorf("unexpected data: %v", b[4:])
	}
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
			if int(opt.UDPPayloadSize) < tt.wantMin || int(opt.UDPPayloadSize) > tt.wantMax {
				t.Errorf("UDPPayloadSize = %d, want between %d and %d",
					opt.UDPPayloadSize, tt.wantMin, tt.wantMax)
			}
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
			if got != tt.want {
				t.Errorf("clampInt(%d, %d, %d) = %d, want %d", tt.v, tt.min, tt.max, got, tt.want)
			}
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
	if b[0] != 0 {
		t.Errorf("expected root name 0x00, got 0x%02x", b[0])
	}

	// Type should be OPT (41)
	typeVal := int(b[1])<<8 | int(b[2])
	if typeVal != int(TypeOPT) {
		t.Errorf("expected type 41, got %d", typeVal)
	}

	// Class should be UDP payload size (4096)
	classVal := int(b[3])<<8 | int(b[4])
	if classVal != 4096 {
		t.Errorf("expected class 4096, got %d", classVal)
	}

	// TTL should have DO bit set (bit 15)
	// TTL is at bytes 5-8
	ttl := uint32(b[5])<<24 | uint32(b[6])<<16 | uint32(b[7])<<8 | uint32(b[8])
	doFlag := (ttl >> 15) & 1
	if doFlag != 1 {
		t.Errorf("expected DO flag set, TTL = 0x%08x", ttl)
	}
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

			if gotExtRCode != tt.extRCode {
				t.Errorf("extRCode: got %d, want %d", gotExtRCode, tt.extRCode)
			}
			if gotVersion != tt.version {
				t.Errorf("version: got %d, want %d", gotVersion, tt.version)
			}
			if gotDO != tt.dnssecOk {
				t.Errorf("dnssecOk: got %v, want %v", gotDO, tt.dnssecOk)
			}
		})
	}
}

func TestExtractOPT(t *testing.T) {
	// Test with no OPT record
	additionals := []Record{
		{Name: "example.com", Type: uint16(TypeA), Class: 1, TTL: 300, Data: []byte{1, 2, 3, 4}},
	}
	opt := ExtractOPT(additionals)
	if opt != nil {
		t.Error("expected nil for no OPT record")
	}

	// Test with OPT record
	// UDP size = 4096, TTL packed with DO flag
	ttl := packOPTTTL(0, 0, true)
	additionals = []Record{
		{Name: "", Type: uint16(TypeOPT), Class: 4096, TTL: ttl, Data: []byte{}},
	}
	opt = ExtractOPT(additionals)
	if opt == nil {
		t.Fatal("expected OPT record to be extracted")
	}
	if opt.UDPPayloadSize != 4096 {
		t.Errorf("expected UDP size 4096, got %d", opt.UDPPayloadSize)
	}
	if !opt.DNSSECOk {
		t.Error("expected DNSSECOk to be true")
	}
}

func TestClientMaxUDPSize(t *testing.T) {
	// No EDNS - should return default
	pkt := Packet{
		Header:      Header{ID: 1234},
		Additionals: nil,
	}
	size := ClientMaxUDPSize(pkt)
	if size != DefaultUDPPayloadSize {
		t.Errorf("expected %d, got %d", DefaultUDPPayloadSize, size)
	}

	// With EDNS advertising 4096
	ttl := packOPTTTL(0, 0, false)
	pkt.Additionals = []Record{
		{Type: uint16(TypeOPT), Class: 4096, TTL: ttl, Data: []byte{}},
	}
	size = ClientMaxUDPSize(pkt)
	if size != 4096 {
		t.Errorf("expected 4096, got %d", size)
	}

	// With EDNS advertising below minimum
	pkt.Additionals = []Record{
		{Type: uint16(TypeOPT), Class: 100, TTL: ttl, Data: []byte{}},
	}
	size = ClientMaxUDPSize(pkt)
	if size != DefaultUDPPayloadSize {
		t.Errorf("expected %d (minimum), got %d", DefaultUDPPayloadSize, size)
	}
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
			if got != tt.want {
				t.Errorf("IsTruncated = %v, want %v", got, tt.want)
			}
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
	if len(newBytes) <= originalLen {
		t.Errorf("expected longer message after adding EDNS, got %d <= %d", len(newBytes), originalLen)
	}

	// Already has EDNS - should return unchanged
	req.Additionals = []Record{
		{Type: uint16(TypeOPT), Class: 4096, Data: []byte{}},
	}
	reqBytes2, _ := req.Marshal()
	newBytes2 := AddEDNSToRequestBytes(req, reqBytes2, 4096)
	if len(newBytes2) != len(reqBytes2) {
		t.Errorf("should not add EDNS when already present")
	}
}
