package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

// initConfig sets up the config loader with defaults, env binding, and config file.
func initConfig(configPath string) (*viper.Viper, error) {
	v := viper.New()

	// Set default values
	setDefaults(v)

	// Environment variable binding
	// Uses HYDRADNS_ prefix: HYDRADNS_SERVER_HOST -> server.host
	v.SetEnvPrefix("HYDRADNS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Load config file if provided
	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	return v, nil
}

// setDefaults configures all default values.
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 1053)
	v.SetDefault("server.workers", "auto")
	v.SetDefault("server.max_concurrency", 0)
	v.SetDefault("server.upstream_socket_pool_size", 0)
	v.SetDefault("server.enable_tcp", true)
	v.SetDefault("server.tcp_fallback", true)

	// Upstream defaults
	v.SetDefault("upstream.servers", []string{"8.8.8.8"})
	v.SetDefault("upstream.udp_timeout", "3s")
	v.SetDefault("upstream.tcp_timeout", "5s")
	v.SetDefault("upstream.max_retries", 3)

	// Zones defaults
	v.SetDefault("zones.directory", "zones")
	v.SetDefault("zones.files", []string{})

	// Logging defaults
	v.SetDefault("logging.level", "INFO")
	v.SetDefault("logging.structured", false)
	v.SetDefault("logging.structured_format", "json")
	v.SetDefault("logging.include_pid", false)
	v.SetDefault("logging.extra_fields", map[string]string{})

	// Filtering defaults
	v.SetDefault("filtering.enabled", false)
	v.SetDefault("filtering.log_blocked", true)
	v.SetDefault("filtering.log_allowed", false)
	v.SetDefault("filtering.whitelist_domains", []string{})
	v.SetDefault("filtering.blacklist_domains", []string{})
	v.SetDefault("filtering.blocklists", []BlocklistConfig{})
	v.SetDefault("filtering.refresh_interval", "24h")

	// Rate limiting defaults
	v.SetDefault("rate_limit.cleanup_seconds", 60.0)
	v.SetDefault("rate_limit.max_ip_entries", 65536)
	v.SetDefault("rate_limit.max_prefix_entries", 16384)
	v.SetDefault("rate_limit.global_qps", 100000.0)
	v.SetDefault("rate_limit.global_burst", 100000)
	v.SetDefault("rate_limit.prefix_qps", 10000.0)
	v.SetDefault("rate_limit.prefix_burst", 20000)
	v.SetDefault("rate_limit.ip_qps", 3000.0)
	v.SetDefault("rate_limit.ip_burst", 6000)
}

// loadFromSource loads configuration from file and environment.
func loadFromSource(configPath string) (*Config, error) {
	v, err := initConfig(configPath)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}

	// Server config
	cfg.Server.Host = v.GetString("server.host")
	cfg.Server.Port = v.GetInt("server.port")
	cfg.Server.MaxConcurrency = v.GetInt("server.max_concurrency")
	cfg.Server.UpstreamSocketPoolSize = v.GetInt("server.upstream_socket_pool_size")
	cfg.Server.EnableTCP = v.GetBool("server.enable_tcp")
	cfg.Server.TCPFallback = v.GetBool("server.tcp_fallback")
	cfg.Server.WorkersRaw = v.GetString("server.workers")

	// Parse workers setting
	cfg.Server.Workers = parseWorkers(cfg.Server.WorkersRaw)

	// Upstream config
	cfg.Upstream.Servers = parseServerList(v.GetStringSlice("upstream.servers"))
	if len(cfg.Upstream.Servers) == 0 {
		// Handle comma-separated string from env
		if s := v.GetString("upstream.servers"); s != "" {
			cfg.Upstream.Servers = parseServerList(strings.Split(s, ","))
		}
	}

	// Zones config
	cfg.Zones.Directory = v.GetString("zones.directory")
	cfg.Zones.Files = v.GetStringSlice("zones.files")

	// Logging config
	cfg.Logging.Level = strings.ToUpper(v.GetString("logging.level"))
	cfg.Logging.Structured = v.GetBool("logging.structured")
	cfg.Logging.StructuredFormat = v.GetString("logging.structured_format")
	cfg.Logging.IncludePID = v.GetBool("logging.include_pid")
	cfg.Logging.ExtraFields = v.GetStringMapString("logging.extra_fields")

	// Filtering config
	cfg.Filtering.Enabled = v.GetBool("filtering.enabled")
	cfg.Filtering.LogBlocked = v.GetBool("filtering.log_blocked")
	cfg.Filtering.LogAllowed = v.GetBool("filtering.log_allowed")
	cfg.Filtering.RefreshInterval = v.GetString("filtering.refresh_interval")

	// Handle whitelist/blacklist (can be slice or comma-separated string)
	cfg.Filtering.WhitelistDomains = getStringSliceOrSplit(v, "filtering.whitelist_domains")
	cfg.Filtering.BlacklistDomains = getStringSliceOrSplit(v, "filtering.blacklist_domains")

	// Parse blocklists
	if err := v.UnmarshalKey("filtering.blocklists", &cfg.Filtering.Blocklists); err != nil {
		// Ignore unmarshal errors for blocklists, use empty slice
		cfg.Filtering.Blocklists = []BlocklistConfig{}
	}

	// Handle single blocklist URL from env
	if url := v.GetString("filtering.blocklist_url"); url != "" {
		cfg.Filtering.Blocklists = append(cfg.Filtering.Blocklists, BlocklistConfig{
			Name:   "env-blocklist",
			URL:    url,
			Format: "auto",
		})
	}

	// Normalize and validate
	if err := normalizeConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// parseWorkers converts the workers string to WorkerSetting.
func parseWorkers(raw string) WorkerSetting {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" || raw == "auto" {
		return WorkerSetting{Mode: WorkersAuto}
	}
	if n, err := strconv.Atoi(raw); err == nil && n > 0 {
		return WorkerSetting{Mode: WorkersFixed, Value: n}
	}
	return WorkerSetting{Mode: WorkersAuto}
}

// parseServerList cleans up a list of server addresses.
func parseServerList(servers []string) []string {
	result := make([]string, 0, len(servers))
	for _, s := range servers {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		// Strip port if present (always use port 53)
		if h, _, ok := strings.Cut(s, ":"); ok {
			s = h
		}
		result = append(result, s)
	}
	return result
}

// getStringSliceOrSplit handles both slice and comma-separated string values.
func getStringSliceOrSplit(v *viper.Viper, key string) []string {
	if slice := v.GetStringSlice(key); len(slice) > 0 {
		// Filter empty entries
		result := make([]string, 0, len(slice))
		for _, s := range slice {
			s = strings.TrimSpace(s)
			if s != "" {
				result = append(result, s)
			}
		}
		return result
	}
	// Try as comma-separated string
	if s := v.GetString(key); s != "" {
		parts := strings.Split(s, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				result = append(result, p)
			}
		}
		return result
	}
	return nil
}

// normalizeConfig validates and normalizes the configuration.
func normalizeConfig(cfg *Config) error {
	// Validate port
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return errors.New("server.port must be 1..65535")
	}

	// Default upstream servers
	if len(cfg.Upstream.Servers) == 0 {
		cfg.Upstream.Servers = []string{"8.8.8.8"}
	}

	// Limit to 3 upstream servers (strict-order failover)
	if len(cfg.Upstream.Servers) > 3 {
		cfg.Upstream.Servers = cfg.Upstream.Servers[:3]
	}

	// Normalize zones directory
	if cfg.Zones.Directory == "" {
		cfg.Zones.Directory = "zones"
	}
	cfg.Zones.Directory = filepath.Clean(cfg.Zones.Directory)

	// Normalize logging
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "INFO"
	}
	if cfg.Logging.StructuredFormat == "" {
		cfg.Logging.StructuredFormat = "json"
	}
	if cfg.Logging.ExtraFields == nil {
		cfg.Logging.ExtraFields = map[string]string{}
	}

	// Normalize filtering
	if cfg.Filtering.RefreshInterval == "" {
		cfg.Filtering.RefreshInterval = "24h"
	}

	return nil
}
