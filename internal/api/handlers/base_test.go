package handlers_test

import (
	"github.com/gin-gonic/gin"
	"github.com/jroosing/hydradns/internal/api/handlers"
)

func setupTestRouter(h *handlers.Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	api := r.Group("/api/v1")
	api.GET("/health", h.Health)
	api.GET("/stats", h.Stats)
	api.GET("/config", h.GetConfig)
	api.PUT("/config", h.PutConfig)
	api.POST("/config/reload", h.ReloadConfig)
	api.GET("/zones", h.ListZones)
	api.GET("/zones/:name", h.GetZone)
	api.GET("/filtering/whitelist", h.GetWhitelist)
	api.POST("/filtering/whitelist", h.AddWhitelist)
	api.DELETE("/filtering/whitelist", h.RemoveWhitelist)
	api.GET("/filtering/blacklist", h.GetBlacklist)
	api.POST("/filtering/blacklist", h.AddBlacklist)
	api.DELETE("/filtering/blacklist", h.RemoveBlacklist)
	api.GET("/filtering/stats", h.FilteringStats)
	api.PUT("/filtering/enabled", h.SetFilteringEnabled)

	return r
}
