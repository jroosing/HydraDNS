// Package middleware_test provides behavior tests for the API middleware package.
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jroosing/hydradns/internal/api/middleware"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ============================================================================
// RequireAPIKey Middleware Tests
// ============================================================================

func TestRequireAPIKey_ValidKey(t *testing.T) {
	router := gin.New()
	router.Use(middleware.RequireAPIKey("test-secret"))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Api-Key", "test-secret")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireAPIKey_InvalidKey(t *testing.T) {
	router := gin.New()
	router.Use(middleware.RequireAPIKey("correct-key"))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Api-Key", "wrong-key")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireAPIKey_MissingKey(t *testing.T) {
	router := gin.New()
	router.Use(middleware.RequireAPIKey("expected-key"))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// No X-API-Key header
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireAPIKey_EmptyExpected(t *testing.T) {
	// When expected key is empty, any request should be allowed
	router := gin.New()
	router.Use(middleware.RequireAPIKey(""))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequireAPIKey_EmptyExpected_WithProvidedKey(t *testing.T) {
	// When expected key is empty, providing any key should still work
	router := gin.New()
	router.Use(middleware.RequireAPIKey(""))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Api-Key", "some-key")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ============================================================================
// SlogRequestLogger Middleware Tests
// ============================================================================

func TestSlogRequestLogger_NilLogger(t *testing.T) {
	// Should not panic with nil logger
	router := gin.New()
	router.Use(middleware.SlogRequestLogger(nil))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	// Should not panic
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSlogRequestLogger_RequestCompletes(t *testing.T) {
	router := gin.New()
	router.Use(middleware.SlogRequestLogger(nil))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
}

func TestSlogRequestLogger_DifferentMethods(t *testing.T) {
	router := gin.New()
	router.Use(middleware.SlogRequestLogger(nil))
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"created": true})
	})
	router.PUT("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"updated": true})
	})
	router.DELETE("/test", func(c *gin.Context) {
		c.JSON(http.StatusNoContent, nil)
	})

	tests := []struct {
		method     string
		statusCode int
	}{
		{"POST", http.StatusCreated},
		{"PUT", http.StatusOK},
		{"DELETE", http.StatusNoContent},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, tt.statusCode, w.Code, "Method: %s", tt.method)
	}
}

func TestSlogRequestLogger_ErrorStatus(t *testing.T) {
	router := gin.New()
	router.Use(middleware.SlogRequestLogger(nil))
	router.GET("/error", func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "something failed"})
	})

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestMiddlewareChain(t *testing.T) {
	router := gin.New()
	router.Use(middleware.SlogRequestLogger(nil))
	router.Use(middleware.RequireAPIKey("secret"))
	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": "protected"})
	})

	// With valid key
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("X-Api-Key", "secret")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Without key - should be rejected
	req2 := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w2 := httptest.NewRecorder()

	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)
}
