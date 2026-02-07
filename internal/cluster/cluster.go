// Package cluster provides primary/secondary configuration synchronization for HydraDNS.
//
// This implements a soft clustering mode where:
//   - Primary nodes serve as the source of truth for configuration
//   - Secondary nodes periodically poll the primary for config changes
//   - All nodes operate independently for DNS resolution
//
// The synchronization is one-way: secondary nodes pull configuration from the primary.
// This is designed for homelab environments where simplicity is valued over
// full HA clustering.
package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/jroosing/hydradns/internal/config"
)

// ExportData represents the configuration data exchanged during sync.
// This is the payload sent from primary to secondary nodes.
type ExportData struct {
	// Version is the configuration version from the primary node.
	Version int64 `json:"version"`

	// Timestamp is when this export was generated.
	Timestamp time.Time `json:"timestamp"`

	// NodeID is the primary node's identifier.
	NodeID string `json:"node_id"`

	// Upstream contains upstream DNS server configuration.
	Upstream config.UpstreamConfig `json:"upstream"`

	// CustomDNS contains custom DNS records (hosts and CNAMEs).
	CustomDNS config.CustomDNSConfig `json:"custom_dns"`

	// Filtering contains domain filtering configuration.
	Filtering config.FilteringConfig `json:"filtering"`
}

// SyncStatus represents the current synchronization status.
type SyncStatus struct {
	// Mode is the cluster mode (standalone, primary, secondary).
	Mode config.ClusterMode `json:"mode"`

	// NodeID is this node's identifier.
	NodeID string `json:"node_id"`

	// PrimaryURL is the URL of the primary node (only for secondary).
	PrimaryURL string `json:"primary_url,omitempty"`

	// LastSyncTime is when the last successful sync occurred.
	LastSyncTime *time.Time `json:"last_sync_time,omitempty"`

	// LastSyncVersion is the config version from the last successful sync.
	LastSyncVersion int64 `json:"last_sync_version,omitempty"`

	// LastSyncError is the error message from the last sync attempt (if any).
	LastSyncError string `json:"last_sync_error,omitempty"`

	// NextSyncTime is when the next sync is scheduled.
	NextSyncTime *time.Time `json:"next_sync_time,omitempty"`

	// SyncCount is the total number of successful syncs.
	SyncCount int64 `json:"sync_count"`

	// ErrorCount is the total number of sync errors.
	ErrorCount int64 `json:"error_count"`

	// ConfigVersion is the current local config version.
	ConfigVersion int64 `json:"config_version"`
}

// ImportFunc is a callback function that imports configuration from ExportData.
// It should update the local database and return any error.
type ImportFunc func(data *ExportData) error

// ReloadFunc is a callback function that triggers a configuration reload.
// It should reload any runtime components that depend on configuration.
type ReloadFunc func() error

// VersionFunc is a callback function that returns the current config version.
type VersionFunc func() (int64, error)

// Syncer handles configuration synchronization for secondary nodes.
type Syncer struct {
	cfg         *config.ClusterConfig
	logger      *slog.Logger
	importFunc  ImportFunc
	reloadFunc  ReloadFunc
	versionFunc VersionFunc
	httpClient  *http.Client

	mu              sync.RWMutex
	running         bool
	lastSyncTime    *time.Time
	lastSyncVersion int64
	lastSyncError   string
	nextSyncTime    *time.Time
	syncCount       int64
	errorCount      int64

	stopCh chan struct{}
	doneCh chan struct{}
}

// NewSyncer creates a new configuration syncer for secondary nodes.
func NewSyncer(
	cfg *config.ClusterConfig,
	logger *slog.Logger,
	importFunc ImportFunc,
	reloadFunc ReloadFunc,
	versionFunc VersionFunc,
) (*Syncer, error) {
	if cfg.Mode != config.ClusterModeSecondary {
		return nil, fmt.Errorf("syncer can only be created for secondary mode, got: %s", cfg.Mode)
	}

	if cfg.PrimaryURL == "" {
		return nil, fmt.Errorf("primary_url is required for secondary mode")
	}

	syncTimeout, err := time.ParseDuration(cfg.SyncTimeout)
	if err != nil {
		syncTimeout = 10 * time.Second
	}

	return &Syncer{
		cfg:         cfg,
		logger:      logger,
		importFunc:  importFunc,
		reloadFunc:  reloadFunc,
		versionFunc: versionFunc,
		httpClient: &http.Client{
			Timeout: syncTimeout,
		},
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}, nil
}

// Start begins the periodic synchronization process.
func (s *Syncer) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("syncer already running")
	}
	s.running = true
	s.mu.Unlock()

	syncInterval, err := time.ParseDuration(s.cfg.SyncInterval)
	if err != nil {
		syncInterval = 30 * time.Second
	}

	s.logger.Info("cluster syncer starting",
		"primary_url", s.cfg.PrimaryURL,
		"sync_interval", syncInterval,
		"node_id", s.cfg.NodeID,
	)

	// Do an initial sync immediately
	if err := s.doSync(ctx); err != nil {
		s.logger.Warn("initial sync failed, will retry", "err", err)
	}

	go s.runLoop(ctx, syncInterval)

	return nil
}

// Stop stops the synchronization process.
func (s *Syncer) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopCh)
	<-s.doneCh
	s.logger.Info("cluster syncer stopped")
}

// Status returns the current synchronization status.
func (s *Syncer) Status() SyncStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	localVersion, _ := s.versionFunc()

	return SyncStatus{
		Mode:            s.cfg.Mode,
		NodeID:          s.cfg.NodeID,
		PrimaryURL:      s.cfg.PrimaryURL,
		LastSyncTime:    s.lastSyncTime,
		LastSyncVersion: s.lastSyncVersion,
		LastSyncError:   s.lastSyncError,
		NextSyncTime:    s.nextSyncTime,
		SyncCount:       s.syncCount,
		ErrorCount:      s.errorCount,
		ConfigVersion:   localVersion,
	}
}

// ForceSync triggers an immediate synchronization.
func (s *Syncer) ForceSync(ctx context.Context) error {
	return s.doSync(ctx)
}

func (s *Syncer) runLoop(ctx context.Context, interval time.Duration) {
	defer close(s.doneCh)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		// Update next sync time
		nextSync := time.Now().Add(interval)
		s.mu.Lock()
		s.nextSyncTime = &nextSync
		s.mu.Unlock()

		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			if err := s.doSync(ctx); err != nil {
				s.logger.Warn("sync failed", "err", err)
			}
		}
	}
}

func (s *Syncer) doSync(ctx context.Context) error {
	s.logger.Debug("starting config sync", "primary", s.cfg.PrimaryURL)

	// Fetch config from primary
	data, err := s.fetchConfig(ctx)
	if err != nil {
		s.recordError(err)
		return fmt.Errorf("fetch config: %w", err)
	}

	// Check if we already have this version
	currentVersion, _ := s.versionFunc()
	if data.Version <= currentVersion {
		s.logger.Debug("config already up to date",
			"local_version", currentVersion,
			"remote_version", data.Version,
		)
		s.recordSuccess(data.Version)
		return nil
	}

	s.logger.Info("applying config from primary",
		"remote_version", data.Version,
		"local_version", currentVersion,
		"primary_node", data.NodeID,
	)

	// Import the configuration
	if err := s.importFunc(data); err != nil {
		s.recordError(err)
		return fmt.Errorf("import config: %w", err)
	}

	// Trigger reload of runtime components
	if s.reloadFunc != nil {
		if err := s.reloadFunc(); err != nil {
			s.logger.Warn("reload after sync failed", "err", err)
			// Don't fail the sync for reload errors
		}
	}

	s.recordSuccess(data.Version)
	s.logger.Info("config sync completed", "version", data.Version)

	return nil
}

func (s *Syncer) fetchConfig(ctx context.Context) (*ExportData, error) {
	url := s.cfg.PrimaryURL + "/api/v1/cluster/export"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Add shared secret for authentication
	if s.cfg.SharedSecret != "" {
		req.Header.Set("X-Cluster-Secret", s.cfg.SharedSecret)
	}

	req.Header.Set("Accept", "application/json")
	if s.cfg.NodeID != "" {
		req.Header.Set("X-Node-ID", s.cfg.NodeID)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var data ExportData
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &data, nil
}

func (s *Syncer) recordSuccess(version int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	s.lastSyncTime = &now
	s.lastSyncVersion = version
	s.lastSyncError = ""
	s.syncCount++
}

func (s *Syncer) recordError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastSyncError = err.Error()
	s.errorCount++
}
