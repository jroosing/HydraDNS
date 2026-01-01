package zone_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/jroosing/hydradns/internal/zone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Zone File Parsing Tests
// =============================================================================

func TestParseText_MinimalZone(t *testing.T) {
	zoneText := `
$ORIGIN example.com.
$TTL 3600
@  IN  A  192.0.2.1
`
	z, err := zone.ParseText(zoneText)
	require.NoError(t, err, "ParseText should succeed")

	assert.Equal(t, "example.com", z.Origin, "Origin should be set")
	assert.Equal(t, uint32(3600), z.DefaultTTL, "Default TTL should be set")
	assert.Len(t, z.Records, 1, "Should have 1 record")
}

func TestParseText_MultipleRecordTypes(t *testing.T) {
	zoneText := `
$ORIGIN example.com.
$TTL 3600

@       IN  SOA   ns1.example.com. hostmaster.example.com. 2024010101 3600 900 604800 86400
@       IN  NS    ns1.example.com.
@       IN  NS    ns2.example.com.
@       IN  A     192.0.2.1
@       IN  AAAA  2001:db8::1
@       IN  MX    10 mail.example.com.
@       IN  TXT   "v=spf1 include:_spf.example.com ~all"

www     IN  A     192.0.2.2
www     IN  AAAA  2001:db8::2
mail    IN  A     192.0.2.3
ftp     IN  CNAME www.example.com.
`
	z, err := zone.ParseText(zoneText)
	require.NoError(t, err, "ParseText should succeed")

	// Count records by type
	typeCounts := make(map[uint16]int)
	for _, rec := range z.Records {
		typeCounts[rec.Type]++
	}

	assert.Equal(t, 1, typeCounts[uint16(dns.TypeSOA)], "Should have 1 SOA record")
	assert.Equal(t, 2, typeCounts[uint16(dns.TypeNS)], "Should have 2 NS records")
	assert.Equal(t, 3, typeCounts[uint16(dns.TypeA)], "Should have 3 A records")
	assert.Equal(t, 2, typeCounts[uint16(dns.TypeAAAA)], "Should have 2 AAAA records")
	assert.Equal(t, 1, typeCounts[uint16(dns.TypeMX)], "Should have 1 MX record")
	assert.Equal(t, 1, typeCounts[uint16(dns.TypeTXT)], "Should have 1 TXT record")
	assert.Equal(t, 1, typeCounts[uint16(dns.TypeCNAME)], "Should have 1 CNAME record")
}

func TestParseText_MissingOrigin_ReturnsError(t *testing.T) {
	zoneText := `
$TTL 3600
@  IN  A  192.0.2.1
`
	_, err := zone.ParseText(zoneText)
	assert.Error(t, err, "Should fail without $ORIGIN")
}

func TestParseText_InvalidTTL_ReturnsError(t *testing.T) {
	zoneText := `
$ORIGIN example.com.
$TTL notanumber
@  IN  A  192.0.2.1
`
	_, err := zone.ParseText(zoneText)
	assert.Error(t, err, "Should fail with invalid TTL")
}

func TestParseText_CommentsAreIgnored(t *testing.T) {
	zoneText := `
$ORIGIN example.com.
$TTL 3600
; This is a comment
@  IN  A  192.0.2.1  ; Inline comment
www IN A 192.0.2.2
`
	z, err := zone.ParseText(zoneText)
	require.NoError(t, err)
	assert.Len(t, z.Records, 2, "Comments should be ignored")
}

func TestParseText_TTLInHumanFormat(t *testing.T) {
	tests := []struct {
		name    string
		ttlStr  string
		wantTTL uint32
	}{
		{"seconds", "300", 300},
		{"minutes", "5m", 300},
		{"hours", "1h", 3600},
		{"days", "1d", 86400},
		{"weeks", "1w", 604800},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zoneText := "$ORIGIN example.com.\n$TTL " + tt.ttlStr + "\n@ IN A 192.0.2.1"
			z, err := zone.ParseText(zoneText)
			require.NoError(t, err)
			assert.Equal(t, tt.wantTTL, z.DefaultTTL)
		})
	}
}

// =============================================================================
// Zone Lookup Tests
// =============================================================================

func TestZone_Lookup_ExactMatch(t *testing.T) {
	zoneText := `
$ORIGIN example.com.
$TTL 3600
www  IN  A  192.0.2.1
www  IN  A  192.0.2.2
`
	z, err := zone.ParseText(zoneText)
	require.NoError(t, err)

	records := z.Lookup("www.example.com", uint16(dns.TypeA), uint16(dns.ClassIN))
	assert.Len(t, records, 2, "Should find 2 A records for www")
}

func TestZone_Lookup_CaseInsensitive(t *testing.T) {
	zoneText := `
$ORIGIN example.com.
$TTL 3600
WWW  IN  A  192.0.2.1
`
	z, err := zone.ParseText(zoneText)
	require.NoError(t, err)

	// Query with different cases
	cases := []string{"www.example.com", "WWW.EXAMPLE.COM", "Www.Example.Com"}
	for _, qname := range cases {
		records := z.Lookup(qname, uint16(dns.TypeA), uint16(dns.ClassIN))
		assert.Len(t, records, 1, "Lookup should be case-insensitive for %s", qname)
	}
}

func TestZone_Lookup_NoMatch(t *testing.T) {
	zoneText := `
$ORIGIN example.com.
$TTL 3600
www  IN  A  192.0.2.1
`
	z, err := zone.ParseText(zoneText)
	require.NoError(t, err)

	// Nonexistent name
	records := z.Lookup("ftp.example.com", uint16(dns.TypeA), uint16(dns.ClassIN))
	assert.Empty(t, records, "Should return empty for nonexistent name")

	// Wrong type
	records = z.Lookup("www.example.com", uint16(dns.TypeAAAA), uint16(dns.ClassIN))
	assert.Empty(t, records, "Should return empty for wrong type")
}

func TestZone_Lookup_TrailingDotHandling(t *testing.T) {
	zoneText := `
$ORIGIN example.com.
$TTL 3600
www  IN  A  192.0.2.1
`
	z, err := zone.ParseText(zoneText)
	require.NoError(t, err)

	// Query with and without trailing dot
	r1 := z.Lookup("www.example.com", uint16(dns.TypeA), uint16(dns.ClassIN))
	r2 := z.Lookup("www.example.com.", uint16(dns.TypeA), uint16(dns.ClassIN))

	assert.Len(t, r1, 1, "Should find record without trailing dot")
	assert.Len(t, r2, 1, "Should find record with trailing dot")
}

// =============================================================================
// Zone Containment Tests
// =============================================================================

func TestZone_ContainsName(t *testing.T) {
	zoneText := `
$ORIGIN example.com.
$TTL 3600
@    IN  A  192.0.2.1
www  IN  A  192.0.2.2
`
	z, err := zone.ParseText(zoneText)
	require.NoError(t, err)

	tests := []struct {
		name     string
		qname    string
		contains bool
	}{
		{"apex", "example.com", true},
		{"subdomain", "www.example.com", true},
		{"deep subdomain", "deep.sub.example.com", true},
		{"different domain", "example.org", false},
		{"partial match", "notexample.com", false},
		{"suffix attack", "malicious-example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := z.ContainsName(tt.qname)
			assert.Equal(t, tt.contains, result)
		})
	}
}

// =============================================================================
// Zone SOA Tests
// =============================================================================

func TestZone_SOA(t *testing.T) {
	zoneText := `
$ORIGIN example.com.
$TTL 3600
@  IN  SOA ns1.example.com. admin.example.com. 1 3600 900 604800 86400
@  IN  A   192.0.2.1
`
	z, err := zone.ParseText(zoneText)
	require.NoError(t, err)

	soa := z.SOA(uint16(dns.ClassIN))
	require.NotNil(t, soa, "SOA record should be found")
	assert.Equal(t, uint16(dns.TypeSOA), soa.Type)
}

func TestZone_SOA_NotPresent(t *testing.T) {
	zoneText := `
$ORIGIN example.com.
$TTL 3600
@  IN  A  192.0.2.1
`
	z, err := zone.ParseText(zoneText)
	require.NoError(t, err)

	soa := z.SOA(uint16(dns.ClassIN))
	assert.Nil(t, soa, "SOA should be nil when not present")
}

// =============================================================================
// Zone Name Existence Tests
// =============================================================================

func TestZone_NameExists(t *testing.T) {
	zoneText := `
$ORIGIN example.com.
$TTL 3600
@    IN  A     192.0.2.1
www  IN  CNAME @
`
	z, err := zone.ParseText(zoneText)
	require.NoError(t, err)

	assert.True(t, z.NameExists("example.com", uint16(dns.ClassIN)), "Apex should exist")
	assert.True(t, z.NameExists("www.example.com", uint16(dns.ClassIN)), "www should exist")
	assert.False(t, z.NameExists("ftp.example.com", uint16(dns.ClassIN)), "ftp should not exist")
}

// =============================================================================
// Zone File Loading Tests
// =============================================================================

func TestLoadFile_ValidFile(t *testing.T) {
	content := `
$ORIGIN test.local.
$TTL 300
@  IN  A  10.0.0.1
`
	tmpFile := filepath.Join(t.TempDir(), "test.zone")
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	z, err := zone.LoadFile(tmpFile)
	require.NoError(t, err)
	assert.Equal(t, "test.local", z.Origin)
	assert.Len(t, z.Records, 1)
}

func TestLoadFile_NonexistentFile(t *testing.T) {
	_, err := zone.LoadFile("/nonexistent/path/to/zone.file")
	assert.Error(t, err, "Should fail for nonexistent file")
}

// =============================================================================
// Zone Record Data Tests
// =============================================================================

func TestZone_ARecord_Data(t *testing.T) {
	zoneText := `
$ORIGIN example.com.
$TTL 3600
www  IN  A  192.0.2.1
`
	z, err := zone.ParseText(zoneText)
	require.NoError(t, err)

	records := z.Lookup("www.example.com", uint16(dns.TypeA), uint16(dns.ClassIN))
	require.Len(t, records, 1)

	ip, ok := records[0].RData.(string)
	require.True(t, ok, "A record RData should be string")
	assert.Equal(t, "192.0.2.1", ip)
}

func TestZone_MXRecord_Data(t *testing.T) {
	zoneText := `
$ORIGIN example.com.
$TTL 3600
@  IN  MX  10 mail.example.com.
@  IN  MX  20 backup.example.com.
`
	z, err := zone.ParseText(zoneText)
	require.NoError(t, err)

	records := z.Lookup("example.com", uint16(dns.TypeMX), uint16(dns.ClassIN))
	require.Len(t, records, 2)

	// Check MX data
	mx1, ok := records[0].RData.(zone.MX)
	require.True(t, ok, "MX record RData should be zone.MX")
	assert.Equal(t, uint16(10), mx1.Preference)
	assert.Contains(t, mx1.Exchange, "mail")
}

func TestZone_CNAMERecord_Data(t *testing.T) {
	zoneText := `
$ORIGIN example.com.
$TTL 3600
www   IN  CNAME  web.example.com.
`
	z, err := zone.ParseText(zoneText)
	require.NoError(t, err)

	records := z.Lookup("www.example.com", uint16(dns.TypeCNAME), uint16(dns.ClassIN))
	require.Len(t, records, 1)

	target, ok := records[0].RData.(string)
	require.True(t, ok, "CNAME record RData should be string")
	assert.Contains(t, target, "web.example.com")
}

// =============================================================================
// Zone @ Symbol Tests
// =============================================================================

func TestZone_AtSymbolExpandsToOrigin(t *testing.T) {
	zoneText := `
$ORIGIN example.com.
$TTL 3600
@  IN  A  192.0.2.1
`
	z, err := zone.ParseText(zoneText)
	require.NoError(t, err)

	// @ should expand to the origin
	records := z.Lookup("example.com", uint16(dns.TypeA), uint16(dns.ClassIN))
	assert.Len(t, records, 1, "@ should be expanded to origin")
}

// =============================================================================
// Zone Relative Name Tests
// =============================================================================

func TestZone_RelativeNamesExpandedWithOrigin(t *testing.T) {
	zoneText := `
$ORIGIN example.com.
$TTL 3600
www  IN  A  192.0.2.1
sub  IN  A  192.0.2.2
`
	z, err := zone.ParseText(zoneText)
	require.NoError(t, err)

	// Relative names should be fully qualified with origin
	records := z.Lookup("www.example.com", uint16(dns.TypeA), uint16(dns.ClassIN))
	assert.Len(t, records, 1, "Relative name www should expand to www.example.com")

	records = z.Lookup("sub.example.com", uint16(dns.TypeA), uint16(dns.ClassIN))
	assert.Len(t, records, 1, "Relative name sub should expand to sub.example.com")
}
