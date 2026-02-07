package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jroosing/hydradns/internal/api/models"
)

// GetConfig godoc
// @Summary Get current configuration
// @Description Returns the current server configuration (sensitive fields redacted)
// @Tags config
// @Produce json
// @Success 200 {object} models.ConfigResponse
// @Failure 500 {object} models.ErrorResponse
// @Security ApiKeyAuth
// @Router /config [get]
func (h *Handler) GetConfig(c *gin.Context) {
	if h.cfg == nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "config unavailable"})
		return
	}

	resp := models.ConfigResponse{
		Server: models.ServerConfigResponse{
			Host:                   h.cfg.Server.Host,
			Port:                   h.cfg.Server.Port,
			Workers:                h.cfg.Server.Workers.String(),
			MaxConcurrency:         h.cfg.Server.MaxConcurrency,
			UpstreamSocketPoolSize: h.cfg.Server.UpstreamSocketPoolSize,
			EnableTCP:              h.cfg.Server.EnableTCP,
			TCPFallback:            h.cfg.Server.TCPFallback,
		},
		Upstream:  h.cfg.Upstream,
		CustomDNS: h.cfg.CustomDNS,
		Logging:   h.cfg.Logging,
		Filtering: h.cfg.Filtering,
		RateLimit: h.cfg.RateLimit,
		API: models.APIConfigResponse{
			Enabled: h.cfg.API.Enabled,
			Host:    h.cfg.API.Host,
			Port:    h.cfg.API.Port,
		},
		Cluster: models.ClusterConfigResponse{
			Mode:         string(h.cfg.Cluster.Mode),
			NodeID:       h.cfg.Cluster.NodeID,
			PrimaryURL:   h.cfg.Cluster.PrimaryURL,
			SyncInterval: h.cfg.Cluster.SyncInterval,
			SyncTimeout:  h.cfg.Cluster.SyncTimeout,
		},
	}

	c.JSON(http.StatusOK, resp)
}

// PutConfig godoc
// @Summary Update configuration
// @Description Updates server configuration (requires restart for some settings)
// @Tags config
// @Accept json
// @Produce json
// @Param config body models.ConfigResponse true "Configuration update"
// @Success 200 {object} models.StatusResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 501 {object} models.ErrorResponse
// @Security ApiKeyAuth
// @Router /config [put]
func (h *Handler) PutConfig(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, models.ErrorResponse{Error: "config updates not yet implemented"})
}

// ReloadConfig godoc
// @Summary Reload configuration
// @Description Triggers a hot reload of configuration from disk
// @Tags config
// @Produce json
// @Success 200 {object} models.StatusResponse
// @Failure 500 {object} models.ErrorResponse
// @Security ApiKeyAuth
// @Router /config/reload [post]
func (h *Handler) ReloadConfig(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, models.ErrorResponse{Error: "config reload not yet implemented"})
}
