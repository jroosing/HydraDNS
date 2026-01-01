package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerSettingString(t *testing.T) {
	tests := []struct {
		name string
		ws   WorkerSetting
		want string
	}{
		{"auto mode", WorkerSetting{Mode: WorkersAuto}, "auto"},
		{"fixed mode 4", WorkerSetting{Mode: WorkersFixed, Value: 4}, "4"},
		{"fixed mode 0", WorkerSetting{Mode: WorkersFixed, Value: 0}, "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ws.String()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveConfigPath(t *testing.T) {
	// Save and restore env
	tests := []struct {
		name     string
		flag     string
		envValue string
		want     string
	}{
		{"flag takes precedence", "/path/from/flag", "/path/from/env", "/path/from/flag"},
		{"env when no flag", "", "/path/from/env", "/path/from/env"},
		{"empty when neither", "", "", ""},
		{"whitespace flag", "  ", "/path/from/env", "/path/from/env"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HYDRADNS_CONFIG", tt.envValue)
			got := ResolveConfigPath(tt.flag)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLoadDefault(t *testing.T) {
	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 1053, cfg.Server.Port)
	assert.Equal(t, WorkersAuto, cfg.Server.Workers.Mode)
	assert.True(t, cfg.Server.EnableTCP)
	assert.True(t, cfg.Server.TCPFallback)
	require.Len(t, cfg.Upstream.Servers, 1)
	assert.Equal(t, "8.8.8.8", cfg.Upstream.Servers[0])
}

func TestLoadFromFile(t *testing.T) {
	content := `
server:
  host: "127.0.0.1"
  port: 5353
  workers: "2"
  enable_tcp: false
  tcp_fallback: false

upstream:
  servers:
    - "1.1.1.1"
    - "9.9.9.9"

zones:
  directory: "test-zones"

logging:
  level: "DEBUG"
  structured: true
  structured_format: "keyvalue"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test-config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	assert.Equal(t, 5353, cfg.Server.Port)
	assert.Equal(t, WorkersFixed, cfg.Server.Workers.Mode)
	assert.Equal(t, 2, cfg.Server.Workers.Value)
	assert.False(t, cfg.Server.EnableTCP)
	assert.False(t, cfg.Server.TCPFallback)
	assert.Len(t, cfg.Upstream.Servers, 2)
	assert.Equal(t, "test-zones", cfg.Zones.Directory)
	assert.Equal(t, "DEBUG", cfg.Logging.Level)
	assert.True(t, cfg.Logging.Structured)
	assert.Equal(t, "keyvalue", cfg.Logging.StructuredFormat)
}

func TestLoadInvalidPath(t *testing.T) {
	_, err := Load("/nonexistent/path/to/config.yaml")
	assert.Error(t, err)
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(path, []byte("server:\n  port: [invalid"), 0644))

	_, err := Load(path)
	assert.Error(t, err)
}

func TestNormalizeInvalidPort(t *testing.T) {
	content := `
server:
  port: 0
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	_, err := Load(path)
	assert.Error(t, err)
}

func TestNormalizeInvalidWorkers(t *testing.T) {
	content := `
server:
  workers: "invalid"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	// With Viper, invalid workers gracefully defaults to "auto"
	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, WorkersAuto, cfg.Server.Workers.Mode)
}

func TestNormalizeTruncatesServers(t *testing.T) {
	content := `
upstream:
  servers:
    - "1.1.1.1"
    - "8.8.8.8"
    - "9.9.9.9"
    - "208.67.222.222"
    - "208.67.220.220"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Len(t, cfg.Upstream.Servers, 3, "expected servers to be truncated to 3")
}

func TestEnvOverrides(t *testing.T) {
	// Set overrides using standard naming
	t.Setenv("HYDRADNS_SERVER_HOST", "192.168.1.1")
	t.Setenv("HYDRADNS_SERVER_PORT", "8053")
	t.Setenv("HYDRADNS_SERVER_WORKERS", "8")
	t.Setenv("HYDRADNS_UPSTREAM_SERVERS", "1.1.1.1, 8.8.8.8:53")
	t.Setenv("HYDRADNS_ZONES_DIRECTORY", "/custom/zones")
	t.Setenv("HYDRADNS_SERVER_ENABLE_TCP", "false")
	t.Setenv("HYDRADNS_SERVER_TCP_FALLBACK", "no")
	t.Setenv("HYDRADNS_LOGGING_LEVEL", "debug")

	cfg, err := Load("")
	require.NoError(t, err)

	assert.Equal(t, "192.168.1.1", cfg.Server.Host)
	assert.Equal(t, 8053, cfg.Server.Port)
	assert.Equal(t, WorkersFixed, cfg.Server.Workers.Mode)
	assert.Equal(t, 8, cfg.Server.Workers.Value)
	assert.Len(t, cfg.Upstream.Servers, 2)
	assert.Equal(t, "/custom/zones", cfg.Zones.Directory)
	assert.False(t, cfg.Server.EnableTCP)
	assert.False(t, cfg.Server.TCPFallback)
	assert.Equal(t, "DEBUG", cfg.Logging.Level)
}
