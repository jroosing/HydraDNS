package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/jroosing/hydradns/internal/zone"
)

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "Usage: print-zone path/to/zonefile\n")
		os.Exit(2)
	}
	path := flag.Arg(0)
	z, err := zone.LoadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load zone: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("ORIGIN: %s\n", z.Origin)
	fmt.Printf("DEFAULT_TTL: %d\n", z.DefaultTTL)
	fmt.Println("RECORDS:")

	recs := append([]zone.Record(nil), z.Records...)
	sort.Slice(recs, func(i, j int) bool {
		a, b := recs[i], recs[j]
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		if a.Class != b.Class {
			return a.Class < b.Class
		}
		if a.TTL != b.TTL {
			return a.TTL < b.TTL
		}
		return fmt.Sprintf("%v", a.RData) < fmt.Sprintf("%v", b.RData)
	})

	for _, rr := range recs {
		rdata := rr.RData
		if b, ok := rdata.([]byte); ok {
			rdata = string(b)
		}
		tname := typeName(rr.Type)
		fmt.Printf("  %s %d IN %s %v\n", rr.Name, rr.TTL, tname, rdata)
	}
}

func typeName(code uint16) string {
	switch dns.RecordType(code) {
	case dns.TypeA:
		return "A"
	case dns.TypeAAAA:
		return "AAAA"
	case dns.TypeCNAME:
		return "CNAME"
	case dns.TypeNS:
		return "NS"
	case dns.TypeMX:
		return "MX"
	case dns.TypeTXT:
		return "TXT"
	case dns.TypePTR:
		return "PTR"
	case dns.TypeSOA:
		return "SOA"
	default:
		return fmt.Sprintf("TYPE%d", code)
	}
}
