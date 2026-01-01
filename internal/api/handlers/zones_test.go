package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jroosing/hydradns/internal/api/handlers"
	"github.com/jroosing/hydradns/internal/api/models"
	"github.com/jroosing/hydradns/internal/config"
	"github.com/jroosing/hydradns/internal/zone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListZones_Empty(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/zones", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.ZoneListResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Count)
	assert.Empty(t, resp.Zones)
}

func TestListZones_WithZones(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)

	zones := []*zone.Zone{
		{
			Origin: "example.com.",
			Records: []zone.Record{
				{Name: "example.com.", Type: 1, TTL: 300, RData: "192.168.1.1"},
				{Name: "www.example.com.", Type: 1, TTL: 300, RData: "192.168.1.2"},
			},
		},
		{
			Origin:  "test.org.",
			Records: []zone.Record{},
		},
	}
	h.SetZones(zones)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/zones", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.ZoneListResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 2, resp.Count)
	assert.Equal(t, "example.com.", resp.Zones[0].Name)
	assert.Equal(t, 2, resp.Zones[0].RecordCount)
	assert.Equal(t, "test.org.", resp.Zones[1].Name)
	assert.Equal(t, 0, resp.Zones[1].RecordCount)
}

func TestGetZone_NotFound(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/zones/nonexistent.com", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetZone_Found(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)

	zones := []*zone.Zone{
		{
			Origin: "example.com.",
			Records: []zone.Record{
				{Name: "example.com.", Type: 1, TTL: 300, RData: "192.168.1.1"},
				{Name: "www.example.com.", Type: 5, TTL: 300, RData: "example.com."},
			},
		},
	}
	h.SetZones(zones)
	r := setupTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/zones/example.com", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.ZoneDetailResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "example.com.", resp.Name)
	assert.Len(t, resp.Records, 2)
	assert.Equal(t, "A", resp.Records[0].Type)
	assert.Equal(t, "CNAME", resp.Records[1].Type)
}

func TestGetZone_FoundWithTrailingDot(t *testing.T) {
	cfg := &config.Config{}
	h := handlers.New(cfg, nil)

	zones := []*zone.Zone{
		{Origin: "example.com.", Records: []zone.Record{}},
	}
	h.SetZones(zones)
	r := setupTestRouter(h)

	// Request with trailing dot should also work
	req := httptest.NewRequest(http.MethodGet, "/api/v1/zones/example.com.", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
