package database

import (
	"database/sql"
	"fmt"
	"net"
)

// CustomDNSHost represents an A or AAAA record.
type CustomDNSHost struct {
	ID         int64
	Hostname   string
	IPAddress  string
	RecordType string // "A" or "AAAA"
}

// CustomDNSCNAME represents a CNAME record.
type CustomDNSCNAME struct {
	ID     int64
	Alias  string
	Target string
}

// AddHost adds a custom DNS A or AAAA record.
func (db *DB) AddHost(hostname, ipAddress string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Determine record type
	ip := net.ParseIP(ipAddress)
	if ip == nil {
		return fmt.Errorf("invalid IP address: %s", ipAddress)
	}

	recordType := "AAAA"
	if ip.To4() != nil {
		recordType = "A"
	}

	query := `
		INSERT INTO custom_dns_records (source, type, target, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(source, target, type) DO UPDATE SET
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := db.conn.Exec(query, hostname, recordType, ipAddress)
	if err != nil {
		return fmt.Errorf("failed to add host %s: %w", hostname, err)
	}

	return nil
}

// GetHosts retrieves all A/AAAA records for a hostname.
func (db *DB) GetHosts(hostname string) ([]CustomDNSHost, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `
		SELECT id, source, target, type
		FROM custom_dns_records
		WHERE source = ? AND type IN ('A','AAAA')
		ORDER BY type, target
	`

	rows, err := db.conn.Query(query, hostname)
	if err != nil {
		return nil, fmt.Errorf("failed to query hosts: %w", err)
	}
	defer rows.Close()

	var hosts []CustomDNSHost
	for rows.Next() {
		var h CustomDNSHost
		if err := rows.Scan(&h.ID, &h.Hostname, &h.IPAddress, &h.RecordType); err != nil {
			return nil, fmt.Errorf("failed to scan host: %w", err)
		}
		hosts = append(hosts, h)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating hosts: %w", err)
	}

	return hosts, nil
}

// GetAllHosts retrieves all custom DNS host records.
func (db *DB) GetAllHosts() ([]CustomDNSHost, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `
		SELECT id, source, target, type
		FROM custom_dns_records
		WHERE type IN ('A','AAAA')
		ORDER BY source, type, target
	`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query hosts: %w", err)
	}
	defer rows.Close()

	var hosts []CustomDNSHost
	for rows.Next() {
		var h CustomDNSHost
		if err := rows.Scan(&h.ID, &h.Hostname, &h.IPAddress, &h.RecordType); err != nil {
			return nil, fmt.Errorf("failed to scan host: %w", err)
		}
		hosts = append(hosts, h)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating hosts: %w", err)
	}

	return hosts, nil
}

// DeleteHost removes a specific host record by hostname and IP.
func (db *DB) DeleteHost(hostname, ipAddress string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Determine record type to delete the precise entry
	ip := net.ParseIP(ipAddress)
	if ip == nil {
		return fmt.Errorf("invalid IP address: %s", ipAddress)
	}
	recordType := "AAAA"
	if ip.To4() != nil {
		recordType = "A"
	}

	query := "DELETE FROM custom_dns_records WHERE source = ? AND target = ? AND type = ?"
	result, err := db.conn.Exec(query, hostname, ipAddress, recordType)
	if err != nil {
		return fmt.Errorf("failed to delete host: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("host not found: %s -> %s", hostname, ipAddress)
	}

	return nil
}

// DeleteAllHostsForHostname removes all records for a hostname.
func (db *DB) DeleteAllHostsForHostname(hostname string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	_, err := db.conn.Exec("DELETE FROM custom_dns_records WHERE source = ? AND type IN ('A','AAAA')", hostname)
	if err != nil {
		return fmt.Errorf("failed to delete hosts for %s: %w", hostname, err)
	}

	return nil
}

// AddCNAME adds a custom DNS CNAME record.
func (db *DB) AddCNAME(alias, target string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// For CNAME, enforce a single target: delete any existing, then insert new
	if _, err := db.conn.Exec("DELETE FROM custom_dns_records WHERE source = ? AND type = 'CNAME'", alias); err != nil {
		return fmt.Errorf("failed to clear existing CNAME %s: %w", alias, err)
	}

	query := `
		INSERT INTO custom_dns_records (source, type, target, updated_at)
		VALUES (?, 'CNAME', ?, CURRENT_TIMESTAMP)
	`

	_, err := db.conn.Exec(query, alias, target)
	if err != nil {
		return fmt.Errorf("failed to add CNAME %s: %w", alias, err)
	}

	return nil
}

// GetCNAME retrieves the target for a CNAME alias.
func (db *DB) GetCNAME(alias string) (string, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var target string
	err := db.conn.QueryRow("SELECT target FROM custom_dns_records WHERE source = ? AND type = 'CNAME'", alias).Scan(&target)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("CNAME not found: %s", alias)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get CNAME: %w", err)
	}

	return target, nil
}

// GetAllCNAMEs retrieves all custom DNS CNAME records.
func (db *DB) GetAllCNAMEs() ([]CustomDNSCNAME, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := "SELECT id, source AS alias, target FROM custom_dns_records WHERE type = 'CNAME' ORDER BY source"

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query CNAMEs: %w", err)
	}
	defer rows.Close()

	var cnames []CustomDNSCNAME
	for rows.Next() {
		var c CustomDNSCNAME
		if err := rows.Scan(&c.ID, &c.Alias, &c.Target); err != nil {
			return nil, fmt.Errorf("failed to scan CNAME: %w", err)
		}
		cnames = append(cnames, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating CNAMEs: %w", err)
	}

	return cnames, nil
}

// DeleteCNAME removes a CNAME record.
func (db *DB) DeleteCNAME(alias string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	result, err := db.conn.Exec("DELETE FROM custom_dns_records WHERE source = ? AND type = 'CNAME'", alias)
	if err != nil {
		return fmt.Errorf("failed to delete CNAME: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("CNAME not found: %s", alias)
	}

	return nil
}
