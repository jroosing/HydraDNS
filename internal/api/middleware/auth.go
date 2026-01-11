// Package middleware provides HTTP middleware for the HydraDNS REST API,
// including API key authentication and request logging.
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jroosing/hydradns/internal/api/models"
)

// RequireAPIKey enforces a simple shared-secret API key.
// Clients must send `X-API-Key: <key>`.
func RequireAPIKey(expected string) gin.HandlerFunc {
	return func(c *gin.Context) {
		got := c.GetHeader("X-API-Key")
		if expected == "" || got == expected {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
	}
}
