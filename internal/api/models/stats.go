package models

import "time"

// CPUStats contains system CPU statistics.
type CPUStats struct {
	NumCPU      int     `json:"num_cpu"`
	UsedPercent float64 `json:"used_percent"`
	IdlePercent float64 `json:"idle_percent"`
}

// MemoryStats contains system memory statistics.
type MemoryStats struct {
	TotalMB     float64 `json:"total_mb"`
	FreeMB      float64 `json:"free_mb"`
	UsedMB      float64 `json:"used_mb"`
	UsedPercent float64 `json:"used_percent"`
}

// ServerStatsResponse contains server runtime statistics.
type ServerStatsResponse struct {
	Uptime         string                  `json:"uptime"`
	UptimeSeconds  int64                   `json:"uptime_seconds"`
	StartTime      time.Time               `json:"start_time"`
	CPU            CPUStats                `json:"cpu"`
	Memory         MemoryStats             `json:"memory"`
	DNSStats       DNSStatsResponse        `json:"dns"`
	FilteringStats *FilteringStatsResponse `json:"filtering,omitempty"`
}

// DNSStatsResponse contains DNS query statistics.
type DNSStatsResponse struct {
	QueriesTotal uint64  `json:"queries_total"`
	QueriesUDP   uint64  `json:"queries_udp"`
	QueriesTCP   uint64  `json:"queries_tcp"`
	ResponsesNX  uint64  `json:"responses_nxdomain"`
	ResponsesErr uint64  `json:"responses_error"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}
