package server

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/jroosing/hydradns/internal/resolvers"
	"github.com/jroosing/hydradns/internal/zone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUDPServer_ZoneAnswer(t *testing.T) {
	z, err := zone.ParseText("$ORIGIN test.local.\n$TTL 300\n@ IN SOA ns1.test.local. admin.test.local. 1 3600 600 604800 86400\n@ IN A 10.0.0.1\nwww IN A 10.0.0.2\n")
	require.NoError(t, err, "zone parse failed")

	resolver := &resolvers.Chained{Resolvers: []resolvers.Resolver{resolvers.NewZoneResolver([]*zone.Zone{z})}}
	defer resolver.Close()

	h := &QueryHandler{Resolver: resolver, Timeout: 2 * time.Second}

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	require.NoError(t, err, "listen udp failed")
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
	require.NoError(t, err, "dial udp failed")
	defer client.Close()

	req := dns.Packet{Header: dns.Header{ID: 0xABCD, Flags: uint16(dns.RDFlag)}, Questions: []dns.Question{{Name: "www.test.local", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)}}}
	b, err := req.Marshal()
	require.NoError(t, err, "marshal failed")

	_ = client.SetDeadline(time.Now().Add(2 * time.Second))
	_, err = client.Write(b)
	require.NoError(t, err, "write failed")

	buf := make([]byte, 2048)
	n, err := client.Read(buf)
	require.NoError(t, err, "read failed")

	resp, err := dns.ParsePacket(buf[:n])
	require.NoError(t, err, "parse failed")

	assert.Equal(t, uint16(0xABCD), resp.Header.ID, "transaction ID mismatch")
	assert.NotZero(t, resp.Header.Flags&uint16(dns.QRFlag), "expected QR=1")
	assert.Equal(t, dns.RCodeNoError, dns.RCodeFromFlags(resp.Header.Flags), "expected NOERROR rcode")
	require.Len(t, resp.Answers, 1, "expected 1 answer")
	assert.Equal(t, dns.TypeA, dns.RecordType(resp.Answers[0].Type), "expected A record")
}
