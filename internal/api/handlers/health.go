package handlers

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jroosing/hydradns/internal/api/models"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
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
// @Description Returns runtime statistics including system CPU usage, memory usage, and DNS metrics
// @Tags system
// @Produce json
// @Success 200 {object} models.ServerStatsResponse
// @Security ApiKeyAuth
// @Router /stats [get]
func (h *Handler) Stats(c *gin.Context) {
	uptime := time.Since(h.startTime)

	// Get system memory stats
	memStats := models.MemoryStats{}
	if vmStat, err := mem.VirtualMemory(); err == nil {
		memStats.TotalMB = float64(vmStat.Total) / 1024 / 1024
		memStats.FreeMB = float64(vmStat.Available) / 1024 / 1024
		memStats.UsedMB = float64(vmStat.Used) / 1024 / 1024
		memStats.UsedPercent = vmStat.UsedPercent
	}

	// Get system CPU stats (average over 200ms sample)
	cpuStats := models.CPUStats{
		NumCPU: runtime.NumCPU(),
	}
	if cpuPercent, err := cpu.Percent(200*time.Millisecond, false); err == nil && len(cpuPercent) > 0 {
		cpuStats.UsedPercent = cpuPercent[0]
		cpuStats.IdlePercent = 100.0 - cpuPercent[0]
	}

	resp := models.ServerStatsResponse{
		Uptime:        uptime.Round(time.Second).String(),
		UptimeSeconds: int64(uptime.Seconds()),
		StartTime:     h.startTime,
		CPU:           cpuStats,
		Memory:        memStats,
		DNSStats:      h.getDNSStats(),
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

// getDNSStats returns the DNS statistics as a model response.
func (h *Handler) getDNSStats() models.DNSStatsResponse {
	fn := h.GetDNSStatsFunc()
	if fn == nil {
		return models.DNSStatsResponse{}
	}
	snapshot := fn()
	return models.DNSStatsResponse{
		QueriesTotal: snapshot.QueriesTotal,
		QueriesUDP:   snapshot.QueriesUDP,
		QueriesTCP:   snapshot.QueriesTCP,
		ResponsesNX:  snapshot.ResponsesNX,
		ResponsesErr: snapshot.ResponsesErr,
		AvgLatencyMs: snapshot.AvgLatencyMs,
	}
}
