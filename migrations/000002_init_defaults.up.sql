-- Initialize default configuration values and data

-- Server defaults (typed)
INSERT INTO config_server (id, host, port, workers, max_concurrency, upstream_socket_pool_size, enable_tcp, tcp_fallback, created_at, updated_at)
VALUES (1, '0.0.0.0', 53, 'auto', 0, 0, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT(id) DO UPDATE SET
	host=excluded.host,
	port=excluded.port,
	workers=excluded.workers,
	max_concurrency=excluded.max_concurrency,
	upstream_socket_pool_size=excluded.upstream_socket_pool_size,
	enable_tcp=excluded.enable_tcp,
	tcp_fallback=excluded.tcp_fallback,
	updated_at=CURRENT_TIMESTAMP;

-- Upstream defaults (typed)
INSERT INTO config_upstream (id, udp_timeout, tcp_timeout, max_retries, created_at, updated_at)
VALUES (1, '3s', '5s', 3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT(id) DO UPDATE SET
	udp_timeout=excluded.udp_timeout,
	tcp_timeout=excluded.tcp_timeout,
	max_retries=excluded.max_retries,
	updated_at=CURRENT_TIMESTAMP;

-- Logging defaults (typed)
INSERT INTO config_logging (id, level, structured, structured_format, include_pid, created_at, updated_at)
VALUES (1, 'INFO', 0, 'json', 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT(id) DO UPDATE SET
	level=excluded.level,
	structured=excluded.structured,
	structured_format=excluded.structured_format,
	include_pid=excluded.include_pid,
	updated_at=CURRENT_TIMESTAMP;

-- Filtering defaults (typed)
INSERT INTO config_filtering (id, enabled, log_blocked, log_allowed, refresh_interval, created_at, updated_at)
VALUES (1, 0, 1, 0, '24h', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT(id) DO UPDATE SET
	enabled=excluded.enabled,
	log_blocked=excluded.log_blocked,
	log_allowed=excluded.log_allowed,
	refresh_interval=excluded.refresh_interval,
	updated_at=CURRENT_TIMESTAMP;

-- Rate limit defaults (typed)
INSERT INTO config_rate_limit (id, cleanup_seconds, max_ip_entries, max_prefix_entries, global_qps, global_burst, prefix_qps, prefix_burst, ip_qps, ip_burst, created_at, updated_at)
VALUES (1, 60.0, 65536, 16384, 100000.0, 100000, 10000.0, 20000, 5000.0, 10000, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT(id) DO UPDATE SET
	cleanup_seconds=excluded.cleanup_seconds,
	max_ip_entries=excluded.max_ip_entries,
	max_prefix_entries=excluded.max_prefix_entries,
	global_qps=excluded.global_qps,
	global_burst=excluded.global_burst,
	prefix_qps=excluded.prefix_qps,
	prefix_burst=excluded.prefix_burst,
	ip_qps=excluded.ip_qps,
	ip_burst=excluded.ip_burst,
	updated_at=CURRENT_TIMESTAMP;

-- API defaults (typed)
INSERT INTO config_api (id, enabled, host, port, api_key, created_at, updated_at)
VALUES (1, 1, '0.0.0.0', 8080, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT(id) DO UPDATE SET
	enabled=excluded.enabled,
	host=excluded.host,
	port=excluded.port,
	api_key=excluded.api_key,
	updated_at=CURRENT_TIMESTAMP;

-- Upstream servers
INSERT OR IGNORE INTO upstream_servers (server_address, priority, enabled, created_at, updated_at) VALUES
('9.9.9.9', 0, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
('1.1.1.1', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
('8.8.8.8', 2, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

-- StevenBlack Hosts blocklist
INSERT OR IGNORE INTO filtering_blocklists (name, url, format, enabled, created_at, updated_at) VALUES
('StevenBlack Hosts', 'https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts', 'hosts', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
