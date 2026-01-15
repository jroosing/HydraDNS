package database

import (
	"fmt"
)

// UpstreamServer represents an upstream DNS server.
type UpstreamServer struct {
	ID            int64
	ServerAddress string
	Priority      int
	Enabled       bool
}

// AddUpstreamServer adds an upstream server with the given priority.
func (db *DB) AddUpstreamServer(serverAddress string, priority int) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `
		INSERT INTO upstream_servers (server_address, priority, enabled, updated_at)
		VALUES (?, ?, 1, CURRENT_TIMESTAMP)
		ON CONFLICT(server_address) DO UPDATE SET
			priority = excluded.priority,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := db.conn.Exec(query, serverAddress, priority)
	if err != nil {
		return fmt.Errorf("failed to add upstream server %s: %w", serverAddress, err)
	}

	return nil
}

// GetUpstreamServers retrieves all upstream servers ordered by priority.
func (db *DB) GetUpstreamServers() ([]UpstreamServer, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `
		SELECT id, server_address, priority, enabled
		FROM upstream_servers
		WHERE enabled = 1
		ORDER BY priority
	`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query upstream servers: %w", err)
	}
	defer rows.Close()

	var servers []UpstreamServer
	for rows.Next() {
		var s UpstreamServer
		if err := rows.Scan(&s.ID, &s.ServerAddress, &s.Priority, &s.Enabled); err != nil {
			return nil, fmt.Errorf("failed to scan upstream server: %w", err)
		}
		servers = append(servers, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating upstream servers: %w", err)
	}

	return servers, nil
}

// SetUpstreamServers replaces all upstream servers with the given list.
// Priority is determined by list order (0 = first, 1 = second, etc.).
func (db *DB) SetUpstreamServers(servers []string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete all existing servers
	if _, err := tx.Exec("DELETE FROM upstream_servers"); err != nil {
		return fmt.Errorf("failed to delete existing servers: %w", err)
	}

	// Insert new servers
	stmt, err := tx.Prepare(`
		INSERT INTO upstream_servers (server_address, priority, enabled, updated_at)
		VALUES (?, ?, 1, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for i, server := range servers {
		if _, err := stmt.Exec(server, i); err != nil {
			return fmt.Errorf("failed to insert server %s: %w", server, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// DeleteUpstreamServer removes an upstream server.
func (db *DB) DeleteUpstreamServer(serverAddress string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	result, err := db.conn.Exec("DELETE FROM upstream_servers WHERE server_address = ?", serverAddress)
	if err != nil {
		return fmt.Errorf("failed to delete upstream server: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("upstream server not found: %s", serverAddress)
	}

	return nil
}

// EnableUpstreamServer enables/disables an upstream server.
func (db *DB) EnableUpstreamServer(serverAddress string, enabled bool) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := "UPDATE upstream_servers SET enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE server_address = ?"

	result, err := db.conn.Exec(query, enabled, serverAddress)
	if err != nil {
		return fmt.Errorf("failed to update upstream server: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("upstream server not found: %s", serverAddress)
	}

	return nil
}
