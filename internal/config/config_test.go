package config_test

import (
	"testing"

	"github.com/jroosing/hydradns/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Default Configuration Tests
// =============================================================================

func TestDefault_ReturnsValidConfig(t *testing.T) {
	cfg := config.Default()
	require.NotNil(t, cfg, "Default should return non-nil config")

	// Check server defaults
	assert.Equal(t, "0.0.0.0", cfg.Server.Host, "Default host should be 0.0.0.0")
	assert.Equal(t, 1053, cfg.Server.Port, "Default port should be 1053")
	assert.True(t, cfg.Server.EnableTCP, "TCP should be enabled by default")
	assert.True(t, cfg.Server.TCPFallback, "TCP fallback should be enabled by default")
	assert.Equal(t, "auto", cfg.Server.WorkersRaw, "Workers should default to auto")

	// Check upstream defaults
	assert.Equal(t, []string{"9.9.9.9", "1.1.1.1", "8.8.8.8"}, cfg.Upstream.Servers)
	assert.Equal(t, "3s", cfg.Upstream.UDPTimeout)
	assert.Equal(t, "5s", cfg.Upstream.TCPTimeout)
	assert.Equal(t, 3, cfg.Upstream.MaxRetries)

	// Check logging defaults
	assert.Equal(t, "INFO", cfg.Logging.Level)
	assert.False(t, cfg.Logging.Structured)
	assert.Equal(t, "json", cfg.Logging.StructuredFormat)

	// Check filtering defaults
	assert.False(t, cfg.Filtering.Enabled, "Filtering should be disabled by default")
	assert.True(t, cfg.Filtering.LogBlocked)
	assert.Equal(t, "24h", cfg.Filtering.RefreshInterval)

	// Check API defaults - API is always enabled (web UI is mandatory)
	assert.True(t, cfg.API.Enabled, "API should be enabled by default")
	assert.Equal(t, "0.0.0.0", cfg.API.Host, "API host should default to 0.0.0.0")
	assert.Equal(t, 8080, cfg.API.Port)
}

// =============================================================================
// Validation Tests
// =============================================================================

func TestValidate_ValidConfig(t *testing.T) {
	cfg := config.Default()
	err := cfg.Validate()
	require.NoError(t, err, "Default config should be valid")
}

func TestValidate_InvalidPort(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Port = 0
	err := cfg.Validate()
	assert.Error(t, err, "Port 0 should be invalid")

	cfg.Server.Port = 70000
	err = cfg.Validate()
	assert.Error(t, err, "Port 70000 should be invalid")
}

func TestValidate_InvalidAPIPort(t *testing.T) {
	cfg := config.Default()
	cfg.API.Enabled = true
	cfg.API.Port = 0
	err := cfg.Validate()
	assert.Error(t, err, "API port 0 should be invalid when API enabled")
}

func TestValidate_EmptyUpstreamServers(t *testing.T) {
	cfg := config.Default()
	cfg.Upstream.Servers = []string{}
	err := cfg.Validate()
	require.NoError(t, err)
	assert.Contains(t, cfg.Upstream.Servers, "8.8.8.8", "Should default to 8.8.8.8 when empty")
}

func TestValidate_LimitsUpstreamTo3(t *testing.T) {
	cfg := config.Default()
	cfg.Upstream.Servers = []string{"1.1.1.1", "8.8.8.8", "9.9.9.9", "208.67.222.222"}
	err := cfg.Validate()
	require.NoError(t, err)
	assert.Len(t, cfg.Upstream.Servers, 3, "Should limit to 3 upstream servers")
}

func TestValidate_NormalizesLogLevel(t *testing.T) {
	cfg := config.Default()
	cfg.Logging.Level = "debug"
	err := cfg.Validate()
	require.NoError(t, err)
	assert.Equal(t, "DEBUG", cfg.Logging.Level, "Log level should be uppercased")
}

func TestValidate_ParsesWorkers(t *testing.T) {
	cfg := config.Default()
	cfg.Server.WorkersRaw = "8"
	err := cfg.Validate()
	require.NoError(t, err)
	assert.Equal(t, config.WorkersFixed, cfg.Server.Workers.Mode)
	assert.Equal(t, 8, cfg.Server.Workers.Value)
}

func TestValidate_WorkersAutoDefault(t *testing.T) {
	cfg := config.Default()
	cfg.Server.WorkersRaw = ""
	err := cfg.Validate()
	require.NoError(t, err)
	assert.Equal(t, config.WorkersAuto, cfg.Server.Workers.Mode)
}

// =============================================================================
// Rate Limit Configuration Tests
// =============================================================================

func TestDefault_RateLimitDefaults(t *testing.T) {
	cfg := config.Default()

	assert.Equal(t, 60.0, cfg.RateLimit.CleanupSeconds)
	assert.Equal(t, 65536, cfg.RateLimit.MaxIPEntries)
	assert.Equal(t, 16384, cfg.RateLimit.MaxPrefixEntries)
	assert.Equal(t, 100000.0, cfg.RateLimit.GlobalQPS)
	assert.Equal(t, 100000, cfg.RateLimit.GlobalBurst)
	assert.Equal(t, 10000.0, cfg.RateLimit.PrefixQPS)
	assert.Equal(t, 20000, cfg.RateLimit.PrefixBurst)
	assert.Equal(t, 5000.0, cfg.RateLimit.IPQPS)
	assert.Equal(t, 10000, cfg.RateLimit.IPBurst)
}

// =============================================================================
// Custom DNS Configuration Tests
// =============================================================================

func TestDefault_CustomDNSEmpty(t *testing.T) {
	cfg := config.Default()

	assert.NotNil(t, cfg.CustomDNS.Hosts)
	assert.NotNil(t, cfg.CustomDNS.CNAMEs)
	assert.Empty(t, cfg.CustomDNS.Hosts)
	assert.Empty(t, cfg.CustomDNS.CNAMEs)
}

// =============================================================================
// WorkerSetting Tests
// =============================================================================

func TestWorkerSetting_String(t *testing.T) {
	auto := config.WorkerSetting{Mode: config.WorkersAuto}
	assert.Equal(t, "auto", auto.String())

	fixed := config.WorkerSetting{Mode: config.WorkersFixed, Value: 4}
	assert.Equal(t, "4", fixed.String())
}
