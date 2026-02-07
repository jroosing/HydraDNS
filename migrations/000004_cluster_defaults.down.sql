-- Remove cluster config defaults (reset to standalone)
UPDATE config_cluster SET 
    mode = 'standalone',
    node_id = '',
    primary_url = '',
    shared_secret = '',
    sync_interval = '30s',
    sync_timeout = '10s',
    updated_at = CURRENT_TIMESTAMP
WHERE id = 1;
