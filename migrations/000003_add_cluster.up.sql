-- Add cluster configuration table
CREATE TABLE IF NOT EXISTS config_cluster (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    mode TEXT NOT NULL DEFAULT 'standalone' CHECK(mode IN ('standalone', 'primary', 'secondary')),
    node_id TEXT NOT NULL DEFAULT '',
    primary_url TEXT NOT NULL DEFAULT '',
    shared_secret TEXT NOT NULL DEFAULT '',
    sync_interval TEXT NOT NULL DEFAULT '30s',
    sync_timeout TEXT NOT NULL DEFAULT '10s',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Trigger to increment config version on cluster config changes
CREATE TRIGGER IF NOT EXISTS trg_config_version_increment_cluster
AFTER UPDATE ON config_cluster
BEGIN
    UPDATE config_version SET version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1;
END;
