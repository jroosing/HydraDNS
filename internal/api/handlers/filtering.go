package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jroosing/hydradns/internal/api/models"
)

// GetWhitelist godoc
// @Summary Get whitelist domains
// @Description Returns all domains in the whitelist
// @Tags filtering
// @Produce json
// @Success 200 {object} models.DomainListResponse
// @Failure 503 {object} models.ErrorResponse
// @Security ApiKeyAuth
// @Router /filtering/whitelist [get]
func (h *Handler) GetWhitelist(c *gin.Context) {
	pe := h.GetPolicyEngine()

	if pe == nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: "filtering not enabled"})
		return
	}

	stats := pe.Stats()
	c.JSON(http.StatusOK, models.DomainListResponse{
		Domains: []string{},
		Count:   stats.WhitelistSize,
	})
}

// AddWhitelist godoc
// @Summary Add domains to whitelist
// @Description Adds one or more domains to the whitelist
// @Tags filtering
// @Accept json
// @Produce json
// @Param domains body models.DomainRequest true "Domains to add"
// @Success 200 {object} models.StatusResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 503 {object} models.ErrorResponse
// @Security ApiKeyAuth
// @Router /filtering/whitelist [post]
func (h *Handler) AddWhitelist(c *gin.Context) {
	pe := h.GetPolicyEngine()

	if pe == nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: "filtering not enabled"})
		return
	}

	var req models.DomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	for _, domain := range req.Domains {
		pe.AddToWhitelist(domain)
	}

	if h.logger != nil {
		h.logger.Info("added domains to whitelist", "count", len(req.Domains))
	}

	c.JSON(http.StatusOK, models.StatusResponse{Status: "ok"})
}

// RemoveWhitelist godoc
// @Summary Remove domains from whitelist
// @Description Removes one or more domains from the whitelist
// @Tags filtering
// @Accept json
// @Produce json
// @Param domains body models.DomainDeleteRequest true "Domains to remove"
// @Success 200 {object} models.StatusResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 501 {object} models.ErrorResponse
// @Security ApiKeyAuth
// @Router /filtering/whitelist [delete]
func (h *Handler) RemoveWhitelist(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, models.ErrorResponse{Error: "whitelist removal not yet implemented"})
}

// GetBlacklist godoc
// @Summary Get blacklist domains
// @Description Returns all domains in the blacklist
// @Tags filtering
// @Produce json
// @Success 200 {object} models.DomainListResponse
// @Failure 503 {object} models.ErrorResponse
// @Security ApiKeyAuth
// @Router /filtering/blacklist [get]
func (h *Handler) GetBlacklist(c *gin.Context) {
	pe := h.GetPolicyEngine()

	if pe == nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: "filtering not enabled"})
		return
	}

	stats := pe.Stats()
	c.JSON(http.StatusOK, models.DomainListResponse{
		Domains: []string{},
		Count:   stats.BlacklistSize,
	})
}

// AddBlacklist godoc
// @Summary Add domains to blacklist
// @Description Adds one or more domains to the blacklist
// @Tags filtering
// @Accept json
// @Produce json
// @Param domains body models.DomainRequest true "Domains to add"
// @Success 200 {object} models.StatusResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 503 {object} models.ErrorResponse
// @Security ApiKeyAuth
// @Router /filtering/blacklist [post]
func (h *Handler) AddBlacklist(c *gin.Context) {
	pe := h.GetPolicyEngine()

	if pe == nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: "filtering not enabled"})
		return
	}

	var req models.DomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	for _, domain := range req.Domains {
		pe.AddToBlacklist(domain)
	}

	if h.logger != nil {
		h.logger.Info("added domains to blacklist", "count", len(req.Domains))
	}

	c.JSON(http.StatusOK, models.StatusResponse{Status: "ok"})
}

// RemoveBlacklist godoc
// @Summary Remove domains from blacklist
// @Description Removes one or more domains from the blacklist
// @Tags filtering
// @Accept json
// @Produce json
// @Param domains body models.DomainDeleteRequest true "Domains to remove"
// @Success 200 {object} models.StatusResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 501 {object} models.ErrorResponse
// @Security ApiKeyAuth
// @Router /filtering/blacklist [delete]
func (h *Handler) RemoveBlacklist(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, models.ErrorResponse{Error: "blacklist removal not yet implemented"})
}

// FilteringStats godoc
// @Summary Get filtering statistics
// @Description Returns detailed filtering statistics
// @Tags filtering
// @Produce json
// @Success 200 {object} models.FilteringStatsResponse
// @Failure 503 {object} models.ErrorResponse
// @Security ApiKeyAuth
// @Router /filtering/stats [get]
func (h *Handler) FilteringStats(c *gin.Context) {
	h.mu.RLock()
	pe := h.policyEngine
	h.mu.RUnlock()

	if pe == nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: "filtering not enabled"})
		return
	}

	stats := pe.Stats()
	c.JSON(http.StatusOK, models.FilteringStatsResponse{
		Enabled:        stats.Enabled,
		QueriesTotal:   stats.QueriesTotal,
		QueriesBlocked: stats.QueriesBlocked,
		QueriesAllowed: stats.QueriesAllowed,
		WhitelistSize:  stats.WhitelistSize,
		BlacklistSize:  stats.BlacklistSize,
	})
}

// SetFilteringEnabled godoc
// @Summary Enable or disable filtering
// @Description Toggles the filtering engine on or off
// @Tags filtering
// @Accept json
// @Produce json
// @Param enabled body models.FilteringEnabledRequest true "Enable state"
// @Success 200 {object} models.StatusResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 503 {object} models.ErrorResponse
// @Security ApiKeyAuth
// @Router /filtering/enabled [put]
func (h *Handler) SetFilteringEnabled(c *gin.Context) {
	h.mu.RLock()
	pe := h.policyEngine
	h.mu.RUnlock()

	if pe == nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: "filtering not available"})
		return
	}

	var req models.FilteringEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	pe.SetEnabled(req.Enabled)

	if h.logger != nil {
		h.logger.Info("filtering enabled state changed", "enabled", req.Enabled)
	}

	c.JSON(http.StatusOK, models.StatusResponse{Status: "ok"})
}
