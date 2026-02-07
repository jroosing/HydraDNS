package models

import "time"

// ClusterStatusResponse represents the cluster status response.
type ClusterStatusResponse struct {
	// Mode is the cluster mode: "standalone", "primary", or "secondary".
	Mode string `json:"mode"`

	// NodeID is this node's unique identifier.
	NodeID string `json:"node_id"`

	// ConfigVersion is the current local configuration version.
	ConfigVersion int64 `json:"config_version"`

	// PrimaryURL is the URL of the primary node (only for secondary mode).
	PrimaryURL string `json:"primary_url,omitempty"`

	// LastSyncTime is when the last successful sync occurred (only for secondary).
	LastSyncTime *time.Time `json:"last_sync_time,omitempty"`

	// LastSyncVersion is the config version from the last successful sync.
	LastSyncVersion int64 `json:"last_sync_version,omitempty"`

	// LastSyncError is the error message from the last sync attempt (if any).
	LastSyncError string `json:"last_sync_error,omitempty"`

	// NextSyncTime is when the next sync is scheduled (only for secondary).
	NextSyncTime *time.Time `json:"next_sync_time,omitempty"`

	// SyncCount is the total number of successful syncs.
	SyncCount int64 `json:"sync_count,omitempty"`

	// ErrorCount is the total number of sync errors.
	ErrorCount int64 `json:"error_count,omitempty"`
}

// ClusterConfigRequest represents a request to configure cluster settings.
type ClusterConfigRequest struct {
	// Mode is the cluster mode: "standalone", "primary", or "secondary".
	Mode string `json:"mode" binding:"required,oneof=standalone primary secondary"`

	// NodeID is a unique identifier for this node (auto-generated if empty).
	NodeID string `json:"node_id,omitempty"`

	// PrimaryURL is the URL of the primary node's API (required for secondary mode).
	// Example: "http://primary.homelab.local:8080"
	PrimaryURL string `json:"primary_url,omitempty"`

	// SharedSecret is used to authenticate sync requests between nodes.
	SharedSecret string `json:"shared_secret,omitempty"`

	// SyncInterval is how often secondary nodes poll for config changes.
	// Default: "30s". Example: "1m", "5m".
	SyncInterval string `json:"sync_interval,omitempty"`

	// SyncTimeout is the HTTP timeout for sync requests.
	// Default: "10s".
	SyncTimeout string `json:"sync_timeout,omitempty"`
}

// SetClusterConfigResponse represents the response after configuring cluster settings.
type SetClusterConfigResponse struct {
	// Status indicates the result of the operation.
	Status string `json:"status"`

	// Mode is the new cluster mode.
	Mode string `json:"mode"`

	// NodeID is this node's identifier.
	NodeID string `json:"node_id"`

	// Message provides additional information about the configuration change.
	Message string `json:"message,omitempty"`

	// RequiresRestart indicates if a restart is needed for changes to take effect.
	RequiresRestart bool `json:"requires_restart"`
}
