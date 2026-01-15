package database

import (
	"database/sql"
	"fmt"
)

// DefaultUpstreamServers are the default upstream DNS servers.
var DefaultUpstreamServers = []string{
	"9.9.9.9", // Quad9 (primary)
	"1.1.1.1", // Cloudflare (fallback)
	"8.8.8.8", // Google (fallback)
}

// InitDefaults populates the database with default configuration values.
// This is called on first database creation to ensure all config keys exist.
// It only inserts values if they don't already exist (won't overwrite).
func (db *DB) InitDefaults() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Check if defaults have already been initialized
	var count int
	err = tx.QueryRow("SELECT COUNT(*) FROM config").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check config count: %w", err)
	}

	// If config table has entries, defaults have already been set
	if count > 0 {
		return nil
	}

	// Initialize all default configuration values
	if err := db.initServerDefaults(tx); err != nil {
		return err
	}

	if err := db.initUpstreamDefaults(tx); err != nil {
		return err
	}

	if err := db.initLoggingDefaults(tx); err != nil {
		return err
	}

	if err := db.initFilteringDefaults(tx); err != nil {
		return err
	}

	if err := db.initRateLimitDefaults(tx); err != nil {
		return err
	}

	if err := db.initAPIDefaults(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit defaults: %w", err)
	}

	return nil
}

func (db *DB) initServerDefaults(tx *sql.Tx) error {
	defaults := map[string]string{
		ConfigKeyServerHost:               "0.0.0.0",
		ConfigKeyServerPort:               "1053",
		ConfigKeyServerWorkers:            "auto",
		ConfigKeyServerMaxConcurrency:     "0",
		ConfigKeyServerUpstreamSocketPool: "0",
		ConfigKeyServerEnableTCP:          "true",
		ConfigKeyServerTCPFallback:        "true",
	}

	return insertDefaults(tx, defaults)
}

func (db *DB) initUpstreamDefaults(tx *sql.Tx) error {
	defaults := map[string]string{
		ConfigKeyUpstreamUDPTimeout: "3s",
		ConfigKeyUpstreamTCPTimeout: "5s",
		ConfigKeyUpstreamMaxRetries: "3",
	}

	if err := insertDefaults(tx, defaults); err != nil {
		return err
	}

	// Insert default upstream servers
	stmt, err := tx.Prepare(`
		INSERT INTO upstream_servers (server_address, priority, enabled)
		VALUES (?, ?, 1)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare upstream insert: %w", err)
	}
	defer stmt.Close()

	for i, server := range DefaultUpstreamServers {
		if _, err := stmt.Exec(server, i); err != nil {
			return fmt.Errorf("failed to insert default upstream %s: %w", server, err)
		}
	}

	return nil
}

func (db *DB) initLoggingDefaults(tx *sql.Tx) error {
	defaults := map[string]string{
		ConfigKeyLoggingLevel:            "INFO",
		ConfigKeyLoggingStructured:       "false",
		ConfigKeyLoggingStructuredFormat: "json",
		ConfigKeyLoggingIncludePID:       "false",
	}

	return insertDefaults(tx, defaults)
}

func (db *DB) initFilteringDefaults(tx *sql.Tx) error {
	defaults := map[string]string{
		ConfigKeyFilteringEnabled:         "false",
		ConfigKeyFilteringLogBlocked:      "true",
		ConfigKeyFilteringLogAllowed:      "false",
		ConfigKeyFilteringRefreshInterval: "24h",
	}

	return insertDefaults(tx, defaults)
}

func (db *DB) initRateLimitDefaults(tx *sql.Tx) error {
	defaults := map[string]string{
		ConfigKeyRateLimitCleanupSeconds:   "60.0",
		ConfigKeyRateLimitMaxIPEntries:     "65536",
		ConfigKeyRateLimitMaxPrefixEntries: "16384",
		ConfigKeyRateLimitGlobalQPS:        "100000.0",
		ConfigKeyRateLimitGlobalBurst:      "100000",
		ConfigKeyRateLimitPrefixQPS:        "10000.0",
		ConfigKeyRateLimitPrefixBurst:      "20000",
		ConfigKeyRateLimitIPQPS:            "5000.0",
		ConfigKeyRateLimitIPBurst:          "10000",
	}

	return insertDefaults(tx, defaults)
}

func (db *DB) initAPIDefaults(tx *sql.Tx) error {
	defaults := map[string]string{
		ConfigKeyAPIEnabled: "true",
		ConfigKeyAPIHost:    "0.0.0.0", // Bind to all interfaces for web UI access
		ConfigKeyAPIPort:    "8080",
		ConfigKeyAPIKey:     "", // No API key by default (for homelab ease of use)
	}

	return insertDefaults(tx, defaults)
}

// insertDefaults inserts config values only if they don't exist.
func insertDefaults(tx *sql.Tx, defaults map[string]string) error {
	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO config (key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare config insert: %w", err)
	}
	defer stmt.Close()

	for key, value := range defaults {
		if _, err := stmt.Exec(key, value); err != nil {
			return fmt.Errorf("failed to insert default %s: %w", key, err)
		}
	}

	return nil
}

// IsInitialized checks if the database has been initialized with defaults.
func (db *DB) IsInitialized() (bool, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM config").Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check config count: %w", err)
	}

	return count > 0, nil
}
