package resolvers

import (
	"context"
	"testing"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/jroosing/hydradns/internal/zone"
)

func TestZoneResolverNXDomainAddsSOA(t *testing.T) {
	z, err := zone.ParseText("$ORIGIN example.com.\n$TTL 3600\n@ IN SOA ns.example.com. host.example.com. 1 3600 600 86400 300\n")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	r := NewZoneResolver([]*zone.Zone{z})
	req := dns.Packet{Header: dns.Header{ID: 1, Flags: 0}, Questions: []dns.Question{{Name: "nope.example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)}}}
	b, _ := req.Marshal()
	res, err := r.Resolve(context.Background(), req, b)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	resp, err := dns.ParsePacket(res.ResponseBytes)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(resp.Authorities) != 1 {
		t.Fatalf("authorities=%d", len(resp.Authorities))
	}
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
	if err == nil {
		t.Error("expected error with no zones configured")
	}
}

func TestZoneResolverNoQuestion(t *testing.T) {
	z, err := zone.ParseText(`
$ORIGIN example.com.
$TTL 3600
@  IN  A  192.0.2.1
`)
	if err != nil {
		t.Fatalf("failed to parse zone: %v", err)
	}

	resolver := NewZoneResolver([]*zone.Zone{z})

	req := dns.Packet{
		Header:    dns.Header{ID: 1234, Flags: 0x0100, QDCount: 0},
		Questions: nil,
	}

	_, err = resolver.Resolve(context.Background(), req, nil)
	if err == nil {
		t.Error("expected error with no question")
	}
}

func TestZoneResolverNameNotInZone(t *testing.T) {
	z, err := zone.ParseText(`
$ORIGIN example.com.
$TTL 3600
@  IN  A  192.0.2.1
`)
	if err != nil {
		t.Fatalf("failed to parse zone: %v", err)
	}

	resolver := NewZoneResolver([]*zone.Zone{z})

	req := dns.Packet{
		Header: dns.Header{ID: 1234, Flags: 0x0100, QDCount: 1},
		Questions: []dns.Question{
			{Name: "other.net", Type: uint16(dns.TypeA), Class: 1},
		},
	}

	_, err = resolver.Resolve(context.Background(), req, nil)
	if err == nil {
		t.Error("expected error for name not in zone")
	}
}

func TestZoneResolverLookupA(t *testing.T) {
	z, err := zone.ParseText(`
$ORIGIN example.com.
$TTL 3600
@    IN  A     192.0.2.1
www  IN  A     192.0.2.2
`)
	if err != nil {
		t.Fatalf("failed to parse zone: %v", err)
	}

	resolver := NewZoneResolver([]*zone.Zone{z})

	req := dns.Packet{
		Header: dns.Header{ID: 1234, Flags: 0x0100, QDCount: 1},
		Questions: []dns.Question{
			{Name: "www.example.com", Type: uint16(dns.TypeA), Class: 1},
		},
	}

	result, err := resolver.Resolve(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.ResponseBytes) == 0 {
		t.Error("expected non-empty response")
	}
	if result.Source != "zone" {
		t.Errorf("expected source 'zone', got %s", result.Source)
	}

	resp, err := dns.ParsePacket(result.ResponseBytes)
	if err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(resp.Answers) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(resp.Answers))
	}
}

func TestZoneResolverCNAME(t *testing.T) {
	z, err := zone.ParseText(`
$ORIGIN example.com.
$TTL 3600
@    IN  A      192.0.2.1
www  IN  CNAME  @
`)
	if err != nil {
		t.Fatalf("failed to parse zone: %v", err)
	}

	resolver := NewZoneResolver([]*zone.Zone{z})

	req := dns.Packet{
		Header: dns.Header{ID: 1234, Flags: 0x0100, QDCount: 1},
		Questions: []dns.Question{
			{Name: "www.example.com", Type: uint16(dns.TypeA), Class: 1},
		},
	}

	result, err := resolver.Resolve(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := dns.ParsePacket(result.ResponseBytes)
	if err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if len(resp.Answers) == 0 {
		t.Error("expected at least one answer (CNAME)")
	}
}

func TestZoneResolverClose(t *testing.T) {
	resolver := NewZoneResolver(nil)
	err := resolver.Close()
	if err != nil {
		t.Errorf("unexpected error from Close: %v", err)
	}
}

func TestZoneResolverMultipleZones(t *testing.T) {
	z1, err := zone.ParseText(`
$ORIGIN example.com.
$TTL 3600
@  IN  A  192.0.2.1
`)
	if err != nil {
		t.Fatalf("failed to parse zone 1: %v", err)
	}

	z2, err := zone.ParseText(`
$ORIGIN example.org.
$TTL 3600
@  IN  A  192.0.2.2
`)
	if err != nil {
		t.Fatalf("failed to parse zone 2: %v", err)
	}

	resolver := NewZoneResolver([]*zone.Zone{z1, z2})

	req := dns.Packet{
		Header: dns.Header{ID: 1234, Flags: 0x0100, QDCount: 1},
		Questions: []dns.Question{
			{Name: "example.com", Type: uint16(dns.TypeA), Class: 1},
		},
	}

	result, err := resolver.Resolve(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, _ := dns.ParsePacket(result.ResponseBytes)
	if len(resp.Answers) != 1 {
		t.Errorf("expected 1 answer for example.com, got %d", len(resp.Answers))
	}

	req.Questions[0].Name = "example.org"
	result, err = resolver.Resolve(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, _ = dns.ParsePacket(result.ResponseBytes)
	if len(resp.Answers) != 1 {
		t.Errorf("expected 1 answer for example.org, got %d", len(resp.Answers))
	}
}

func TestZoneResolverSetsAuthoritativeFlag(t *testing.T) {
	z, err := zone.ParseText(`
$ORIGIN example.com.
$TTL 3600
@    IN  SOA   ns.example.com. admin.example.com. 1 3600 600 86400 300
@    IN  A     192.0.2.1
www  IN  A     192.0.2.2
`)
	if err != nil {
		t.Fatalf("failed to parse zone: %v", err)
	}

	resolver := NewZoneResolver([]*zone.Zone{z})

	tests := []struct {
		name     string
		qname    string
		qtype    dns.RecordType
		wantAA   bool
		wantQR   bool
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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			resp, err := dns.ParsePacket(result.ResponseBytes)
			if err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}

			// Check QR flag (response bit)
			gotQR := (resp.Header.Flags & dns.QRFlag) != 0
			if gotQR != tt.wantQR {
				t.Errorf("QR flag: got %v, want %v", gotQR, tt.wantQR)
			}

			// Check AA flag (authoritative answer)
			gotAA := (resp.Header.Flags & dns.AAFlag) != 0
			if gotAA != tt.wantAA {
				t.Errorf("AA flag: got %v, want %v (flags=0x%04x)", gotAA, tt.wantAA, resp.Header.Flags)
			}

			// Verify RD flag is preserved from request
			gotRD := (resp.Header.Flags & dns.RDFlag) != 0
			if !gotRD {
				t.Errorf("RD flag should be preserved from request (flags=0x%04x)", resp.Header.Flags)
			}
		})
	}
}