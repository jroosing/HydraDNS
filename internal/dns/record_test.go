package dns_test

import (
	"net"
	"testing"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIPRecord_IPv4(t *testing.T) {
	r := &dns.IPRecord{
		H:    dns.RRHeader{Name: "example.com.", Class: 1, TTL: 300},
		Addr: net.ParseIP("192.0.2.1"),
	}

	assert.Equal(t, dns.TypeA, r.Type())
	assert.Equal(t, "example.com.", r.Header().Name)
	assert.Equal(t, uint16(1), r.Header().Class)
	assert.Equal(t, uint32(300), r.Header().TTL)

	rdata, err := r.MarshalRData()
	require.NoError(t, err)
	assert.Equal(t, []byte{192, 0, 2, 1}, rdata)
}

func TestIPRecord_IPv6(t *testing.T) {
	r := &dns.IPRecord{
		H:    dns.RRHeader{Name: "example.com.", Class: 1, TTL: 300},
		Addr: net.ParseIP("2001:db8::1"),
	}

	assert.Equal(t, dns.TypeAAAA, r.Type())

	rdata, err := r.MarshalRData()
	require.NoError(t, err)
	assert.Len(t, rdata, 16)
}

func TestNameRecord(t *testing.T) {
	r := &dns.NameRecord{
		H:      dns.RRHeader{Name: "www.example.com.", Class: 1, TTL: 300},
		T:      dns.TypeCNAME,
		Target: "example.com.",
	}

	assert.Equal(t, dns.TypeCNAME, r.Type())

	rdata, err := r.MarshalRData()
	require.NoError(t, err)
	assert.NotEmpty(t, rdata)
}

// MX, SRV, CAA, TXT records are now parsed as OpaqueRecord for forwarding.
// The following tests verify that opaque records can be marshaled/unmarshaled.

func TestOpaqueRecord(t *testing.T) {
	r := &dns.OpaqueRecord{
		H:    dns.RRHeader{Name: "example.com.", Class: 1, TTL: 300},
		T:    dns.RecordType(99),
		Data: []byte{1, 2, 3, 4},
	}

	assert.Equal(t, dns.RecordType(99), r.Type())

	rdata, err := r.MarshalRData()
	require.NoError(t, err)
	assert.Equal(t, []byte{1, 2, 3, 4}, rdata)
}

func TestOpaqueRecord_TypedData(t *testing.T) {
	// Test that opaque records preserve typed data (e.g. MX records)
	testData := []byte{0x00, 0x0a, 0x04, 'm', 'a', 'i', 'l'}
	r := dns.NewOpaqueRecord(
		dns.NewRRHeader("example.com.", dns.ClassIN, 300),
		dns.TypeMX,
		testData,
	)

	assert.Equal(t, dns.TypeMX, r.Type())

	rdata, err := r.MarshalRData()
	require.NoError(t, err)
	assert.Equal(t, testData, rdata)
}
