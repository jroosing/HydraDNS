package dns_test

import (
	"net"
	"testing"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewIPRecord(t *testing.T) {
	t.Run("IPv4", func(t *testing.T) {
		h := dns.NewRRHeader("example.com.", dns.ClassIN, 300)
		ip := net.ParseIP("192.0.2.1")
		rec := dns.NewIPRecord(h, ip)

		assert.Equal(t, dns.TypeA, rec.Type())
		assert.Equal(t, "example.com.", rec.Header().Name)
		assert.Equal(t, uint16(dns.ClassIN), rec.Header().Class)
		assert.Equal(t, uint32(300), rec.Header().TTL)
		assert.True(t, rec.Addr.Equal(ip))
	})

	t.Run("IPv6", func(t *testing.T) {
		h := dns.NewRRHeader("example.com.", dns.ClassIN, 600)
		ip := net.ParseIP("2001:db8::1")
		rec := dns.NewIPRecord(h, ip)

		assert.Equal(t, dns.TypeAAAA, rec.Type())
		assert.Equal(t, "example.com.", rec.Header().Name)
		assert.True(t, rec.Addr.Equal(ip))
	})
}

func TestIPRecord_MarshalRData(t *testing.T) {
	t.Run("IPv4", func(t *testing.T) {
		h := dns.NewRRHeader("example.com.", dns.ClassIN, 300)
		ip := net.ParseIP("192.0.2.1")
		rec := dns.NewIPRecord(h, ip)

		data, err := rec.MarshalRData()
		require.NoError(t, err)
		assert.Equal(t, []byte{192, 0, 2, 1}, data)
	})

	t.Run("IPv6", func(t *testing.T) {
		h := dns.NewRRHeader("example.com.", dns.ClassIN, 300)
		ip := net.ParseIP("2001:db8::1")
		rec := dns.NewIPRecord(h, ip)

		data, err := rec.MarshalRData()
		require.NoError(t, err)
		assert.Len(t, data, 16)
	})
}

func TestParseIPRData(t *testing.T) {
	t.Run("IPv4", func(t *testing.T) {
		msg := []byte{192, 0, 2, 1}
		off := 0
		rec, err := dns.ParseIPRData(msg, &off, 4)
		require.NoError(t, err)
		assert.Equal(t, 4, off)
		assert.True(t, rec.Addr.Equal(net.ParseIP("192.0.2.1")))
		assert.Equal(t, dns.TypeA, rec.Type())
	})

	t.Run("IPv6", func(t *testing.T) {
		ip := net.ParseIP("2001:db8::1")
		msg := []byte(ip.To16())
		off := 0
		rec, err := dns.ParseIPRData(msg, &off, 16)
		require.NoError(t, err)
		assert.Equal(t, 16, off)
		assert.True(t, rec.Addr.Equal(ip))
		assert.Equal(t, dns.TypeAAAA, rec.Type())
	})

	t.Run("invalid length", func(t *testing.T) {
		msg := []byte{192, 0, 2}
		off := 0
		_, err := dns.ParseIPRData(msg, &off, 4)
		assert.Error(t, err)
	})
}

func TestIPRecord_SetHeader(t *testing.T) {
	rec := &dns.IPRecord{Addr: net.ParseIP("192.0.2.1")}
	h := dns.NewRRHeader("test.com.", dns.ClassIN, 600)
	rec.SetHeader(h)

	assert.Equal(t, "test.com.", rec.Header().Name)
	assert.Equal(t, uint16(dns.ClassIN), rec.Header().Class)
	assert.Equal(t, uint32(600), rec.Header().TTL)
}
