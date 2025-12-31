package server

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/jroosing/hydradns/internal/resolvers"
	"github.com/jroosing/hydradns/internal/zone"
)

func TestUDPServer_ZoneAnswer(t *testing.T) {
	z, err := zone.ParseText("$ORIGIN test.local.\n$TTL 300\n@ IN SOA ns1.test.local. admin.test.local. 1 3600 600 604800 86400\n@ IN A 10.0.0.1\nwww IN A 10.0.0.2\n")
	if err != nil {
		t.Fatalf("zone parse: %v", err)
	}
	resolver := &resolvers.Chained{Resolvers: []resolvers.Resolver{resolvers.NewZoneResolver([]*zone.Zone{z})}}
	defer resolver.Close()

	h := &QueryHandler{Resolver: resolver, Timeout: 2 * time.Second}

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	addr := conn.LocalAddr().(*net.UDPAddr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := &UDPServer{Handler: h, WorkersPerSocket: 8}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.RunOnConn(ctx, conn) }()
	defer func() {
		_ = srv.Stop(2 * time.Second)
		cancel()
		<-errCh
	}()

	client, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: addr.IP, Port: addr.Port})
	if err != nil {
		t.Fatalf("dial udp: %v", err)
	}
	defer client.Close()

	req := dns.Packet{Header: dns.Header{ID: 0xABCD, Flags: uint16(dns.RDFlag)}, Questions: []dns.Question{{Name: "www.test.local", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)}}}
	b, err := req.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	_ = client.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := client.Write(b); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, 2048)
	n, err := client.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	resp, err := dns.ParsePacket(buf[:n])
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if resp.Header.ID != 0xABCD {
		t.Fatalf("txid: got %d", resp.Header.ID)
	}
	if (resp.Header.Flags & uint16(dns.QRFlag)) == 0 {
		t.Fatalf("expected QR=1")
	}
	if dns.RCodeFromFlags(resp.Header.Flags) != dns.RCodeNoError {
		t.Fatalf("rcode=%d", dns.RCodeFromFlags(resp.Header.Flags))
	}
	if len(resp.Answers) != 1 {
		t.Fatalf("answers=%d", len(resp.Answers))
	}
	if dns.RecordType(resp.Answers[0].Type) != dns.TypeA {
		t.Fatalf("type=%d", resp.Answers[0].Type)
	}
}
