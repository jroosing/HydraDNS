package database

import (
	"fmt"
)

// Blocklist represents a remote blocklist source.
type Blocklist struct {
	ID          int64
	Name        string
	URL         string
	Format      string
	Enabled     bool
	LastFetched *string
}

// AddWhitelistDomain adds a domain to the whitelist.
func (db *DB) AddWhitelistDomain(domain string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := "INSERT OR IGNORE INTO filtering_whitelist (domain) VALUES (?)"

	_, err := db.conn.Exec(query, domain)
	if err != nil {
		return fmt.Errorf("failed to add whitelist domain %s: %w", domain, err)
	}

	return nil
}

// GetWhitelistDomains retrieves all whitelisted domains.
func (db *DB) GetWhitelistDomains() ([]string, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	rows, err := db.conn.Query("SELECT domain FROM filtering_whitelist ORDER BY domain")
	if err != nil {
		return nil, fmt.Errorf("failed to query whitelist: %w", err)
	}
	defer rows.Close()

	var domains []string
	for rows.Next() {
		var domain string
		if err := rows.Scan(&domain); err != nil {
			return nil, fmt.Errorf("failed to scan whitelist domain: %w", err)
		}
		domains = append(domains, domain)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating whitelist: %w", err)
	}

	return domains, nil
}

// DeleteWhitelistDomain removes a domain from the whitelist.
func (db *DB) DeleteWhitelistDomain(domain string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	result, err := db.conn.Exec("DELETE FROM filtering_whitelist WHERE domain = ?", domain)
	if err != nil {
		return fmt.Errorf("failed to delete whitelist domain: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("whitelist domain not found: %s", domain)
	}

	return nil
}

// AddBlacklistDomain adds a domain to the blacklist.
func (db *DB) AddBlacklistDomain(domain string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := "INSERT OR IGNORE INTO filtering_blacklist (domain) VALUES (?)"

	_, err := db.conn.Exec(query, domain)
	if err != nil {
		return fmt.Errorf("failed to add blacklist domain %s: %w", domain, err)
	}

	return nil
}

// GetBlacklistDomains retrieves all blacklisted domains.
func (db *DB) GetBlacklistDomains() ([]string, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	rows, err := db.conn.Query("SELECT domain FROM filtering_blacklist ORDER BY domain")
	if err != nil {
		return nil, fmt.Errorf("failed to query blacklist: %w", err)
	}
	defer rows.Close()

	var domains []string
	for rows.Next() {
		var domain string
		if err := rows.Scan(&domain); err != nil {
			return nil, fmt.Errorf("failed to scan blacklist domain: %w", err)
		}
		domains = append(domains, domain)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating blacklist: %w", err)
	}

	return domains, nil
}

// DeleteBlacklistDomain removes a domain from the blacklist.
func (db *DB) DeleteBlacklistDomain(domain string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	result, err := db.conn.Exec("DELETE FROM filtering_blacklist WHERE domain = ?", domain)
	if err != nil {
		return fmt.Errorf("failed to delete blacklist domain: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("blacklist domain not found: %s", domain)
	}

	return nil
}

// AddBlocklist adds a remote blocklist source.
func (db *DB) AddBlocklist(name, url, format string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := `
		INSERT INTO filtering_blocklists (name, url, format, enabled, updated_at)
		VALUES (?, ?, ?, 1, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET
			url = excluded.url,
			format = excluded.format,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := db.conn.Exec(query, name, url, format)
	if err != nil {
		return fmt.Errorf("failed to add blocklist %s: %w", name, err)
	}

	return nil
}

// GetBlocklists retrieves all blocklist sources.
func (db *DB) GetBlocklists() ([]Blocklist, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	query := `
		SELECT id, name, url, format, enabled, last_fetched
		FROM filtering_blocklists
		ORDER BY name
	`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query blocklists: %w", err)
	}
	defer rows.Close()

	var blocklists []Blocklist
	for rows.Next() {
		var b Blocklist
		if err := rows.Scan(&b.ID, &b.Name, &b.URL, &b.Format, &b.Enabled, &b.LastFetched); err != nil {
			return nil, fmt.Errorf("failed to scan blocklist: %w", err)
		}
		blocklists = append(blocklists, b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating blocklists: %w", err)
	}

	return blocklists, nil
}

// DeleteBlocklist removes a blocklist source.
func (db *DB) DeleteBlocklist(name string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	result, err := db.conn.Exec("DELETE FROM filtering_blocklists WHERE name = ?", name)
	if err != nil {
		return fmt.Errorf("failed to delete blocklist: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("blocklist not found: %s", name)
	}

	return nil
}

// EnableBlocklist enables/disables a blocklist.
func (db *DB) EnableBlocklist(name string, enabled bool) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := "UPDATE filtering_blocklists SET enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE name = ?"

	result, err := db.conn.Exec(query, enabled, name)
	if err != nil {
		return fmt.Errorf("failed to update blocklist: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("blocklist not found: %s", name)
	}

	return nil
}

// UpdateBlocklistFetchTime updates the last_fetched timestamp for a blocklist.
func (db *DB) UpdateBlocklistFetchTime(name string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	query := "UPDATE filtering_blocklists SET last_fetched = CURRENT_TIMESTAMP WHERE name = ?"

	result, err := db.conn.Exec(query, name)
	if err != nil {
		return fmt.Errorf("failed to update blocklist fetch time: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("blocklist not found: %s", name)
	}

	return nil
}
