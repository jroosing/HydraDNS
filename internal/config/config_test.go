package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jroosing/hydradns/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Configuration Loading Tests
// =============================================================================

func TestLoad_DefaultValues(t *testing.T) {
	// Load with empty path to get defaults
	cfg, err := config.Load("")
	require.NoError(t, err, "Load should succeed with no config file")

	// Check server defaults
	assert.Equal(t, "0.0.0.0", cfg.Server.Host, "Default host should be 0.0.0.0")
	assert.Equal(t, 1053, cfg.Server.Port, "Default port should be 1053")
	assert.True(t, cfg.Server.EnableTCP, "TCP should be enabled by default")

	// Check upstream defaults
	assert.Contains(t, cfg.Upstream.Servers, "8.8.8.8", "Default upstream should include 8.8.8.8")

	// Check filtering defaults
	assert.False(t, cfg.Filtering.Enabled, "Filtering should be disabled by default")

	// Check API defaults
	assert.False(t, cfg.API.Enabled, "API should be disabled by default")
	assert.Equal(t, "127.0.0.1", cfg.API.Host, "API host should default to localhost")
}

func TestLoad_FromYAMLFile(t *testing.T) {
	configContent := `
server:
  host: "192.168.1.1"
  port: 5353
  workers: 4

upstream:
  servers:
    - "1.1.1.1"
    - "9.9.9.9"

logging:
  level: DEBUG

filtering:
  enabled: true
`
	tmpFile := filepath.Join(t.TempDir(), "test-config.yaml")
	err := os.WriteFile(tmpFile, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := config.Load(tmpFile)
	require.NoError(t, err)

	assert.Equal(t, "192.168.1.1", cfg.Server.Host)
	assert.Equal(t, 5353, cfg.Server.Port)
	assert.Equal(t, config.WorkersFixed, cfg.Server.Workers.Mode)
	assert.Equal(t, 4, cfg.Server.Workers.Value)
	assert.Equal(t, []string{"1.1.1.1", "9.9.9.9"}, cfg.Upstream.Servers)
	assert.Equal(t, "DEBUG", cfg.Logging.Level)
	assert.True(t, cfg.Filtering.Enabled)
}

func TestLoad_NonexistentFile_ReturnsError(t *testing.T) {
	_, err := config.Load("/nonexistent/path/config.yaml")
	assert.Error(t, err, "Load should fail for nonexistent file")
}

func TestLoad_InvalidYAML_ReturnsError(t *testing.T) {
	configContent := `
this is not: valid: yaml:
  - broken
    indentation
`
	tmpFile := filepath.Join(t.TempDir(), "invalid.yaml")
	err := os.WriteFile(tmpFile, []byte(configContent), 0644)
	require.NoError(t, err)

	_, err = config.Load(tmpFile)
	assert.Error(t, err, "Load should fail for invalid YAML")
}

// =============================================================================
// Environment Variable Override Tests
// =============================================================================

func TestLoad_EnvironmentOverrides(t *testing.T) {
	// Set environment variables
	t.Setenv("HYDRADNS_SERVER_HOST", "10.0.0.1")
	t.Setenv("HYDRADNS_SERVER_PORT", "5353")
	t.Setenv("HYDRADNS_LOGGING_LEVEL", "WARN")

	cfg, err := config.Load("")
	require.NoError(t, err)

	assert.Equal(t, "10.0.0.1", cfg.Server.Host, "Environment should override default host")
	assert.Equal(t, 5353, cfg.Server.Port, "Environment should override default port")
	assert.Equal(t, "WARN", cfg.Logging.Level, "Environment should override default log level")
}

func TestLoad_EnvironmentOverridesConfigFile(t *testing.T) {
	configContent := `
server:
  host: "192.168.1.1"
  port: 5353
`
	tmpFile := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(tmpFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// Environment should take precedence over config file
	t.Setenv("HYDRADNS_SERVER_HOST", "10.0.0.99")

	cfg, err := config.Load(tmpFile)
	require.NoError(t, err)

	assert.Equal(t, "10.0.0.99", cfg.Server.Host, "Environment should override config file")
	assert.Equal(t, 5353, cfg.Server.Port, "Config file value should be used when no env var")
}

// =============================================================================
// ResolveConfigPath Tests
// =============================================================================

func TestResolveConfigPath_FlagTakesPrecedence(t *testing.T) {
	t.Setenv("HYDRADNS_CONFIG", "/env/path.yaml")

	result := config.ResolveConfigPath("/flag/path.yaml")
	assert.Equal(t, "/flag/path.yaml", result, "Flag value should take precedence")
}

func TestResolveConfigPath_FallsBackToEnv(t *testing.T) {
	t.Setenv("HYDRADNS_CONFIG", "/env/path.yaml")

	result := config.ResolveConfigPath("")
	assert.Equal(t, "/env/path.yaml", result, "Should fall back to environment variable")
}

func TestResolveConfigPath_EmptyWhenNeitherSet(t *testing.T) {
	// Make sure env is not set
	os.Unsetenv("HYDRADNS_CONFIG")

	result := config.ResolveConfigPath("")
	assert.Empty(t, result, "Should return empty when neither flag nor env is set")
}

// =============================================================================
// Workers Configuration Tests
// =============================================================================

func TestLoad_WorkersAuto(t *testing.T) {
	configContent := `
server:
  workers: auto
`
	tmpFile := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(tmpFile, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := config.Load(tmpFile)
	require.NoError(t, err)

	// "auto" should result in WorkersAuto mode
	assert.Equal(t, config.WorkersAuto, cfg.Server.Workers.Mode, "Workers mode should be Auto")
}

func TestLoad_WorkersNumeric(t *testing.T) {
	configContent := `
server:
  workers: 8
`
	tmpFile := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(tmpFile, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := config.Load(tmpFile)
	require.NoError(t, err)

	assert.Equal(t, config.WorkersFixed, cfg.Server.Workers.Mode)
	assert.Equal(t, 8, cfg.Server.Workers.Value)
}

// =============================================================================
// Rate Limit Configuration Tests
// =============================================================================

func TestLoad_RateLimitDefaults(t *testing.T) {
	cfg, err := config.Load("")
	require.NoError(t, err)

	assert.Greater(t, cfg.RateLimit.GlobalQPS, float64(0), "Global QPS should have default")
	assert.Positive(t, cfg.RateLimit.GlobalBurst, "Global burst should have default")
	assert.Greater(t, cfg.RateLimit.IPQPS, float64(0), "IP QPS should have default")
	assert.Positive(t, cfg.RateLimit.IPBurst, "IP burst should have default")
}

// =============================================================================
// Filtering Configuration Tests
// =============================================================================

func TestLoad_FilteringConfig(t *testing.T) {
	configContent := `
filtering:
  enabled: true
  log_blocked: true
  whitelist_domains:
    - "allowed.example.com"
  blacklist_domains:
    - "blocked.example.com"
`
	tmpFile := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(tmpFile, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := config.Load(tmpFile)
	require.NoError(t, err)

	assert.True(t, cfg.Filtering.Enabled)
	assert.True(t, cfg.Filtering.LogBlocked)
	assert.Contains(t, cfg.Filtering.WhitelistDomains, "allowed.example.com")
	assert.Contains(t, cfg.Filtering.BlacklistDomains, "blocked.example.com")
}

// =============================================================================
// Zones Configuration Tests
// =============================================================================

func TestLoad_ZonesConfig(t *testing.T) {
	configContent := `
zones:
  directory: "/etc/hydradns/zones"
  files:
    - "example.zone"
    - "local.zone"
`
	tmpFile := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(tmpFile, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := config.Load(tmpFile)
	require.NoError(t, err)

	assert.Equal(t, "/etc/hydradns/zones", cfg.Zones.Directory)
	assert.Len(t, cfg.Zones.Files, 2)
}
