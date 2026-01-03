package dns_test

import (
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
			{
				Name:  "example.com",
				Type:  uint16(dns.TypeA),
				Class: uint16(dns.ClassIN),
				TTL:   300,
				Data:  []byte{192, 0, 2, 1}, // 192.0.2.1
			},
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
	assert.Equal(t, "example.com", parsed.Answers[0].Name)
	assert.Equal(t, uint32(300), parsed.Answers[0].TTL)
}

func TestPacket_MarshalAndParse_MultipleRecordTypes(t *testing.T) {
	tests := []struct {
		name   string
		record dns.Record
	}{
		{
			name: "A record",
			record: dns.Record{
				Name: "host.example.com", Type: uint16(dns.TypeA),
				Class: uint16(dns.ClassIN), TTL: 3600,
				Data: []byte{10, 0, 0, 1},
			},
		},
		{
			name: "AAAA record",
			record: dns.Record{
				Name: "host.example.com", Type: uint16(dns.TypeAAAA),
				Class: uint16(dns.ClassIN), TTL: 3600,
				Data: []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			},
		},
		{
			name: "CNAME record",
			record: dns.Record{
				Name: "www.example.com", Type: uint16(dns.TypeCNAME),
				Class: uint16(dns.ClassIN), TTL: 3600,
				Data: "example.com",
			},
		},
		{
			name: "MX record",
			record: dns.Record{
				Name: "example.com", Type: uint16(dns.TypeMX),
				Class: uint16(dns.ClassIN), TTL: 3600,
				Data: dns.MXData{Preference: 10, Exchange: "mail.example.com"},
			},
		},
		{
			name: "NS record",
			record: dns.Record{
				Name: "example.com", Type: uint16(dns.TypeNS),
				Class: uint16(dns.ClassIN), TTL: 86400,
				Data: "ns1.example.com",
			},
		},
		{
			name: "TXT record",
			record: dns.Record{
				Name: "example.com", Type: uint16(dns.TypeTXT),
				Class: uint16(dns.ClassIN), TTL: 300,
				Data: "v=spf1 include:_spf.example.com ~all",
			},
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
			assert.Equal(t, tt.record.Name, parsed.Answers[0].Name)
			assert.Equal(t, tt.record.Type, parsed.Answers[0].Type)
			assert.Equal(t, tt.record.TTL, parsed.Answers[0].TTL)
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
			Answers: []dns.Record{{
				Name: "test.example.com", Type: uint16(dns.TypeA),
				Class: uint16(dns.ClassIN), TTL: 300, Data: addr,
			}},
		}

		data, err := pkt.Marshal()
		require.NoError(t, err)

		parsed, err := dns.ParsePacket(data)
		require.NoError(t, err)
		require.Len(t, parsed.Answers, 1)

		parsedData, ok := parsed.Answers[0].Data.([]byte)
		require.True(t, ok, "A record data should be []byte")
		assert.Equal(t, addr, parsedData)
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
			Answers: []dns.Record{{
				Name: "test.example.com", Type: uint16(dns.TypeAAAA),
				Class: uint16(dns.ClassIN), TTL: 300, Data: addr,
			}},
		}

		data, err := pkt.Marshal()
		require.NoError(t, err)

		parsed, err := dns.ParsePacket(data)
		require.NoError(t, err)
		require.Len(t, parsed.Answers, 1)

		parsedData, ok := parsed.Answers[0].Data.([]byte)
		require.True(t, ok, "AAAA record data should be []byte")
		assert.Equal(t, addr, parsedData)
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
			{
				Name:  "example.com",
				Type:  uint16(dns.TypeA),
				Class: uint16(dns.ClassIN),
				TTL:   300,
				Data:  []byte{192, 0, 2, 1},
			},
		},
		Authorities: []dns.Record{
			{
				Name:  "example.com",
				Type:  uint16(dns.TypeNS),
				Class: uint16(dns.ClassIN),
				TTL:   86400,
				Data:  "ns1.example.com",
			},
		},
		Additionals: []dns.Record{
			{
				Name:  "ns1.example.com",
				Type:  uint16(dns.TypeA),
				Class: uint16(dns.ClassIN),
				TTL:   86400,
				Data:  []byte{192, 0, 2, 2},
			},
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
	assert.Equal(t, "example.com", parsed.Authorities[0].Name)
	assert.Equal(t, uint16(dns.TypeNS), parsed.Authorities[0].Type)

	// Verify additional section
	assert.Equal(t, "ns1.example.com", parsed.Additionals[0].Name)
}
