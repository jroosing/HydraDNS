package main

import (
	"flag"
	"fmt"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/jroosing/hydradns/internal/dns"
)

func main() {
	var (
		server      = flag.String("server", "127.0.0.1:1053", "DNS server HOST:PORT")
		name        = flag.String("name", "tweakers.nl", "Query name")
		qtype       = flag.Int("qtype", 1, "Query type (numeric, A=1)")
		concurrency = flag.Int("concurrency", 200, "Number of concurrent workers")
		requests    = flag.Int("requests", 20000, "Total number of requests")
		timeout     = flag.Duration("timeout", 2*time.Second, "Per-request timeout")
		recvSize    = flag.Int("recv-size", 2048, "UDP receive buffer size")
	)
	flag.Parse()

	addr, err := net.ResolveUDPAddr("udp", *server)
	if err != nil {
		panic(err)
	}

	reqBytes, err := buildQuery(*name, uint16(*qtype))
	if err != nil {
		panic(err)
	}

	conc := *concurrency
	if conc < 1 {
		conc = 1
	}
	total := *requests
	if total < 1 {
		total = 1
	}
	per := total / conc
	rem := total % conc

	lat := make([]float64, 0, total)
	var latMu sync.Mutex

	t0 := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < conc; i++ {
		n := per
		if i < rem {
			n++
		}
		if n <= 0 {
			continue
		}
		wg.Add(1)
		go func(num int) {
			defer wg.Done()
			c, err := net.DialUDP("udp", nil, addr)
			if err != nil {
				return
			}
			defer c.Close()
			buf := make([]byte, *recvSize)
			for j := 0; j < num; j++ {
				start := time.Now()
				_ = c.SetDeadline(time.Now().Add(*timeout))
				_, err := c.Write(reqBytes)
				if err != nil {
					continue
				}
				nn, err := c.Read(buf)
				if err != nil {
					continue
				}
				_, _ = dns.ParsePacket(buf[:nn])
				ms := float64(time.Since(start).Microseconds()) / 1000.0
				latMu.Lock()
				lat = append(lat, ms)
				latMu.Unlock()
			}
		}(n)
	}
	wg.Wait()
	elapsed := time.Since(t0).Seconds()

	if len(lat) == 0 {
		fmt.Printf("no successful requests\n")
		return
	}
	sort.Float64s(lat)
	p50 := percentile(lat, 50)
	p95 := percentile(lat, 95)
	p99 := percentile(lat, 99)
	qps := float64(len(lat)) / elapsed

	fmt.Printf("server=%s name=%q qtype=%d concurrency=%d requests=%d\n", *server, *name, *qtype, conc, len(lat))
	fmt.Printf("elapsed_s=%.3f qps=%.1f\n", elapsed, qps)
	fmt.Printf("latency_ms p50=%.3f p95=%.3f p99=%.3f min=%.3f max=%.3f\n", p50, p95, p99, lat[0], lat[len(lat)-1])
}

func percentile(sorted []float64, p int) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[len(sorted)-1]
	}
	idx := int(float64(len(sorted))*float64(p)/100.0) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func buildQuery(name string, qtype uint16) ([]byte, error) {
	p := dns.Packet{
		Header:    dns.Header{ID: 0xBEEF, Flags: uint16(dns.RDFlag)},
		Questions: []dns.Question{{Name: name, Type: qtype, Class: uint16(dns.ClassIN)}},
	}
	return p.Marshal()
}
