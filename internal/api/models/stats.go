package models

import "time"

// ServerStatsResponse contains server runtime statistics.
type ServerStatsResponse struct {
	Uptime         string              `json:"uptime"`
	UptimeSeconds  int64               `json:"uptime_seconds"`
	StartTime      time.Time           `json:"start_time"`
	GoRoutines     int                 `json:"goroutines"`
	MemoryAllocMB  float64             `json:"memory_alloc_mb"`
	NumCPU         int                 `json:"num_cpu"`
	DNSStats       DNSStatsResponse    `json:"dns"`
	FilteringStats *FilteringStatsResponse `json:"filtering,omitempty"`
}

// DNSStatsResponse contains DNS query statistics.
type DNSStatsResponse struct {
	QueriesTotal  uint64 `json:"queries_total"`
	QueriesUDP    uint64 `json:"queries_udp"`
	QueriesTCP    uint64 `json:"queries_tcp"`
	ResponsesNX   uint64 `json:"responses_nxdomain"`
	ResponsesErr  uint64 `json:"responses_error"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
}
