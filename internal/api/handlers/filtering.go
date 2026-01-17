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
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: "database not available"})
		return
	}

	domains, err := h.db.GetWhitelistDomains()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.DomainListResponse{Domains: domains, Count: len(domains)})
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
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: "database not available"})
		return
	}

	var req models.DomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	for _, domain := range req.Domains {
		if err := h.db.AddWhitelistDomain(domain); err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
			return
		}
		if pe != nil {
			pe.AddToWhitelist(domain)
		}
	}

	if h.logger != nil {
		h.logger.Info("added domains to whitelist", "count", len(req.Domains))
	}

	domains, err := h.db.GetWhitelistDomains()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, models.DomainListResponse{Domains: domains, Count: len(domains)})
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
	pe := h.GetPolicyEngine()
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: "database not available"})
		return
	}

	var req models.DomainDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	for _, domain := range req.Domains {
		if err := h.db.DeleteWhitelistDomain(domain); err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
			return
		}
		if pe != nil {
			pe.RemoveFromWhitelist(domain)
		}
	}

	domains, err := h.db.GetWhitelistDomains()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, models.DomainListResponse{Domains: domains, Count: len(domains)})
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
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: "database not available"})
		return
	}

	domains, err := h.db.GetBlacklistDomains()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.DomainListResponse{Domains: domains, Count: len(domains)})
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
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: "database not available"})
		return
	}

	var req models.DomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	for _, domain := range req.Domains {
		if err := h.db.AddBlacklistDomain(domain); err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
			return
		}
		if pe != nil {
			pe.AddToBlacklist(domain)
		}
	}

	if h.logger != nil {
		h.logger.Info("added domains to blacklist", "count", len(req.Domains))
	}

	domains, err := h.db.GetBlacklistDomains()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, models.DomainListResponse{Domains: domains, Count: len(domains)})
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
	pe := h.GetPolicyEngine()
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: "database not available"})
		return
	}

	var req models.DomainDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	for _, domain := range req.Domains {
		if err := h.db.DeleteBlacklistDomain(domain); err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
			return
		}
		if pe != nil {
			pe.RemoveFromBlacklist(domain)
		}
	}

	domains, err := h.db.GetBlacklistDomains()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, models.DomainListResponse{Domains: domains, Count: len(domains)})
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

// GetBlocklists lists all configured remote blocklists.
// @Summary Get blocklists
// @Description Returns all configured blocklists
// @Tags filtering
// @Produce json
// @Success 200 {object} models.BlocklistsResponse
// @Failure 503 {object} models.ErrorResponse
// @Security ApiKeyAuth
// @Router /filtering/blocklists [get]
func (h *Handler) GetBlocklists(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: "database not available"})
		return
	}

	bls, err := h.db.GetBlocklists()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: err.Error()})
		return
	}

	resp := models.BlocklistsResponse{Blocklists: make([]models.Blocklist, 0, len(bls)), Count: len(bls)}
	for _, b := range bls {
		resp.Blocklists = append(resp.Blocklists, models.Blocklist{
			Name:        b.Name,
			URL:         b.URL,
			Format:      b.Format,
			Enabled:     b.Enabled,
			LastFetched: b.LastFetched,
		})
	}

	c.JSON(http.StatusOK, resp)
}

// SetBlocklistEnabled godoc
// @Summary Enable or disable a blocklist
// @Description Toggles a specific blocklist on or off (takes effect after restart until hot-reload is implemented)
// @Tags filtering
// @Accept json
// @Produce json
// @Param name path string true "Blocklist name"
// @Param enabled body models.FilteringEnabledRequest true "Enable state"
// @Success 200 {object} models.StatusResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 503 {object} models.ErrorResponse
// @Security ApiKeyAuth
// @Router /filtering/blocklists/{name}/enabled [put]
func (h *Handler) SetBlocklistEnabled(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: "database not available"})
		return
	}

	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "missing blocklist name"})
		return
	}

	var req models.FilteringEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.db.EnableBlocklist(name, req.Enabled); err != nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: err.Error()})
		return
	}

	if h.logger != nil {
		h.logger.Info("blocklist enabled state changed", "name", name, "enabled", req.Enabled)
	}

	c.JSON(http.StatusOK, models.StatusResponse{Status: "ok"})
}

// RefreshBlocklist godoc
// @Summary Refresh a blocklist
// @Description Marks a blocklist as refreshed (updates last_fetched); engine reload pending future hot-reload
// @Tags filtering
// @Produce json
// @Param name path string true "Blocklist name"
// @Success 200 {object} models.StatusResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 503 {object} models.ErrorResponse
// @Security ApiKeyAuth
// @Router /filtering/blocklists/{name}/refresh [post]
func (h *Handler) RefreshBlocklist(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: "database not available"})
		return
	}

	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "missing blocklist name"})
		return
	}

	if err := h.db.UpdateBlocklistFetchTime(name); err != nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: err.Error()})
		return
	}

	if h.logger != nil {
		h.logger.Info("blocklist refreshed", "name", name)
	}

	c.JSON(http.StatusOK, models.StatusResponse{Status: "ok"})
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
