package server

import (
	"sync/atomic"
)

// DNSStats collects DNS query statistics.
// All methods are safe for concurrent use.
type DNSStats struct {
	queriesTotal   atomic.Uint64
	queriesUDP     atomic.Uint64
	queriesTCP     atomic.Uint64
	responsesNX    atomic.Uint64
	responsesErr   atomic.Uint64
	latencyTotalNs atomic.Uint64
}

// NewDNSStats creates a new DNS statistics collector.
func NewDNSStats() *DNSStats {
	return &DNSStats{}
}

// RecordQuery records a DNS query for the given transport (udp or tcp).
func (s *DNSStats) RecordQuery(transport string) {
	s.queriesTotal.Add(1)
	switch transport {
	case "udp":
		s.queriesUDP.Add(1)
	case "tcp":
		s.queriesTCP.Add(1)
	}
}

// RecordNXDOMAIN records an NXDOMAIN response.
func (s *DNSStats) RecordNXDOMAIN() {
	s.responsesNX.Add(1)
}

// RecordError records an error response (SERVFAIL, FORMERR, etc.).
func (s *DNSStats) RecordError() {
	s.responsesErr.Add(1)
}

// RecordLatency records query latency in nanoseconds.
func (s *DNSStats) RecordLatency(ns int64) {
	if ns > 0 {
		s.latencyTotalNs.Add(uint64(ns))
	}
}

// DNSStatsSnapshot is a point-in-time snapshot of DNS server statistics.
type DNSStatsSnapshot struct {
	QueriesTotal uint64
	QueriesUDP   uint64
	QueriesTCP   uint64
	ResponsesNX  uint64
	ResponsesErr uint64
	AvgLatencyMs float64
}

// Snapshot returns the current statistics.
func (s *DNSStats) Snapshot() DNSStatsSnapshot {
	total := s.queriesTotal.Load()
	latencyNs := s.latencyTotalNs.Load()

	avgLatencyMs := 0.0
	if total > 0 {
		avgLatencyMs = float64(latencyNs) / float64(total) / 1e6
	}

	return DNSStatsSnapshot{
		QueriesTotal: total,
		QueriesUDP:   s.queriesUDP.Load(),
		QueriesTCP:   s.queriesTCP.Load(),
		ResponsesNX:  s.responsesNX.Load(),
		ResponsesErr: s.responsesErr.Load(),
		AvgLatencyMs: avgLatencyMs,
	}
}
