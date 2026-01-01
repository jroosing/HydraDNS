package resolvers

import (
	"context"
	"testing"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/jroosing/hydradns/internal/zone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZoneResolverNXDomainAddsSOA(t *testing.T) {
	z, err := zone.ParseText(
		"$ORIGIN example.com.\n$TTL 3600\n@ IN SOA ns.example.com. host.example.com. 1 3600 600 86400 300\n",
	)
	require.NoError(t, err)
	r := NewZoneResolver([]*zone.Zone{z})
	req := dns.Packet{
		Header:    dns.Header{ID: 1, Flags: 0},
		Questions: []dns.Question{{Name: "nope.example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)}},
	}
	b, _ := req.Marshal()
	res, err := r.Resolve(context.Background(), req, b)
	require.NoError(t, err)
	resp, err := dns.ParsePacket(res.ResponseBytes)
	require.NoError(t, err)
	assert.Len(t, resp.Authorities, 1)
}

func TestZoneResolverNoZones(t *testing.T) {
	resolver := NewZoneResolver(nil)

	req := dns.Packet{
		Header: dns.Header{ID: 1234, Flags: 0x0100, QDCount: 1},
		Questions: []dns.Question{
			{Name: "example.com", Type: uint16(dns.TypeA), Class: 1},
		},
	}

	_, err := resolver.Resolve(context.Background(), req, nil)
	assert.Error(t, err, "expected error with no zones configured")
}

func TestZoneResolverNoQuestion(t *testing.T) {
	z, err := zone.ParseText(`
$ORIGIN example.com.
$TTL 3600
@  IN  A  192.0.2.1
`)
	require.NoError(t, err, "failed to parse zone")

	resolver := NewZoneResolver([]*zone.Zone{z})

	req := dns.Packet{
		Header:    dns.Header{ID: 1234, Flags: 0x0100, QDCount: 0},
		Questions: nil,
	}

	_, err = resolver.Resolve(context.Background(), req, nil)
	assert.Error(t, err, "expected error with no question")
}

func TestZoneResolverNameNotInZone(t *testing.T) {
	z, err := zone.ParseText(`
$ORIGIN example.com.
$TTL 3600
@  IN  A  192.0.2.1
`)
	require.NoError(t, err, "failed to parse zone")

	resolver := NewZoneResolver([]*zone.Zone{z})

	req := dns.Packet{
		Header: dns.Header{ID: 1234, Flags: 0x0100, QDCount: 1},
		Questions: []dns.Question{
			{Name: "other.net", Type: uint16(dns.TypeA), Class: 1},
		},
	}

	_, err = resolver.Resolve(context.Background(), req, nil)
	assert.Error(t, err, "expected error for name not in zone")
}

func TestZoneResolverLookupA(t *testing.T) {
	z, err := zone.ParseText(`
$ORIGIN example.com.
$TTL 3600
@    IN  A     192.0.2.1
www  IN  A     192.0.2.2
`)
	require.NoError(t, err, "failed to parse zone")

	resolver := NewZoneResolver([]*zone.Zone{z})

	req := dns.Packet{
		Header: dns.Header{ID: 1234, Flags: 0x0100, QDCount: 1},
		Questions: []dns.Question{
			{Name: "www.example.com", Type: uint16(dns.TypeA), Class: 1},
		},
	}

	result, err := resolver.Resolve(context.Background(), req, nil)
	require.NoError(t, err)

	assert.NotEmpty(t, result.ResponseBytes, "expected non-empty response")
	assert.Equal(t, "zone", result.Source)

	resp, err := dns.ParsePacket(result.ResponseBytes)
	require.NoError(t, err, "failed to parse response")

	assert.Len(t, resp.Answers, 1)
}

func TestZoneResolverCNAME(t *testing.T) {
	z, err := zone.ParseText(`
$ORIGIN example.com.
$TTL 3600
@    IN  A      192.0.2.1
www  IN  CNAME  @
`)
	require.NoError(t, err, "failed to parse zone")

	resolver := NewZoneResolver([]*zone.Zone{z})

	req := dns.Packet{
		Header: dns.Header{ID: 1234, Flags: 0x0100, QDCount: 1},
		Questions: []dns.Question{
			{Name: "www.example.com", Type: uint16(dns.TypeA), Class: 1},
		},
	}

	result, err := resolver.Resolve(context.Background(), req, nil)
	require.NoError(t, err)

	resp, err := dns.ParsePacket(result.ResponseBytes)
	require.NoError(t, err, "failed to parse response")

	assert.NotEmpty(t, resp.Answers, "expected at least one answer (CNAME)")
}

func TestZoneResolverClose(t *testing.T) {
	resolver := NewZoneResolver(nil)
	err := resolver.Close()
	assert.NoError(t, err)
}

func TestZoneResolverMultipleZones(t *testing.T) {
	z1, err := zone.ParseText(`
$ORIGIN example.com.
$TTL 3600
@  IN  A  192.0.2.1
`)
	require.NoError(t, err, "failed to parse zone 1")

	z2, err := zone.ParseText(`
$ORIGIN example.org.
$TTL 3600
@  IN  A  192.0.2.2
`)
	require.NoError(t, err, "failed to parse zone 2")

	resolver := NewZoneResolver([]*zone.Zone{z1, z2})

	req := dns.Packet{
		Header: dns.Header{ID: 1234, Flags: 0x0100, QDCount: 1},
		Questions: []dns.Question{
			{Name: "example.com", Type: uint16(dns.TypeA), Class: 1},
		},
	}

	result, err := resolver.Resolve(context.Background(), req, nil)
	require.NoError(t, err)

	resp, _ := dns.ParsePacket(result.ResponseBytes)
	assert.Len(t, resp.Answers, 1, "expected 1 answer for example.com")

	req.Questions[0].Name = "example.org"
	result, err = resolver.Resolve(context.Background(), req, nil)
	require.NoError(t, err)

	resp, _ = dns.ParsePacket(result.ResponseBytes)
	assert.Len(t, resp.Answers, 1, "expected 1 answer for example.org")
}

func TestZoneResolverSetsAuthoritativeFlag(t *testing.T) {
	z, err := zone.ParseText(`
$ORIGIN example.com.
$TTL 3600
@    IN  SOA   ns.example.com. admin.example.com. 1 3600 600 86400 300
@    IN  A     192.0.2.1
www  IN  A     192.0.2.2
`)
	require.NoError(t, err, "failed to parse zone")

	resolver := NewZoneResolver([]*zone.Zone{z})

	tests := []struct {
		name   string
		qname  string
		qtype  dns.RecordType
		wantAA bool
		wantQR bool
	}{
		{
			name:   "existing record sets AA flag",
			qname:  "www.example.com",
			qtype:  dns.TypeA,
			wantAA: true,
			wantQR: true,
		},
		{
			name:   "NXDOMAIN still sets AA flag",
			qname:  "nonexistent.example.com",
			qtype:  dns.TypeA,
			wantAA: true,
			wantQR: true,
		},
		{
			name:   "NODATA still sets AA flag",
			qname:  "www.example.com",
			qtype:  dns.TypeAAAA, // No AAAA record exists
			wantAA: true,
			wantQR: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := dns.Packet{
				Header: dns.Header{ID: 1234, Flags: dns.RDFlag, QDCount: 1},
				Questions: []dns.Question{
					{Name: tt.qname, Type: uint16(tt.qtype), Class: uint16(dns.ClassIN)},
				},
			}

			result, err := resolver.Resolve(context.Background(), req, nil)
			require.NoError(t, err)

			resp, err := dns.ParsePacket(result.ResponseBytes)
			require.NoError(t, err, "failed to parse response")

			// Check QR flag (response bit)
			gotQR := (resp.Header.Flags & dns.QRFlag) != 0
			assert.Equal(t, tt.wantQR, gotQR, "QR flag mismatch")

			// Check AA flag (authoritative answer)
			gotAA := (resp.Header.Flags & dns.AAFlag) != 0
			assert.Equal(t, tt.wantAA, gotAA, "AA flag mismatch (flags=0x%04x)", resp.Header.Flags)

			// Verify RD flag is preserved from request
			gotRD := (resp.Header.Flags & dns.RDFlag) != 0
			assert.True(t, gotRD, "RD flag should be preserved from request (flags=0x%04x)", resp.Header.Flags)
		})
	}
}
