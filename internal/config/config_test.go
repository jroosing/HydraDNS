package config

import (
	"os"
	"path/filepath"
	"testing"
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
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveConfigPath(t *testing.T) {
	// Save and restore env
	orig := os.Getenv("HYDRADNS_CONFIG")
	defer os.Setenv("HYDRADNS_CONFIG", orig)

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
			os.Setenv("HYDRADNS_CONFIG", tt.envValue)
			got := ResolveConfigPath(tt.flag)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadDefault(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("expected host 0.0.0.0, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 1053 {
		t.Errorf("expected port 1053, got %d", cfg.Server.Port)
	}
	if cfg.Server.Workers.Mode != WorkersAuto {
		t.Errorf("expected workers auto mode")
	}
	if !cfg.Server.EnableTCP {
		t.Error("expected EnableTCP true")
	}
	if !cfg.Server.TCPFallback {
		t.Error("expected TCPFallback true")
	}
	if len(cfg.Upstream.Servers) != 1 || cfg.Upstream.Servers[0] != "8.8.8.8" {
		t.Errorf("unexpected upstream servers: %v", cfg.Upstream.Servers)
	}
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
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 5353 {
		t.Errorf("expected port 5353, got %d", cfg.Server.Port)
	}
	if cfg.Server.Workers.Mode != WorkersFixed || cfg.Server.Workers.Value != 2 {
		t.Errorf("expected fixed workers 2, got %v", cfg.Server.Workers)
	}
	if cfg.Server.EnableTCP {
		t.Error("expected EnableTCP false")
	}
	if cfg.Server.TCPFallback {
		t.Error("expected TCPFallback false")
	}
	if len(cfg.Upstream.Servers) != 2 {
		t.Errorf("expected 2 servers, got %d", len(cfg.Upstream.Servers))
	}
	if cfg.Zones.Directory != "test-zones" {
		t.Errorf("expected zones directory test-zones, got %s", cfg.Zones.Directory)
	}
	if cfg.Logging.Level != "DEBUG" {
		t.Errorf("expected log level DEBUG, got %s", cfg.Logging.Level)
	}
	if !cfg.Logging.Structured {
		t.Error("expected structured logging enabled")
	}
	if cfg.Logging.StructuredFormat != "keyvalue" {
		t.Errorf("expected format keyvalue, got %s", cfg.Logging.StructuredFormat)
	}
}

func TestLoadInvalidPath(t *testing.T) {
	_, err := Load("/nonexistent/path/to/config.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	// Use truly invalid YAML syntax
	if err := os.WriteFile(path, []byte("server:\n  port: [invalid"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestNormalizeInvalidPort(t *testing.T) {
	content := `
server:
  port: 0
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid port")
	}
}

func TestNormalizeInvalidWorkers(t *testing.T) {
	content := `
server:
  workers: "invalid"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid workers")
	}
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
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Upstream.Servers) != 3 {
		t.Errorf("expected 3 servers (truncated), got %d", len(cfg.Upstream.Servers))
	}
}

func TestEnvOverrides(t *testing.T) {
	// Save and restore env
	envVars := []string{
		"HYDRADNS_HOST", "HYDRADNS_PORT", "HYDRADNS_WORKERS",
		"HYDRADNS_UPSTREAM_SERVERS", "HYDRADNS_ZONES_DIR",
		"HYDRADNS_ENABLE_TCP", "HYDRADNS_TCP_FALLBACK", "LOG_LEVEL",
	}
	origValues := make(map[string]string)
	for _, k := range envVars {
		origValues[k] = os.Getenv(k)
	}
	defer func() {
		for k, v := range origValues {
			os.Setenv(k, v)
		}
	}()

	// Set overrides
	os.Setenv("HYDRADNS_HOST", "192.168.1.1")
	os.Setenv("HYDRADNS_PORT", "8053")
	os.Setenv("HYDRADNS_WORKERS", "8")
	os.Setenv("HYDRADNS_UPSTREAM_SERVERS", "1.1.1.1, 8.8.8.8:53")
	os.Setenv("HYDRADNS_ZONES_DIR", "/custom/zones")
	os.Setenv("HYDRADNS_ENABLE_TCP", "false")
	os.Setenv("HYDRADNS_TCP_FALLBACK", "no")
	os.Setenv("LOG_LEVEL", "debug")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Host != "192.168.1.1" {
		t.Errorf("expected host 192.168.1.1, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 8053 {
		t.Errorf("expected port 8053, got %d", cfg.Server.Port)
	}
	if cfg.Server.Workers.Mode != WorkersFixed || cfg.Server.Workers.Value != 8 {
		t.Errorf("expected workers 8, got %v", cfg.Server.Workers)
	}
	if len(cfg.Upstream.Servers) != 2 {
		t.Errorf("expected 2 servers, got %d", len(cfg.Upstream.Servers))
	}
	if cfg.Zones.Directory != "/custom/zones" {
		t.Errorf("expected zones dir /custom/zones, got %s", cfg.Zones.Directory)
	}
	if cfg.Server.EnableTCP {
		t.Error("expected EnableTCP false")
	}
	if cfg.Server.TCPFallback {
		t.Error("expected TCPFallback false")
	}
	if cfg.Logging.Level != "DEBUG" {
		t.Errorf("expected log level DEBUG, got %s", cfg.Logging.Level)
	}
}

func TestEnvBool(t *testing.T) {
	tests := []struct {
		raw  string
		def  bool
		want bool
	}{
		{"1", false, true},
		{"true", false, true},
		{"yes", false, true},
		{"y", false, true},
		{"on", false, true},
		{"TRUE", false, true},
		{"0", true, false},
		{"false", true, false},
		{"no", true, false},
		{"n", true, false},
		{"off", true, false},
		{"FALSE", true, false},
		{"invalid", true, true},
		{"invalid", false, false},
		{"", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got := envBool(tt.raw, tt.def)
			if got != tt.want {
				t.Errorf("envBool(%q, %v) = %v, want %v", tt.raw, tt.def, got, tt.want)
			}
		})
	}
}
