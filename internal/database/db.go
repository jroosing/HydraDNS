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
	"embed"
	"fmt"
	"sync"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

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

	// Run migrations
	if err := db.runMigrations(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
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

// runMigrations runs database migrations using golang-migrate.
func (db *DB) runMigrations() error {
	// Create migration source from embedded FS
	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	// Create database driver
	dbDriver, err := sqlite.WithInstance(db.conn, &sqlite.Config{})
	if err != nil {
		return fmt.Errorf("failed to create database driver: %w", err)
	}

	// Create migrator
	m, err := migrate.NewWithInstance("iofs", sourceDriver, "sqlite", dbDriver)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	// Run migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
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
