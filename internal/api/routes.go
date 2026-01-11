package api

import (
	"github.com/gin-gonic/gin"
	"github.com/jroosing/hydradns/internal/api/handlers"
	"github.com/jroosing/hydradns/internal/api/middleware"
	"github.com/jroosing/hydradns/internal/config"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/jroosing/hydradns/internal/api/docs" // swagger docs
)

func RegisterRoutes(r *gin.Engine, h *handlers.Handler, cfg *config.Config) {
	// Swagger UI at /swagger/*
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := r.Group("/api/v1")

	// Optional API key protection.
	if cfg != nil && cfg.API.APIKey != "" {
		api.Use(middleware.RequireAPIKey(cfg.API.APIKey))
	}

	api.GET("/health", h.Health)
	api.GET("/stats", h.Stats)

	api.GET("/config", h.GetConfig)
	api.PUT("/config", h.PutConfig)
	api.POST("/config/reload", h.ReloadConfig)

	api.GET("/filtering/whitelist", h.GetWhitelist)
	api.POST("/filtering/whitelist", h.AddWhitelist)
	api.DELETE("/filtering/whitelist", h.RemoveWhitelist)

	api.GET("/filtering/blacklist", h.GetBlacklist)
	api.POST("/filtering/blacklist", h.AddBlacklist)
	api.DELETE("/filtering/blacklist", h.RemoveBlacklist)

	api.GET("/filtering/stats", h.FilteringStats)
	api.PUT("/filtering/enabled", h.SetFilteringEnabled)

	// Custom DNS endpoints
	api.GET("/custom-dns", h.ListCustomDNS)
	api.POST("/custom-dns/hosts", h.AddHost)
	api.PUT("/custom-dns/hosts/:name", h.UpdateHost)
	api.DELETE("/custom-dns/hosts/:name", h.DeleteHost)
	api.POST("/custom-dns/cnames", h.AddCNAME)
	api.PUT("/custom-dns/cnames/:alias", h.UpdateCNAME)
	api.DELETE("/custom-dns/cnames/:alias", h.DeleteCNAME)
}
