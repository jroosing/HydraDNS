-- Initialize default cluster configuration
INSERT INTO config_cluster (id, mode, node_id, primary_url, shared_secret, sync_interval, sync_timeout, created_at, updated_at)
VALUES (1, 'standalone', '', '', '', '30s', '10s', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT(id) DO UPDATE SET
	mode=excluded.mode,
	node_id=excluded.node_id,
	primary_url=excluded.primary_url,
	shared_secret=excluded.shared_secret,
	sync_interval=excluded.sync_interval,
	sync_timeout=excluded.sync_timeout,
	updated_at=CURRENT_TIMESTAMP;

-- Seed default cluster config into the generic config table for backward compat
INSERT OR IGNORE INTO config (key, value) VALUES ('cluster.mode', 'standalone');
INSERT OR IGNORE INTO config (key, value) VALUES ('cluster.node_id', '');
INSERT OR IGNORE INTO config (key, value) VALUES ('cluster.primary_url', '');
INSERT OR IGNORE INTO config (key, value) VALUES ('cluster.shared_secret', '');
INSERT OR IGNORE INTO config (key, value) VALUES ('cluster.sync_interval', '30s');
INSERT OR IGNORE INTO config (key, value) VALUES ('cluster.sync_timeout', '10s');
