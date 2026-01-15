// Package config provides configuration types and validation for HydraDNS.
//
// Configuration is stored in a SQLite database and exported to a Config struct
// for use by the server. The database package (internal/database) handles
// persistence and defaults.
//
// This package defines the Config struct used throughout the application
// and provides validation and default utilities.
package config

import (
	"errors"
	"strconv"
	"strings"
)

// Default returns a Config with sensible default values.
// This is used by the database package to initialize defaults.
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Host:                   "0.0.0.0",
			Port:                   1053,
			WorkersRaw:             "auto",
			Workers:                WorkerSetting{Mode: WorkersAuto},
			MaxConcurrency:         0,
			UpstreamSocketPoolSize: 0,
			EnableTCP:              true,
			TCPFallback:            true,
		},
		Upstream: UpstreamConfig{
			Servers:    []string{"9.9.9.9", "1.1.1.1", "8.8.8.8"},
			UDPTimeout: "3s",
			TCPTimeout: "5s",
			MaxRetries: 3,
		},
		CustomDNS: CustomDNSConfig{
			Hosts:  make(map[string][]string),
			CNAMEs: make(map[string]string),
		},
		Logging: LoggingConfig{
			Level:            "INFO",
			Structured:       false,
			StructuredFormat: "json",
			IncludePID:       false,
			ExtraFields:      make(map[string]string),
		},
		Filtering: FilteringConfig{
			Enabled:          false,
			LogBlocked:       true,
			LogAllowed:       false,
			WhitelistDomains: []string{},
			BlacklistDomains: []string{},
			Blocklists:       []BlocklistConfig{},
			RefreshInterval:  "24h",
		},
		RateLimit: RateLimitConfig{
			CleanupSeconds:   60.0,
			MaxIPEntries:     65536,
			MaxPrefixEntries: 16384,
			GlobalQPS:        100000.0,
			GlobalBurst:      100000,
			PrefixQPS:        10000.0,
			PrefixBurst:      20000,
			IPQPS:            5000.0,
			IPBurst:          10000,
		},
		API: APIConfig{
			Enabled: true,
			Host:    "0.0.0.0",
			Port:    8080,
			APIKey:  "",
		},
	}
}

// Validate validates and normalizes the configuration.
func (cfg *Config) Validate() error {
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

	// Normalize logging
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "INFO"
	}
	cfg.Logging.Level = strings.ToUpper(cfg.Logging.Level)
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

	// Normalize management API
	if cfg.API.Host == "" {
		cfg.API.Host = "0.0.0.0"
	}
	if cfg.API.Enabled {
		if cfg.API.Port <= 0 || cfg.API.Port > 65535 {
			return errors.New("api.port must be 1..65535")
		}
	}

	// Parse workers
	cfg.Server.Workers = parseWorkers(cfg.Server.WorkersRaw)

	return nil
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
