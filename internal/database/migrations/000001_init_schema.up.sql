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

-- Custom DNS: Hosts (A/AAAA records)
CREATE TABLE IF NOT EXISTS custom_dns_hosts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    hostname TEXT NOT NULL,
    ip_address TEXT NOT NULL,
    record_type TEXT NOT NULL CHECK(record_type IN ('A', 'AAAA')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(hostname, ip_address)
);

CREATE INDEX IF NOT EXISTS idx_hosts_hostname ON custom_dns_hosts(hostname);
CREATE INDEX IF NOT EXISTS idx_hosts_record_type ON custom_dns_hosts(record_type);

-- Custom DNS: CNAMEs
CREATE TABLE IF NOT EXISTS custom_dns_cnames (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    alias TEXT NOT NULL UNIQUE,
    target TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_cnames_alias ON custom_dns_cnames(alias);

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
CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_hosts
AFTER INSERT ON custom_dns_hosts
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_hosts_update
AFTER UPDATE ON custom_dns_hosts
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_hosts_delete
AFTER DELETE ON custom_dns_hosts
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_cnames
AFTER INSERT ON custom_dns_cnames
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_cnames_update
AFTER UPDATE ON custom_dns_cnames
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_cnames_delete
AFTER DELETE ON custom_dns_cnames
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

-- Initialize config_version if not exists
INSERT OR IGNORE INTO config_version (id, version) VALUES (1, 1);
