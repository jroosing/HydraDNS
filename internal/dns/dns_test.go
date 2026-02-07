package dns_test

import (
	"encoding/binary"
	"testing"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// DNS Packet Round-Trip Tests
// =============================================================================

func TestPacket_MarshalAndParse_SimpleQuery(t *testing.T) {
	// Create a simple A record query
	query := dns.Packet{
		Header: dns.Header{
			ID:    0x1234,
			Flags: dns.RDFlag, // Recursion Desired
		},
		Questions: []dns.Question{
			{Name: "example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)},
		},
	}

	// Marshal to wire format
	data, err := query.Marshal()
	require.NoError(t, err, "Marshal should succeed")
	require.NotEmpty(t, data, "Marshal should produce non-empty output")

	// Parse back
	parsed, err := dns.ParsePacket(data)
	require.NoError(t, err, "ParsePacket should succeed")

	// Verify the packet was preserved
	assert.Equal(t, query.Header.ID, parsed.Header.ID, "ID should be preserved")
	assert.Equal(t, query.Header.Flags, parsed.Header.Flags, "Flags should be preserved")
	require.Len(t, parsed.Questions, 1, "Should have 1 question")
	assert.Equal(t, "example.com", parsed.Questions[0].Name, "Question name should be preserved")
	assert.Equal(t, uint16(dns.TypeA), parsed.Questions[0].Type, "Question type should be preserved")
}

func TestPacket_MarshalAndParse_Response(t *testing.T) {
	// Create a response with answers
	response := dns.Packet{
		Header: dns.Header{
			ID:    0xABCD,
			Flags: dns.QRFlag | dns.AAFlag | dns.RDFlag | dns.RAFlag, // Response, Authoritative, RD, RA
		},
		Questions: []dns.Question{
			{Name: "example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)},
		},
		Answers: []dns.Record{
			dns.NewIPRecord(
				dns.NewRRHeader("example.com", dns.ClassIN, 300),
				[]byte{192, 0, 2, 1}, // 192.0.2.1
			),
		},
	}

	data, err := response.Marshal()
	require.NoError(t, err)

	parsed, err := dns.ParsePacket(data)
	require.NoError(t, err)

	assert.Equal(t, response.Header.ID, parsed.Header.ID)
	assert.NotEqual(t, 0, parsed.Header.Flags&dns.QRFlag, "QR flag should be set")
	assert.NotEqual(t, 0, parsed.Header.Flags&dns.AAFlag, "AA flag should be set")
	require.Len(t, parsed.Answers, 1, "Should have 1 answer")

	// Type assert to IPRecord to check fields
	ipRec, ok := parsed.Answers[0].(*dns.IPRecord)
	require.True(t, ok, "Answer should be an IPRecord")
	assert.Equal(t, "example.com", ipRec.Header().Name)
	assert.Equal(t, uint32(300), ipRec.Header().TTL)
}

func TestPacket_MarshalAndParse_MultipleRecordTypes(t *testing.T) {
	tests := []struct {
		name   string
		record dns.Record
	}{
		{
			name: "A record",
			record: dns.NewIPRecord(
				dns.NewRRHeader("host.example.com", dns.ClassIN, 3600),
				[]byte{10, 0, 0, 1},
			),
		},
		{
			name: "AAAA record",
			record: dns.NewIPRecord(
				dns.NewRRHeader("host.example.com", dns.ClassIN, 3600),
				[]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			),
		},
		{
			name: "CNAME record",
			record: dns.NewNameRecord(
				dns.NewRRHeader("www.example.com", dns.ClassIN, 3600),
				dns.TypeCNAME,
				"example.com",
			),
		},
		{
			name: "NS record",
			record: dns.NewNameRecord(
				dns.NewRRHeader("example.com", dns.ClassIN, 86400),
				dns.TypeNS,
				"ns1.example.com",
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkt := dns.Packet{
				Header:  dns.Header{ID: 1, Flags: dns.QRFlag},
				Answers: []dns.Record{tt.record},
			}

			data, err := pkt.Marshal()
			require.NoError(t, err, "Marshal should succeed for %s", tt.name)

			parsed, err := dns.ParsePacket(data)
			require.NoError(t, err, "Parse should succeed for %s", tt.name)

			require.Len(t, parsed.Answers, 1)
			expected := tt.record.Header()
			actual := parsed.Answers[0].Header()
			assert.Equal(t, expected.Name, actual.Name)
			assert.Equal(t, tt.record.Type(), parsed.Answers[0].Type())
			assert.Equal(t, expected.TTL, actual.TTL)
		})
	}
}

// =============================================================================
// DNS Header Flag Tests
// =============================================================================

func TestHeader_Flags(t *testing.T) {
	tests := []struct {
		name    string
		flags   uint16
		isQuery bool
		isAuth  bool
		isTrunc bool
		wantRD  bool
		wantRA  bool
		rcode   dns.RCode
	}{
		{
			name:    "standard query",
			flags:   dns.RDFlag,
			isQuery: true,
			wantRD:  true,
			rcode:   dns.RCodeNoError,
		},
		{
			name:    "authoritative response",
			flags:   dns.QRFlag | dns.AAFlag | dns.RDFlag | dns.RAFlag,
			isQuery: false,
			isAuth:  true,
			wantRD:  true,
			wantRA:  true,
			rcode:   dns.RCodeNoError,
		},
		{
			name:    "truncated response",
			flags:   dns.QRFlag | dns.TCFlag,
			isQuery: false,
			isTrunc: true,
			rcode:   dns.RCodeNoError,
		},
		{
			name:    "NXDOMAIN response",
			flags:   dns.QRFlag | dns.AAFlag | uint16(dns.RCodeNXDomain),
			isQuery: false,
			isAuth:  true,
			rcode:   dns.RCodeNXDomain,
		},
		{
			name:    "SERVFAIL response",
			flags:   dns.QRFlag | uint16(dns.RCodeServFail),
			isQuery: false,
			rcode:   dns.RCodeServFail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := dns.Header{ID: 1234, Flags: tt.flags}

			data, err := header.Marshal()
			require.NoError(t, err)

			var off int
			parsed, err := dns.ParseHeader(data, &off)
			require.NoError(t, err)

			// Check flags
			isQuery := (parsed.Flags & dns.QRFlag) == 0
			assert.Equal(t, tt.isQuery, isQuery, "Query/Response flag mismatch")

			isAuth := (parsed.Flags & dns.AAFlag) != 0
			assert.Equal(t, tt.isAuth, isAuth, "Authoritative flag mismatch")

			isTrunc := (parsed.Flags & dns.TCFlag) != 0
			assert.Equal(t, tt.isTrunc, isTrunc, "Truncated flag mismatch")

			hasRD := (parsed.Flags & dns.RDFlag) != 0
			assert.Equal(t, tt.wantRD, hasRD, "Recursion Desired flag mismatch")

			hasRA := (parsed.Flags & dns.RAFlag) != 0
			assert.Equal(t, tt.wantRA, hasRA, "Recursion Available flag mismatch")

			rcode := dns.RCodeFromFlags(parsed.Flags)
			assert.Equal(t, tt.rcode, rcode, "RCODE mismatch")
		})
	}
}

// =============================================================================
// DNS Name Encoding Tests
// =============================================================================

func TestEncodeName_ValidNames(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantLen  int // Expected wire format length
		wantBack string
	}{
		{"root domain", ".", 1, ""},                         // Root decodes to empty string
		{"simple domain", "example.com", 13, "example.com"}, // 7+example + 3+com + 1+null
		{"subdomain", "www.example.com", 17, "www.example.com"},
		{"trailing dot", "example.com.", 13, "example.com"},
		{"single label", "localhost", 11, "localhost"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := dns.EncodeName(tt.input)
			require.NoError(t, err)
			assert.Len(t, encoded, tt.wantLen)

			// Verify round-trip
			var off int
			decoded, err := dns.DecodeName(encoded, &off)
			require.NoError(t, err)
			assert.Equal(t, tt.wantBack, decoded)
		})
	}
}

func TestEncodeName_InvalidNames(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"label too long", "a" + string(make([]byte, 64)) + ".com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := dns.EncodeName(tt.input)
			assert.Error(t, err, "Should reject invalid name: %s", tt.input)
		})
	}
}

// =============================================================================
// DNS Question Tests
// =============================================================================

func TestQuestion_MarshalAndParse(t *testing.T) {
	tests := []struct {
		name  string
		qname string
		qtype dns.RecordType
	}{
		{"A query", "example.com", dns.TypeA},
		{"AAAA query", "ipv6.example.com", dns.TypeAAAA},
		{"MX query", "example.org", dns.TypeMX},
		{"TXT query", "_dmarc.example.com", dns.TypeTXT},
		{"NS query", "example.net", dns.TypeNS},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := dns.Question{
				Name:  tt.qname,
				Type:  uint16(tt.qtype),
				Class: uint16(dns.ClassIN),
			}

			data, err := q.Marshal()
			require.NoError(t, err)

			var off int
			parsed, err := dns.ParseQuestion(data, &off)
			require.NoError(t, err)

			assert.Equal(t, tt.qname, parsed.Name)
			assert.Equal(t, uint16(tt.qtype), parsed.Type)
			assert.Equal(t, uint16(dns.ClassIN), parsed.Class)
		})
	}
}

// =============================================================================
// DNS Parsing Error Tests
// =============================================================================

func TestParsePacket_TruncatedData(t *testing.T) {
	// Valid packet first
	pkt := dns.Packet{
		Header:    dns.Header{ID: 1, Flags: 0},
		Questions: []dns.Question{{Name: "example.com", Type: 1, Class: 1}},
	}
	data, err := pkt.Marshal()
	require.NoError(t, err)

	tests := []struct {
		name string
		data []byte
	}{
		{"empty", []byte{}},
		{"partial header", data[:6]},
		{"header only, missing question", data[:12]},
		{"partial question", data[:15]},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := dns.ParsePacket(tt.data)
			assert.Error(t, err, "Should fail to parse truncated data")
		})
	}
}

// =============================================================================
// DNS Record Data Tests
// =============================================================================

func TestRecord_ARecord_IPv4Addresses(t *testing.T) {
	addresses := [][]byte{
		{127, 0, 0, 1},       // localhost
		{192, 168, 1, 1},     // private
		{8, 8, 8, 8},         // Google DNS
		{0, 0, 0, 0},         // any
		{255, 255, 255, 255}, // broadcast
	}

	for _, addr := range addresses {
		pkt := dns.Packet{
			Header: dns.Header{ID: 1, Flags: dns.QRFlag},
			Answers: []dns.Record{
				dns.NewIPRecord(
					dns.NewRRHeader("test.example.com", dns.ClassIN, 300),
					addr,
				),
			},
		}

		data, err := pkt.Marshal()
		require.NoError(t, err)

		parsed, err := dns.ParsePacket(data)
		require.NoError(t, err)
		require.Len(t, parsed.Answers, 1)

		ipRec, ok := parsed.Answers[0].(*dns.IPRecord)
		require.True(t, ok, "A record should be IPRecord")
		assert.Equal(t, addr, []byte(ipRec.Addr))
	}
}

func TestRecord_AAAARecord_IPv6Addresses(t *testing.T) {
	addresses := [][]byte{
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},             // ::1 (localhost)
		{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, // 2001:db8::1
	}

	for _, addr := range addresses {
		pkt := dns.Packet{
			Header: dns.Header{ID: 1, Flags: dns.QRFlag},
			Answers: []dns.Record{
				dns.NewIPRecord(
					dns.NewRRHeader("test.example.com", dns.ClassIN, 300),
					addr,
				),
			},
		}

		data, err := pkt.Marshal()
		require.NoError(t, err)

		parsed, err := dns.ParsePacket(data)
		require.NoError(t, err)
		require.Len(t, parsed.Answers, 1)

		ipRec, ok := parsed.Answers[0].(*dns.IPRecord)
		require.True(t, ok, "AAAA record should be IPRecord")
		assert.Equal(t, addr, []byte(ipRec.Addr))
	}
}

// =============================================================================
// DNS Packet With Authority and Additional Sections
// =============================================================================

func TestPacket_AllSections(t *testing.T) {
	pkt := dns.Packet{
		Header: dns.Header{ID: 0x5678, Flags: dns.QRFlag | dns.AAFlag},
		Questions: []dns.Question{
			{Name: "example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)},
		},
		Answers: []dns.Record{
			dns.NewIPRecord(
				dns.NewRRHeader("example.com", dns.ClassIN, 300),
				[]byte{192, 0, 2, 1},
			),
		},
		Authorities: []dns.Record{
			dns.NewNameRecord(
				dns.NewRRHeader("example.com", dns.ClassIN, 86400),
				dns.TypeNS,
				"ns1.example.com",
			),
		},
		Additionals: []dns.Record{
			dns.NewIPRecord(
				dns.NewRRHeader("ns1.example.com", dns.ClassIN, 86400),
				[]byte{192, 0, 2, 2},
			),
		},
	}

	data, err := pkt.Marshal()
	require.NoError(t, err)

	parsed, err := dns.ParsePacket(data)
	require.NoError(t, err)

	assert.Equal(t, pkt.Header.ID, parsed.Header.ID)
	assert.Len(t, parsed.Questions, 1)
	assert.Len(t, parsed.Answers, 1)
	assert.Len(t, parsed.Authorities, 1)
	assert.Len(t, parsed.Additionals, 1)

	// Verify authority section
	authRec := parsed.Authorities[0]
	assert.Equal(t, "example.com", authRec.Header().Name)
	assert.Equal(t, dns.TypeNS, authRec.Type())

	// Verify additional section
	addRec := parsed.Additionals[0]
	assert.Equal(t, "ns1.example.com", addRec.Header().Name)
}

// =============================================================================
// EDNS Option Parsing Tests
// =============================================================================

func TestParseEDNSOptions_FiltersUnknownAndOversized(t *testing.T) {
	cookieData := []byte("abcdefgh")
	unknownData := []byte{1, 2, 3, 4}
	oversized := make([]byte, dns.EDNSMaxUDPPayloadSize+1)

	rdata := make([]byte, 0)
	rdata = append(rdata, marshalTestEDNSOption(10, cookieData)...)
	rdata = append(rdata, marshalTestEDNSOption(65001, unknownData)...)
	rdata = append(rdata, marshalTestEDNSOption(12, oversized)...)

	opts := dns.ParseEDNSOptions(rdata)

	require.Len(t, opts, 1, "only allowed, in-bounds options should remain")
	assert.Equal(t, uint16(10), opts[0].Code)
	assert.Equal(t, cookieData, opts[0].Data)
}

func TestMarshalEDNSOptions_SkipsOversized(t *testing.T) {
	opts := []dns.EDNSOption{
		{Code: 10, Data: []byte("ok")},
		{Code: 10, Data: make([]byte, dns.EDNSMaxUDPPayloadSize+10)},
	}

	w := dns.MarshalEDNSOptions(opts)
	require.NotNil(t, w)
	parsed := dns.ParseEDNSOptions(w)
	require.Len(t, parsed, 1)
	assert.Equal(t, uint16(10), parsed[0].Code)
	assert.Equal(t, []byte("ok"), parsed[0].Data)
}

func marshalTestEDNSOption(code uint16, data []byte) []byte {
	buf := make([]byte, 4+len(data))
	binary.BigEndian.PutUint16(buf[0:2], code)
	binary.BigEndian.PutUint16(buf[2:4], uint16(len(data)))
	copy(buf[4:], data)
	return buf
}
