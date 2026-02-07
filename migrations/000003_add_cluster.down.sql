-- Remove cluster configuration table
DROP TRIGGER IF EXISTS trg_config_version_increment_cluster;
DROP TABLE IF EXISTS config_cluster;
