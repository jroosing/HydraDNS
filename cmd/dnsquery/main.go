package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jroosing/hydradns/internal/dns"
)

func main() {
	var (
		server   = flag.String("server", "8.8.8.8:53", "DNS server HOST:PORT")
		name     = flag.String("name", "example.com", "Query name")
		qtype    = flag.Int("qtype", 1, "Query type (numeric, A=1)")
		timeout  = flag.Duration("timeout", 2*time.Second, "Timeout")
		recvSize = flag.Int("recv-size", 2048, "UDP receive buffer size")
		quiet    = flag.Bool("quiet", false, "Suppress output (exit status indicates success)")
	)
	flag.Parse()

	resp, err := queryUDP(*server, *name, uint16(*qtype), *timeout, *recvSize)
	if err != nil {
		if !*quiet {
			fmt.Fprintf(os.Stderr, "dnsquery error: %v\n", err)
		}
		os.Exit(1)
	}
	if *quiet {
		return
	}

	p, err := dns.ParsePacket(resp)
	if err != nil {
		fmt.Printf("received %d bytes (unparseable)\n", len(resp))
		return
	}

	fmt.Printf("id=%d rcode=%d answers=%d authorities=%d additionals=%d\n",
		p.Header.ID,
		dns.RCodeFromFlags(p.Header.Flags),
		len(p.Answers),
		len(p.Authorities),
		len(p.Additionals),
	)

	rows := make([]string, 0, len(p.Answers))
	for _, rr := range p.Answers {
		rows = append(rows, formatRR(rr))
	}
	sort.Strings(rows)
	for _, s := range rows {
		fmt.Println(s)
	}
}

func queryUDP(server, name string, qtype uint16, timeout time.Duration, recvSize int) ([]byte, error) {
	addr, err := net.ResolveUDPAddr("udp", server)
	if err != nil {
		return nil, err
	}
	c, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	reqBytes, err := buildQuery(name, qtype)
	if err != nil {
		return nil, err
	}
	_ = c.SetDeadline(time.Now().Add(timeout))
	if _, err := c.Write(reqBytes); err != nil {
		return nil, err
	}
	buf := make([]byte, recvSize)
	n, err := c.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func buildQuery(name string, qtype uint16) ([]byte, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("name required")
	}
	// RD=1
	flags := uint16(dns.RDFlag)
	p := dns.Packet{
		Header:    dns.Header{ID: uint16(time.Now().UnixNano()), Flags: flags},
		Questions: []dns.Question{{Name: strings.TrimSuffix(name, "."), Type: qtype, Class: uint16(dns.ClassIN)}},
	}
	b, err := p.Marshal()
	if err != nil {
		return nil, err
	}
	// make ID non-zero but stable-ish length; keep 16-bit
	id := binary.BigEndian.Uint16(b[0:2])
	if id == 0 {
		binary.BigEndian.PutUint16(b[0:2], 0x1234)
	}
	return b, nil
}

func formatRR(rr dns.Record) string {
	name := rr.Name
	if name == "" {
		name = "."
	}
	switch dns.RecordType(rr.Type) {
	case dns.TypeA:
		if b, ok := rr.Data.([]byte); ok && len(b) == 4 {
			return fmt.Sprintf("%s %d IN A %d.%d.%d.%d", name, rr.TTL, b[0], b[1], b[2], b[3])
		}
	case dns.TypeAAAA:
		if b, ok := rr.Data.([]byte); ok && len(b) == 16 {
			ip := net.IP(b)
			return fmt.Sprintf("%s %d IN AAAA %s", name, rr.TTL, ip.String())
		}
	case dns.TypeCNAME:
		if s, ok := rr.Data.(string); ok {
			return fmt.Sprintf("%s %d IN CNAME %s", name, rr.TTL, s)
		}
	}
	return fmt.Sprintf("%s %d IN TYPE%d (unparsed)", name, rr.TTL, rr.Type)
}
