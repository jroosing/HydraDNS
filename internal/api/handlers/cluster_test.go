package handlers_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jroosing/hydradns/internal/api/handlers"
	"github.com/jroosing/hydradns/internal/api/models"
	"github.com/jroosing/hydradns/internal/config"
	"github.com/jroosing/hydradns/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func createClusterTestHandler(t *testing.T, clusterMode config.ClusterMode) *handlers.Handler {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 5353,
		},
		Upstream: config.UpstreamConfig{
			Servers: []string{"8.8.8.8"},
		},
		Cluster: config.ClusterConfig{
			Mode:         clusterMode,
			NodeID:       "test-node-1",
			SyncInterval: "30s",
			SyncTimeout:  "10s",
		},
	}
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	return handlers.New(cfg, db, testLogger())
}

func clusterPerformRequest(
	r http.Handler,
	method, path string,
	body string,
	headers map[string]string,
) *httptest.ResponseRecorder {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ============================================================================
// GetClusterStatus Tests
// ============================================================================

func TestGetClusterStatus_Standalone(t *testing.T) {
	h := createClusterTestHandler(t, config.ClusterModeStandalone)
	router := gin.New()
	router.GET("/cluster/status", h.GetClusterStatus)

	w := clusterPerformRequest(router, http.MethodGet, "/cluster/status", "", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.ClusterStatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "standalone", resp.Mode)
	assert.Equal(t, "test-node-1", resp.NodeID)
	assert.GreaterOrEqual(t, resp.ConfigVersion, int64(1))
}

func TestGetClusterStatus_Primary(t *testing.T) {
	h := createClusterTestHandler(t, config.ClusterModePrimary)
	router := gin.New()
	router.GET("/cluster/status", h.GetClusterStatus)

	w := clusterPerformRequest(router, http.MethodGet, "/cluster/status", "", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.ClusterStatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "primary", resp.Mode)
	assert.Equal(t, "test-node-1", resp.NodeID)
}

// ============================================================================
// GetClusterConfig Tests
// ============================================================================

func TestGetClusterConfig_ReturnsConfig(t *testing.T) {
	h := createClusterTestHandler(t, config.ClusterModePrimary)
	router := gin.New()
	router.GET("/cluster/config", h.GetClusterConfig)

	w := clusterPerformRequest(router, http.MethodGet, "/cluster/config", "", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.ClusterConfigRequest
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "primary", resp.Mode)
	assert.Equal(t, "test-node-1", resp.NodeID)
	assert.Equal(t, "30s", resp.SyncInterval)
	assert.Equal(t, "10s", resp.SyncTimeout)
}

func TestGetClusterConfig_RedactsSecret(t *testing.T) {
	cfg := &config.Config{
		Cluster: config.ClusterConfig{
			Mode:         config.ClusterModePrimary,
			NodeID:       "test-node",
			SharedSecret: "super-secret-key",
		},
	}
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	h := handlers.New(cfg, db, testLogger())
	router := gin.New()
	router.GET("/cluster/config", h.GetClusterConfig)

	w := clusterPerformRequest(router, http.MethodGet, "/cluster/config", "", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.ClusterConfigRequest
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// Secret should be redacted
	assert.Equal(t, "********", resp.SharedSecret)
}

// ============================================================================
// PutClusterConfig Tests
// ============================================================================

func TestPutClusterConfig_SetPrimary(t *testing.T) {
	h := createClusterTestHandler(t, config.ClusterModeStandalone)
	router := gin.New()
	router.PUT("/cluster/config", h.PutClusterConfig)

	body := `{"mode": "primary", "node_id": "my-primary", "shared_secret": "test-secret"}`
	w := clusterPerformRequest(router, http.MethodPut, "/cluster/config", body, nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.SetClusterConfigResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "configured", resp.Status)
	assert.Equal(t, "primary", resp.Mode)
	assert.Equal(t, "my-primary", resp.NodeID)
	assert.False(t, resp.RequiresRestart)
}

func TestPutClusterConfig_SetSecondary(t *testing.T) {
	h := createClusterTestHandler(t, config.ClusterModeStandalone)
	router := gin.New()
	router.PUT("/cluster/config", h.PutClusterConfig)

	body := `{
		"mode": "secondary",
		"primary_url": "http://primary:8080",
		"shared_secret": "test-secret",
		"sync_interval": "1m"
	}`
	w := clusterPerformRequest(router, http.MethodPut, "/cluster/config", body, nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.SetClusterConfigResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "configured", resp.Status)
	assert.Equal(t, "secondary", resp.Mode)
	assert.True(t, resp.RequiresRestart) // Secondary requires restart to start syncer
}

func TestPutClusterConfig_SecondaryRequiresPrimaryURL(t *testing.T) {
	h := createClusterTestHandler(t, config.ClusterModeStandalone)
	router := gin.New()
	router.PUT("/cluster/config", h.PutClusterConfig)

	body := `{"mode": "secondary"}`
	w := clusterPerformRequest(router, http.MethodPut, "/cluster/config", body, nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Contains(t, resp.Error, "primary_url is required")
}

func TestPutClusterConfig_InvalidMode(t *testing.T) {
	h := createClusterTestHandler(t, config.ClusterModeStandalone)
	router := gin.New()
	router.PUT("/cluster/config", h.PutClusterConfig)

	body := `{"mode": "invalid-mode"}`
	w := clusterPerformRequest(router, http.MethodPut, "/cluster/config", body, nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPutClusterConfig_GeneratesNodeID(t *testing.T) {
	h := createClusterTestHandler(t, config.ClusterModeStandalone)
	router := gin.New()
	router.PUT("/cluster/config", h.PutClusterConfig)

	body := `{"mode": "primary"}`
	w := clusterPerformRequest(router, http.MethodPut, "/cluster/config", body, nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.SetClusterConfigResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	// NodeID should be auto-generated (8 characters from UUID)
	assert.Len(t, resp.NodeID, 8)
}

func TestPutClusterConfig_DefaultsSyncInterval(t *testing.T) {
	h := createClusterTestHandler(t, config.ClusterModeStandalone)
	router := gin.New()
	router.PUT("/cluster/config", h.PutClusterConfig)
	router.GET("/cluster/config", h.GetClusterConfig)

	// Set config without sync_interval
	body := `{"mode": "primary", "node_id": "test"}`
	w := clusterPerformRequest(router, http.MethodPut, "/cluster/config", body, nil)
	assert.Equal(t, http.StatusOK, w.Code)

	// Get config to verify defaults
	w = clusterPerformRequest(router, http.MethodGet, "/cluster/config", "", nil)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.ClusterConfigRequest
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "30s", resp.SyncInterval)
	assert.Equal(t, "10s", resp.SyncTimeout)
}

// ============================================================================
// GetClusterExport Tests
// ============================================================================

func TestGetClusterExport_Primary(t *testing.T) {
	h := createClusterTestHandler(t, config.ClusterModePrimary)
	router := gin.New()
	router.GET("/cluster/export", h.GetClusterExport)

	w := clusterPerformRequest(router, http.MethodGet, "/cluster/export", "", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotZero(t, resp["version"])
	assert.NotEmpty(t, resp["timestamp"])
	assert.Equal(t, "test-node-1", resp["node_id"])
}

func TestGetClusterExport_Standalone(t *testing.T) {
	h := createClusterTestHandler(t, config.ClusterModeStandalone)
	router := gin.New()
	router.GET("/cluster/export", h.GetClusterExport)

	// Standalone can also export (for testing/migration purposes)
	w := clusterPerformRequest(router, http.MethodGet, "/cluster/export", "", nil)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetClusterExport_SecondaryForbidden(t *testing.T) {
	h := createClusterTestHandler(t, config.ClusterModeSecondary)
	router := gin.New()
	router.GET("/cluster/export", h.GetClusterExport)

	w := clusterPerformRequest(router, http.MethodGet, "/cluster/export", "", nil)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var resp models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Contains(t, resp.Error, "export not allowed from secondary")
}

func TestGetClusterExport_ValidatesSecret(t *testing.T) {
	cfg := &config.Config{
		Upstream: config.UpstreamConfig{
			Servers: []string{"8.8.8.8"},
		},
		Cluster: config.ClusterConfig{
			Mode:         config.ClusterModePrimary,
			NodeID:       "test-node",
			SharedSecret: "my-secret",
		},
	}
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	h := handlers.New(cfg, db, testLogger())
	router := gin.New()
	router.GET("/cluster/export", h.GetClusterExport)

	// Without secret - should fail
	w := clusterPerformRequest(router, http.MethodGet, "/cluster/export", "", nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// With wrong secret - should fail
	w = clusterPerformRequest(router, http.MethodGet, "/cluster/export", "", map[string]string{
		"X-Cluster-Secret": "wrong-secret",
	})
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// With correct secret - should succeed
	w = clusterPerformRequest(router, http.MethodGet, "/cluster/export", "", map[string]string{
		"X-Cluster-Secret": "my-secret",
	})
	assert.Equal(t, http.StatusOK, w.Code)
}

// ============================================================================
// PostClusterSync Tests
// ============================================================================

func TestPostClusterSync_NotSecondaryForbidden(t *testing.T) {
	h := createClusterTestHandler(t, config.ClusterModePrimary)
	router := gin.New()
	router.POST("/cluster/sync", h.PostClusterSync)

	w := clusterPerformRequest(router, http.MethodPost, "/cluster/sync", "", nil)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var resp models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Contains(t, resp.Error, "sync only available in secondary mode")
}

func TestPostClusterSync_StandaloneForbidden(t *testing.T) {
	h := createClusterTestHandler(t, config.ClusterModeStandalone)
	router := gin.New()
	router.POST("/cluster/sync", h.PostClusterSync)

	w := clusterPerformRequest(router, http.MethodPost, "/cluster/sync", "", nil)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestPostClusterSync_NoSyncerReturnsError(t *testing.T) {
	h := createClusterTestHandler(t, config.ClusterModeSecondary)
	router := gin.New()
	router.POST("/cluster/sync", h.PostClusterSync)

	// No syncer is set, so it should return error
	w := clusterPerformRequest(router, http.MethodPost, "/cluster/sync", "", nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Contains(t, resp.Error, "syncer not initialized")
}

// ============================================================================
// Configuration Persistence Tests
// ============================================================================

func TestClusterConfig_PersistsToDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	cfg := &config.Config{
		Cluster: config.ClusterConfig{
			Mode:   config.ClusterModeStandalone,
			NodeID: "initial",
		},
	}
	h := handlers.New(cfg, db, testLogger())

	router := gin.New()
	router.PUT("/cluster/config", h.PutClusterConfig)

	// Set new config
	body := `{
		"mode": "primary",
		"node_id": "persisted-node",
		"shared_secret": "secret123",
		"sync_interval": "5m"
	}`
	w := clusterPerformRequest(router, http.MethodPut, "/cluster/config", body, nil)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify it was persisted to database
	savedCfg, err := db.GetClusterConfig(context.Background())
	require.NoError(t, err)

	assert.Equal(t, config.ClusterModePrimary, savedCfg.Mode)
	assert.Equal(t, "persisted-node", savedCfg.NodeID)
	assert.Equal(t, "secret123", savedCfg.SharedSecret)
	assert.Equal(t, "5m", savedCfg.SyncInterval)
}
