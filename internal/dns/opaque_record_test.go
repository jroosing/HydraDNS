package dns_test

import (
	"testing"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOpaqueRecord(t *testing.T) {
	h := dns.NewRRHeader("example.com.", dns.ClassIN, 300)
	data := []byte{0x01, 0x02, 0x03, 0x04}
	rec := dns.NewOpaqueRecord(h, dns.RecordType(99), data)

	assert.Equal(t, dns.RecordType(99), rec.Type())
	assert.Equal(t, "example.com.", rec.Header().Name)
	assert.Equal(t, data, rec.Data)
}

func TestOpaqueRecord_MarshalRData(t *testing.T) {
	t.Run("with data", func(t *testing.T) {
		h := dns.NewRRHeader("example.com.", dns.ClassIN, 300)
		data := []byte{0xAB, 0xCD, 0xEF}
		rec := dns.NewOpaqueRecord(h, dns.RecordType(99), data)

		out, err := rec.MarshalRData()
		require.NoError(t, err)
		assert.Equal(t, data, out)
	})

	t.Run("nil data", func(t *testing.T) {
		h := dns.NewRRHeader("example.com.", dns.ClassIN, 300)
		rec := dns.NewOpaqueRecord(h, dns.RecordType(99), nil)

		out, err := rec.MarshalRData()
		require.NoError(t, err)
		assert.Nil(t, out)
	})

	t.Run("invalid data type", func(t *testing.T) {
		rec := &dns.OpaqueRecord{T: dns.RecordType(99), Data: "not bytes"}
		_, err := rec.MarshalRData()
		assert.Error(t, err)
	})
}

func TestParseOpaqueRData(t *testing.T) {
	msg := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	off := 0
	rec, err := dns.ParseOpaqueRData(msg, &off, 5, dns.RecordType(99))
	require.NoError(t, err)
	assert.Equal(t, 5, off)
	assert.Equal(t, dns.RecordType(99), rec.Type())
	data, ok := rec.Data.([]byte)
	require.True(t, ok)
	assert.Equal(t, []byte{0x01, 0x02, 0x03, 0x04, 0x05}, data)
}

func TestOpaqueRecord_SetHeader(t *testing.T) {
	rec := &dns.OpaqueRecord{T: dns.RecordType(99), Data: []byte{1, 2, 3}}
	h := dns.NewRRHeader("test.com.", dns.ClassIN, 600)
	rec.SetHeader(h)

	assert.Equal(t, "test.com.", rec.Header().Name)
	assert.Equal(t, uint16(dns.ClassIN), rec.Header().Class)
	assert.Equal(t, uint32(600), rec.Header().TTL)
}

func TestOpaqueRecord_UsedForUnknownTypes(t *testing.T) {
	// When parsing an unknown record type, it should use OpaqueRecord
	h := dns.NewRRHeader("example.com.", dns.ClassIN, 300)
	unknownType := dns.RecordType(65000)
	data := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	rec := dns.NewOpaqueRecord(h, unknownType, data)

	assert.Equal(t, unknownType, rec.Type())
	out, err := rec.MarshalRData()
	require.NoError(t, err)
	assert.Equal(t, data, out)
}
