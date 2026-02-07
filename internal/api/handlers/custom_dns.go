package handlers

import (
	"maps"
	"net/http"
	"net/netip"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jroosing/hydradns/internal/api/models"
)

// ListCustomDNS returns all custom DNS records (hosts and CNAMEs).
// @Summary List all custom DNS records
// @Description Returns all configured custom DNS host and CNAME records
// @Tags custom-dns
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} models.CustomDNSRecordsResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /custom-dns [get]
func (h *Handler) ListCustomDNS(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Create deep copies of maps to avoid data races during JSON marshaling
	hosts := make(map[string][]string)
	maps.Copy(hosts, h.cfg.CustomDNS.Hosts)
	cnames := make(map[string]string)
	maps.Copy(cnames, h.cfg.CustomDNS.CNAMEs)

	resp := models.CustomDNSRecordsResponse{
		Hosts:  hosts,
		CNAMEs: cnames,
		Count: models.CustomDNSCountsResponse{
			Hosts:  len(hosts),
			CNAMEs: len(cnames),
			Total:  len(hosts) + len(cnames),
		},
	}

	c.JSON(http.StatusOK, resp)
}

// AddHost adds a new host record to custom DNS.
// @Summary Add a host record
// @Description Adds a new custom DNS host record (A/AAAA)
// @Tags custom-dns
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param record body models.AddHostRequest true "Host record to add"
// @Success 201 {object} models.CustomDNSOperationResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse "Host already exists"
// @Failure 500 {object} models.ErrorResponse
// @Router /custom-dns/hosts [post]
func (h *Handler) AddHost(c *gin.Context) {
	var req models.AddHostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request: " + err.Error()})
		return
	}

	// Validate name
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Host name cannot be empty"})
		return
	}

	// Validate IPs
	if err := validateIPs(req.IPs); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	h.mu.Lock()

	// Check if host already exists
	if _, exists := h.cfg.CustomDNS.Hosts[name]; exists {
		h.mu.Unlock()
		c.JSON(http.StatusConflict, models.ErrorResponse{Error: "Host already exists: " + name})
		return
	}

	// Add the host
	if h.cfg.CustomDNS.Hosts == nil {
		h.cfg.CustomDNS.Hosts = make(map[string][]string)
	}
	h.cfg.CustomDNS.Hosts[name] = req.IPs

	// Get reload function before releasing lock
	reloadFunc := h.customDNSReloadFunc
	h.mu.Unlock()

	// Trigger resolver reload (outside of lock to avoid deadlock)
	h.triggerReload(reloadFunc)

	c.JSON(http.StatusCreated, models.CustomDNSOperationResponse{
		Message: "Host record added successfully",
		Data:    models.HostRecord{Name: name, IPs: req.IPs},
	})
}

// UpdateHost updates an existing host record.
// @Summary Update a host record
// @Description Updates the IP addresses for an existing custom DNS host
// @Tags custom-dns
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param name path string true "Host name"
// @Param record body models.UpdateHostRequest true "Updated IP addresses"
// @Success 200 {object} models.CustomDNSOperationResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /custom-dns/hosts/{name} [put]
func (h *Handler) UpdateHost(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Host name is required"})
		return
	}

	var req models.UpdateHostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request: " + err.Error()})
		return
	}

	// Validate IPs
	if err := validateIPs(req.IPs); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	h.mu.Lock()

	// Check if host exists
	if _, exists := h.cfg.CustomDNS.Hosts[name]; !exists {
		h.mu.Unlock()
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Host not found: " + name})
		return
	}

	// Update the host
	h.cfg.CustomDNS.Hosts[name] = req.IPs

	// Get reload function before releasing lock
	reloadFunc := h.customDNSReloadFunc
	h.mu.Unlock()

	// Trigger resolver reload (outside of lock to avoid deadlock)
	h.triggerReload(reloadFunc)

	c.JSON(http.StatusOK, models.CustomDNSOperationResponse{
		Message: "Host record updated successfully",
		Data:    models.HostRecord{Name: name, IPs: req.IPs},
	})
}

// DeleteHost removes a host record from custom DNS.
// @Summary Delete a host record
// @Description Removes a custom DNS host record
// @Tags custom-dns
// @Produce json
// @Security ApiKeyAuth
// @Param name path string true "Host name"
// @Success 200 {object} models.CustomDNSOperationResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /custom-dns/hosts/{name} [delete]
func (h *Handler) DeleteHost(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Host name is required"})
		return
	}

	h.mu.Lock()

	// Check if host exists
	if _, exists := h.cfg.CustomDNS.Hosts[name]; !exists {
		h.mu.Unlock()
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Host not found: " + name})
		return
	}

	// Delete the host
	delete(h.cfg.CustomDNS.Hosts, name)

	// Get reload function before releasing lock
	reloadFunc := h.customDNSReloadFunc
	h.mu.Unlock()

	// Trigger resolver reload (outside of lock to avoid deadlock)
	h.triggerReload(reloadFunc)

	c.JSON(http.StatusOK, models.CustomDNSOperationResponse{
		Message: "Host record deleted successfully",
	})
}

// AddCNAME adds a new CNAME record to custom DNS.
// @Summary Add a CNAME record
// @Description Adds a new custom DNS CNAME record
// @Tags custom-dns
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param record body models.AddCNAMERequest true "CNAME record to add"
// @Success 201 {object} models.CustomDNSOperationResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse "CNAME already exists"
// @Failure 500 {object} models.ErrorResponse
// @Router /custom-dns/cnames [post]
func (h *Handler) AddCNAME(c *gin.Context) {
	var req models.AddCNAMERequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request: " + err.Error()})
		return
	}

	// Validate alias and target
	alias := strings.TrimSpace(req.Alias)
	target := strings.TrimSpace(req.Target)
	if alias == "" || target == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Alias and target cannot be empty"})
		return
	}

	h.mu.Lock()

	// Check if CNAME already exists
	if _, exists := h.cfg.CustomDNS.CNAMEs[alias]; exists {
		h.mu.Unlock()
		c.JSON(http.StatusConflict, models.ErrorResponse{Error: "CNAME already exists: " + alias})
		return
	}

	// Add the CNAME
	if h.cfg.CustomDNS.CNAMEs == nil {
		h.cfg.CustomDNS.CNAMEs = make(map[string]string)
	}
	h.cfg.CustomDNS.CNAMEs[alias] = target

	// Get reload function before releasing lock
	reloadFunc := h.customDNSReloadFunc
	h.mu.Unlock()

	// Trigger resolver reload (outside of lock to avoid deadlock)
	h.triggerReload(reloadFunc)

	c.JSON(http.StatusCreated, models.CustomDNSOperationResponse{
		Message: "CNAME record added successfully",
		Data:    models.CNAMERecord{Alias: alias, Target: target},
	})
}

// UpdateCNAME updates an existing CNAME record.
// @Summary Update a CNAME record
// @Description Updates the target for an existing custom DNS CNAME
// @Tags custom-dns
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param alias path string true "CNAME alias"
// @Param record body models.UpdateCNAMERequest true "Updated target"
// @Success 200 {object} models.CustomDNSOperationResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /custom-dns/cnames/{alias} [put]
func (h *Handler) UpdateCNAME(c *gin.Context) {
	alias := c.Param("alias")
	if alias == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "CNAME alias is required"})
		return
	}

	var req models.UpdateCNAMERequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request: " + err.Error()})
		return
	}

	target := strings.TrimSpace(req.Target)
	if target == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Target cannot be empty"})
		return
	}

	h.mu.Lock()

	// Check if CNAME exists
	if _, exists := h.cfg.CustomDNS.CNAMEs[alias]; !exists {
		h.mu.Unlock()
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "CNAME not found: " + alias})
		return
	}

	// Update the CNAME
	h.cfg.CustomDNS.CNAMEs[alias] = target

	// Get reload function before releasing lock
	reloadFunc := h.customDNSReloadFunc
	h.mu.Unlock()

	// Trigger resolver reload (outside of lock to avoid deadlock)
	h.triggerReload(reloadFunc)

	c.JSON(http.StatusOK, models.CustomDNSOperationResponse{
		Message: "CNAME record updated successfully",
		Data:    models.CNAMERecord{Alias: alias, Target: target},
	})
}

// DeleteCNAME removes a CNAME record from custom DNS.
// @Summary Delete a CNAME record
// @Description Removes a custom DNS CNAME record
// @Tags custom-dns
// @Produce json
// @Security ApiKeyAuth
// @Param alias path string true "CNAME alias"
// @Success 200 {object} models.CustomDNSOperationResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /custom-dns/cnames/{alias} [delete]
func (h *Handler) DeleteCNAME(c *gin.Context) {
	alias := c.Param("alias")
	if alias == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "CNAME alias is required"})
		return
	}

	h.mu.Lock()

	// Check if CNAME exists
	if _, exists := h.cfg.CustomDNS.CNAMEs[alias]; !exists {
		h.mu.Unlock()
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "CNAME not found: " + alias})
		return
	}

	// Delete the CNAME
	delete(h.cfg.CustomDNS.CNAMEs, alias)

	// Get reload function before releasing lock
	reloadFunc := h.customDNSReloadFunc
	h.mu.Unlock()

	// Trigger resolver reload (outside of lock to avoid deadlock)
	h.triggerReload(reloadFunc)

	c.JSON(http.StatusOK, models.CustomDNSOperationResponse{
		Message: "CNAME record deleted successfully",
	})
}

// validateIPs checks that all provided IPs are valid IPv4 or IPv6 addresses.
func validateIPs(ips []string) error {
	if len(ips) == 0 {
		return &ValidationError{Message: "At least one IP address is required"}
	}

	for _, ip := range ips {
		trimmed := strings.TrimSpace(ip)
		if trimmed == "" {
			return &ValidationError{Message: "IP address cannot be empty"}
		}
		if _, err := netip.ParseAddr(trimmed); err != nil {
			return &ValidationError{Message: "Invalid IP address: " + trimmed}
		}
	}
	return nil
}

// ValidationError represents a validation error.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// triggerReload calls the reload function if one is registered.
// This must be called WITHOUT holding any locks.
func (h *Handler) triggerReload(reloadFunc func() error) {
	if reloadFunc == nil {
		h.logWarn("custom DNS configuration updated but no reload function registered")
		return
	}

	if err := reloadFunc(); err != nil {
		h.logError("failed to reload custom DNS resolver", err)
		return
	}

	h.logInfo("custom DNS resolver reloaded successfully")
}

// logInfo logs an info message if a logger is available.
func (h *Handler) logInfo(msg string) {
	if h.logger != nil {
		h.logger.Info(msg)
	}
}

// logWarn logs a warning message if a logger is available.
func (h *Handler) logWarn(msg string) {
	if h.logger != nil {
		h.logger.Warn(msg)
	}
}

// logError logs an error message if a logger is available.
func (h *Handler) logError(msg string, err error) {
	if h.logger != nil {
		h.logger.Error(msg, "err", err)
	}
}
