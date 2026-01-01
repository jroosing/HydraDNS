package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jroosing/hydradns/internal/api/handlers"
	"github.com/jroosing/hydradns/internal/api/models"
	"github.com/jroosing/hydradns/internal/config"
	"github.com/jroosing/hydradns/internal/filtering"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilteringStats_NoEngine(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/filtering/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestFilteringStats_WithEngine(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)

	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{
		Enabled:          true,
		WhitelistDomains: []string{"safe.com"},
		BlacklistDomains: []string{"blocked.com"},
	})
	defer pe.Close()

	h.SetPolicyEngine(pe)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/filtering/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.FilteringStatsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Enabled)
	assert.Equal(t, 1, resp.WhitelistSize)
	assert.Equal(t, 1, resp.BlacklistSize)
}

func TestAddBlacklist(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)

	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{
		Enabled: true,
	})
	defer pe.Close()

	h.SetPolicyEngine(pe)
	r := setupTestRouter(h)

	body := models.DomainRequest{Domains: []string{"ads.example.com", "tracker.example.com"}}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/filtering/blacklist", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	stats := pe.Stats()
	assert.Equal(t, 2, stats.BlacklistSize)
}

func TestAddBlacklist_InvalidRequest(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)

	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{
		Enabled: true,
	})
	defer pe.Close()

	h.SetPolicyEngine(pe)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/filtering/blacklist", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAddBlacklist_NoEngine(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)
	r := setupTestRouter(h)

	body := models.DomainRequest{Domains: []string{"blocked.com"}}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/filtering/blacklist", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestAddWhitelist(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)

	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{
		Enabled: true,
	})
	defer pe.Close()

	h.SetPolicyEngine(pe)
	r := setupTestRouter(h)

	body := models.DomainRequest{Domains: []string{"safe.example.com"}}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/filtering/whitelist", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	stats := pe.Stats()
	assert.Equal(t, 1, stats.WhitelistSize)
}

func TestAddWhitelist_NoEngine(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)
	r := setupTestRouter(h)

	body := models.DomainRequest{Domains: []string{"safe.com"}}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/filtering/whitelist", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestSetFilteringEnabled(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)

	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{
		Enabled:          true,
		BlacklistDomains: []string{"blocked.com"},
	})
	defer pe.Close()

	h.SetPolicyEngine(pe)
	r := setupTestRouter(h)

	stats := pe.Stats()
	assert.True(t, stats.Enabled)

	// Disable
	body := models.FilteringEnabledRequest{Enabled: false}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/filtering/enabled", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	stats = pe.Stats()
	assert.False(t, stats.Enabled)

	// Re-enable
	body.Enabled = true
	jsonBody, _ = json.Marshal(body)

	req = httptest.NewRequest(http.MethodPut, "/api/v1/filtering/enabled", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	stats = pe.Stats()
	assert.True(t, stats.Enabled)
}

func TestSetFilteringEnabled_NoEngine(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)
	r := setupTestRouter(h)

	body := models.FilteringEnabledRequest{Enabled: true}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/filtering/enabled", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestSetFilteringEnabled_InvalidRequest(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)

	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{Enabled: true})
	defer pe.Close()

	h.SetPolicyEngine(pe)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/filtering/enabled", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetWhitelist_NoEngine(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/filtering/whitelist", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestGetWhitelist_WithEngine(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)

	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{
		Enabled:          true,
		WhitelistDomains: []string{"safe.com", "trusted.org"},
	})
	defer pe.Close()

	h.SetPolicyEngine(pe)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/filtering/whitelist", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.DomainListResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 2, resp.Count)
}

func TestGetBlacklist_NoEngine(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/filtering/blacklist", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestGetBlacklist_WithEngine(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)

	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{
		Enabled:          true,
		BlacklistDomains: []string{"blocked.com"},
	})
	defer pe.Close()

	h.SetPolicyEngine(pe)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/filtering/blacklist", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.DomainListResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Count)
}

func TestRemoveWhitelist_NotImplemented(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)

	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{Enabled: true})
	defer pe.Close()

	h.SetPolicyEngine(pe)
	r := setupTestRouter(h)

	body := models.DomainDeleteRequest{Domains: []string{"example.com"}}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/filtering/whitelist", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

func TestRemoveBlacklist_NotImplemented(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)

	pe := filtering.NewPolicyEngine(filtering.PolicyEngineConfig{Enabled: true})
	defer pe.Close()

	h.SetPolicyEngine(pe)
	r := setupTestRouter(h)

	body := models.DomainDeleteRequest{Domains: []string{"example.com"}}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/filtering/blacklist", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
}
