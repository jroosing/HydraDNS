-- Rollback default configuration data

-- Remove blocklists
DELETE FROM filtering_blocklists;

-- Remove upstream servers
DELETE FROM upstream_servers;

-- Remove typed config defaults
DELETE FROM config_api;
DELETE FROM config_rate_limit;
DELETE FROM config_filtering;
DELETE FROM config_logging;
DELETE FROM config_upstream;
DELETE FROM config_server;
