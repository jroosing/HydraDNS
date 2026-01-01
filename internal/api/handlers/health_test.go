package handlers_test

import (
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

func TestHealth(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.StatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Status)
}

func TestStats(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.ServerStatsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Uptime)
	assert.Greater(t, resp.GoRoutines, 0)
}

func TestStats_WithFilteringEngine(t *testing.T) {
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.ServerStatsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotNil(t, resp.FilteringStats)
	assert.True(t, resp.FilteringStats.Enabled)
}
