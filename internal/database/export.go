package database

import (
	"fmt"

	"github.com/jroosing/hydradns/internal/config"
)

// ExportToConfig converts database configuration to a Config struct.
// This is used for compatibility with existing code that expects config.Config.
func (db *DB) ExportToConfig() (*config.Config, error) {
	cfg := &config.Config{}

	// Export server config
	if err := db.exportServerConfig(cfg); err != nil {
		return nil, err
	}

	// Export upstream config
	if err := db.exportUpstreamConfig(cfg); err != nil {
		return nil, err
	}

	// Export custom DNS
	if err := db.exportCustomDNS(cfg); err != nil {
		return nil, err
	}

	// Export logging config
	if err := db.exportLoggingConfig(cfg); err != nil {
		return nil, err
	}

	// Export filtering config
	if err := db.exportFilteringConfig(cfg); err != nil {
		return nil, err
	}

	// Export rate limit config
	if err := db.exportRateLimitConfig(cfg); err != nil {
		return nil, err
	}

	// Export API config
	if err := db.exportAPIConfig(cfg); err != nil {
		return nil, err
	}

	// Export cluster config
	if err := db.exportClusterConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (db *DB) exportServerConfig(cfg *config.Config) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var enableTCP, tcpFallback int
	err := db.conn.QueryRow(`
		SELECT host, port, workers, max_concurrency, upstream_socket_pool_size, enable_tcp, tcp_fallback
		FROM config_server WHERE id = 1
	`).Scan(
		&cfg.Server.Host,
		&cfg.Server.Port,
		&cfg.Server.WorkersRaw,
		&cfg.Server.MaxConcurrency,
		&cfg.Server.UpstreamSocketPoolSize,
		&enableTCP,
		&tcpFallback,
	)
	if err != nil {
		return fmt.Errorf("failed to read server config: %w", err)
	}

	cfg.Server.EnableTCP = enableTCP != 0
	cfg.Server.TCPFallback = tcpFallback != 0

	if err := cfg.Server.ParseWorkers(); err != nil {
		return fmt.Errorf("failed to parse workers: %w", err)
	}

	return nil
}

func (db *DB) exportUpstreamConfig(cfg *config.Config) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	err := db.conn.QueryRow(`
		SELECT udp_timeout, tcp_timeout, max_retries
		FROM config_upstream WHERE id = 1
	`).Scan(&cfg.Upstream.UDPTimeout, &cfg.Upstream.TCPTimeout, &cfg.Upstream.MaxRetries)
	if err != nil {
		return fmt.Errorf("failed to read upstream config: %w", err)
	}

	// Get upstream servers (need to release lock first)
	db.mu.RUnlock()
	servers, err := db.GetUpstreamServers()
	db.mu.RLock()
	if err != nil {
		return fmt.Errorf("failed to get upstream servers: %w", err)
	}

	cfg.Upstream.Servers = make([]string, len(servers))
	for i, server := range servers {
		cfg.Upstream.Servers[i] = server.ServerAddress
	}

	return nil
}

func (db *DB) exportCustomDNS(cfg *config.Config) error {
	// Get all hosts
	hosts, err := db.GetAllHosts()
	if err != nil {
		return fmt.Errorf("failed to get custom DNS hosts: %w", err)
	}

	// Group by hostname
	hostsMap := make(map[string][]string)
	for _, host := range hosts {
		hostsMap[host.Hostname] = append(hostsMap[host.Hostname], host.IPAddress)
	}
	cfg.CustomDNS.Hosts = hostsMap

	// Get all CNAMEs
	cnames, err := db.GetAllCNAMEs()
	if err != nil {
		return fmt.Errorf("failed to get custom DNS CNAMEs: %w", err)
	}

	cnamesMap := make(map[string]string)
	for _, cname := range cnames {
		cnamesMap[cname.Alias] = cname.Target
	}
	cfg.CustomDNS.CNAMEs = cnamesMap

	return nil
}

func (db *DB) exportLoggingConfig(cfg *config.Config) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var structured, includePID int
	err := db.conn.QueryRow(`
		SELECT level, structured, structured_format, include_pid
		FROM config_logging WHERE id = 1
	`).Scan(&cfg.Logging.Level, &structured, &cfg.Logging.StructuredFormat, &includePID)
	if err != nil {
		return fmt.Errorf("failed to read logging config: %w", err)
	}

	cfg.Logging.Structured = structured != 0
	cfg.Logging.IncludePID = includePID != 0

	// Extra fields not currently stored separately in DB
	cfg.Logging.ExtraFields = make(map[string]string)

	return nil
}

func (db *DB) exportFilteringConfig(cfg *config.Config) error {
	// Get filtering config from typed table
	filteringCfg, err := db.GetFilteringConfig()
	if err != nil {
		return fmt.Errorf("failed to get filtering config: %w", err)
	}

	cfg.Filtering.Enabled = filteringCfg.Enabled
	cfg.Filtering.LogBlocked = filteringCfg.LogBlocked
	cfg.Filtering.LogAllowed = filteringCfg.LogAllowed
	cfg.Filtering.RefreshInterval = filteringCfg.RefreshInterval

	// Get whitelist domains
	whitelist, err := db.GetWhitelistDomains()
	if err != nil {
		return fmt.Errorf("failed to get whitelist: %w", err)
	}
	cfg.Filtering.WhitelistDomains = whitelist

	// Get blacklist domains
	blacklist, err := db.GetBlacklistDomains()
	if err != nil {
		return fmt.Errorf("failed to get blacklist: %w", err)
	}
	cfg.Filtering.BlacklistDomains = blacklist

	// Get enabled blocklists only
	blocklists, err := db.GetBlocklists()
	if err != nil {
		return fmt.Errorf("failed to get blocklists: %w", err)
	}

	// Filter out disabled entries (engine currently does not track enabled state)
	enabled := make([]config.BlocklistConfig, 0, len(blocklists))
	for _, blocklist := range blocklists {
		if !blocklist.Enabled {
			continue
		}
		enabled = append(enabled, config.BlocklistConfig{
			Name:   blocklist.Name,
			URL:    blocklist.URL,
			Format: blocklist.Format,
		})
	}
	cfg.Filtering.Blocklists = enabled

	return nil
}

func (db *DB) exportRateLimitConfig(cfg *config.Config) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	err := db.conn.QueryRow(`
		SELECT cleanup_seconds, max_ip_entries, max_prefix_entries,
		       global_qps, global_burst, prefix_qps, prefix_burst, ip_qps, ip_burst
		FROM config_rate_limit WHERE id = 1
	`).Scan(
		&cfg.RateLimit.CleanupSeconds,
		&cfg.RateLimit.MaxIPEntries,
		&cfg.RateLimit.MaxPrefixEntries,
		&cfg.RateLimit.GlobalQPS,
		&cfg.RateLimit.GlobalBurst,
		&cfg.RateLimit.PrefixQPS,
		&cfg.RateLimit.PrefixBurst,
		&cfg.RateLimit.IPQPS,
		&cfg.RateLimit.IPBurst,
	)
	if err != nil {
		return fmt.Errorf("failed to read rate limit config: %w", err)
	}

	return nil
}

func (db *DB) exportAPIConfig(cfg *config.Config) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var enabled int
	err := db.conn.QueryRow(`
		SELECT enabled, host, port, api_key
		FROM config_api WHERE id = 1
	`).Scan(&enabled, &cfg.API.Host, &cfg.API.Port, &cfg.API.APIKey)
	if err != nil {
		return fmt.Errorf("failed to read API config: %w", err)
	}

	cfg.API.Enabled = enabled != 0

	return nil
}

func (db *DB) exportClusterConfig(cfg *config.Config) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var modeStr string
	err := db.conn.QueryRow(`
		SELECT mode, node_id, primary_url, shared_secret, sync_interval, sync_timeout
		FROM config_cluster WHERE id = 1
	`).Scan(
		&modeStr,
		&cfg.Cluster.NodeID,
		&cfg.Cluster.PrimaryURL,
		&cfg.Cluster.SharedSecret,
		&cfg.Cluster.SyncInterval,
		&cfg.Cluster.SyncTimeout,
	)
	if err != nil {
		return fmt.Errorf("failed to read cluster config: %w", err)
	}

	cfg.Cluster.Mode = config.ClusterMode(modeStr)

	return nil
}
