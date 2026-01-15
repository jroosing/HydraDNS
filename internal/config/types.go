package config

import (
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

// ParseWorkers parses a workers string into WorkerSetting.
func (s *ServerConfig) ParseWorkers() error {
	raw := strings.TrimSpace(strings.ToLower(s.WorkersRaw))
	if raw == "" || raw == "auto" {
		s.Workers = WorkerSetting{Mode: WorkersAuto}
		return nil
	}
	if n, err := strconv.Atoi(raw); err == nil && n > 0 {
		s.Workers = WorkerSetting{Mode: WorkersFixed, Value: n}
		return nil
	}
	s.Workers = WorkerSetting{Mode: WorkersAuto}
	return nil
}

// ServerConfig contains server-related settings.
type ServerConfig struct {
	Host                   string        `json:"host"`
	Port                   int           `json:"port"`
	Workers                WorkerSetting `json:"-"`
	WorkersRaw             string        `json:"workers"`
	MaxConcurrency         int           `json:"max_concurrency"`
	UpstreamSocketPoolSize int           `json:"upstream_socket_pool_size"`
	EnableTCP              bool          `json:"enable_tcp"`
	TCPFallback            bool          `json:"tcp_fallback"`
}

// UpstreamConfig contains upstream DNS server settings.
type UpstreamConfig struct {
	Servers    []string `json:"servers"`
	UDPTimeout string   `json:"udp_timeout"` // Timeout for UDP queries (e.g., "3s")
	TCPTimeout string   `json:"tcp_timeout"` // Timeout for TCP queries (e.g., "5s")
	MaxRetries int      `json:"max_retries"` // Max retries per upstream on timeout
}

// CustomDNSConfig contains simple custom DNS mappings for homelab use.
// This provides dnsmasq-style configuration for basic DNS records.
type CustomDNSConfig struct {
	// Hosts maps domain names to IP addresses (IPv4 or IPv6)
	// Example: "homelab.local": "192.168.1.10" or "server.local": "2001:db8::1"
	Hosts map[string][]string `json:"hosts,omitempty"`

	// CNAMEs maps alias names to canonical names
	// Example: "www.homelab.local": "homelab.local"
	CNAMEs map[string]string `json:"cnames,omitempty"`
}

// LoggingConfig contains logging settings.
type LoggingConfig struct {
	Level            string            `json:"level"`
	Structured       bool              `json:"structured"`
	StructuredFormat string            `json:"structured_format"`
	IncludePID       bool              `json:"include_pid"`
	ExtraFields      map[string]string `json:"extra_fields,omitempty"`
}

// FilteringConfig controls domain filtering (blocklists/whitelists).
type FilteringConfig struct {
	Enabled          bool              `json:"enabled"`
	LogBlocked       bool              `json:"log_blocked"`
	LogAllowed       bool              `json:"log_allowed"`
	WhitelistDomains []string          `json:"whitelist_domains,omitempty"`
	BlacklistDomains []string          `json:"blacklist_domains,omitempty"`
	Blocklists       []BlocklistConfig `json:"blocklists,omitempty"`
	RefreshInterval  string            `json:"refresh_interval"`
}

// BlocklistConfig defines a remote blocklist source.
type BlocklistConfig struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Format string `json:"format"` // "auto", "adblock", "hosts", "domains"
}

// RateLimitConfig controls rate limiting settings.
type RateLimitConfig struct {
	// CleanupSeconds is how often stale entries are cleaned up (default: 60)
	CleanupSeconds float64 `json:"cleanup_seconds"`
	// MaxIPEntries is the maximum number of tracked IPs (default: 65536)
	MaxIPEntries int `json:"max_ip_entries"`
	// MaxPrefixEntries is the maximum number of tracked prefixes (default: 16384)
	MaxPrefixEntries int `json:"max_prefix_entries"`
	// GlobalQPS is the server-wide queries per second limit (default: 100000, 0 = disabled)
	GlobalQPS float64 `json:"global_qps"`
	// GlobalBurst is the global burst size (default: 100000)
	GlobalBurst int `json:"global_burst"`
	// PrefixQPS is the per-prefix QPS limit (default: 10000, 0 = disabled)
	PrefixQPS float64 `json:"prefix_qps"`
	// PrefixBurst is the per-prefix burst size (default: 20000)
	PrefixBurst int `json:"prefix_burst"`
	// IPQPS is the per-IP QPS limit (default: 3000, 0 = disabled)
	IPQPS float64 `json:"ip_qps"`
	// IPBurst is the per-IP burst size (default: 6000)
	IPBurst int `json:"ip_burst"`
}

// APIConfig contains management API settings.
//
// Note: APIKey is intentionally treated as a secret and should not be returned by API endpoints.
type APIConfig struct {
	Enabled bool   `json:"enabled"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	APIKey  string `json:"api_key,omitempty"`
}

// Config is the root configuration structure.
type Config struct {
	Server    ServerConfig    `json:"server"`
	Upstream  UpstreamConfig  `json:"upstream"`
	CustomDNS CustomDNSConfig `json:"custom_dns"`
	Logging   LoggingConfig   `json:"logging"`
	Filtering FilteringConfig `json:"filtering"`
	RateLimit RateLimitConfig `json:"rate_limit"`
	API       APIConfig       `json:"api"`
}
