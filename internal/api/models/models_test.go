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
		CPU: models.CPUStats{
			NumCPU:      8,
			UsedPercent: 25.5,
			IdlePercent: 74.5,
		},
		Memory: models.MemoryStats{
			TotalMB:     16384.0,
			FreeMB:      8192.0,
			UsedMB:      8192.0,
			UsedPercent: 50.0,
		},
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
	assert.Equal(t, 8, decoded.CPU.NumCPU)
	assert.InDelta(t, 25.5, decoded.CPU.UsedPercent, 0.001)
	assert.InDelta(t, 50.0, decoded.Memory.UsedPercent, 0.001)
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
