package zone_test

import (
	"testing"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/jroosing/hydradns/internal/zone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseText_OPTRecord_DefaultTTLAndGenericRData(t *testing.T) {
	zoneText := `
$ORIGIN example.com.
$TTL 3600
@ 1232 OPT \# 4 DEADBEEF
`
	z, err := zone.ParseText(zoneText)
	require.NoError(t, err)
	require.Len(t, z.Records, 1)

	rr := z.Records[0]
	assert.Equal(t, "example.com", rr.Name)
	assert.Equal(t, uint16(dns.TypeOPT), rr.Type)
	assert.Equal(t, uint16(1232), rr.Class)
	assert.Equal(t, uint32(0), rr.TTL, "OPT TTL defaults to 0")
	assert.Equal(t, []byte{0xDE, 0xAD, 0xBE, 0xEF}, rr.RData)
}

func TestParseText_OPTRecord_WithTTLAndUDPSize(t *testing.T) {
	zoneText := `
$ORIGIN example.com.
$TTL 3600
@ 10m 4096 OPT
`
	z, err := zone.ParseText(zoneText)
	require.NoError(t, err)
	require.Len(t, z.Records, 1)

	rr := z.Records[0]
	assert.Equal(t, uint16(dns.TypeOPT), rr.Type)
	assert.Equal(t, uint16(4096), rr.Class)
	assert.Equal(t, uint32(600), rr.TTL)
	assert.Nil(t, rr.RData)
}
