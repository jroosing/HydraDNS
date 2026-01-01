package dns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)

	// Should have: name + 10 bytes fixed + 4 bytes rdata
	assert.GreaterOrEqual(t, len(b), 17, "unexpected length")

	// Verify RDATA length (last 4 bytes before RDATA)
	// The structure is: name | type(2) | class(2) | ttl(4) | rdlen(2) | rdata
	// Find rdlen position - it's 2 bytes before the last 4
	rdlenPos := len(b) - 4 - 2
	if rdlenPos > 0 {
		rdlen := int(b[rdlenPos])<<8 | int(b[rdlenPos+1])
		assert.Equal(t, 4, rdlen)
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
	require.NoError(t, err)
	assert.NotEmpty(t, b)
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
	require.NoError(t, err)
	assert.NotEmpty(t, b)
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
			require.NoError(t, err)
			assert.NotEmpty(t, b)
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
	require.NoError(t, err)
	assert.NotEmpty(t, b)
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
	require.NoError(t, err)
	assert.NotEmpty(t, b)
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
	require.NoError(t, err)
	assert.NotEmpty(t, b)
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
	assert.Error(t, err, "expected error for invalid A record data")
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
	assert.Error(t, err, "expected error for invalid AAAA record data")
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
	require.True(t, ok)
	assert.Equal(t, "192.0.2.1", ip)
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
	assert.False(t, ok, "expected ok to be false for non-A record")
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
	require.True(t, ok)
	assert.Equal(t, "2001:db8::1", ip)
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
	assert.False(t, ok, "expected ok to be false for non-AAAA record")
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
		0,    // End of name
		0, 1, // Type A
		0, 1, // Class IN
		0, 0, 1, 44, // TTL 300
		0, 4, // RDLEN
		192, 0, 2, 1, // RDATA
	}

	off := 0
	rr, err := ParseRecord(msg, &off)
	require.NoError(t, err)

	assert.Equal(t, "example.com", rr.Name)
	assert.Equal(t, uint16(TypeA), rr.Type)
	assert.Equal(t, uint16(1), rr.Class)
	assert.Equal(t, uint32(300), rr.TTL)

	data, ok := rr.Data.([]byte)
	require.True(t, ok, "expected []byte data, got %T", rr.Data)
	assert.Len(t, data, 4)
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
	require.NoError(t, err, "Marshal failed")

	off := 0
	parsed, err := ParseRecord(b, &off)
	require.NoError(t, err)

	assert.Equal(t, uint16(TypeCNAME), parsed.Type)

	target, ok := parsed.Data.(string)
	require.True(t, ok, "expected string data, got %T", parsed.Data)
	assert.Equal(t, "target.example.com", target)
}

func TestParseRecordMX(t *testing.T) {
	// MX record with preference 10, exchange mail.example.com
	msg := []byte{
		7, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		3, 'c', 'o', 'm',
		0,     // End of name
		0, 15, // Type MX
		0, 1, // Class IN
		0, 0, 14, 16, // TTL 3600
		0, 20, // RDLEN
		0, 10, // Preference
		4, 'm', 'a', 'i', 'l',
		7, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		3, 'c', 'o', 'm',
		0, // End of exchange name
	}

	off := 0
	rr, err := ParseRecord(msg, &off)
	require.NoError(t, err)

	assert.Equal(t, uint16(TypeMX), rr.Type)

	mx, ok := rr.Data.(MXData)
	require.True(t, ok, "expected MXData, got %T", rr.Data)
	assert.Equal(t, uint16(10), mx.Preference)
	assert.Equal(t, "mail.example.com", mx.Exchange)
}

func TestParseRecordTruncated(t *testing.T) {
	// Truncated record (missing RDATA)
	msg := []byte{
		7, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		3, 'c', 'o', 'm',
		0,    // End of name
		0, 1, // Type A
		0, 1, // Class IN
		0, 0, 1, 44, // TTL 300
		0, 4, // RDLEN says 4 bytes
		// But no RDATA follows
	}

	off := 0
	_, err := ParseRecord(msg, &off)
	assert.Error(t, err, "expected error for truncated record")
}
