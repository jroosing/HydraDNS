// Package handlers_test provides behavior tests for the API handlers package.
package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jroosing/hydradns/internal/api/handlers"
	"github.com/jroosing/hydradns/internal/api/models"
	"github.com/jroosing/hydradns/internal/config"
	"github.com/jroosing/hydradns/internal/filtering"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func createTestHandler(_ *testing.T) *handlers.Handler {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 5353,
		},
		Upstream: config.UpstreamConfig{
			Servers: []string{"8.8.8.8"},
		},
	}
	return handlers.New(cfg, nil)
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
// Health Endpoint Tests
// ============================================================================

func TestHealth_ReturnsOK(t *testing.T) {
	h := createTestHandler(t)
	router := gin.New()
	router.GET("/health", h.Health)

	w := performRequest(router, "GET", "/health", "")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.StatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Status)
}

// ============================================================================
// Stats Endpoint Tests
// ============================================================================

func TestStats_ReturnsServerStats(t *testing.T) {
	h := createTestHandler(t)
	router := gin.New()
	router.GET("/stats", h.Stats)

	w := performRequest(router, "GET", "/stats", "")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.ServerStatsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotEmpty(t, resp.Uptime)
	assert.GreaterOrEqual(t, resp.GoRoutines, 1)
	assert.Positive(t, resp.NumCPU)
}

func TestStats_WithPolicyEngine(t *testing.T) {
	h := createTestHandler(t)
	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{Enabled: true})
	defer pe.Close()
	h.SetPolicyEngine(pe)

	router := gin.New()
	router.GET("/stats", h.Stats)

	w := performRequest(router, "GET", "/stats", "")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.ServerStatsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.NotNil(t, resp.FilteringStats)
	assert.True(t, resp.FilteringStats.Enabled)
}

// ============================================================================
// Filtering Endpoint Tests
// ============================================================================

func TestGetWhitelist_NoPolicyEngine(t *testing.T) {
	h := createTestHandler(t)
	router := gin.New()
	router.GET("/filtering/whitelist", h.GetWhitelist)

	w := performRequest(router, "GET", "/filtering/whitelist", "")

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestGetWhitelist_WithPolicyEngine(t *testing.T) {
	h := createTestHandler(t)
	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{Enabled: true})
	defer pe.Close()
	h.SetPolicyEngine(pe)

	router := gin.New()
	router.GET("/filtering/whitelist", h.GetWhitelist)

	w := performRequest(router, "GET", "/filtering/whitelist", "")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.DomainListResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
}

func TestAddWhitelist_NoPolicyEngine(t *testing.T) {
	h := createTestHandler(t)
	router := gin.New()
	router.POST("/filtering/whitelist", h.AddWhitelist)

	w := performRequest(router, "POST", "/filtering/whitelist", `{"domains":["example.com"]}`)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestAddWhitelist_Success(t *testing.T) {
	h := createTestHandler(t)
	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{Enabled: true})
	defer pe.Close()
	h.SetPolicyEngine(pe)

	router := gin.New()
	router.POST("/filtering/whitelist", h.AddWhitelist)

	w := performRequest(router, "POST", "/filtering/whitelist", `{"domains":["example.com","test.com"]}`)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.StatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Status)
}

func TestAddWhitelist_InvalidJSON(t *testing.T) {
	h := createTestHandler(t)
	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{Enabled: true})
	defer pe.Close()
	h.SetPolicyEngine(pe)

	router := gin.New()
	router.POST("/filtering/whitelist", h.AddWhitelist)

	w := performRequest(router, "POST", "/filtering/whitelist", `invalid json`)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRemoveWhitelist_NotImplemented(t *testing.T) {
	h := createTestHandler(t)
	router := gin.New()
	router.DELETE("/filtering/whitelist", h.RemoveWhitelist)

	w := performRequest(router, "DELETE", "/filtering/whitelist", `{"domains":["example.com"]}`)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

func TestGetBlacklist_NoPolicyEngine(t *testing.T) {
	h := createTestHandler(t)
	router := gin.New()
	router.GET("/filtering/blacklist", h.GetBlacklist)

	w := performRequest(router, "GET", "/filtering/blacklist", "")

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestGetBlacklist_WithPolicyEngine(t *testing.T) {
	h := createTestHandler(t)
	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{Enabled: true})
	defer pe.Close()
	h.SetPolicyEngine(pe)

	router := gin.New()
	router.GET("/filtering/blacklist", h.GetBlacklist)

	w := performRequest(router, "GET", "/filtering/blacklist", "")

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAddBlacklist_Success(t *testing.T) {
	h := createTestHandler(t)
	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{Enabled: true})
	defer pe.Close()
	h.SetPolicyEngine(pe)

	router := gin.New()
	router.POST("/filtering/blacklist", h.AddBlacklist)

	w := performRequest(router, "POST", "/filtering/blacklist", `{"domains":["ads.example.com"]}`)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRemoveBlacklist_NotImplemented(t *testing.T) {
	h := createTestHandler(t)
	router := gin.New()
	router.DELETE("/filtering/blacklist", h.RemoveBlacklist)

	w := performRequest(router, "DELETE", "/filtering/blacklist", `{"domains":["example.com"]}`)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

func TestFilteringStats_NoPolicyEngine(t *testing.T) {
	h := createTestHandler(t)
	router := gin.New()
	router.GET("/filtering/stats", h.FilteringStats)

	w := performRequest(router, "GET", "/filtering/stats", "")

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestSetFilteringEnabled_NoPolicyEngine(t *testing.T) {
	h := createTestHandler(t)
	router := gin.New()
	router.PUT("/filtering/enabled", h.SetFilteringEnabled)

	w := performRequest(router, "PUT", "/filtering/enabled", `{"enabled":true}`)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestSetFilteringEnabled_Success(t *testing.T) {
	h := createTestHandler(t)
	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{Enabled: true})
	defer pe.Close()
	h.SetPolicyEngine(pe)

	router := gin.New()
	router.PUT("/filtering/enabled", h.SetFilteringEnabled)

	w := performRequest(router, "PUT", "/filtering/enabled", `{"enabled":false}`)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.StatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Status)
}

// ============================================================================
// Config Endpoint Tests
// ============================================================================

func TestGetConfig_Success(t *testing.T) {
	h := createTestHandler(t)
	router := gin.New()
	router.GET("/config", h.GetConfig)

	w := performRequest(router, "GET", "/config", "")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.ConfigResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "localhost", resp.Server.Host)
	assert.Equal(t, 5353, resp.Server.Port)
}

func TestPutConfig_NotImplemented(t *testing.T) {
	h := createTestHandler(t)
	router := gin.New()
	router.PUT("/config", h.PutConfig)

	w := performRequest(router, "PUT", "/config", `{}`)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

func TestReloadConfig_NotImplemented(t *testing.T) {
	h := createTestHandler(t)
	router := gin.New()
	router.POST("/config/reload", h.ReloadConfig)

	w := performRequest(router, "POST", "/config/reload", "")

	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

// ============================================================================
// Handler Initialization Tests
// ============================================================================

func TestHandler_New(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)

	assert.NotNil(t, h)
}

func TestHandler_SetPolicyEngine(t *testing.T) {
	h := createTestHandler(t)
	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{Enabled: true})
	defer pe.Close()

	h.SetPolicyEngine(pe)

	// Verify it's set by checking stats endpoint
	router := gin.New()
	router.GET("/stats", h.Stats)

	w := performRequest(router, "GET", "/stats", "")

	var resp models.ServerStatsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotNil(t, resp.FilteringStats)
}

