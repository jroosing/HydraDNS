// Package config provides configuration types and validation for HydraDNS.
//
// Configuration is stored in a SQLite database and exported to a Config struct
// for use by the server. The database package (internal/database) handles
// persistence; defaults are seeded by migrations.
//
// This package defines the Config struct used throughout the application
// and provides validation utilities.
package config

import (
	"errors"
	"strconv"
	"strings"
)

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
