package dns

import (
	"testing"
)

func TestRecordMarshalA(t *testing.T) {
	rr := Record{
		Name:  "example.com",
		Type:  uint16(TypeA),
		Class: 1,
		TTL:   300,
		Data:  []byte{192, 0, 2, 1},
	}

	b, err := rr.Marshal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have: name + 10 bytes fixed + 4 bytes rdata
	if len(b) < 17 { // minimum: 1 (root) + 10 + 4 = 15, but name is longer
		t.Errorf("unexpected length: %d", len(b))
	}

	// Verify RDATA length (last 4 bytes before RDATA)
	// The structure is: name | type(2) | class(2) | ttl(4) | rdlen(2) | rdata
	// Find rdlen position - it's 2 bytes before the last 4
	rdlenPos := len(b) - 4 - 2
	if rdlenPos > 0 {
		rdlen := int(b[rdlenPos])<<8 | int(b[rdlenPos+1])
		if rdlen != 4 {
			t.Errorf("expected rdlen 4, got %d", rdlen)
		}
	}
}

func TestRecordMarshalCNAME(t *testing.T) {
	rr := Record{
		Name:  "www.example.com",
		Type:  uint16(TypeCNAME),
		Class: 1,
		TTL:   3600,
		Data:  "example.com",
	}

	b, err := rr.Marshal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(b) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestRecordMarshalMX(t *testing.T) {
	rr := Record{
		Name:  "example.com",
		Type:  uint16(TypeMX),
		Class: 1,
		TTL:   3600,
		Data:  MXData{Preference: 10, Exchange: "mail.example.com"},
	}

	b, err := rr.Marshal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(b) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestRecordMarshalTXT(t *testing.T) {
	tests := []struct {
		name string
		data any
	}{
		{"string", "hello world"},
		{"string slice", []string{"hello", "world"}},
		{"byte slice", []byte("raw bytes")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := Record{
				Name:  "example.com",
				Type:  uint16(TypeTXT),
				Class: 1,
				TTL:   300,
				Data:  tt.data,
			}

			b, err := rr.Marshal()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(b) == 0 {
				t.Error("expected non-empty result")
			}
		})
	}
}

func TestRecordMarshalAAAA(t *testing.T) {
	rr := Record{
		Name:  "example.com",
		Type:  uint16(TypeAAAA),
		Class: 1,
		TTL:   300,
		Data:  []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
	}

	b, err := rr.Marshal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(b) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestRecordMarshalNS(t *testing.T) {
	rr := Record{
		Name:  "example.com",
		Type:  uint16(TypeNS),
		Class: 1,
		TTL:   86400,
		Data:  "ns1.example.com",
	}

	b, err := rr.Marshal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(b) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestRecordMarshalSOA(t *testing.T) {
	// SOA data is stored as raw bytes
	rr := Record{
		Name:  "example.com",
		Type:  uint16(TypeSOA),
		Class: 1,
		TTL:   86400,
		Data:  []byte{0x01, 0x02, 0x03}, // Simplified - actual SOA is more complex
	}

	b, err := rr.Marshal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(b) == 0 {
		t.Error("expected non-empty result")
	}
}

func TestRecordMarshalInvalidAData(t *testing.T) {
	rr := Record{
		Name:  "example.com",
		Type:  uint16(TypeA),
		Class: 1,
		TTL:   300,
		Data:  "not bytes", // Wrong type
	}

	_, err := rr.Marshal()
	if err == nil {
		t.Error("expected error for invalid A record data")
	}
}

func TestRecordMarshalInvalidAAAAData(t *testing.T) {
	rr := Record{
		Name:  "example.com",
		Type:  uint16(TypeAAAA),
		Class: 1,
		TTL:   300,
		Data:  []byte{1, 2, 3, 4}, // Only 4 bytes, need 16
	}

	_, err := rr.Marshal()
	if err == nil {
		t.Error("expected error for invalid AAAA record data")
	}
}

func TestRecordIPv4(t *testing.T) {
	rr := Record{
		Name:  "example.com",
		Type:  uint16(TypeA),
		Class: 1,
		TTL:   300,
		Data:  []byte{192, 0, 2, 1},
	}

	ip, ok := rr.IPv4()
	if !ok {
		t.Fatal("expected ok to be true")
	}
	if ip != "192.0.2.1" {
		t.Errorf("expected 192.0.2.1, got %s", ip)
	}
}

func TestRecordIPv4NotA(t *testing.T) {
	rr := Record{
		Name:  "example.com",
		Type:  uint16(TypeAAAA),
		Class: 1,
		TTL:   300,
		Data:  []byte{1, 2, 3, 4},
	}

	_, ok := rr.IPv4()
	if ok {
		t.Error("expected ok to be false for non-A record")
	}
}

func TestRecordIPv6(t *testing.T) {
	rr := Record{
		Name:  "example.com",
		Type:  uint16(TypeAAAA),
		Class: 1,
		TTL:   300,
		Data:  []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
	}

	ip, ok := rr.IPv6()
	if !ok {
		t.Fatal("expected ok to be true")
	}
	if ip != "2001:db8::1" {
		t.Errorf("expected 2001:db8::1, got %s", ip)
	}
}

func TestRecordIPv6NotAAAA(t *testing.T) {
	rr := Record{
		Name:  "example.com",
		Type:  uint16(TypeA),
		Class: 1,
		TTL:   300,
		Data:  []byte{1, 2, 3, 4},
	}

	_, ok := rr.IPv6()
	if ok {
		t.Error("expected ok to be false for non-AAAA record")
	}
}

func TestParseRecord(t *testing.T) {
	// Build a simple A record
	// Name: example.com
	// Type: A (1)
	// Class: IN (1)
	// TTL: 300
	// RDLEN: 4
	// RDATA: 192.0.2.1
	msg := []byte{
		7, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		3, 'c', 'o', 'm',
		0,          // End of name
		0, 1,       // Type A
		0, 1,       // Class IN
		0, 0, 1, 44, // TTL 300
		0, 4,             // RDLEN
		192, 0, 2, 1,     // RDATA
	}

	off := 0
	rr, err := ParseRecord(msg, &off)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rr.Name != "example.com" {
		t.Errorf("expected name example.com, got %s", rr.Name)
	}
	if rr.Type != uint16(TypeA) {
		t.Errorf("expected type A, got %d", rr.Type)
	}
	if rr.Class != 1 {
		t.Errorf("expected class 1, got %d", rr.Class)
	}
	if rr.TTL != 300 {
		t.Errorf("expected TTL 300, got %d", rr.TTL)
	}

	data, ok := rr.Data.([]byte)
	if !ok {
		t.Fatalf("expected []byte data, got %T", rr.Data)
	}
	if len(data) != 4 {
		t.Errorf("expected 4 bytes, got %d", len(data))
	}
}

func TestParseRecordCNAME(t *testing.T) {
	// Build and marshal a CNAME record, then parse it
	rr := Record{
		Name:  "www.example.com",
		Type:  uint16(TypeCNAME),
		Class: 1,
		TTL:   3600,
		Data:  "target.example.com",
	}

	b, err := rr.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	off := 0
	parsed, err := ParseRecord(b, &off)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parsed.Type != uint16(TypeCNAME) {
		t.Errorf("expected type CNAME, got %d", parsed.Type)
	}

	target, ok := parsed.Data.(string)
	if !ok {
		t.Fatalf("expected string data, got %T", parsed.Data)
	}
	if target != "target.example.com" {
		t.Errorf("expected target.example.com, got %s", target)
	}
}

func TestParseRecordMX(t *testing.T) {
	// MX record with preference 10, exchange mail.example.com
	msg := []byte{
		7, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		3, 'c', 'o', 'm',
		0,          // End of name
		0, 15,      // Type MX
		0, 1,       // Class IN
		0, 0, 14, 16, // TTL 3600
		0, 20,      // RDLEN
		0, 10,      // Preference
		4, 'm', 'a', 'i', 'l',
		7, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		3, 'c', 'o', 'm',
		0, // End of exchange name
	}

	off := 0
	rr, err := ParseRecord(msg, &off)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rr.Type != uint16(TypeMX) {
		t.Errorf("expected type MX, got %d", rr.Type)
	}

	mx, ok := rr.Data.(MXData)
	if !ok {
		t.Fatalf("expected MXData, got %T", rr.Data)
	}
	if mx.Preference != 10 {
		t.Errorf("expected preference 10, got %d", mx.Preference)
	}
	if mx.Exchange != "mail.example.com" {
		t.Errorf("expected mail.example.com, got %s", mx.Exchange)
	}
}

func TestParseRecordTruncated(t *testing.T) {
	// Truncated record (missing RDATA)
	msg := []byte{
		7, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		3, 'c', 'o', 'm',
		0,          // End of name
		0, 1,       // Type A
		0, 1,       // Class IN
		0, 0, 1, 44, // TTL 300
		0, 4,       // RDLEN says 4 bytes
		// But no RDATA follows
	}

	off := 0
	_, err := ParseRecord(msg, &off)
	if err == nil {
		t.Error("expected error for truncated record")
	}
}
