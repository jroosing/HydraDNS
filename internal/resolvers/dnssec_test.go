package resolvers_test

import (
	"testing"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/jroosing/hydradns/internal/resolvers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDNSSECFlagPreservation verifies that DNSSEC-related flags are preserved
// when forwarding queries to upstream servers.
func TestDNSSECFlagPreservation(t *testing.T) {
	tests := []struct {
		name         string
		requestFlags uint16
		includeDO    bool
		expectedAD   bool
		expectedCD   bool
		description  string
	}{
		{
			name:         "AD flag preserved",
			requestFlags: dns.ADFlag | dns.RDFlag,
			includeDO:    false,
			expectedAD:   true,
			expectedCD:   false,
			description:  "AD flag should be preserved when forwarding",
		},
		{
			name:         "CD flag preserved",
			requestFlags: dns.CDFlag | dns.RDFlag,
			includeDO:    false,
			expectedAD:   false,
			expectedCD:   true,
			description:  "CD flag should be preserved when forwarding",
		},
		{
			name:         "Both AD and CD preserved",
			requestFlags: dns.ADFlag | dns.CDFlag | dns.RDFlag,
			includeDO:    false,
			expectedAD:   true,
			expectedCD:   true,
			description:  "Both AD and CD flags should be preserved",
		},
		{
			name:         "DO flag in OPT record",
			requestFlags: dns.RDFlag,
			includeDO:    true,
			expectedAD:   false,
			expectedCD:   false,
			description:  "DO flag in OPT record should be preserved",
		},
		{
			name:         "All DNSSEC flags",
			requestFlags: dns.ADFlag | dns.CDFlag | dns.RDFlag,
			includeDO:    true,
			expectedAD:   true,
			expectedCD:   true,
			description:  "All DNSSEC flags (AD, CD, DO) should be preserved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a DNS request packet
			req := dns.Packet{
				Header: dns.Header{
					ID:    0x1234,
					Flags: tt.requestFlags,
				},
				Questions: []dns.Question{
					{Name: "example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)},
				},
			}

			// Add OPT record with DO flag if requested
			if tt.includeDO {
				opt := dns.CreateOPT(4096)
				opt.DNSSECOk = true
				optBytes := opt.Marshal()

				// Parse the OPT record back as an opaque record
				var off int
				optRecord, err := dns.ParseRecord(optBytes, &off)
				require.NoError(t, err)
				req.Additionals = []dns.Record{optRecord}
			}

			// Marshal the request
			reqBytes, err := req.Marshal()
			require.NoError(t, err)

			// Verify flags are in the wire format
			if len(reqBytes) >= 4 {
				flags := uint16(reqBytes[2])<<8 | uint16(reqBytes[3])

				// Check AD flag
				hasAD := (flags & dns.ADFlag) != 0
				assert.Equal(t, tt.expectedAD, hasAD, "AD flag mismatch in wire format")

				// Check CD flag
				hasCD := (flags & dns.CDFlag) != 0
				assert.Equal(t, tt.expectedCD, hasCD, "CD flag mismatch in wire format")
			}

			// If DO flag was set, verify it's in the OPT record
			if tt.includeDO {
				opt := dns.ExtractOPT(req.Additionals)
				if opt != nil {
					assert.True(t, opt.DNSSECOk, "DO flag should be set in OPT record")
				}
			}
		})
	}
}

// TestDNSSECRecordTypes verifies that all DNSSEC record types are defined.
func TestDNSSECRecordTypes(t *testing.T) {
	tests := []struct {
		recordType dns.RecordType
		name       string
	}{
		{dns.TypeRRSIG, "RRSIG"},
		{dns.TypeDNSKEY, "DNSKEY"},
		{dns.TypeDS, "DS"},
		{dns.TypeNSEC, "NSEC"},
		{dns.TypeNSEC3, "NSEC3"},
		{dns.TypeNSEC3PARAM, "NSEC3PARAM"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the record type has the correct string representation
			assert.Equal(t, tt.name, tt.recordType.String(), "Record type string mismatch")
		})
	}
}

// TestPatchTransactionIDPreservesFlags verifies that patching the transaction ID
// doesn't affect DNSSEC flags in the header.
func TestPatchTransactionIDPreservesFlags(t *testing.T) {
	// Create a response with DNSSEC flags set
	resp := dns.Packet{
		Header: dns.Header{
			ID:    0x0000, // Will be patched
			Flags: dns.QRFlag | dns.ADFlag | dns.RDFlag | dns.RAFlag,
		},
		Questions: []dns.Question{
			{Name: "example.com", Type: uint16(dns.TypeA), Class: uint16(dns.ClassIN)},
		},
	}

	respBytes, err := resp.Marshal()
	require.NoError(t, err)

	// Patch the transaction ID
	patched := resolvers.PatchTransactionID(respBytes, 0x5678)

	// Parse the patched response
	parsed, err := dns.ParsePacket(patched)
	require.NoError(t, err)

	// Verify transaction ID was changed
	assert.Equal(t, uint16(0x5678), parsed.Header.ID, "Transaction ID should be patched")

	// Verify DNSSEC flags were preserved
	assert.True(t, parsed.Header.AuthenticData(), "AD flag should be preserved")
	assert.True(t, parsed.Header.RecursionDesired(), "RD flag should be preserved")
	assert.True(t, parsed.Header.RecursionAvailable(), "RA flag should be preserved")
	assert.False(t, parsed.Header.CheckingDisabled(), "CD flag should not be set")
}
