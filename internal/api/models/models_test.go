// Package models_test provides behavior tests for the API models package.
package models_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jroosing/hydradns/internal/api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Common Models Tests
// ============================================================================

func TestErrorResponse_JSON(t *testing.T) {
	resp := models.ErrorResponse{Error: "something went wrong"}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded models.ErrorResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "something went wrong", decoded.Error)
}

func TestStatusResponse_JSON(t *testing.T) {
	resp := models.StatusResponse{Status: "ok"}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded models.StatusResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "ok", decoded.Status)
}

// ============================================================================
// Stats Models Tests
// ============================================================================

func TestServerStatsResponse_JSON(t *testing.T) {
	startTime := time.Now()
	resp := models.ServerStatsResponse{
		Uptime:        "1h30m",
		UptimeSeconds: 5400,
		StartTime:     startTime,
		GoRoutines:    42,
		MemoryAllocMB: 123.45,
		NumCPU:        8,
		DNSStats: models.DNSStatsResponse{
			QueriesTotal: 1000,
			QueriesUDP:   900,
			QueriesTCP:   100,
		},
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded models.ServerStatsResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "1h30m", decoded.Uptime)
	assert.Equal(t, int64(5400), decoded.UptimeSeconds)
	assert.Equal(t, 42, decoded.GoRoutines)
	assert.Equal(t, 8, decoded.NumCPU)
	assert.Equal(t, uint64(1000), decoded.DNSStats.QueriesTotal)
}

func TestServerStatsResponse_WithFilteringStats(t *testing.T) {
	resp := models.ServerStatsResponse{
		Uptime: "1h",
		FilteringStats: &models.FilteringStatsResponse{
			Enabled:        true,
			QueriesTotal:   500,
			QueriesBlocked: 50,
			QueriesAllowed: 450,
			WhitelistSize:  10,
			BlacklistSize:  1000,
		},
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded models.ServerStatsResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	require.NotNil(t, decoded.FilteringStats)
	assert.True(t, decoded.FilteringStats.Enabled)
	assert.Equal(t, uint64(50), decoded.FilteringStats.QueriesBlocked)
}

func TestServerStatsResponse_FilteringOmittedWhenNil(t *testing.T) {
	resp := models.ServerStatsResponse{
		Uptime:         "1h",
		FilteringStats: nil,
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	// Should not contain "filtering" key when nil
	assert.NotContains(t, string(data), `"filtering":`)
}

func TestDNSStatsResponse_JSON(t *testing.T) {
	resp := models.DNSStatsResponse{
		QueriesTotal: 10000,
		QueriesUDP:   8000,
		QueriesTCP:   2000,
		ResponsesNX:  100,
		ResponsesErr: 50,
		AvgLatencyMs: 1.5,
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded models.DNSStatsResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, uint64(10000), decoded.QueriesTotal)
	assert.InEpsilon(t, 1.5, decoded.AvgLatencyMs, 0.1)
}

// ============================================================================
// Filtering Models Tests
// ============================================================================

func TestFilteringStatsResponse_JSON(t *testing.T) {
	resp := models.FilteringStatsResponse{
		Enabled:        true,
		QueriesTotal:   1000,
		QueriesBlocked: 200,
		QueriesAllowed: 800,
		WhitelistSize:  5,
		BlacklistSize:  500,
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded models.FilteringStatsResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.True(t, decoded.Enabled)
	assert.Equal(t, uint64(200), decoded.QueriesBlocked)
}

func TestDomainListResponse_JSON(t *testing.T) {
	resp := models.DomainListResponse{
		Domains: []string{"example.com", "test.org"},
		Count:   2,
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded models.DomainListResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Len(t, decoded.Domains, 2)
	assert.Equal(t, 2, decoded.Count)
}

func TestDomainRequest_JSON(t *testing.T) {
	req := models.DomainRequest{
		Domains: []string{"ads.example.com", "tracking.test.com"},
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var decoded models.DomainRequest
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Len(t, decoded.Domains, 2)
}

func TestFilteringEnabledRequest_JSON(t *testing.T) {
	req := models.FilteringEnabledRequest{Enabled: true}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var decoded models.FilteringEnabledRequest
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.True(t, decoded.Enabled)
}

// ============================================================================
// Zone Models Tests
// ============================================================================

func TestZoneSummary_JSON(t *testing.T) {
	summary := models.ZoneSummary{
		Name:        "example.com",
		RecordCount: 15,
		FilePath:    "/etc/zones/example.com.zone",
	}

	data, err := json.Marshal(summary)
	require.NoError(t, err)

	var decoded models.ZoneSummary
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "example.com", decoded.Name)
	assert.Equal(t, 15, decoded.RecordCount)
}

func TestZoneSummary_FilePathOmittedWhenEmpty(t *testing.T) {
	summary := models.ZoneSummary{
		Name:        "example.com",
		RecordCount: 5,
		FilePath:    "",
	}

	data, err := json.Marshal(summary)
	require.NoError(t, err)

	// Should not contain "file_path" key when empty
	assert.NotContains(t, string(data), `"file_path"`)
}

func TestZoneListResponse_JSON(t *testing.T) {
	resp := models.ZoneListResponse{
		Zones: []models.ZoneSummary{
			{Name: "zone1.com", RecordCount: 10},
			{Name: "zone2.org", RecordCount: 20},
		},
		Count: 2,
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded models.ZoneListResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Len(t, decoded.Zones, 2)
	assert.Equal(t, 2, decoded.Count)
}

func TestZoneDetailResponse_JSON(t *testing.T) {
	resp := models.ZoneDetailResponse{
		Name: "example.com",
		Records: []models.ZoneRecord{
			{Name: "@", TTL: 3600, Type: "A", Value: "192.0.2.1"},
			{Name: "www", TTL: 3600, Type: "A", Value: "192.0.2.2"},
		},
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded models.ZoneDetailResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "example.com", decoded.Name)
	assert.Len(t, decoded.Records, 2)
}

func TestZoneRecord_JSON(t *testing.T) {
	record := models.ZoneRecord{
		Name:  "mail",
		TTL:   7200,
		Type:  "MX",
		Value: "10 mail.example.com",
	}

	data, err := json.Marshal(record)
	require.NoError(t, err)

	var decoded models.ZoneRecord
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "mail", decoded.Name)
	assert.Equal(t, uint32(7200), decoded.TTL)
	assert.Equal(t, "MX", decoded.Type)
}

func TestZoneCreateRequest_JSON(t *testing.T) {
	req := models.ZoneCreateRequest{
		Name: "newzone.com",
		Records: []models.ZoneRecord{
			{Name: "@", TTL: 3600, Type: "A", Value: "10.0.0.1"},
		},
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var decoded models.ZoneCreateRequest
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "newzone.com", decoded.Name)
	assert.Len(t, decoded.Records, 1)
}
