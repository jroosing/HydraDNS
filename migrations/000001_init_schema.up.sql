-- HydraDNS SQLite Schema
-- Initial migration creating all tables

-- Schema version tracking
CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Configuration metadata (server, upstream, logging, etc.)
CREATE TABLE IF NOT EXISTS config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Track overall config version for sync
CREATE TABLE IF NOT EXISTS config_version (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    version INTEGER NOT NULL DEFAULT 1,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Typed configuration tables (single-row each, id = 1)
CREATE TABLE IF NOT EXISTS config_server (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    host TEXT NOT NULL DEFAULT '0.0.0.0',
    port INTEGER NOT NULL DEFAULT 1053,
    workers TEXT NOT NULL DEFAULT 'auto',
    max_concurrency INTEGER NOT NULL DEFAULT 0,
    upstream_socket_pool_size INTEGER NOT NULL DEFAULT 0,
    enable_tcp BOOLEAN NOT NULL DEFAULT 1,
    tcp_fallback BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS config_upstream (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    udp_timeout TEXT NOT NULL DEFAULT '3s',
    tcp_timeout TEXT NOT NULL DEFAULT '5s',
    max_retries INTEGER NOT NULL DEFAULT 3,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS config_logging (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    level TEXT NOT NULL DEFAULT 'INFO',
    structured BOOLEAN NOT NULL DEFAULT 0,
    structured_format TEXT NOT NULL DEFAULT 'json',
    include_pid BOOLEAN NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS config_filtering (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    enabled BOOLEAN NOT NULL DEFAULT 0,
    log_blocked BOOLEAN NOT NULL DEFAULT 1,
    log_allowed BOOLEAN NOT NULL DEFAULT 0,
    refresh_interval TEXT NOT NULL DEFAULT '24h',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS config_rate_limit (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    cleanup_seconds REAL NOT NULL DEFAULT 60.0,
    max_ip_entries INTEGER NOT NULL DEFAULT 65536,
    max_prefix_entries INTEGER NOT NULL DEFAULT 16384,
    global_qps REAL NOT NULL DEFAULT 100000.0,
    global_burst INTEGER NOT NULL DEFAULT 100000,
    prefix_qps REAL NOT NULL DEFAULT 10000.0,
    prefix_burst INTEGER NOT NULL DEFAULT 20000,
    ip_qps REAL NOT NULL DEFAULT 5000.0,
    ip_burst INTEGER NOT NULL DEFAULT 10000,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS config_api (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    enabled BOOLEAN NOT NULL DEFAULT 1,
    host TEXT NOT NULL DEFAULT '0.0.0.0',
    port INTEGER NOT NULL DEFAULT 8080,
    api_key TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Custom DNS: Unified records table
CREATE TABLE IF NOT EXISTS custom_dns_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL,
    type TEXT NOT NULL CHECK(type IN ('A', 'AAAA', 'CNAME')),
    target TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(source, target, type)
);

CREATE INDEX IF NOT EXISTS idx_dns_records_source ON custom_dns_records(source);
CREATE INDEX IF NOT EXISTS idx_dns_records_type ON custom_dns_records(type);

-- Upstream servers (ordered list)
CREATE TABLE IF NOT EXISTS upstream_servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_address TEXT NOT NULL UNIQUE,
    priority INTEGER NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_upstream_priority ON upstream_servers(priority);

-- Filtering: Whitelist domains
CREATE TABLE IF NOT EXISTS filtering_whitelist (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain TEXT NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_whitelist_domain ON filtering_whitelist(domain);

-- Filtering: Blacklist domains
CREATE TABLE IF NOT EXISTS filtering_blacklist (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain TEXT NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_blacklist_domain ON filtering_blacklist(domain);

-- Filtering: Remote blocklists
CREATE TABLE IF NOT EXISTS filtering_blocklists (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    url TEXT NOT NULL,
    format TEXT NOT NULL DEFAULT 'auto' CHECK(format IN ('auto', 'adblock', 'hosts', 'domains')),
    enabled BOOLEAN NOT NULL DEFAULT 1,
    last_fetched TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_blocklists_enabled ON filtering_blocklists(enabled);

-- Triggers to increment config_version on any data change
CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_records
AFTER INSERT ON custom_dns_records
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_records_update
AFTER UPDATE ON custom_dns_records
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_records_delete
AFTER DELETE ON custom_dns_records
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_config
AFTER UPDATE ON config
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_upstream
AFTER INSERT ON upstream_servers
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_upstream_update
AFTER UPDATE ON upstream_servers
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_upstream_delete
AFTER DELETE ON upstream_servers
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_whitelist
AFTER INSERT ON filtering_whitelist
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_whitelist_delete
AFTER DELETE ON filtering_whitelist
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_blacklist
AFTER INSERT ON filtering_blacklist
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_blacklist_delete
AFTER DELETE ON filtering_blacklist
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_blocklists
AFTER INSERT ON filtering_blocklists
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_blocklists_update
AFTER UPDATE ON filtering_blocklists
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_blocklists_delete
AFTER DELETE ON filtering_blocklists
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;

-- Increment on typed config table updates
CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_server
AFTER UPDATE ON config_server
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_upstream_cfg
AFTER UPDATE ON config_upstream
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_logging
AFTER UPDATE ON config_logging
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_filtering
AFTER UPDATE ON config_filtering
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_rate_limit
AFTER UPDATE ON config_rate_limit
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_api
AFTER UPDATE ON config_api
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;

-- Initialize config_version if not exists
INSERT OR IGNORE INTO config_version (id, version) VALUES (1, 1);
