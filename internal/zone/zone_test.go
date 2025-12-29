package zone

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jroosing/hydradns/internal/dns"
)

func TestParseZoneBasic(t *testing.T) {
	z, err := ParseText("$ORIGIN example.com.\n$TTL 3600\n@ IN A 1.2.3.4\n")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if z.Origin != "example.com" {
		t.Fatalf("origin=%q", z.Origin)
	}
	rrs := z.Lookup("example.com", uint16(dns.TypeA), uint16(dns.ClassIN))
	if len(rrs) != 1 {
		t.Fatalf("rrs=%d", len(rrs))
	}
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
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Should have 2 A records at apex
	rrs := z.Lookup("example.com", uint16(dns.TypeA), uint16(dns.ClassIN))
	if len(rrs) != 2 {
		t.Errorf("expected 2 A records at apex, got %d", len(rrs))
	}

	// Should have 1 A record for www
	rrs = z.Lookup("www.example.com", uint16(dns.TypeA), uint16(dns.ClassIN))
	if len(rrs) != 1 {
		t.Errorf("expected 1 A record for www, got %d", len(rrs))
	}

	// Should have 1 MX record
	rrs = z.Lookup("mail.example.com", uint16(dns.TypeMX), uint16(dns.ClassIN))
	if len(rrs) != 1 {
		t.Errorf("expected 1 MX record, got %d", len(rrs))
	}
}

func TestParseZoneWithCNAME(t *testing.T) {
	z, err := ParseText(`
$ORIGIN example.com.
$TTL 3600
@    IN  A      192.0.2.1
www  IN  CNAME  @
`)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	rrs := z.Lookup("www.example.com", uint16(dns.TypeCNAME), uint16(dns.ClassIN))
	if len(rrs) != 1 {
		t.Errorf("expected 1 CNAME record, got %d", len(rrs))
	}
}

func TestParseZoneWithNS(t *testing.T) {
	z, err := ParseText(`
$ORIGIN example.com.
$TTL 3600
@  IN  NS  ns1.example.com.
@  IN  NS  ns2.example.com.
`)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	rrs := z.Lookup("example.com", uint16(dns.TypeNS), uint16(dns.ClassIN))
	if len(rrs) != 2 {
		t.Errorf("expected 2 NS records, got %d", len(rrs))
	}
}

func TestParseZoneWithSOA(t *testing.T) {
	z, err := ParseText(`
$ORIGIN example.com.
$TTL 3600
@  IN  SOA  ns1.example.com. admin.example.com. 2024010101 3600 900 604800 86400
`)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	soa := z.SOA(uint16(dns.ClassIN))
	if soa == nil {
		t.Fatal("expected SOA record")
	}
}

func TestParseZoneWithAAAA(t *testing.T) {
	z, err := ParseText(`
$ORIGIN example.com.
$TTL 3600
@  IN  AAAA  2001:db8::1
`)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	rrs := z.Lookup("example.com", uint16(dns.TypeAAAA), uint16(dns.ClassIN))
	if len(rrs) != 1 {
		t.Errorf("expected 1 AAAA record, got %d", len(rrs))
	}
}

func TestParseZoneWithTXT(t *testing.T) {
	z, err := ParseText(`
$ORIGIN example.com.
$TTL 3600
@  IN  TXT  "v=spf1 include:_spf.example.com ~all"
`)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	rrs := z.Lookup("example.com", uint16(dns.TypeTXT), uint16(dns.ClassIN))
	if len(rrs) != 1 {
		t.Errorf("expected 1 TXT record, got %d", len(rrs))
	}
}

func TestZoneContainsName(t *testing.T) {
	z, err := ParseText(`
$ORIGIN example.com.
$TTL 3600
@  IN  A  192.0.2.1
`)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !z.ContainsName("example.com") {
		t.Error("expected ContainsName to return true for apex")
	}
	if !z.ContainsName("www.example.com") {
		t.Error("expected ContainsName to return true for subdomain")
	}
	if z.ContainsName("other.net") {
		t.Error("expected ContainsName to return false for other domain")
	}
}

func TestZoneNameExists(t *testing.T) {
	z, err := ParseText(`
$ORIGIN example.com.
$TTL 3600
@    IN  A  192.0.2.1
www  IN  A  192.0.2.2
`)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !z.NameExists("example.com", uint16(dns.ClassIN)) {
		t.Error("expected NameExists to return true for apex")
	}
	if !z.NameExists("www.example.com", uint16(dns.ClassIN)) {
		t.Error("expected NameExists to return true for www")
	}
	if z.NameExists("nonexistent.example.com", uint16(dns.ClassIN)) {
		t.Error("expected NameExists to return false for nonexistent")
	}
}

func TestLoadFile(t *testing.T) {
	content := `
$ORIGIN test.local.
$TTL 300
@  IN  A  10.0.0.1
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.zone")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	z, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	if z.Origin != "test.local" {
		t.Errorf("expected origin test.local, got %s", z.Origin)
	}
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := LoadFile("/nonexistent/path/to/zone.file")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestParseZoneNoOrigin(t *testing.T) {
	_, err := ParseText(`
$TTL 3600
@  IN  A  192.0.2.1
`)
	if err == nil {
		t.Error("expected error for zone without origin")
	}
}

func TestParseZoneComments(t *testing.T) {
	z, err := ParseText(`
; This is a comment
$ORIGIN example.com.
$TTL 3600
@  IN  A  192.0.2.1  ; inline comment
`)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	rrs := z.Lookup("example.com", uint16(dns.TypeA), uint16(dns.ClassIN))
	if len(rrs) != 1 {
		t.Errorf("expected 1 record, got %d", len(rrs))
	}
}

func TestParseZoneRelativeNames(t *testing.T) {
	z, err := ParseText(`
$ORIGIN example.com.
$TTL 3600
www     IN  A  192.0.2.1
mail    IN  A  192.0.2.2
`)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// www.example.com should exist
	rrs := z.Lookup("www.example.com", uint16(dns.TypeA), uint16(dns.ClassIN))
	if len(rrs) != 1 {
		t.Errorf("expected 1 record for www, got %d", len(rrs))
	}

	// mail.example.com should exist
	rrs = z.Lookup("mail.example.com", uint16(dns.TypeA), uint16(dns.ClassIN))
	if len(rrs) != 1 {
		t.Errorf("expected 1 record for mail, got %d", len(rrs))
	}
}

func TestDiscoverZoneFiles(t *testing.T) {
	dir := t.TempDir()

	// Create some zone files
	if err := os.WriteFile(filepath.Join(dir, "example.zone"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "test.zone"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := DiscoverZoneFiles(dir)
	if err != nil {
		t.Fatalf("DiscoverZoneFiles failed: %v", err)
	}

	// DiscoverZoneFiles returns all files, not just .zone files
	if len(files) < 2 {
		t.Errorf("expected at least 2 files, got %d", len(files))
	}
}

func TestDiscoverZoneFilesNonexistentDir(t *testing.T) {
	files, err := DiscoverZoneFiles("/nonexistent/directory")
	// Should return an error for nonexistent directory
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}
