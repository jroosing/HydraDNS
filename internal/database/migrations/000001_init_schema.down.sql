-- Rollback initial schema
DROP TRIGGER IF EXISTS trg_config_version_increment_blocklists_delete;
DROP TRIGGER IF EXISTS trg_config_version_increment_blocklists_update;
DROP TRIGGER IF EXISTS trg_config_version_increment_blocklists;
DROP TRIGGER IF EXISTS trg_config_version_increment_blacklist_delete;
DROP TRIGGER IF EXISTS trg_config_version_increment_blacklist;
DROP TRIGGER IF EXISTS trg_config_version_increment_whitelist_delete;
DROP TRIGGER IF EXISTS trg_config_version_increment_whitelist;
DROP TRIGGER IF EXISTS trg_config_version_increment_upstream_delete;
DROP TRIGGER IF EXISTS trg_config_version_increment_upstream_update;
DROP TRIGGER IF EXISTS trg_config_version_increment_upstream;
DROP TRIGGER IF EXISTS trg_config_version_increment_config;
DROP TRIGGER IF EXISTS trg_config_version_increment_cnames_delete;
DROP TRIGGER IF EXISTS trg_config_version_increment_cnames_update;
DROP TRIGGER IF EXISTS trg_config_version_increment_cnames;
DROP TRIGGER IF EXISTS trg_config_version_increment_hosts_delete;
DROP TRIGGER IF EXISTS trg_config_version_increment_hosts_update;
DROP TRIGGER IF EXISTS trg_config_version_increment_hosts;

DROP INDEX IF EXISTS idx_blocklists_enabled;
DROP INDEX IF EXISTS idx_blacklist_domain;
DROP INDEX IF EXISTS idx_whitelist_domain;
DROP INDEX IF EXISTS idx_upstream_priority;
DROP INDEX IF EXISTS idx_cnames_alias;
DROP INDEX IF EXISTS idx_hosts_record_type;
DROP INDEX IF EXISTS idx_hosts_hostname;

DROP TABLE IF EXISTS filtering_blocklists;
DROP TABLE IF EXISTS filtering_blacklist;
DROP TABLE IF EXISTS filtering_whitelist;
DROP TABLE IF EXISTS upstream_servers;
DROP TABLE IF EXISTS custom_dns_cnames;
DROP TABLE IF EXISTS custom_dns_hosts;
DROP TABLE IF EXISTS config_version;
DROP TABLE IF EXISTS config;
DROP TABLE IF EXISTS schema_version;
