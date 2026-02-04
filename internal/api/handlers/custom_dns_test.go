package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jroosing/hydradns/internal/api/handlers"
	"github.com/jroosing/hydradns/internal/api/models"
	"github.com/jroosing/hydradns/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListCustomDNS(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		CustomDNS: config.CustomDNSConfig{
			Hosts: map[string][]string{
				"test.local":   {"192.168.1.10", "2001:db8::1"},
				"server.local": {"10.0.0.1"},
			},
			CNAMEs: map[string]string{
				"www.test.local": "test.local",
			},
		},
	}

	h := handlers.New(cfg, nil, nil)

	router := gin.New()
	router.GET("/custom-dns", h.ListCustomDNS)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/custom-dns", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.CustomDNSRecordsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 2, resp.Count.Hosts)
	assert.Equal(t, 1, resp.Count.CNAMEs)
	assert.Equal(t, 3, resp.Count.Total)
	assert.Len(t, resp.Hosts, 2)
	assert.Len(t, resp.CNAMEs, 1)
}

func TestAddHost_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		CustomDNS: config.CustomDNSConfig{
			Hosts:  make(map[string][]string),
			CNAMEs: make(map[string]string),
		},
	}

	h := handlers.New(cfg, nil, nil)

	router := gin.New()
	router.POST("/custom-dns/hosts", h.AddHost)

	reqBody := models.AddHostRequest{
		Name: "test.local",
		IPs:  []string{"192.168.1.10", "2001:db8::1"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/custom-dns/hosts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp models.CustomDNSOperationResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp.Message, "added successfully")

	// Verify host was added to config
	assert.Equal(t, []string{"192.168.1.10", "2001:db8::1"}, cfg.CustomDNS.Hosts["test.local"])
}

func TestAddHost_Conflict(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		CustomDNS: config.CustomDNSConfig{
			Hosts: map[string][]string{
				"test.local": {"192.168.1.10"},
			},
		},
	}

	h := handlers.New(cfg, nil, nil)

	router := gin.New()
	router.POST("/custom-dns/hosts", h.AddHost)

	reqBody := models.AddHostRequest{
		Name: "test.local",
		IPs:  []string{"192.168.1.20"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/custom-dns/hosts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestAddHost_InvalidIP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		CustomDNS: config.CustomDNSConfig{
			Hosts: make(map[string][]string),
		},
	}

	h := handlers.New(cfg, nil, nil)

	router := gin.New()
	router.POST("/custom-dns/hosts", h.AddHost)

	reqBody := models.AddHostRequest{
		Name: "test.local",
		IPs:  []string{"invalid-ip"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/custom-dns/hosts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateHost_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		CustomDNS: config.CustomDNSConfig{
			Hosts: map[string][]string{
				"test.local": {"192.168.1.10"},
			},
		},
	}

	h := handlers.New(cfg, nil, nil)

	router := gin.New()
	router.PUT("/custom-dns/hosts/:name", h.UpdateHost)

	reqBody := models.UpdateHostRequest{
		IPs: []string{"192.168.1.20", "10.0.0.1"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/custom-dns/hosts/test.local", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify host was updated
	assert.Equal(t, []string{"192.168.1.20", "10.0.0.1"}, cfg.CustomDNS.Hosts["test.local"])
}

func TestUpdateHost_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		CustomDNS: config.CustomDNSConfig{
			Hosts: make(map[string][]string),
		},
	}

	h := handlers.New(cfg, nil, nil)

	router := gin.New()
	router.PUT("/custom-dns/hosts/:name", h.UpdateHost)

	reqBody := models.UpdateHostRequest{
		IPs: []string{"192.168.1.20"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/custom-dns/hosts/notfound.local", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteHost_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		CustomDNS: config.CustomDNSConfig{
			Hosts: map[string][]string{
				"test.local": {"192.168.1.10"},
			},
		},
	}

	h := handlers.New(cfg, nil, nil)

	router := gin.New()
	router.DELETE("/custom-dns/hosts/:name", h.DeleteHost)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/custom-dns/hosts/test.local", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify host was deleted
	_, exists := cfg.CustomDNS.Hosts["test.local"]
	assert.False(t, exists)
}

func TestDeleteHost_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		CustomDNS: config.CustomDNSConfig{
			Hosts: make(map[string][]string),
		},
	}

	h := handlers.New(cfg, nil, nil)

	router := gin.New()
	router.DELETE("/custom-dns/hosts/:name", h.DeleteHost)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/custom-dns/hosts/notfound.local", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAddCNAME_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		CustomDNS: config.CustomDNSConfig{
			Hosts:  make(map[string][]string),
			CNAMEs: make(map[string]string),
		},
	}

	h := handlers.New(cfg, nil, nil)

	router := gin.New()
	router.POST("/custom-dns/cnames", h.AddCNAME)

	reqBody := models.AddCNAMERequest{
		Alias:  "www.test.local",
		Target: "test.local",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/custom-dns/cnames", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	// Verify CNAME was added
	assert.Equal(t, "test.local", cfg.CustomDNS.CNAMEs["www.test.local"])
}

func TestAddCNAME_Conflict(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		CustomDNS: config.CustomDNSConfig{
			CNAMEs: map[string]string{
				"www.test.local": "test.local",
			},
		},
	}

	h := handlers.New(cfg, nil, nil)

	router := gin.New()
	router.POST("/custom-dns/cnames", h.AddCNAME)

	reqBody := models.AddCNAMERequest{
		Alias:  "www.test.local",
		Target: "other.local",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/custom-dns/cnames", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestUpdateCNAME_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		CustomDNS: config.CustomDNSConfig{
			CNAMEs: map[string]string{
				"www.test.local": "test.local",
			},
		},
	}

	h := handlers.New(cfg, nil, nil)

	router := gin.New()
	router.PUT("/custom-dns/cnames/:alias", h.UpdateCNAME)

	reqBody := models.UpdateCNAMERequest{
		Target: "newserver.local",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/custom-dns/cnames/www.test.local", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify CNAME was updated
	assert.Equal(t, "newserver.local", cfg.CustomDNS.CNAMEs["www.test.local"])
}

func TestUpdateCNAME_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		CustomDNS: config.CustomDNSConfig{
			CNAMEs: make(map[string]string),
		},
	}

	h := handlers.New(cfg, nil, nil)

	router := gin.New()
	router.PUT("/custom-dns/cnames/:alias", h.UpdateCNAME)

	reqBody := models.UpdateCNAMERequest{
		Target: "test.local",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/custom-dns/cnames/notfound.local", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteCNAME_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		CustomDNS: config.CustomDNSConfig{
			CNAMEs: map[string]string{
				"www.test.local": "test.local",
			},
		},
	}

	h := handlers.New(cfg, nil, nil)

	router := gin.New()
	router.DELETE("/custom-dns/cnames/:alias", h.DeleteCNAME)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/custom-dns/cnames/www.test.local", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify CNAME was deleted
	_, exists := cfg.CustomDNS.CNAMEs["www.test.local"]
	assert.False(t, exists)
}

func TestDeleteCNAME_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		CustomDNS: config.CustomDNSConfig{
			CNAMEs: make(map[string]string),
		},
	}

	h := handlers.New(cfg, nil, nil)

	router := gin.New()
	router.DELETE("/custom-dns/cnames/:alias", h.DeleteCNAME)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodDelete, "/custom-dns/cnames/notfound.local", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
