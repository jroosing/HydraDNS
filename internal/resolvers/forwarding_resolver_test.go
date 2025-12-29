package resolvers

import (
	"testing"

	"github.com/jroosing/hydradns/internal/dns"
)

func TestAnalyzeCacheDecision_PositiveMinTTL(t *testing.T) {
	resp := dns.Packet{
		Header:    dns.Header{ID: 0, Flags: uint16(dns.QRFlag)},
		Questions: []dns.Question{{Name: "example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)}},
		Answers:   []dns.Record{{Name: "example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN), TTL: 10, Data: []byte{1, 2, 3, 4}}},
	}
	b, err := resp.Marshal()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	d := analyzeCacheDecision(b)
	if d.ttlSeconds != 10 || d.entryType != CachePositive {
		t.Fatalf("got %+v", d)
	}
}
