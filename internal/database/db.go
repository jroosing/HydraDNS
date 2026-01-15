// Package database provides SQLite-backed configuration storage for HydraDNS.
//
// This package replaces YAML-based configuration with a relational database,
// enabling Technitium-style primary/secondary synchronization.
//
// The database stores:
//   - Server configuration (host, port, workers, etc.)
//   - Upstream DNS servers
//   - Custom DNS records (A, AAAA, CNAME)
//   - Filtering rules (whitelist, blacklist, blocklists)
//   - Logging and rate limit settings
//
// Config Version Tracking:
// Every modification to the database increments a global version counter
// via SQLite triggers. This enables efficient sync checks between nodes.
package database

import (
	"database/sql"
	_ "embed"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

//go:embed schema.sql
var schemaSQL string

// DB wraps a SQLite database connection with thread-safe operations.
type DB struct {
	conn *sql.DB
	mu   sync.RWMutex // Protects config reads/writes
}

// Open opens or creates a SQLite database at the given path.
// If the database doesn't exist, it will be created with the schema.
func Open(path string) (*DB, error) {
	// Use WAL mode for better concurrency
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL", path)

	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set reasonable connection pool limits
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(time.Hour)

	db := &DB{conn: conn}

	// Initialize schema if needed
	if err := db.initSchema(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Initialize defaults if this is a fresh database
	if err := db.InitDefaults(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize defaults: %w", err)
	}

	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// initSchema creates tables if they don't exist.
func (db *DB) initSchema() error {
	_, err := db.conn.Exec(schemaSQL)
	if err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}
	return nil
}

// GetVersion returns the current configuration version.
// This version increments on every modification (via triggers).
func (db *DB) GetVersion() (int64, error) {
	var version int64
	err := db.conn.QueryRow("SELECT version FROM config_version WHERE id = 1").Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("failed to get config version: %w", err)
	}
	return version, nil
}

// BeginTx starts a transaction for atomic multi-table operations.
func (db *DB) BeginTx() (*sql.Tx, error) {
	return db.conn.Begin()
}

// Health checks database connectivity.
func (db *DB) Health() error {
	return db.conn.Ping()
}
