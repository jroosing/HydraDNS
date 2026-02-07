// Package handlers implements the REST API endpoint handlers for HydraDNS.
//
// REST API Endpoints:
//
// System Health:
//   - GET /api/v1/health - Health check status
//   - GET /api/v1/stats - Server statistics (uptime, memory, goroutines, filtering stats)
//   - GET /api/v1/config - Current configuration (sensitive values redacted)
//
// Zones (Authoritative DNS):
//   - GET /api/v1/zones - List all loaded zones
//   - GET /api/v1/zones/:name - Get zone details with all records
//
// Filtering (Domain Filtering):
//   - GET /api/v1/filtering/stats - Filtering statistics (queries blocked/allowed)
//   - PUT /api/v1/filtering/enabled - Enable/disable filtering at runtime
//   - GET /api/v1/filtering/whitelist - List whitelisted domains
//   - POST /api/v1/filtering/whitelist - Add domains to whitelist
//   - GET /api/v1/filtering/blacklist - List blacklisted domains
//   - POST /api/v1/filtering/blacklist - Add domains to blacklist
//
// Authentication:
//
// All endpoints except /health support optional API key authentication via
// the X-API-Key header. If configured, the API key is required for all
// endpoints except /health and /config (which may be public depending on policy).
//
// Security Considerations:
//
// - API is bound to localhost:8080 by default (not exposed to network)
// - Enable firewall rules to restrict access from trusted networks only
// - Use strong API keys (minimum 32 characters recommended)
// - Rotate API keys regularly
// - Log all API access in production
//
// @title HydraDNS Management API
// @version 1.0
// @description REST API for managing HydraDNS server configuration and filtering.
//
// @contact.name HydraDNS Support
// @contact.url https://github.com/jroosing/hydradns
//
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
//
// @host localhost:8080
// @BasePath /api/v1
//
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
package handlers

import (
	"log/slog"
	"sync"
	"time"

	"github.com/jroosing/hydradns/internal/cluster"
	"github.com/jroosing/hydradns/internal/config"
	"github.com/jroosing/hydradns/internal/database"
	"github.com/jroosing/hydradns/internal/filtering"
)

// DNSStatsSnapshot contains a point-in-time snapshot of DNS statistics.
type DNSStatsSnapshot struct {
	QueriesTotal uint64
	QueriesUDP   uint64
	QueriesTCP   uint64
	ResponsesNX  uint64
	ResponsesErr uint64
	AvgLatencyMs float64
}

// DNSStatsFunc is a function that returns DNS statistics.
type DNSStatsFunc func() DNSStatsSnapshot

// Handler contains dependencies for API handlers.
type Handler struct {
	cfg       *config.Config
	db        *database.DB
	logger    *slog.Logger
	startTime time.Time

	// Runtime components (set after server starts)
	policyEngine        *filtering.PolicyEngine
	customDNSReloadFunc func() error    // Callback to reload custom DNS resolver
	dnsStatsFunc        DNSStatsFunc    // Function to get DNS query statistics
	clusterSyncer       *cluster.Syncer // Cluster syncer for secondary mode
	mu                  sync.RWMutex
}

// New creates a new Handler with the given configuration and database.
func New(cfg *config.Config, db *database.DB, logger *slog.Logger) *Handler {
	return &Handler{
		cfg:       cfg,
		db:        db,
		logger:    logger,
		startTime: time.Now(),
	}
}

// DB returns the database connection for handlers that need it.
func (h *Handler) DB() *database.DB {
	return h.db
}

// SetPolicyEngine sets the filtering policy engine for runtime access.
func (h *Handler) SetPolicyEngine(pe *filtering.PolicyEngine) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.policyEngine = pe
}

// GetPolicyEngine retrieves the policy engine with safe read access.
func (h *Handler) GetPolicyEngine() *filtering.PolicyEngine {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.policyEngine
}

// SetCustomDNSReloadFunc sets the callback function for reloading custom DNS.
// This enables the API to trigger resolver rebuilds when custom DNS config changes.
func (h *Handler) SetCustomDNSReloadFunc(reloadFunc func() error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.customDNSReloadFunc = reloadFunc
}

// SetDNSStatsFunc sets the function to retrieve DNS statistics.
func (h *Handler) SetDNSStatsFunc(fn DNSStatsFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.dnsStatsFunc = fn
}

// GetDNSStatsFunc retrieves the DNS statistics function.
func (h *Handler) GetDNSStatsFunc() DNSStatsFunc {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.dnsStatsFunc
}

// SetClusterSyncer sets the cluster syncer for secondary mode.
func (h *Handler) SetClusterSyncer(syncer *cluster.Syncer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clusterSyncer = syncer
}

// GetClusterSyncer retrieves the cluster syncer.
func (h *Handler) GetClusterSyncer() *cluster.Syncer {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.clusterSyncer
}
