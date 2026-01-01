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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetConfig(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:      "127.0.0.1",
			Port:      1053,
			EnableTCP: true,
		},
		API: config.APIConfig{
			Enabled: true,
			Host:    "127.0.0.1",
			Port:    8080,
		},
	}
	h := handlers.New(cfg, nil)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.ConfigResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", resp.Server.Host)
	assert.Equal(t, 1053, resp.Server.Port)
	assert.True(t, resp.Server.EnableTCP)
	assert.Equal(t, 8080, resp.API.Port)
}

func TestGetConfig_NilConfig(t *testing.T) {
	h := handlers.New(nil, nil)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPutConfig_NotImplemented(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/config", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

func TestReloadConfig_NotImplemented(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/config/reload", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
}
