// Package config provides configuration loading for HydraDNS using Viper.
// Configuration is loaded from YAML files with automatic environment variable binding.
//
// Environment variables use the HYDRADNS_ prefix and underscore-separated keys:
//   - HYDRADNS_SERVER_HOST -> server.host
//   - HYDRADNS_SERVER_PORT -> server.port
//   - HYDRADNS_UPSTREAM_SERVERS -> upstream.servers (comma-separated)
//   - HYDRADNS_FILTERING_ENABLED -> filtering.enabled
//
// Legacy environment variable names are also supported for backward compatibility.
package config

import (
	"os"
	"strconv"
	"strings"
)

// WorkersMode specifies how worker count is determined.
type WorkersMode int

const (
	// WorkersAuto automatically determines worker count based on available CPUs.
	WorkersAuto WorkersMode = iota
	// WorkersFixed uses a specific worker count.
	WorkersFixed
)

// WorkerSetting represents the workers configuration.
type WorkerSetting struct {
	Mode  WorkersMode
	Value int
}

// String returns the string representation of the worker setting.
func (w WorkerSetting) String() string {
	if w.Mode == WorkersAuto {
		return "auto"
	}
	return strconv.Itoa(w.Value)
}

// ServerConfig contains server-related settings.
type ServerConfig struct {
	Host                   string        `yaml:"host"                      mapstructure:"host"`
	Port                   int           `yaml:"port"                      mapstructure:"port"`
	Workers                WorkerSetting `yaml:"-"                         mapstructure:"-"`
	WorkersRaw             string        `yaml:"workers"                   mapstructure:"workers"`
	MaxConcurrency         int           `yaml:"max_concurrency"           mapstructure:"max_concurrency"`
	UpstreamSocketPoolSize int           `yaml:"upstream_socket_pool_size" mapstructure:"upstream_socket_pool_size"`
	EnableTCP              bool          `yaml:"enable_tcp"                mapstructure:"enable_tcp"`
	TCPFallback            bool          `yaml:"tcp_fallback"              mapstructure:"tcp_fallback"`
}

// UpstreamConfig contains upstream DNS server settings.
type UpstreamConfig struct {
	Servers    []string `yaml:"servers"     mapstructure:"servers"     json:"servers"`
	UDPTimeout string   `yaml:"udp_timeout" mapstructure:"udp_timeout" json:"udp_timeout"` // Timeout for UDP queries (e.g., "3s")
	TCPTimeout string   `yaml:"tcp_timeout" mapstructure:"tcp_timeout" json:"tcp_timeout"` // Timeout for TCP queries (e.g., "5s")
	MaxRetries int      `yaml:"max_retries" mapstructure:"max_retries" json:"max_retries"` // Max retries per upstream on timeout
}

// ZonesConfig contains zone file settings.
type ZonesConfig struct {
	Directory string   `yaml:"directory" mapstructure:"directory" json:"directory"`
	Files     []string `yaml:"files"     mapstructure:"files"     json:"files,omitempty"`
}

// LoggingConfig contains logging settings.
type LoggingConfig struct {
	Level            string            `yaml:"level"             mapstructure:"level"             json:"level"`
	Structured       bool              `yaml:"structured"        mapstructure:"structured"        json:"structured"`
	StructuredFormat string            `yaml:"structured_format" mapstructure:"structured_format" json:"structured_format"`
	IncludePID       bool              `yaml:"include_pid"       mapstructure:"include_pid"       json:"include_pid"`
	ExtraFields      map[string]string `yaml:"extra_fields"      mapstructure:"extra_fields"      json:"extra_fields,omitempty"`
}

// FilteringConfig controls domain filtering (blocklists/whitelists).
type FilteringConfig struct {
	Enabled          bool              `yaml:"enabled"           mapstructure:"enabled"           json:"enabled"`
	LogBlocked       bool              `yaml:"log_blocked"       mapstructure:"log_blocked"       json:"log_blocked"`
	LogAllowed       bool              `yaml:"log_allowed"       mapstructure:"log_allowed"       json:"log_allowed"`
	WhitelistDomains []string          `yaml:"whitelist_domains" mapstructure:"whitelist_domains" json:"whitelist_domains,omitempty"`
	BlacklistDomains []string          `yaml:"blacklist_domains" mapstructure:"blacklist_domains" json:"blacklist_domains,omitempty"`
	Blocklists       []BlocklistConfig `yaml:"blocklists"        mapstructure:"blocklists"        json:"blocklists,omitempty"`
	RefreshInterval  string            `yaml:"refresh_interval"  mapstructure:"refresh_interval"  json:"refresh_interval"`
}

// BlocklistConfig defines a remote blocklist source.
type BlocklistConfig struct {
	Name   string `yaml:"name"   mapstructure:"name"   json:"name"`
	URL    string `yaml:"url"    mapstructure:"url"    json:"url"`
	Format string `yaml:"format" mapstructure:"format" json:"format"` // "auto", "adblock", "hosts", "domains"
}

// RateLimitConfig controls rate limiting settings.
type RateLimitConfig struct {
	// CleanupSeconds is how often stale entries are cleaned up (default: 60)
	CleanupSeconds float64 `yaml:"cleanup_seconds"    mapstructure:"cleanup_seconds"    json:"cleanup_seconds"`
	// MaxIPEntries is the maximum number of tracked IPs (default: 65536)
	MaxIPEntries int `yaml:"max_ip_entries"     mapstructure:"max_ip_entries"     json:"max_ip_entries"`
	// MaxPrefixEntries is the maximum number of tracked prefixes (default: 16384)
	MaxPrefixEntries int `yaml:"max_prefix_entries" mapstructure:"max_prefix_entries" json:"max_prefix_entries"`
	// GlobalQPS is the server-wide queries per second limit (default: 100000, 0 = disabled)
	GlobalQPS float64 `yaml:"global_qps"         mapstructure:"global_qps"         json:"global_qps"`
	// GlobalBurst is the global burst size (default: 100000)
	GlobalBurst int `yaml:"global_burst"       mapstructure:"global_burst"       json:"global_burst"`
	// PrefixQPS is the per-prefix QPS limit (default: 10000, 0 = disabled)
	PrefixQPS float64 `yaml:"prefix_qps"         mapstructure:"prefix_qps"         json:"prefix_qps"`
	// PrefixBurst is the per-prefix burst size (default: 20000)
	PrefixBurst int `yaml:"prefix_burst"       mapstructure:"prefix_burst"       json:"prefix_burst"`
	// IPQPS is the per-IP QPS limit (default: 3000, 0 = disabled)
	IPQPS float64 `yaml:"ip_qps"             mapstructure:"ip_qps"             json:"ip_qps"`
	// IPBurst is the per-IP burst size (default: 6000)
	IPBurst int `yaml:"ip_burst"           mapstructure:"ip_burst"           json:"ip_burst"`
}

// APIConfig contains management API settings.
//
// Note: APIKey is intentionally treated as a secret and should not be returned by API endpoints.
type APIConfig struct {
	Enabled bool   `yaml:"enabled" mapstructure:"enabled"`
	Host    string `yaml:"host"    mapstructure:"host"`
	Port    int    `yaml:"port"    mapstructure:"port"`
	APIKey  string `yaml:"api_key" mapstructure:"api_key"`
}

// Config is the root configuration structure.
type Config struct {
	Server    ServerConfig    `yaml:"server"     mapstructure:"server"`
	Upstream  UpstreamConfig  `yaml:"upstream"   mapstructure:"upstream"`
	Zones     ZonesConfig     `yaml:"zones"      mapstructure:"zones"`
	Logging   LoggingConfig   `yaml:"logging"    mapstructure:"logging"`
	Filtering FilteringConfig `yaml:"filtering"  mapstructure:"filtering"`
	RateLimit RateLimitConfig `yaml:"rate_limit" mapstructure:"rate_limit"`
	API       APIConfig       `yaml:"api"        mapstructure:"api"`
}

// ResolveConfigPath determines the config file path from flag or environment.
func ResolveConfigPath(flagValue string) string {
	if strings.TrimSpace(flagValue) != "" {
		return flagValue
	}
	if v := strings.TrimSpace(os.Getenv("HYDRADNS_CONFIG")); v != "" {
		return v
	}
	return ""
}

// Load loads configuration from a YAML file with environment variable overrides.
// This is the main entry point for loading configuration.
//
// Configuration priority (highest to lowest):
//  1. Environment variables (HYDRADNS_*)
//  2. Config file values
//  3. Default values
func Load(path string) (*Config, error) {
	return loadFromSource(path)
}
