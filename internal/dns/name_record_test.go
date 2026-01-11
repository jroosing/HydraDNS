package dns_test

import (
	"testing"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNameRecord(t *testing.T) {
	h := dns.NewRRHeader("example.com.", dns.ClassIN, 300)

	t.Run("CNAME", func(t *testing.T) {
		rec := dns.NewCNAMERecord(h, "www.example.com")
		assert.Equal(t, dns.TypeCNAME, rec.Type())
		assert.Equal(t, "www.example.com", rec.Target)
	})

	t.Run("NS", func(t *testing.T) {
		rec := dns.NewNSRecord(h, "ns1.example.com.")
		assert.Equal(t, dns.TypeNS, rec.Type())
		assert.Equal(t, "ns1.example.com.", rec.Target)
	})

	t.Run("PTR", func(t *testing.T) {
		rec := dns.NewPTRRecord(h, "host.example.com.")
		assert.Equal(t, dns.TypePTR, rec.Type())
		assert.Equal(t, "host.example.com.", rec.Target)
	})

	t.Run("generic", func(t *testing.T) {
		rec := dns.NewNameRecord(h, dns.TypeCNAME, "target.example.com")
		assert.Equal(t, dns.TypeCNAME, rec.Type())
		assert.Equal(t, "target.example.com", rec.Target)
		assert.Equal(t, "example.com.", rec.Header().Name)
	})
}

func TestNameRecord_MarshalRData(t *testing.T) {
	h := dns.NewRRHeader("example.com.", dns.ClassIN, 300)
	rec := dns.NewCNAMERecord(h, "www.example.com")

	data, err := rec.MarshalRData()
	require.NoError(t, err)

	// Verify it's a valid DNS name encoding
	// "www" (3) + "example" (7) + "com" (3) + null terminator
	assert.NotEmpty(t, data)
	assert.Equal(t, byte(3), data[0]) // length of "www"
}

func TestParseNameRData(t *testing.T) {
	// Encode "www.example.com"
	encoded, err := dns.EncodeName("www.example.com")
	require.NoError(t, err)

	off := 0
	rec, err := dns.ParseNameRData(encoded, &off, 0, len(encoded), dns.TypeCNAME)
	require.NoError(t, err)
	assert.Equal(t, "www.example.com", rec.Target)
	assert.Equal(t, dns.TypeCNAME, rec.Type())
}

func TestNameRecord_SetHeader(t *testing.T) {
	rec := &dns.NameRecord{T: dns.TypeNS, Target: "ns1.example.com."}
	h := dns.NewRRHeader("test.com.", dns.ClassIN, 600)
	rec.SetHeader(h)

	assert.Equal(t, "test.com.", rec.Header().Name)
	assert.Equal(t, uint16(dns.ClassIN), rec.Header().Class)
	assert.Equal(t, uint32(600), rec.Header().TTL)
}
