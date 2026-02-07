package database

import (
	"database/sql"
	"fmt"
	"net"
	"strconv"

	"github.com/jroosing/hydradns/internal/cluster"
	"github.com/jroosing/hydradns/internal/config"
)

// ImportFromCluster imports configuration data from a cluster export.
// This is used by secondary nodes to sync configuration from the primary.
// It replaces the following configuration sections:
//   - Upstream servers
//   - Custom DNS (hosts and CNAMEs)
//   - Filtering (whitelist, blacklist, enabled state)
//
// It does NOT replace:
//   - Server settings (host, port, workers)
//   - API settings
//   - Cluster settings
//   - Rate limit settings (node-specific)
//   - Logging settings (node-specific)
func (db *DB) ImportFromCluster(data *cluster.ExportData) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Import upstream servers
	if err := db.importUpstreamTx(tx, data.Upstream); err != nil {
		return fmt.Errorf("import upstream: %w", err)
	}

	// Import custom DNS
	if err := db.importCustomDNSTx(tx, data.CustomDNS); err != nil {
		return fmt.Errorf("import custom DNS: %w", err)
	}

	// Import filtering config
	if err := db.importFilteringTx(tx, data.Filtering); err != nil {
		return fmt.Errorf("import filtering: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (db *DB) importUpstreamTx(tx *sql.Tx, upstream config.UpstreamConfig) error {
	// Clear existing upstream servers
	if _, err := tx.Exec("DELETE FROM upstream_servers"); err != nil {
		return fmt.Errorf("clear upstream servers: %w", err)
	}

	// Insert new upstream servers
	for i, server := range upstream.Servers {
		_, err := tx.Exec(`
			INSERT INTO upstream_servers (server_address, priority, enabled, updated_at)
			VALUES (?, ?, 1, CURRENT_TIMESTAMP)
		`, server, i)
		if err != nil {
			return fmt.Errorf("insert upstream server %s: %w", server, err)
		}
	}

	// Update upstream config settings
	if _, err := tx.Exec(`
		UPDATE config_upstream SET
			udp_timeout = ?,
			tcp_timeout = ?,
			max_retries = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`, upstream.UDPTimeout, upstream.TCPTimeout, upstream.MaxRetries); err != nil {
		return fmt.Errorf("update upstream config: %w", err)
	}

	return nil
}

func (db *DB) importCustomDNSTx(tx *sql.Tx, customDNS config.CustomDNSConfig) error {
	// Clear existing custom DNS records
	if _, err := tx.Exec("DELETE FROM custom_dns_records"); err != nil {
		return fmt.Errorf("clear custom DNS records: %w", err)
	}

	// Insert host records
	for hostname, ips := range customDNS.Hosts {
		for _, ipStr := range ips {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				continue // Skip invalid IPs
			}

			recordType := "AAAA"
			if ip.To4() != nil {
				recordType = "A"
			}

			_, err := tx.Exec(`
				INSERT INTO custom_dns_records (source, type, target, updated_at)
				VALUES (?, ?, ?, CURRENT_TIMESTAMP)
			`, hostname, recordType, ipStr)
			if err != nil {
				return fmt.Errorf("insert host %s: %w", hostname, err)
			}
		}
	}

	// Insert CNAME records
	for alias, target := range customDNS.CNAMEs {
		_, err := tx.Exec(`
			INSERT INTO custom_dns_records (source, type, target, updated_at)
			VALUES (?, 'CNAME', ?, CURRENT_TIMESTAMP)
		`, alias, target)
		if err != nil {
			return fmt.Errorf("insert CNAME %s: %w", alias, err)
		}
	}

	return nil
}

func (db *DB) importFilteringTx(tx *sql.Tx, filtering config.FilteringConfig) error {
	// Update filtering config
	if _, err := tx.Exec(`
		UPDATE config_filtering SET
			enabled = ?,
			log_blocked = ?,
			log_allowed = ?,
			refresh_interval = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`, filtering.Enabled, filtering.LogBlocked, filtering.LogAllowed, filtering.RefreshInterval); err != nil {
		return fmt.Errorf("update filtering config: %w", err)
	}

	// Clear and repopulate whitelist
	if _, err := tx.Exec("DELETE FROM filtering_whitelist"); err != nil {
		return fmt.Errorf("clear whitelist: %w", err)
	}

	for _, domain := range filtering.WhitelistDomains {
		_, err := tx.Exec("INSERT INTO filtering_whitelist (domain) VALUES (?)", domain)
		if err != nil {
			return fmt.Errorf("insert whitelist domain %s: %w", domain, err)
		}
	}

	// Clear and repopulate blacklist
	if _, err := tx.Exec("DELETE FROM filtering_blacklist"); err != nil {
		return fmt.Errorf("clear blacklist: %w", err)
	}

	for _, domain := range filtering.BlacklistDomains {
		_, err := tx.Exec("INSERT INTO filtering_blacklist (domain) VALUES (?)", domain)
		if err != nil {
			return fmt.Errorf("insert blacklist domain %s: %w", domain, err)
		}
	}

	// Clear and repopulate blocklists
	if _, err := tx.Exec("DELETE FROM filtering_blocklists"); err != nil {
		return fmt.Errorf("clear blocklists: %w", err)
	}

	for _, blocklist := range filtering.Blocklists {
		_, err := tx.Exec(`
			INSERT INTO filtering_blocklists (name, url, format, enabled, updated_at)
			VALUES (?, ?, ?, 1, CURRENT_TIMESTAMP)
		`, blocklist.Name, blocklist.URL, blocklist.Format)
		if err != nil {
			return fmt.Errorf("insert blocklist %s: %w", blocklist.Name, err)
		}
	}

	return nil
}

// SetClusterConfig updates cluster configuration settings.
func (db *DB) SetClusterConfig(cfg *config.ClusterConfig) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	_, err := db.conn.Exec(`
		UPDATE config_cluster SET
			mode = ?,
			node_id = ?,
			primary_url = ?,
			shared_secret = ?,
			sync_interval = ?,
			sync_timeout = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`, string(cfg.Mode), cfg.NodeID, cfg.PrimaryURL, cfg.SharedSecret, cfg.SyncInterval, cfg.SyncTimeout)
	if err != nil {
		return fmt.Errorf("failed to update cluster config: %w", err)
	}

	return nil
}

// SetUpstreamConfigTyped updates the typed upstream configuration.
func (db *DB) SetUpstreamConfigTyped(cfg *config.UpstreamConfig) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	_, err := db.conn.Exec(`
		UPDATE config_upstream SET
			udp_timeout = ?,
			tcp_timeout = ?,
			max_retries = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`, cfg.UDPTimeout, cfg.TCPTimeout, cfg.MaxRetries)

	if err != nil {
		return fmt.Errorf("failed to update upstream config: %w", err)
	}

	return nil
}

// SetFilteringConfigTyped updates the typed filtering configuration.
func (db *DB) SetFilteringConfigTyped(cfg *config.FilteringConfig) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	_, err := db.conn.Exec(`
		UPDATE config_filtering SET
			enabled = ?,
			log_blocked = ?,
			log_allowed = ?,
			refresh_interval = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`, cfg.Enabled, cfg.LogBlocked, cfg.LogAllowed, cfg.RefreshInterval)

	if err != nil {
		return fmt.Errorf("failed to update filtering config: %w", err)
	}

	return nil
}

// GetClusterConfig retrieves the cluster configuration.
func (db *DB) GetClusterConfig() (*config.ClusterConfig, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var modeStr string
	cfg := &config.ClusterConfig{}

	err := db.conn.QueryRow(`
		SELECT mode, node_id, primary_url, shared_secret, sync_interval, sync_timeout
		FROM config_cluster WHERE id = 1
	`).Scan(
		&modeStr,
		&cfg.NodeID,
		&cfg.PrimaryURL,
		&cfg.SharedSecret,
		&cfg.SyncInterval,
		&cfg.SyncTimeout,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to read cluster config: %w", err)
	}

	cfg.Mode = config.ClusterMode(modeStr)
	return cfg, nil
}

// IncrementVersion manually increments the config version.
// This is useful after bulk imports.
func (db *DB) IncrementVersion() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	_, err := db.conn.Exec(
		"UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1",
	)
	if err != nil {
		return fmt.Errorf("failed to increment version: %w", err)
	}

	return nil
}

// SetVersion sets the config version to a specific value.
// This is used during cluster sync to match the primary's version.
func (db *DB) SetVersion(version int64) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	_, err := db.conn.Exec(
		"UPDATE config_version SET version = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1",
		version,
	)
	if err != nil {
		return fmt.Errorf("failed to set version: %w", err)
	}

	return nil
}

// boolToInt converts a bool to 0 or 1 for SQLite.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// intToBool converts 0/1 to bool.
func intToBool(i int) bool {
	return i != 0
}

// intToStr converts int to string.
func intToStr(i int) string {
	return strconv.Itoa(i)
}
