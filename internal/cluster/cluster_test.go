package cluster

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jroosing/hydradns/internal/config"
)

func TestNewSyncer_RequiresSecondaryMode(t *testing.T) {
	cfg := &config.ClusterConfig{
		Mode:       config.ClusterModePrimary,
		PrimaryURL: "http://primary:8080",
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	_, err := NewSyncer(cfg, logger, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for non-secondary mode")
	}
}

func TestNewSyncer_RequiresPrimaryURL(t *testing.T) {
	cfg := &config.ClusterConfig{
		Mode:       config.ClusterModeSecondary,
		PrimaryURL: "",
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	_, err := NewSyncer(cfg, logger, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for missing primary URL")
	}
}

func TestSyncer_FetchesConfigFromPrimary(t *testing.T) {
	// Set up mock primary server
	exported := ExportData{
		Version:   42,
		Timestamp: time.Now().UTC(),
		NodeID:    "primary-1",
		Upstream: config.UpstreamConfig{
			Servers:    []string{"8.8.8.8", "1.1.1.1"},
			UDPTimeout: "3s",
		},
		CustomDNS: config.CustomDNSConfig{
			Hosts:  map[string][]string{"test.local": {"192.168.1.1"}},
			CNAMEs: map[string]string{"www.test.local": "test.local"},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/cluster/export" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(exported)
	}))
	defer server.Close()

	// Track import calls
	var importCalled atomic.Bool
	var importedData *ExportData

	cfg := &config.ClusterConfig{
		Mode:         config.ClusterModeSecondary,
		PrimaryURL:   server.URL,
		SyncInterval: "1h", // Long interval to prevent auto-sync
		SyncTimeout:  "5s",
		NodeID:       "secondary-1",
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

	importFunc := func(data *ExportData) error {
		importCalled.Store(true)
		importedData = data
		return nil
	}

	versionFunc := func() (int64, error) {
		return 1, nil // Local version is lower than remote
	}

	syncer, err := NewSyncer(cfg, logger, importFunc, nil, versionFunc)
	if err != nil {
		t.Fatalf("NewSyncer failed: %v", err)
	}

	// Trigger a sync
	ctx := context.Background()
	if err := syncer.ForceSync(ctx); err != nil {
		t.Fatalf("ForceSync failed: %v", err)
	}

	if !importCalled.Load() {
		t.Fatal("import function was not called")
	}

	if importedData.Version != 42 {
		t.Errorf("expected version 42, got %d", importedData.Version)
	}

	if len(importedData.Upstream.Servers) != 2 {
		t.Errorf("expected 2 upstream servers, got %d", len(importedData.Upstream.Servers))
	}
}

func TestSyncer_SkipsWhenVersionCurrent(t *testing.T) {
	exported := ExportData{
		Version:   10,
		Timestamp: time.Now().UTC(),
		NodeID:    "primary-1",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(exported)
	}))
	defer server.Close()

	var importCalled atomic.Bool

	cfg := &config.ClusterConfig{
		Mode:         config.ClusterModeSecondary,
		PrimaryURL:   server.URL,
		SyncInterval: "1h",
		SyncTimeout:  "5s",
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	importFunc := func(data *ExportData) error {
		importCalled.Store(true)
		return nil
	}

	versionFunc := func() (int64, error) {
		return 15, nil // Local version is higher than remote
	}

	syncer, err := NewSyncer(cfg, logger, importFunc, nil, versionFunc)
	if err != nil {
		t.Fatalf("NewSyncer failed: %v", err)
	}

	ctx := context.Background()
	if err := syncer.ForceSync(ctx); err != nil {
		t.Fatalf("ForceSync failed: %v", err)
	}

	if importCalled.Load() {
		t.Fatal("import function should not be called when local version is current")
	}
}

func TestSyncer_ValidatesSharedSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret := r.Header.Get("X-Cluster-Secret")
		if secret != "test-secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		exported := ExportData{Version: 1}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(exported)
	}))
	defer server.Close()

	cfg := &config.ClusterConfig{
		Mode:         config.ClusterModeSecondary,
		PrimaryURL:   server.URL,
		SharedSecret: "wrong-secret",
		SyncInterval: "1h",
		SyncTimeout:  "5s",
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	syncer, err := NewSyncer(
		cfg,
		logger,
		func(*ExportData) error { return nil },
		nil,
		func() (int64, error) { return 0, nil },
	)
	if err != nil {
		t.Fatalf("NewSyncer failed: %v", err)
	}

	ctx := context.Background()
	err = syncer.ForceSync(ctx)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestSyncer_Status(t *testing.T) {
	cfg := &config.ClusterConfig{
		Mode:         config.ClusterModeSecondary,
		PrimaryURL:   "http://primary:8080",
		SyncInterval: "30s",
		SyncTimeout:  "5s",
		NodeID:       "test-node",
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	syncer, err := NewSyncer(
		cfg,
		logger,
		func(*ExportData) error { return nil },
		nil,
		func() (int64, error) { return 5, nil },
	)
	if err != nil {
		t.Fatalf("NewSyncer failed: %v", err)
	}

	status := syncer.Status()

	if status.Mode != config.ClusterModeSecondary {
		t.Errorf("expected secondary mode, got %s", status.Mode)
	}

	if status.NodeID != "test-node" {
		t.Errorf("expected node_id test-node, got %s", status.NodeID)
	}

	if status.PrimaryURL != "http://primary:8080" {
		t.Errorf("expected primary_url http://primary:8080, got %s", status.PrimaryURL)
	}

	if status.ConfigVersion != 5 {
		t.Errorf("expected config_version 5, got %d", status.ConfigVersion)
	}
}
