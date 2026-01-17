package database

import (
	"database/sql"
	"fmt"
)

// ConfigKey represents configuration key names in the database.
const (
	ConfigKeyServerHost               = "server.host"
	ConfigKeyServerPort               = "server.port"
	ConfigKeyServerWorkers            = "server.workers"
	ConfigKeyServerMaxConcurrency     = "server.max_concurrency"
	ConfigKeyServerUpstreamSocketPool = "server.upstream_socket_pool_size"
	ConfigKeyServerEnableTCP          = "server.enable_tcp"
	ConfigKeyServerTCPFallback        = "server.tcp_fallback"

	ConfigKeyUpstreamUDPTimeout = "upstream.udp_timeout"
	ConfigKeyUpstreamTCPTimeout = "upstream.tcp_timeout"
	ConfigKeyUpstreamMaxRetries = "upstream.max_retries"

	ConfigKeyLoggingLevel            = "logging.level"
	ConfigKeyLoggingStructured       = "logging.structured"
	ConfigKeyLoggingStructuredFormat = "logging.structured_format"
	ConfigKeyLoggingIncludePID       = "logging.include_pid"

	ConfigKeyFilteringEnabled         = "filtering.enabled"
	ConfigKeyFilteringLogBlocked      = "filtering.log_blocked"
	ConfigKeyFilteringLogAllowed      = "filtering.log_allowed"
	ConfigKeyFilteringRefreshInterval = "filtering.refresh_interval"

	ConfigKeyRateLimitCleanupSeconds   = "rate_limit.cleanup_seconds"
	ConfigKeyRateLimitMaxIPEntries     = "rate_limit.max_ip_entries"
	ConfigKeyRateLimitMaxPrefixEntries = "rate_limit.max_prefix_entries"
	ConfigKeyRateLimitGlobalQPS        = "rate_limit.global_qps"
	ConfigKeyRateLimitGlobalBurst      = "rate_limit.global_burst"
	ConfigKeyRateLimitPrefixQPS        = "rate_limit.prefix_qps"
	ConfigKeyRateLimitPrefixBurst      = "rate_limit.prefix_burst"
	ConfigKeyRateLimitIPQPS            = "rate_limit.ip_qps"
	ConfigKeyRateLimitIPBurst          = "rate_limit.ip_burst"

	ConfigKeyAPIEnabled = "api.enabled"
	ConfigKeyAPIHost    = "api.host"
	ConfigKeyAPIPort    = "api.port"
	ConfigKeyAPIKey     = "api.api_key"
)

// SetConfig sets a configuration value.
func (db *DB) SetConfig(key, value string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `
		INSERT INTO config (key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := db.conn.Exec(query, key, value)
	if err != nil {
		return fmt.Errorf("failed to set config %s: %w", key, err)
	}

	return nil
}

// GetConfig retrieves a configuration value.
func (db *DB) GetConfig(key string) (string, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var value string
	err := db.conn.QueryRow("SELECT value FROM config WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("config key not found: %s", key)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get config %s: %w", key, err)
	}

	return value, nil
}

// GetConfigWithDefault retrieves a configuration value or returns a default.
func (db *DB) GetConfigWithDefault(key, defaultValue string) string {
	value, err := db.GetConfig(key)
	if err != nil {
		return defaultValue
	}
	return value
}

// GetAllConfig retrieves all configuration key-value pairs.
func (db *DB) GetAllConfig() (map[string]string, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	rows, err := db.conn.Query("SELECT key, value FROM config ORDER BY key")
	if err != nil {
		return nil, fmt.Errorf("failed to query config: %w", err)
	}
	defer rows.Close()

	config := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("failed to scan config row: %w", err)
		}
		config[key] = value
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating config rows: %w", err)
	}

	return config, nil
}

// DeleteConfig removes a configuration key.
func (db *DB) DeleteConfig(key string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	_, err := db.conn.Exec("DELETE FROM config WHERE key = ?", key)
	if err != nil {
		return fmt.Errorf("failed to delete config %s: %w", key, err)
	}

	return nil
}

// SetMultipleConfig sets multiple config values in a transaction.
func (db *DB) SetMultipleConfig(configs map[string]string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO config (key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for key, value := range configs {
		if _, err := stmt.Exec(key, value); err != nil {
			return fmt.Errorf("failed to set config %s: %w", key, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
