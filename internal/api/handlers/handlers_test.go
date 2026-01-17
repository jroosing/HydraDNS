// Package handlers_test provides behavior tests for the API handlers package.
package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jroosing/hydradns/internal/api/handlers"
	"github.com/jroosing/hydradns/internal/api/models"
	"github.com/jroosing/hydradns/internal/config"
	"github.com/jroosing/hydradns/internal/database"
	"github.com/jroosing/hydradns/internal/filtering"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func createTestHandler(t *testing.T) *handlers.Handler {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 5353,
		},
		Upstream: config.UpstreamConfig{
			Servers: []string{"8.8.8.8"},
		},
	}
	// Create a temporary database file for tests
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	return handlers.New(cfg, db, nil)
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

func TestGetWhitelist_ReturnsList(t *testing.T) {
	h := createTestHandler(t)
	router := gin.New()
	router.GET("/filtering/whitelist", h.GetWhitelist)

	w := performRequest(router, "GET", "/filtering/whitelist", "")

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.DomainListResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
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

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.DomainListResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, resp.Count, 1)
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

	var resp models.DomainListResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, resp.Count, 2)
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

func TestRemoveWhitelist_Success(t *testing.T) {
	h := createTestHandler(t)
	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{Enabled: true})
	defer pe.Close()
	h.SetPolicyEngine(pe)

	router := gin.New()
	router.POST("/filtering/whitelist", h.AddWhitelist)
	router.DELETE("/filtering/whitelist", h.RemoveWhitelist)

	// Add then remove
	_ = performRequest(router, "POST", "/filtering/whitelist", `{"domains":["example.com"]}`)

	w := performRequest(router, "DELETE", "/filtering/whitelist", `{"domains":["example.com"]}`)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.DomainListResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
}

func TestGetBlacklist_ReturnsList(t *testing.T) {
	h := createTestHandler(t)
	router := gin.New()
	router.GET("/filtering/blacklist", h.GetBlacklist)

	w := performRequest(router, "GET", "/filtering/blacklist", "")

	assert.Equal(t, http.StatusOK, w.Code)
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

	var resp models.DomainListResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
}

func TestRemoveBlacklist_Success(t *testing.T) {
	h := createTestHandler(t)
	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{Enabled: true})
	defer pe.Close()
	h.SetPolicyEngine(pe)

	router := gin.New()
	router.POST("/filtering/blacklist", h.AddBlacklist)
	router.DELETE("/filtering/blacklist", h.RemoveBlacklist)

	_ = performRequest(router, "POST", "/filtering/blacklist", `{"domains":["to.remove.example.com"]}`)

	w := performRequest(router, "DELETE", "/filtering/blacklist", `{"domains":["to.remove.example.com"]}`)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.DomainListResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
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
	h := handlers.New(cfg, nil, nil)

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

// ============================================================================
// Blocklist Endpoint Tests
// ============================================================================

func TestBlocklists_ToggleEnabled_And_Refresh(t *testing.T) {
	h := createTestHandler(t)
	router := gin.New()
	router.GET("/filtering/blocklists", h.GetBlocklists)
	router.PUT("/filtering/blocklists/:name/enabled", h.SetBlocklistEnabled)
	router.POST("/filtering/blocklists/:name/refresh", h.RefreshBlocklist)

	// Initial list fetch
	w := performRequest(router, "GET", "/filtering/blocklists", "")
	assert.Equal(t, http.StatusOK, w.Code)

	var list models.BlocklistsResponse
	err := json.Unmarshal(w.Body.Bytes(), &list)
	require.NoError(t, err)
	require.GreaterOrEqual(t, list.Count, 1, "Default blocklist should be present from migrations")

	name := list.Blocklists[0].Name
	// URL-escape name for path usage
	escName := url.PathEscape(name)

	// Toggle enabled to false
	w = performRequest(router, "PUT", "/filtering/blocklists/"+escName+"/enabled", `{"enabled":false}`)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify list shows disabled
	w = performRequest(router, "GET", "/filtering/blocklists", "")
	var list2 models.BlocklistsResponse
	err = json.Unmarshal(w.Body.Bytes(), &list2)
	require.NoError(t, err)

	// Find the updated blocklist
	var found *models.Blocklist
	for i := range list2.Blocklists {
		if list2.Blocklists[i].Name == name {
			found = &list2.Blocklists[i]
			break
		}
	}
	require.NotNil(t, found)
	assert.False(t, found.Enabled)

	// Refresh should set last_fetched
	w = performRequest(router, "POST", "/filtering/blocklists/"+escName+"/refresh", "")
	assert.Equal(t, http.StatusOK, w.Code)

	w = performRequest(router, "GET", "/filtering/blocklists", "")
	var list3 models.BlocklistsResponse
	err = json.Unmarshal(w.Body.Bytes(), &list3)
	require.NoError(t, err)

	var found3 *models.Blocklist
	for i := range list3.Blocklists {
		if list3.Blocklists[i].Name == name {
			found3 = &list3.Blocklists[i]
			break
		}
	}
	require.NotNil(t, found3)
	assert.NotNil(t, found3.LastFetched)
}
