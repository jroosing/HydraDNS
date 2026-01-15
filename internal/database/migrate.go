package database

import (
	"database/sql"
	"fmt"

	"github.com/jroosing/hydradns/internal/config"
)

// MigrateFromConfig populates the database from a YAML-based config.
// This is used for initial migration or importing config.
func (db *DB) MigrateFromConfig(cfg *config.Config) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Migrate server config
	if err := db.migrateServerConfig(tx, cfg); err != nil {
		return err
	}

	// Migrate upstream config
	if err := db.migrateUpstreamConfig(tx, cfg); err != nil {
		return err
	}

	// Migrate custom DNS
	if err := db.migrateCustomDNS(tx, cfg); err != nil {
		return err
	}

	// Migrate logging config
	if err := db.migrateLoggingConfig(tx, cfg); err != nil {
		return err
	}

	// Migrate filtering config
	if err := db.migrateFilteringConfig(tx, cfg); err != nil {
		return err
	}

	// Migrate rate limit config
	if err := db.migrateRateLimitConfig(tx, cfg); err != nil {
		return err
	}

	// Migrate API config
	if err := db.migrateAPIConfig(tx, cfg); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration: %w", err)
	}

	return nil
}

func (db *DB) migrateServerConfig(tx txExec, cfg *config.Config) error {
	configs := map[string]string{
		ConfigKeyServerHost:               cfg.Server.Host,
		ConfigKeyServerPort:               fmt.Sprintf("%d", cfg.Server.Port),
		ConfigKeyServerWorkers:            cfg.Server.Workers.String(),
		ConfigKeyServerMaxConcurrency:     fmt.Sprintf("%d", cfg.Server.MaxConcurrency),
		ConfigKeyServerUpstreamSocketPool: fmt.Sprintf("%d", cfg.Server.UpstreamSocketPoolSize),
		ConfigKeyServerEnableTCP:          fmt.Sprintf("%t", cfg.Server.EnableTCP),
		ConfigKeyServerTCPFallback:        fmt.Sprintf("%t", cfg.Server.TCPFallback),
	}

	return setConfigInTx(tx, configs)
}

func (db *DB) migrateUpstreamConfig(tx txExec, cfg *config.Config) error {
	// Set upstream timeouts and retries
	configs := map[string]string{
		ConfigKeyUpstreamUDPTimeout: cfg.Upstream.UDPTimeout,
		ConfigKeyUpstreamTCPTimeout: cfg.Upstream.TCPTimeout,
		ConfigKeyUpstreamMaxRetries: fmt.Sprintf("%d", cfg.Upstream.MaxRetries),
	}

	if err := setConfigInTx(tx, configs); err != nil {
		return err
	}

	// Clear existing upstream servers
	if _, err := tx.Exec("DELETE FROM upstream_servers"); err != nil {
		return fmt.Errorf("failed to clear upstream servers: %w", err)
	}

	// Insert upstream servers
	stmt, err := tx.Prepare(`
		INSERT INTO upstream_servers (server_address, priority, enabled)
		VALUES (?, ?, 1)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare upstream insert: %w", err)
	}
	defer stmt.Close()

	for i, server := range cfg.Upstream.Servers {
		if _, err := stmt.Exec(server, i); err != nil {
			return fmt.Errorf("failed to insert upstream server %s: %w", server, err)
		}
	}

	return nil
}

func (db *DB) migrateCustomDNS(tx txExec, cfg *config.Config) error {
	// Clear existing custom DNS records
	if _, err := tx.Exec("DELETE FROM custom_dns_hosts"); err != nil {
		return fmt.Errorf("failed to clear custom DNS hosts: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM custom_dns_cnames"); err != nil {
		return fmt.Errorf("failed to clear custom DNS CNAMEs: %w", err)
	}

	// Insert hosts
	if len(cfg.CustomDNS.Hosts) > 0 {
		hostStmt, err := tx.Prepare(`
			INSERT INTO custom_dns_hosts (hostname, ip_address, record_type)
			VALUES (?, ?, ?)
		`)
		if err != nil {
			return fmt.Errorf("failed to prepare host insert: %w", err)
		}
		defer hostStmt.Close()

		for hostname, ips := range cfg.CustomDNS.Hosts {
			for _, ip := range ips {
				recordType := determineRecordType(ip)
				if _, err := hostStmt.Exec(hostname, ip, recordType); err != nil {
					return fmt.Errorf("failed to insert host %s -> %s: %w", hostname, ip, err)
				}
			}
		}
	}

	// Insert CNAMEs
	if len(cfg.CustomDNS.CNAMEs) > 0 {
		cnameStmt, err := tx.Prepare(`
			INSERT INTO custom_dns_cnames (alias, target)
			VALUES (?, ?)
		`)
		if err != nil {
			return fmt.Errorf("failed to prepare CNAME insert: %w", err)
		}
		defer cnameStmt.Close()

		for alias, target := range cfg.CustomDNS.CNAMEs {
			if _, err := cnameStmt.Exec(alias, target); err != nil {
				return fmt.Errorf("failed to insert CNAME %s -> %s: %w", alias, target, err)
			}
		}
	}

	return nil
}

func (db *DB) migrateLoggingConfig(tx txExec, cfg *config.Config) error {
	configs := map[string]string{
		ConfigKeyLoggingLevel:            cfg.Logging.Level,
		ConfigKeyLoggingStructured:       fmt.Sprintf("%t", cfg.Logging.Structured),
		ConfigKeyLoggingStructuredFormat: cfg.Logging.StructuredFormat,
		ConfigKeyLoggingIncludePID:       fmt.Sprintf("%t", cfg.Logging.IncludePID),
	}

	// Add extra fields as separate keys (if needed in the future)
	// For now, we'll skip extra_fields as they're rarely used

	return setConfigInTx(tx, configs)
}

func (db *DB) migrateFilteringConfig(tx txExec, cfg *config.Config) error {
	// Set filtering config values
	configs := map[string]string{
		ConfigKeyFilteringEnabled:         fmt.Sprintf("%t", cfg.Filtering.Enabled),
		ConfigKeyFilteringLogBlocked:      fmt.Sprintf("%t", cfg.Filtering.LogBlocked),
		ConfigKeyFilteringLogAllowed:      fmt.Sprintf("%t", cfg.Filtering.LogAllowed),
		ConfigKeyFilteringRefreshInterval: cfg.Filtering.RefreshInterval,
	}

	if err := setConfigInTx(tx, configs); err != nil {
		return err
	}

	// Clear existing filtering data
	if _, err := tx.Exec("DELETE FROM filtering_whitelist"); err != nil {
		return fmt.Errorf("failed to clear whitelist: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM filtering_blacklist"); err != nil {
		return fmt.Errorf("failed to clear blacklist: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM filtering_blocklists"); err != nil {
		return fmt.Errorf("failed to clear blocklists: %w", err)
	}

	// Insert whitelist domains
	if len(cfg.Filtering.WhitelistDomains) > 0 {
		whitelistStmt, err := tx.Prepare("INSERT INTO filtering_whitelist (domain) VALUES (?)")
		if err != nil {
			return fmt.Errorf("failed to prepare whitelist insert: %w", err)
		}
		defer whitelistStmt.Close()

		for _, domain := range cfg.Filtering.WhitelistDomains {
			if _, err := whitelistStmt.Exec(domain); err != nil {
				return fmt.Errorf("failed to insert whitelist domain %s: %w", domain, err)
			}
		}
	}

	// Insert blacklist domains
	if len(cfg.Filtering.BlacklistDomains) > 0 {
		blacklistStmt, err := tx.Prepare("INSERT INTO filtering_blacklist (domain) VALUES (?)")
		if err != nil {
			return fmt.Errorf("failed to prepare blacklist insert: %w", err)
		}
		defer blacklistStmt.Close()

		for _, domain := range cfg.Filtering.BlacklistDomains {
			if _, err := blacklistStmt.Exec(domain); err != nil {
				return fmt.Errorf("failed to insert blacklist domain %s: %w", domain, err)
			}
		}
	}

	// Insert blocklists
	if len(cfg.Filtering.Blocklists) > 0 {
		blocklistStmt, err := tx.Prepare(`
			INSERT INTO filtering_blocklists (name, url, format, enabled)
			VALUES (?, ?, ?, 1)
		`)
		if err != nil {
			return fmt.Errorf("failed to prepare blocklist insert: %w", err)
		}
		defer blocklistStmt.Close()

		for _, blocklist := range cfg.Filtering.Blocklists {
			if _, err := blocklistStmt.Exec(blocklist.Name, blocklist.URL, blocklist.Format); err != nil {
				return fmt.Errorf("failed to insert blocklist %s: %w", blocklist.Name, err)
			}
		}
	}

	return nil
}

func (db *DB) migrateRateLimitConfig(tx txExec, cfg *config.Config) error {
	configs := map[string]string{
		ConfigKeyRateLimitCleanupSeconds:   fmt.Sprintf("%f", cfg.RateLimit.CleanupSeconds),
		ConfigKeyRateLimitMaxIPEntries:     fmt.Sprintf("%d", cfg.RateLimit.MaxIPEntries),
		ConfigKeyRateLimitMaxPrefixEntries: fmt.Sprintf("%d", cfg.RateLimit.MaxPrefixEntries),
		ConfigKeyRateLimitGlobalQPS:        fmt.Sprintf("%f", cfg.RateLimit.GlobalQPS),
		ConfigKeyRateLimitGlobalBurst:      fmt.Sprintf("%d", cfg.RateLimit.GlobalBurst),
		ConfigKeyRateLimitPrefixQPS:        fmt.Sprintf("%f", cfg.RateLimit.PrefixQPS),
		ConfigKeyRateLimitPrefixBurst:      fmt.Sprintf("%d", cfg.RateLimit.PrefixBurst),
		ConfigKeyRateLimitIPQPS:            fmt.Sprintf("%f", cfg.RateLimit.IPQPS),
		ConfigKeyRateLimitIPBurst:          fmt.Sprintf("%d", cfg.RateLimit.IPBurst),
	}

	return setConfigInTx(tx, configs)
}

func (db *DB) migrateAPIConfig(tx txExec, cfg *config.Config) error {
	configs := map[string]string{
		ConfigKeyAPIEnabled: fmt.Sprintf("%t", cfg.API.Enabled),
		ConfigKeyAPIHost:    cfg.API.Host,
		ConfigKeyAPIPort:    fmt.Sprintf("%d", cfg.API.Port),
		ConfigKeyAPIKey:     cfg.API.APIKey,
	}

	return setConfigInTx(tx, configs)
}

// Helper types and functions

type txExec interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)
}

func setConfigInTx(tx txExec, configs map[string]string) error {
	stmt, err := tx.Prepare(`
		INSERT INTO config (key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare config insert: %w", err)
	}
	defer stmt.Close()

	for key, value := range configs {
		if _, err := stmt.Exec(key, value); err != nil {
			return fmt.Errorf("failed to set config %s: %w", key, err)
		}
	}

	return nil
}

func determineRecordType(ipAddress string) string {
	// Simple heuristic: if it contains ':', it's IPv6
	for _, c := range ipAddress {
		if c == ':' {
			return "AAAA"
		}
	}
	return "A"
}
