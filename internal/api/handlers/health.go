package handlers

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jroosing/hydradns/internal/api/models"
)

// Health godoc
// @Summary Health check
// @Description Returns server health status
// @Tags system
// @Produce json
// @Success 200 {object} models.StatusResponse
// @Router /health [get]
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, models.StatusResponse{Status: "ok"})
}

// Stats godoc
// @Summary Server statistics
// @Description Returns runtime statistics including memory, goroutines, and DNS metrics
// @Tags system
// @Produce json
// @Success 200 {object} models.ServerStatsResponse
// @Security ApiKeyAuth
// @Router /stats [get]
func (h *Handler) Stats(c *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	uptime := time.Since(h.startTime)

	resp := models.ServerStatsResponse{
		Uptime:        uptime.Round(time.Second).String(),
		UptimeSeconds: int64(uptime.Seconds()),
		StartTime:     h.startTime,
		GoRoutines:    runtime.NumGoroutine(),
		MemoryAllocMB: float64(m.Alloc) / 1024 / 1024,
		NumCPU:        runtime.NumCPU(),
		DNSStats: models.DNSStatsResponse{
			QueriesTotal: 0,
		},
	}

	pe := h.GetPolicyEngine()

	if pe != nil {
		stats := pe.Stats()
		resp.FilteringStats = &models.FilteringStatsResponse{
			Enabled:        stats.Enabled,
			QueriesTotal:   stats.QueriesTotal,
			QueriesBlocked: stats.QueriesBlocked,
			QueriesAllowed: stats.QueriesAllowed,
			WhitelistSize:  stats.WhitelistSize,
			BlacklistSize:  stats.BlacklistSize,
		}
	}

	c.JSON(http.StatusOK, resp)
}
