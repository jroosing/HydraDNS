package zone

import (
	"bufio"
	"bytes"
	"errors"
	"net/netip"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jroosing/hydradns/internal/dns"
)

type Record struct {
	Name  string
	Type  uint16
	Class uint16
	TTL   uint32
	// RData depends on Type:
	// - A/AAAA: string (ip)
	// - CNAME/NS/PTR: string (fqdn)
	// - MX: MX
	// - SOA: []byte (wire format)
	// - TXT: string
	RData any
}

type MX struct {
	Preference uint16
	Exchange   string
}

type Zone struct {
	Origin     string
	DefaultTTL uint32
	Records    []Record

	// Indexes for fast lookup (built lazily on first query)
	indexBuilt  bool
	nameIndex   map[string][]int // normalized name -> indices into Records
	originLower string           // cached lowercase origin without trailing dot
}

func LoadFile(path string) (*Zone, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseText(string(b))
}

func ParseText(text string) (*Zone, error) {
	origin := ""
	defaultTTL := uint32(3600)
	lastOwner := ""
	recs := make([]Record, 0)

	for _, line := range logicalLines(text) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		upper := strings.ToUpper(line)
		if strings.HasPrefix(upper, "$ORIGIN") {
			parts := strings.Fields(line)
			if len(parts) != 2 {
				return nil, errors.New("invalid $ORIGIN directive")
			}
			origin = normalizeFQDN(parts[1], "")
			continue
		}
		if strings.HasPrefix(upper, "$TTL") {
			parts := strings.Fields(line)
			if len(parts) != 2 {
				return nil, errors.New("invalid $TTL directive")
			}
			ttl, err := parseTTL(parts[1])
			if err != nil {
				return nil, err
			}
			defaultTTL = ttl
			continue
		}
		if origin == "" {
			return nil, errors.New("zone file missing $ORIGIN")
		}

		tokens := strings.Fields(line)
		owner, rest, err := parseOwner(tokens, origin, lastOwner)
		if err != nil {
			return nil, err
		}
		lastOwner = owner
		ttl, class, typ, rdata, err := parseRRFields(rest, defaultTTL)
		if err != nil {
			return nil, err
		}
		typeCode, ok := rrTypeToCode(typ)
		if !ok {
			continue // ignore unsupported types
		}
		final, err := transformRData(typeCode, rdata, origin)
		if err != nil {
			return nil, err
		}

		recs = append(recs, Record{Name: owner, Type: typeCode, Class: class, TTL: ttl, RData: final})
	}

	z := &Zone{Origin: origin, DefaultTTL: defaultTTL, Records: recs}
	z.buildIndex() // Build index immediately after parsing
	return z, nil
}

// buildIndex creates lookup indexes for fast record queries.
func (z *Zone) buildIndex() {
	if z.indexBuilt {
		return
	}
	z.originLower = strings.ToLower(strings.TrimSuffix(z.Origin, "."))
	z.nameIndex = make(map[string][]int, len(z.Records))

	for i, rr := range z.Records {
		key := strings.ToLower(strings.TrimSuffix(rr.Name, "."))
		z.nameIndex[key] = append(z.nameIndex[key], i)
	}
	z.indexBuilt = true
}

func (z *Zone) ContainsName(qname string) bool {
	q := strings.ToLower(strings.TrimSuffix(qname, "."))
	return q == z.originLower || strings.HasSuffix(q, "."+z.originLower)
}

// NameExists checks if any records exist for the given name.
// Uses the name index for O(1) lookup instead of O(n) scan.
func (z *Zone) NameExists(qname string, qclass uint16) bool {
	q := strings.ToLower(strings.TrimSuffix(qname, "."))
	indices := z.nameIndex[q]
	for _, idx := range indices {
		if z.Records[idx].Class == qclass {
			return true
		}
	}
	return false
}

// Lookup retrieves records matching the given name, type, and class.
// Uses the name index for O(1) name lookup instead of O(n) scan.
func (z *Zone) Lookup(qname string, qtype uint16, qclass uint16) []Record {
	q := strings.ToLower(strings.TrimSuffix(qname, "."))
	indices := z.nameIndex[q]
	if len(indices) == 0 {
		return nil
	}

	out := make([]Record, 0, len(indices))
	for _, idx := range indices {
		rr := z.Records[idx]
		if rr.Class == qclass && rr.Type == qtype {
			out = append(out, rr)
		}
	}
	return out
}

// SOA returns the SOA record for this zone, or nil if not found.
// Uses the name index for fast lookup.
func (z *Zone) SOA(qclass uint16) *Record {
	indices := z.nameIndex[z.originLower]
	for _, idx := range indices {
		rr := &z.Records[idx]
		if rr.Class == qclass && rr.Type == uint16(dns.TypeSOA) {
			return rr
		}
	}
	return nil
}

// --- parsing helpers ---

func logicalLines(text string) []string {
	// Join parentheses blocks and strip ';' comments per-line before joining.
	var (
		buf     []string
		depth   int
		out     []string
		scanner = bufio.NewScanner(strings.NewReader(text))
	)
	for scanner.Scan() {
		raw := scanner.Text()
		line := stripComment(raw)
		line = strings.TrimRight(line, " \t\r\n")
		if strings.TrimSpace(line) == "" && depth == 0 {
			continue
		}
		depth += strings.Count(line, "(")
		depth -= strings.Count(line, ")")
		buf = append(buf, line)
		if depth <= 0 {
			joined := strings.Join(compactFields(buf), " ")
			buf = buf[:0]
			depth = 0
			joined = strings.ReplaceAll(joined, "(", " ")
			joined = strings.ReplaceAll(joined, ")", " ")
			joined = strings.TrimSpace(joined)
			if joined != "" {
				out = append(out, joined)
			}
		}
	}
	if len(buf) > 0 {
		return append(out, "") // force later error
	}
	return out
}

func compactFields(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, s := range lines {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

func stripComment(line string) string {
	if i := strings.IndexByte(line, ';'); i >= 0 {
		return line[:i]
	}
	return line
}

func normalizeFQDN(name string, origin string) string {
	name = strings.TrimSpace(name)
	if name == "@" {
		return strings.TrimSuffix(origin, ".")
	}
	name = strings.TrimSuffix(name, ".")
	if origin == "" {
		return name
	}
	if strings.HasSuffix(name, origin) {
		return strings.TrimSuffix(name, ".")
	}
	if strings.TrimSpace(name) == "" {
		return ""
	}
	if origin == "" {
		return name
	}
	return strings.TrimSuffix(name+"."+strings.TrimSuffix(origin, "."), ".")
}

var ttlRE = regexp.MustCompile(`^(?:\d+[wdhmsWDHMS]?)+$`)

func looksLikeTTL(tok string) bool { return ttlRE.MatchString(strings.TrimSpace(tok)) }

func parseTTL(tok string) (uint32, error) {
	tok = strings.TrimSpace(tok)
	if !ttlRE.MatchString(tok) {
		return 0, errors.New("TTL must be an integer seconds or use suffixes (w/d/h/m/s)")
	}
	// parse repeated number+unit
	total := uint32(0)
	num := ""
	for i := range len(tok) {
		c := tok[i]
		if c >= '0' && c <= '9' {
			num += string(c)
			continue
		}
		unit := byte('s')
		if c != 0 {
			unit = strings.ToLower(string(c))[0]
		}
		if num == "" {
			continue
		}
		n, err := strconv.ParseUint(num, 10, 64)
		if err != nil {
			return 0, errors.New("TTL must be an integer seconds or use suffixes (w/d/h/m/s)")
		}
		num = ""
		mul := uint64(1)
		switch unit {
		case 's':
			mul = 1
		case 'm':
			mul = 60
		case 'h':
			mul = 3600
		case 'd':
			mul = 86400
		case 'w':
			mul = 604800
		default:
			return 0, errors.New("TTL must be an integer seconds or use suffixes (w/d/h/m/s)")
		}
		if mul != 0 && n > (uint64(^uint32(0))/mul) {
			return 0, errors.New("TTL too large")
		}
		add := uint32(n * mul)
		if add > (^uint32(0) - total) {
			return 0, errors.New("TTL too large")
		}
		total += add
	}
	if num != "" {
		n, err := strconv.ParseUint(num, 10, 64)
		if err != nil {
			return 0, errors.New("TTL must be an integer seconds or use suffixes (w/d/h/m/s)")
		}
		if n > uint64(^uint32(0)) {
			return 0, errors.New("TTL too large")
		}
		add := uint32(n)
		if add > (^uint32(0) - total) {
			return 0, errors.New("TTL too large")
		}
		total += add
	}
	return total, nil
}

func looksLikeClass(tok string) bool { return strings.ToUpper(tok) == "IN" }

func looksLikeType(tok string) bool {
	s := strings.ToUpper(tok)
	switch s {
	case "A", "AAAA", "CNAME", "NS", "SOA", "MX", "TXT", "PTR":
		return true
	default:
		return false
	}
}

func parseOwner(tokens []string, origin, lastOwner string) (string, []string, error) {
	if len(tokens) == 0 {
		return "", nil, errors.New("invalid empty RR")
	}
	first := tokens[0]
	if looksLikeTTL(first) || looksLikeClass(first) || looksLikeType(first) {
		if lastOwner == "" {
			return "", nil, errors.New("owner name omitted on first RR")
		}
		return lastOwner, tokens, nil
	}
	return normalizeFQDN(first, origin), tokens[1:], nil
}

func parseRRFields(rest []string, defaultTTL uint32) (uint32, uint16, string, string, error) {
	var (
		haveTTL   bool
		haveClass bool
		idx       int
	)
	ttl := defaultTTL
	class := uint16(dns.ClassIN)
	for idx < len(rest) {
		tok := rest[idx]
		if !haveTTL && looksLikeTTL(tok) {
			n, e := parseTTL(tok)
			if e != nil {
				return 0, 0, "", "", e
			}
			ttl = n
			haveTTL = true
			idx++
			continue
		}
		if !haveClass && looksLikeClass(tok) {
			class = uint16(dns.ClassIN)
			haveClass = true
			idx++
			continue
		}
		break
	}
	if idx >= len(rest) {
		return 0, 0, "", "", errors.New("missing RR type")
	}
	typ := strings.ToUpper(rest[idx])
	idx++
	if idx >= len(rest) {
		return 0, 0, "", "", errors.New("missing RR rdata")
	}
	rdata := strings.Join(rest[idx:], " ")
	return ttl, class, typ, rdata, nil
}

func rrTypeToCode(typ string) (uint16, bool) {
	switch strings.ToUpper(typ) {
	case "A":
		return uint16(dns.TypeA), true
	case "AAAA":
		return uint16(dns.TypeAAAA), true
	case "CNAME":
		return uint16(dns.TypeCNAME), true
	case "NS":
		return uint16(dns.TypeNS), true
	case "MX":
		return uint16(dns.TypeMX), true
	case "TXT":
		return uint16(dns.TypeTXT), true
	case "PTR":
		return uint16(dns.TypePTR), true
	case "SOA":
		return uint16(dns.TypeSOA), true
	default:
		return 0, false
	}
}

func transformRData(typeCode uint16, rdata, origin string) (any, error) {
	switch dns.RecordType(typeCode) {
	case dns.TypeA:
		if _, err := netip.ParseAddr(strings.TrimSpace(rdata)); err != nil {
			return nil, errors.New("invalid IPv4 address")
		}
		return strings.TrimSpace(rdata), nil
	case dns.TypeAAAA:
		if _, err := netip.ParseAddr(strings.TrimSpace(rdata)); err != nil {
			return nil, errors.New("invalid IPv6 address")
		}
		return strings.TrimSpace(rdata), nil
	case dns.TypeMX:
		parts := strings.Fields(rdata)
		if len(parts) != 2 {
			return nil, errors.New("MX rdata must be: <preference> <exchange>")
		}
		pref, err := strconv.Atoi(parts[0])
		if err != nil || pref < 0 || pref > 65535 {
			return nil, errors.New("MX preference must be 0..65535")
		}
		ex := normalizeFQDN(parts[1], origin)
		return MX{Preference: uint16(pref), Exchange: ex}, nil
	case dns.TypeSOA:
		return parseSOARData(rdata, origin)
	case dns.TypeTXT:
		return rdata, nil
	case dns.TypePTR, dns.TypeCNAME, dns.TypeNS:
		return normalizeFQDN(rdata, origin), nil
	default:
		return rdata, nil
	}
}

func parseSOARData(rdata, origin string) ([]byte, error) {
	// MNAME RNAME SERIAL REFRESH RETRY EXPIRE MINIMUM
	parts := strings.Fields(rdata)
	if len(parts) != 7 {
		return nil, errors.New("SOA rdata must be: MNAME RNAME SERIAL REFRESH RETRY EXPIRE MINIMUM")
	}
	mname := normalizeFQDN(parts[0], origin)
	rname := normalizeFQDN(parts[1], origin)
	serial, err := parseUint32(parts[2])
	if err != nil {
		return nil, errors.New("invalid SOA serial")
	}
	refresh, err := parseTTL(parts[3])
	if err != nil {
		return nil, errors.New("invalid SOA refresh")
	}

	retryV, err := parseTTL(parts[4])
	if err != nil {
		return nil, errors.New("invalid SOA retry")
	}

	expire, err := parseTTL(parts[5])
	if err != nil {
		return nil, errors.New("invalid SOA expire")
	}

	minimum, err := parseTTL(parts[6])
	if err != nil {
		return nil, errors.New("invalid SOA minimum")
	}

	mwire, err := dns.EncodeName(mname)
	if err != nil {
		return nil, err
	}
	rwire, err := dns.EncodeName(rname)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(nil)
	buf.Write(mwire)
	buf.Write(rwire)
	w := make([]byte, 20)
	binaryPutU32(w[0:4], serial)
	binaryPutU32(w[4:8], refresh)
	binaryPutU32(w[8:12], retryV)
	binaryPutU32(w[12:16], expire)
	binaryPutU32(w[16:20], minimum)
	buf.Write(w)
	return buf.Bytes(), nil
}

func parseUint32(s string) (uint32, error) {
	v, err := strconv.ParseUint(s, 10, 32)
	return uint32(v), err
}

func binaryPutU32(dst []byte, v uint32) {
	dst[0] = byte(v >> 24)
	dst[1] = byte(v >> 16)
	dst[2] = byte(v >> 8)
	dst[3] = byte(v)
}

// DiscoverZoneFiles returns sorted list of files in dir.
func DiscoverZoneFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := make([]string, 0)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		files = append(files, dir+"/"+e.Name())
	}
	sort.Strings(files)
	return files, nil
}
