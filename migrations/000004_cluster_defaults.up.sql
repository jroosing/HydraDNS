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
