package zone

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseZoneBasic(t *testing.T) {
	z, err := ParseText("$ORIGIN example.com.\n$TTL 3600\n@ IN A 1.2.3.4\n")
	require.NoError(t, err)
	assert.Equal(t, "example.com", z.Origin)

	rrs := z.Lookup("example.com", uint16(dns.TypeA), uint16(dns.ClassIN))
	assert.Len(t, rrs, 1)
}

func TestParseZoneMultipleRecords(t *testing.T) {
	z, err := ParseText(`
$ORIGIN example.com.
$TTL 3600
@    IN  A     192.0.2.1
@    IN  A     192.0.2.2
www  IN  A     192.0.2.3
mail IN  MX    10 mail.example.com.
`)
	require.NoError(t, err)

	// Should have 2 A records at apex
	rrs := z.Lookup("example.com", uint16(dns.TypeA), uint16(dns.ClassIN))
	assert.Len(t, rrs, 2, "expected 2 A records at apex")

	// Should have 1 A record for www
	rrs = z.Lookup("www.example.com", uint16(dns.TypeA), uint16(dns.ClassIN))
	assert.Len(t, rrs, 1, "expected 1 A record for www")

	// Should have 1 MX record
	rrs = z.Lookup("mail.example.com", uint16(dns.TypeMX), uint16(dns.ClassIN))
	assert.Len(t, rrs, 1, "expected 1 MX record")
}

func TestParseZoneWithCNAME(t *testing.T) {
	z, err := ParseText(`
$ORIGIN example.com.
$TTL 3600
@    IN  A      192.0.2.1
www  IN  CNAME  @
`)
	require.NoError(t, err)

	rrs := z.Lookup("www.example.com", uint16(dns.TypeCNAME), uint16(dns.ClassIN))
	assert.Len(t, rrs, 1, "expected 1 CNAME record")
}

func TestParseZoneWithNS(t *testing.T) {
	z, err := ParseText(`
$ORIGIN example.com.
$TTL 3600
@  IN  NS  ns1.example.com.
@  IN  NS  ns2.example.com.
`)
	require.NoError(t, err)

	rrs := z.Lookup("example.com", uint16(dns.TypeNS), uint16(dns.ClassIN))
	assert.Len(t, rrs, 2, "expected 2 NS records")
}

func TestParseZoneWithSOA(t *testing.T) {
	z, err := ParseText(`
$ORIGIN example.com.
$TTL 3600
@  IN  SOA  ns1.example.com. admin.example.com. 2024010101 3600 900 604800 86400
`)
	require.NoError(t, err)

	soa := z.SOA(uint16(dns.ClassIN))
	require.NotNil(t, soa, "expected SOA record")
}

func TestParseZoneWithAAAA(t *testing.T) {
	z, err := ParseText(`
$ORIGIN example.com.
$TTL 3600
@  IN  AAAA  2001:db8::1
`)
	require.NoError(t, err)

	rrs := z.Lookup("example.com", uint16(dns.TypeAAAA), uint16(dns.ClassIN))
	assert.Len(t, rrs, 1, "expected 1 AAAA record")
}

func TestParseZoneWithTXT(t *testing.T) {
	z, err := ParseText(`
$ORIGIN example.com.
$TTL 3600
@  IN  TXT  "v=spf1 include:_spf.example.com ~all"
`)
	require.NoError(t, err)

	rrs := z.Lookup("example.com", uint16(dns.TypeTXT), uint16(dns.ClassIN))
	assert.Len(t, rrs, 1, "expected 1 TXT record")
}

func TestZoneContainsName(t *testing.T) {
	z, err := ParseText(`
$ORIGIN example.com.
$TTL 3600
@  IN  A  192.0.2.1
`)
	require.NoError(t, err)

	assert.True(t, z.ContainsName("example.com"), "expected ContainsName to return true for apex")
	assert.True(t, z.ContainsName("www.example.com"), "expected ContainsName to return true for subdomain")
	assert.False(t, z.ContainsName("other.net"), "expected ContainsName to return false for other domain")
}

func TestZoneNameExists(t *testing.T) {
	z, err := ParseText(`
$ORIGIN example.com.
$TTL 3600
@    IN  A  192.0.2.1
www  IN  A  192.0.2.2
`)
	require.NoError(t, err)

	assert.True(t, z.NameExists("example.com", uint16(dns.ClassIN)), "expected NameExists to return true for apex")
	assert.True(t, z.NameExists("www.example.com", uint16(dns.ClassIN)), "expected NameExists to return true for www")
	assert.False(t, z.NameExists("nonexistent.example.com", uint16(dns.ClassIN)), "expected NameExists to return false for nonexistent")
}

func TestLoadFile(t *testing.T) {
	content := `
$ORIGIN test.local.
$TTL 300
@  IN  A  10.0.0.1
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.zone")
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err, "failed to write test file")

	z, err := LoadFile(path)
	require.NoError(t, err, "LoadFile failed")
	assert.Equal(t, "test.local", z.Origin)
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := LoadFile("/nonexistent/path/to/zone.file")
	assert.Error(t, err, "expected error for nonexistent file")
}

func TestParseZoneNoOrigin(t *testing.T) {
	_, err := ParseText(`
$TTL 3600
@  IN  A  192.0.2.1
`)
	assert.Error(t, err, "expected error for zone without origin")
}

func TestParseZoneComments(t *testing.T) {
	z, err := ParseText(`
; This is a comment
$ORIGIN example.com.
$TTL 3600
@  IN  A  192.0.2.1  ; inline comment
`)
	require.NoError(t, err)

	rrs := z.Lookup("example.com", uint16(dns.TypeA), uint16(dns.ClassIN))
	assert.Len(t, rrs, 1, "expected 1 record")
}

func TestParseZoneRelativeNames(t *testing.T) {
	z, err := ParseText(`
$ORIGIN example.com.
$TTL 3600
www     IN  A  192.0.2.1
mail    IN  A  192.0.2.2
`)
	require.NoError(t, err)

	// www.example.com should exist
	rrs := z.Lookup("www.example.com", uint16(dns.TypeA), uint16(dns.ClassIN))
	assert.Len(t, rrs, 1, "expected 1 record for www")

	// mail.example.com should exist
	rrs = z.Lookup("mail.example.com", uint16(dns.TypeA), uint16(dns.ClassIN))
	assert.Len(t, rrs, 1, "expected 1 record for mail")
}

func TestDiscoverZoneFiles(t *testing.T) {
	dir := t.TempDir()

	// Create some zone files
	err := os.WriteFile(filepath.Join(dir, "example.zone"), []byte("test"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "test.zone"), []byte("test"), 0644)
	require.NoError(t, err)

	files, err := DiscoverZoneFiles(dir)
	require.NoError(t, err, "DiscoverZoneFiles failed")

	// DiscoverZoneFiles returns all files, not just .zone files
	assert.GreaterOrEqual(t, len(files), 2, "expected at least 2 files")
}

func TestDiscoverZoneFilesNonexistentDir(t *testing.T) {
	files, err := DiscoverZoneFiles("/nonexistent/directory")
	// Should return an error for nonexistent directory
	assert.Error(t, err, "expected error for nonexistent directory")
	assert.Empty(t, files, "expected 0 files")
}
