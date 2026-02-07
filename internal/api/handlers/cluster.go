package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jroosing/hydradns/internal/api/models"
	"github.com/jroosing/hydradns/internal/cluster"
	"github.com/jroosing/hydradns/internal/config"
)

// GetClusterStatus godoc
// @Summary Get cluster status
// @Description Returns the current cluster mode and synchronization status
// @Tags cluster
// @Produce json
// @Success 200 {object} models.ClusterStatusResponse
// @Failure 500 {object} models.ErrorResponse
// @Security ApiKeyAuth
// @Router /cluster/status [get]
func (h *Handler) GetClusterStatus(c *gin.Context) {
	if h.cfg == nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "config unavailable"})
		return
	}

	h.mu.RLock()
	syncer := h.clusterSyncer
	h.mu.RUnlock()

	resp := models.ClusterStatusResponse{
		Mode:   string(h.cfg.Cluster.Mode),
		NodeID: h.cfg.Cluster.NodeID,
	}

	// Get config version
	if h.db != nil {
		if version, err := h.db.GetVersion(); err == nil {
			resp.ConfigVersion = version
		}
	}

	// If we have a syncer (secondary mode), include sync status
	if syncer != nil {
		status := syncer.Status()
		resp.PrimaryURL = status.PrimaryURL
		resp.LastSyncTime = status.LastSyncTime
		resp.LastSyncVersion = status.LastSyncVersion
		resp.LastSyncError = status.LastSyncError
		resp.NextSyncTime = status.NextSyncTime
		resp.SyncCount = status.SyncCount
		resp.ErrorCount = status.ErrorCount
	}

	c.JSON(http.StatusOK, resp)
}

// GetClusterExport godoc
// @Summary Export configuration for cluster sync
// @Description Returns configuration data for secondary nodes to import (primary only)
// @Tags cluster
// @Produce json
// @Success 200 {object} cluster.ExportData
// @Failure 403 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /cluster/export [get]
func (h *Handler) GetClusterExport(c *gin.Context) {
	if h.cfg == nil || h.db == nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "config unavailable"})
		return
	}

	// Only allow export from primary or standalone mode
	if h.cfg.Cluster.Mode == config.ClusterModeSecondary {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error: "export not allowed from secondary node",
		})
		return
	}

	// Validate shared secret if configured
	if h.cfg.Cluster.SharedSecret != "" {
		secret := c.GetHeader("X-Cluster-Secret")
		if secret != h.cfg.Cluster.SharedSecret {
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Error: "invalid cluster secret",
			})
			return
		}
	}

	// Get configuration version
	version, err := h.db.GetVersion()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to get config version",
		})
		return
	}

	// Build export data
	data := cluster.ExportData{
		Version:   version,
		Timestamp: time.Now().UTC(),
		NodeID:    h.cfg.Cluster.NodeID,
		Upstream:  h.cfg.Upstream,
		CustomDNS: h.cfg.CustomDNS,
		Filtering: h.cfg.Filtering,
	}

	// Log the sync request
	requestingNode := c.GetHeader("X-Node-ID")
	if requestingNode != "" {
		h.logger.Info("cluster export requested",
			"requesting_node", requestingNode,
			"version", version,
		)
	}

	c.JSON(http.StatusOK, data)
}

// PostClusterSync godoc
// @Summary Force immediate sync (secondary only)
// @Description Triggers an immediate configuration sync from the primary node
// @Tags cluster
// @Produce json
// @Success 200 {object} models.StatusResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security ApiKeyAuth
// @Router /cluster/sync [post]
func (h *Handler) PostClusterSync(c *gin.Context) {
	if h.cfg == nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "config unavailable"})
		return
	}

	if h.cfg.Cluster.Mode != config.ClusterModeSecondary {
		c.JSON(http.StatusForbidden, models.ErrorResponse{
			Error: "sync only available in secondary mode",
		})
		return
	}

	h.mu.RLock()
	syncer := h.clusterSyncer
	h.mu.RUnlock()

	if syncer == nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "syncer not initialized",
		})
		return
	}

	if err := syncer.ForceSync(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "sync failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.StatusResponse{Status: "sync completed"})
}

// PutClusterConfig godoc
// @Summary Configure cluster settings
// @Description Sets the cluster mode and configuration for this node
// @Tags cluster
// @Accept json
// @Produce json
// @Param config body models.ClusterConfigRequest true "Cluster configuration"
// @Success 200 {object} models.SetClusterConfigResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Security ApiKeyAuth
// @Router /cluster/config [put]
func (h *Handler) PutClusterConfig(c *gin.Context) {
	if h.cfg == nil || h.db == nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "config unavailable"})
		return
	}

	var req models.ClusterConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request: " + err.Error()})
		return
	}

	// Validate secondary mode requirements
	if req.Mode == "secondary" && req.PrimaryURL == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "primary_url is required for secondary mode",
		})
		return
	}

	// Generate node ID if not provided
	nodeID := req.NodeID
	if nodeID == "" {
		nodeID = uuid.New().String()[:8]
	}

	// Set defaults for optional fields
	syncInterval := req.SyncInterval
	if syncInterval == "" {
		syncInterval = "30s"
	}
	syncTimeout := req.SyncTimeout
	if syncTimeout == "" {
		syncTimeout = "10s"
	}

	// Build cluster config
	clusterCfg := &config.ClusterConfig{
		Mode:         config.ClusterMode(req.Mode),
		NodeID:       nodeID,
		PrimaryURL:   req.PrimaryURL,
		SharedSecret: req.SharedSecret,
		SyncInterval: syncInterval,
		SyncTimeout:  syncTimeout,
	}

	// Save to database
	if err := h.db.SetClusterConfig(clusterCfg); err != nil {
		h.logger.Error("failed to save cluster config", "err", err)
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to save cluster configuration",
		})
		return
	}

	// Update in-memory config
	h.cfg.Cluster = *clusterCfg

	h.logger.Info("cluster configuration updated",
		"mode", req.Mode,
		"node_id", nodeID,
		"primary_url", req.PrimaryURL,
	)

	// Determine if restart is needed
	// A restart is needed if switching to/from secondary mode (syncer lifecycle)
	requiresRestart := true
	message := "Cluster configuration saved. Restart required for changes to take effect."

	if req.Mode == "standalone" || req.Mode == "primary" {
		// For primary/standalone, we can potentially stop the syncer without restart
		h.mu.Lock()
		if h.clusterSyncer != nil {
			h.clusterSyncer.Stop()
			h.clusterSyncer = nil
			message = "Cluster configuration saved. Syncer stopped."
			requiresRestart = false
		} else {
			message = "Cluster configuration saved."
			requiresRestart = false
		}
		h.mu.Unlock()
	}

	c.JSON(http.StatusOK, models.SetClusterConfigResponse{
		Status:          "configured",
		Mode:            req.Mode,
		NodeID:          nodeID,
		Message:         message,
		RequiresRestart: requiresRestart,
	})
}

// GetClusterConfig godoc
// @Summary Get cluster configuration
// @Description Returns the current cluster configuration (secrets redacted)
// @Tags cluster
// @Produce json
// @Success 200 {object} models.ClusterConfigRequest
// @Failure 500 {object} models.ErrorResponse
// @Security ApiKeyAuth
// @Router /cluster/config [get]
func (h *Handler) GetClusterConfig(c *gin.Context) {
	if h.cfg == nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "config unavailable"})
		return
	}

	// Return config with secret redacted
	resp := models.ClusterConfigRequest{
		Mode:         string(h.cfg.Cluster.Mode),
		NodeID:       h.cfg.Cluster.NodeID,
		PrimaryURL:   h.cfg.Cluster.PrimaryURL,
		SharedSecret: "", // Redacted for security
		SyncInterval: h.cfg.Cluster.SyncInterval,
		SyncTimeout:  h.cfg.Cluster.SyncTimeout,
	}

	// Indicate if a secret is configured
	if h.cfg.Cluster.SharedSecret != "" {
		resp.SharedSecret = "********"
	}

	c.JSON(http.StatusOK, resp)
}
