// Package api_test provides behavior tests for the API package.
package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jroosing/hydradns/internal/api"
	"github.com/jroosing/hydradns/internal/api/models"
	"github.com/jroosing/hydradns/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 5353,
		},
		Upstream: config.UpstreamConfig{
			Servers: []string{"8.8.8.8"},
		},
		API: config.APIConfig{
			Enabled: true,
			Host:    "127.0.0.1",
			Port:    8080,
			APIKey:  "",
		},
	}
}

func performRequest(r http.Handler, method, path string, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ============================================================================
// Server Creation Tests
// ============================================================================

func TestNew_CreatesServer(t *testing.T) {
	cfg := createTestConfig()

	server := api.New(cfg, nil, nil)

	assert.NotNil(t, server)
}

func TestNew_PanicsOnNilConfig(t *testing.T) {
	assert.Panics(t, func() {
		api.New(nil, nil, nil)
	})
}

func TestServer_Addr(t *testing.T) {
	cfg := createTestConfig()
	cfg.API.Host = "0.0.0.0"
	cfg.API.Port = 9090

	server := api.New(cfg, nil, nil)

	assert.Equal(t, "0.0.0.0:9090", server.Addr())
}

func TestServer_Engine(t *testing.T) {
	cfg := createTestConfig()
	server := api.New(cfg, nil, nil)

	engine := server.Engine()

	assert.NotNil(t, engine)
}

// ============================================================================
// Routes Tests
// ============================================================================

func TestRoutes_HealthEndpoint(t *testing.T) {
	cfg := createTestConfig()
	server := api.New(cfg, nil, nil)

	w := performRequest(server.Engine(), http.MethodGet, "/api/v1/health", "")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.StatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Status)
}

func TestRoutes_StatsEndpoint(t *testing.T) {
	cfg := createTestConfig()
	server := api.New(cfg, nil, nil)

	w := performRequest(server.Engine(), http.MethodGet, "/api/v1/stats", "")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.ServerStatsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Uptime)
}

func TestRoutes_ConfigEndpoint(t *testing.T) {
	cfg := createTestConfig()
	server := api.New(cfg, nil, nil)

	w := performRequest(server.Engine(), http.MethodGet, "/api/v1/config", "")

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRoutes_FilteringEndpoints(t *testing.T) {
	cfg := createTestConfig()
	server := api.New(cfg, nil, nil)

	// Without PolicyEngine, filtering endpoints return 503
	w := performRequest(server.Engine(), http.MethodGet, "/api/v1/filtering/whitelist", "")
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	w = performRequest(server.Engine(), http.MethodGet, "/api/v1/filtering/blacklist", "")
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// ============================================================================
// API Key Protection Tests
// ============================================================================

func TestRoutes_WithAPIKey_ValidKey(t *testing.T) {
	cfg := createTestConfig()
	cfg.API.APIKey = "secret-key"
	server := api.New(cfg, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("X-Api-Key", "secret-key")
	w := httptest.NewRecorder()

	server.Engine().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRoutes_WithAPIKey_InvalidKey(t *testing.T) {
	cfg := createTestConfig()
	cfg.API.APIKey = "secret-key"
	server := api.New(cfg, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	req.Header.Set("X-Api-Key", "wrong-key")
	w := httptest.NewRecorder()

	server.Engine().ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRoutes_WithAPIKey_MissingKey(t *testing.T) {
	cfg := createTestConfig()
	cfg.API.APIKey = "secret-key"
	server := api.New(cfg, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	// No X-API-Key header
	w := httptest.NewRecorder()

	server.Engine().ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRoutes_NoAPIKey_NoAuth(t *testing.T) {
	cfg := createTestConfig()
	cfg.API.APIKey = "" // No API key configured
	server := api.New(cfg, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()

	server.Engine().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ============================================================================
// Server Lifecycle Tests
// ============================================================================

func TestServer_Shutdown(t *testing.T) {
	cfg := createTestConfig()
	cfg.API.Port = 0 // Let the OS pick a port
	server := api.New(cfg, nil, nil)

	// Shutdown should not error even if never started
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := server.Shutdown(ctx)
	assert.NoError(t, err)
}

// ============================================================================
// Swagger Endpoint Tests
// ============================================================================

func TestRoutes_SwaggerEndpoint(t *testing.T) {
	cfg := createTestConfig()
	server := api.New(cfg, nil, nil)

	w := performRequest(server.Engine(), http.MethodGet, "/swagger/index.html", "")

	// Swagger UI should be accessible
	assert.Equal(t, http.StatusOK, w.Code)
}

// ============================================================================
// Not Found Tests
// ============================================================================

func TestRoutes_NotFound(t *testing.T) {
	cfg := createTestConfig()
	server := api.New(cfg, nil, nil)

	w := performRequest(server.Engine(), http.MethodGet, "/api/v1/nonexistent", "")

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ============================================================================
// Method Tests
// ============================================================================

func TestRoutes_PutConfig_NotImplemented(t *testing.T) {
	cfg := createTestConfig()
	server := api.New(cfg, nil, nil)

	w := performRequest(server.Engine(), http.MethodPut, "/api/v1/config", `{}`)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
}
