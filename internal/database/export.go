package database

import (
	"fmt"
	"strconv"

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

	return cfg, nil
}

func (db *DB) exportServerConfig(cfg *config.Config) error {
	cfg.Server.Host = db.GetConfigWithDefault(ConfigKeyServerHost, "0.0.0.0")

	portStr := db.GetConfigWithDefault(ConfigKeyServerPort, "1053")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid server.port: %w", err)
	}
	cfg.Server.Port = port

	cfg.Server.WorkersRaw = db.GetConfigWithDefault(ConfigKeyServerWorkers, "auto")
	if err := cfg.Server.ParseWorkers(); err != nil {
		return fmt.Errorf("failed to parse workers: %w", err)
	}

	maxConcurrencyStr := db.GetConfigWithDefault(ConfigKeyServerMaxConcurrency, "0")
	maxConcurrency, err := strconv.Atoi(maxConcurrencyStr)
	if err != nil {
		return fmt.Errorf("invalid max_concurrency: %w", err)
	}
	cfg.Server.MaxConcurrency = maxConcurrency

	poolSizeStr := db.GetConfigWithDefault(ConfigKeyServerUpstreamSocketPool, "0")
	poolSize, err := strconv.Atoi(poolSizeStr)
	if err != nil {
		return fmt.Errorf("invalid upstream_socket_pool_size: %w", err)
	}
	cfg.Server.UpstreamSocketPoolSize = poolSize

	enableTCPStr := db.GetConfigWithDefault(ConfigKeyServerEnableTCP, "true")
	cfg.Server.EnableTCP, _ = strconv.ParseBool(enableTCPStr)

	tcpFallbackStr := db.GetConfigWithDefault(ConfigKeyServerTCPFallback, "true")
	cfg.Server.TCPFallback, _ = strconv.ParseBool(tcpFallbackStr)

	return nil
}

func (db *DB) exportUpstreamConfig(cfg *config.Config) error {
	cfg.Upstream.UDPTimeout = db.GetConfigWithDefault(ConfigKeyUpstreamUDPTimeout, "3s")
	cfg.Upstream.TCPTimeout = db.GetConfigWithDefault(ConfigKeyUpstreamTCPTimeout, "5s")

	maxRetriesStr := db.GetConfigWithDefault(ConfigKeyUpstreamMaxRetries, "3")
	maxRetries, err := strconv.Atoi(maxRetriesStr)
	if err != nil {
		return fmt.Errorf("invalid upstream.max_retries: %w", err)
	}
	cfg.Upstream.MaxRetries = maxRetries

	// Get upstream servers
	servers, err := db.GetUpstreamServers()
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
	cfg.Logging.Level = db.GetConfigWithDefault(ConfigKeyLoggingLevel, "INFO")

	structuredStr := db.GetConfigWithDefault(ConfigKeyLoggingStructured, "false")
	cfg.Logging.Structured, _ = strconv.ParseBool(structuredStr)

	cfg.Logging.StructuredFormat = db.GetConfigWithDefault(ConfigKeyLoggingStructuredFormat, "json")

	includePIDStr := db.GetConfigWithDefault(ConfigKeyLoggingIncludePID, "false")
	cfg.Logging.IncludePID, _ = strconv.ParseBool(includePIDStr)

	// Extra fields not currently stored separately in DB
	cfg.Logging.ExtraFields = make(map[string]string)

	return nil
}

func (db *DB) exportFilteringConfig(cfg *config.Config) error {
	enabledStr := db.GetConfigWithDefault(ConfigKeyFilteringEnabled, "false")
	cfg.Filtering.Enabled, _ = strconv.ParseBool(enabledStr)

	logBlockedStr := db.GetConfigWithDefault(ConfigKeyFilteringLogBlocked, "true")
	cfg.Filtering.LogBlocked, _ = strconv.ParseBool(logBlockedStr)

	logAllowedStr := db.GetConfigWithDefault(ConfigKeyFilteringLogAllowed, "false")
	cfg.Filtering.LogAllowed, _ = strconv.ParseBool(logAllowedStr)

	cfg.Filtering.RefreshInterval = db.GetConfigWithDefault(ConfigKeyFilteringRefreshInterval, "24h")

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
	cleanupSecondsStr := db.GetConfigWithDefault(ConfigKeyRateLimitCleanupSeconds, "60.0")
	cleanupSeconds, err := strconv.ParseFloat(cleanupSecondsStr, 64)
	if err != nil {
		return fmt.Errorf("invalid rate_limit.cleanup_seconds: %w", err)
	}
	cfg.RateLimit.CleanupSeconds = cleanupSeconds

	maxIPEntriesStr := db.GetConfigWithDefault(ConfigKeyRateLimitMaxIPEntries, "65536")
	maxIPEntries, err := strconv.Atoi(maxIPEntriesStr)
	if err != nil {
		return fmt.Errorf("invalid rate_limit.max_ip_entries: %w", err)
	}
	cfg.RateLimit.MaxIPEntries = maxIPEntries

	maxPrefixEntriesStr := db.GetConfigWithDefault(ConfigKeyRateLimitMaxPrefixEntries, "16384")
	maxPrefixEntries, err := strconv.Atoi(maxPrefixEntriesStr)
	if err != nil {
		return fmt.Errorf("invalid rate_limit.max_prefix_entries: %w", err)
	}
	cfg.RateLimit.MaxPrefixEntries = maxPrefixEntries

	globalQPSStr := db.GetConfigWithDefault(ConfigKeyRateLimitGlobalQPS, "100000.0")
	globalQPS, err := strconv.ParseFloat(globalQPSStr, 64)
	if err != nil {
		return fmt.Errorf("invalid rate_limit.global_qps: %w", err)
	}
	cfg.RateLimit.GlobalQPS = globalQPS

	globalBurstStr := db.GetConfigWithDefault(ConfigKeyRateLimitGlobalBurst, "100000")
	globalBurst, err := strconv.Atoi(globalBurstStr)
	if err != nil {
		return fmt.Errorf("invalid rate_limit.global_burst: %w", err)
	}
	cfg.RateLimit.GlobalBurst = globalBurst

	prefixQPSStr := db.GetConfigWithDefault(ConfigKeyRateLimitPrefixQPS, "10000.0")
	prefixQPS, err := strconv.ParseFloat(prefixQPSStr, 64)
	if err != nil {
		return fmt.Errorf("invalid rate_limit.prefix_qps: %w", err)
	}
	cfg.RateLimit.PrefixQPS = prefixQPS

	prefixBurstStr := db.GetConfigWithDefault(ConfigKeyRateLimitPrefixBurst, "20000")
	prefixBurst, err := strconv.Atoi(prefixBurstStr)
	if err != nil {
		return fmt.Errorf("invalid rate_limit.prefix_burst: %w", err)
	}
	cfg.RateLimit.PrefixBurst = prefixBurst

	ipQPSStr := db.GetConfigWithDefault(ConfigKeyRateLimitIPQPS, "5000.0")
	ipQPS, err := strconv.ParseFloat(ipQPSStr, 64)
	if err != nil {
		return fmt.Errorf("invalid rate_limit.ip_qps: %w", err)
	}
	cfg.RateLimit.IPQPS = ipQPS

	ipBurstStr := db.GetConfigWithDefault(ConfigKeyRateLimitIPBurst, "10000")
	ipBurst, err := strconv.Atoi(ipBurstStr)
	if err != nil {
		return fmt.Errorf("invalid rate_limit.ip_burst: %w", err)
	}
	cfg.RateLimit.IPBurst = ipBurst

	return nil
}

func (db *DB) exportAPIConfig(cfg *config.Config) error {
	enabledStr := db.GetConfigWithDefault(ConfigKeyAPIEnabled, "true")
	cfg.API.Enabled, _ = strconv.ParseBool(enabledStr)

	cfg.API.Host = db.GetConfigWithDefault(ConfigKeyAPIHost, "127.0.0.1")

	portStr := db.GetConfigWithDefault(ConfigKeyAPIPort, "8080")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid api.port: %w", err)
	}
	cfg.API.Port = port

	cfg.API.APIKey = db.GetConfigWithDefault(ConfigKeyAPIKey, "")

	return nil
}
