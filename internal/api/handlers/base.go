// Package handlers implements the REST API endpoint handlers for HydraDNS.
//
// @title HydraDNS Management API
// @version 1.0
// @description REST API for managing HydraDNS server configuration, zones, and filtering.
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
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jroosing/hydradns/internal/config"
	"github.com/jroosing/hydradns/internal/filtering"
	"github.com/jroosing/hydradns/internal/zone"
)

// Handler contains dependencies for API handlers.
type Handler struct {
	cfg       *config.Config
	logger    *slog.Logger
	startTime time.Time

	// Runtime components (set after server starts)
	policyEngine *filtering.PolicyEngine
	zones        []*zone.Zone
	mu           sync.RWMutex
}

// New creates a new Handler with the given configuration.
func New(cfg *config.Config, logger *slog.Logger) *Handler {
	return &Handler{
		cfg:       cfg,
		logger:    logger,
		startTime: time.Now(),
	}
}

// SetPolicyEngine sets the filtering policy engine for runtime access.
func (h *Handler) SetPolicyEngine(pe *filtering.PolicyEngine) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.policyEngine = pe
}

// SetZones sets the loaded zones for runtime access.
func (h *Handler) SetZones(zones []*zone.Zone) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.zones = zones
}

// formatRData converts zone record RData to a display string.
func formatRData(rdata any) string {
	if rdata == nil {
		return ""
	}
	return fmt.Sprintf("%v", rdata)
}

// formatRecordType converts a DNS record type to its name.
func formatRecordType(t uint16) string {
	switch t {
	case 1:
		return "A"
	case 2:
		return "NS"
	case 5:
		return "CNAME"
	case 6:
		return "SOA"
	case 12:
		return "PTR"
	case 15:
		return "MX"
	case 16:
		return "TXT"
	case 28:
		return "AAAA"
	default:
		return fmt.Sprintf("TYPE%d", t)
	}
}
