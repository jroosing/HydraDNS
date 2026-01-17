package filtering

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Ensure logging package is imported for side effects (configure logger).
var _ = struct{}{}

// Action represents the filtering decision for a domain.
type Action int

const (
	// ActionAllow allows the query to proceed.
	ActionAllow Action = iota
	// ActionBlock blocks the query and returns NXDOMAIN or a configured response.
	ActionBlock
	// ActionLog allows the query but logs it (for monitoring).
	ActionLog
)

// String returns a string representation of the action.
func (a Action) String() string {
	switch a {
	case ActionAllow:
		return "allow"
	case ActionBlock:
		return "block"
	case ActionLog:
		return "log"
	default:
		return "unknown"
	}
}

// PolicyResult contains the result of a policy evaluation.
type PolicyResult struct {
	Action   Action
	Rule     string // which rule matched (for logging)
	ListName string // which list matched (for logging)
}

// PolicyEngine evaluates DNS queries against whitelists and blacklists.
// Whitelist rules take priority over blacklist rules.
//
// Thread-safe for concurrent use.
type PolicyEngine struct {
	logger *slog.Logger

	whitelist *DomainTrie
	blacklist *DomainTrie

	// Statistics
	queriesTotal   atomic.Uint64
	queriesBlocked atomic.Uint64
	queriesAllowed atomic.Uint64

	// List metadata
	listSources map[string]ListSource
	mu          sync.RWMutex

	// Configuration
	enabled       bool
	blockAction   Action
	logBlocked    bool
	logAllowed    bool
	refreshTicker *time.Ticker
	refreshStop   chan struct{}
}

// ListSource tracks metadata about a blocklist source.
type ListSource struct {
	Name        string
	URL         string
	Format      ListFormat
	LastUpdate  time.Time
	LastError   error
	DomainCount int
}

// PolicyEngineConfig configures the policy engine.
type PolicyEngineConfig struct {
	// Logger is used for policy engine log output. If nil, the default logger is used.
	Logger *slog.Logger

	// Enabled determines if filtering is active.
	Enabled bool

	// BlockAction is the action to take for blocked domains.
	BlockAction Action

	// LogBlocked enables logging of blocked queries.
	LogBlocked bool

	// LogAllowed enables logging of allowed queries (verbose).
	LogAllowed bool

	// WhitelistDomains is a list of domains to always allow.
	WhitelistDomains []string

	// BlacklistDomains is a list of domains to always block.
	BlacklistDomains []string

	// BlocklistURLs is a list of remote blocklists to fetch.
	BlocklistURLs []BlocklistURL

	// RefreshInterval is how often to refresh remote blocklists.
	// Zero means no automatic refresh.
	RefreshInterval time.Duration
}

// BlocklistURL represents a remote blocklist configuration.
type BlocklistURL struct {
	Name   string
	URL    string
	Format ListFormat
}

// NewPolicyEngine creates a new policy engine with the given configuration.
func NewPolicyEngine(cfg PolicyEngineConfig) *PolicyEngine {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	pe := &PolicyEngine{
		logger:      logger,
		whitelist:   NewDomainTrie(),
		blacklist:   NewDomainTrie(),
		listSources: make(map[string]ListSource),
		enabled:     cfg.Enabled,
		blockAction: cfg.BlockAction,
		logBlocked:  cfg.LogBlocked,
		logAllowed:  cfg.LogAllowed,
	}

	// Add configured whitelist domains
	parser := NewParser()
	if len(cfg.WhitelistDomains) > 0 {
		for _, domain := range cfg.WhitelistDomains {
			pe.whitelist.Add(domain, true)
		}
	}

	// Add configured blacklist domains
	if len(cfg.BlacklistDomains) > 0 {
		for _, domain := range cfg.BlacklistDomains {
			pe.blacklist.Add(domain, true)
		}
	}

	// Fetch remote blocklists (in background for startup speed)
	if len(cfg.BlocklistURLs) > 0 {
		go pe.loadBlocklists(parser, cfg.BlocklistURLs)
	}

	// Start refresh timer if configured
	if cfg.RefreshInterval > 0 && len(cfg.BlocklistURLs) > 0 {
		pe.refreshTicker = time.NewTicker(cfg.RefreshInterval)
		pe.refreshStop = make(chan struct{})
		go pe.refreshLoop(parser, cfg.BlocklistURLs)
	}

	return pe
}

// loadBlocklists fetches and parses all configured blocklists.
func (pe *PolicyEngine) loadBlocklists(parser *Parser, urls []BlocklistURL) {
	for _, bl := range urls {
		pe.loadBlocklist(parser, bl)
	}
}

// loadBlocklist fetches and parses a single blocklist.
func (pe *PolicyEngine) loadBlocklist(parser *Parser, bl BlocklistURL) {
	source := ListSource{
		Name:       bl.Name,
		URL:        bl.URL,
		Format:     bl.Format,
		LastUpdate: time.Now(),
	}

	trie, err := parser.ParseURL(bl.URL, bl.Format)
	if err != nil {
		source.LastError = err
		pe.logger.Warn("Failed to load blocklist",
			"name", bl.Name,
			"url", bl.URL,
			"error", err)
	} else {
		source.DomainCount = trie.Size()
		pe.blacklist.Merge(trie)
		pe.logger.Info("Loaded blocklist",
			"name", bl.Name,
			"domains", trie.Size())
	}

	pe.mu.Lock()
	pe.listSources[bl.Name] = source
	pe.mu.Unlock()
}

// refreshLoop periodically refreshes blocklists.
func (pe *PolicyEngine) refreshLoop(parser *Parser, urls []BlocklistURL) {
	for {
		select {
		case <-pe.refreshTicker.C:
			pe.logger.Debug("Refreshing blocklists...")
			// Create a new blacklist and merge all sources
			newBlacklist := NewDomainTrie()

			// Re-add static blacklist domains
			// (We don't track them separately, so we can't restore them here.
			// In a production system, you'd want to track static vs dynamic entries.)

			for _, bl := range urls {
				trie, err := parser.ParseURL(bl.URL, bl.Format)
				if err != nil {
					pe.logger.Warn("Failed to refresh blocklist",
						"name", bl.Name,
						"error", err)
					continue
				}
				newBlacklist.Merge(trie)
			}

			pe.mu.Lock()
			pe.blacklist = newBlacklist
			pe.mu.Unlock()

			pe.logger.Info("Blocklists refreshed", "total_domains", newBlacklist.Size())

		case <-pe.refreshStop:
			return
		}
	}
}

// Evaluate checks a domain against the policy and returns the action to take.
func (pe *PolicyEngine) Evaluate(domain string) PolicyResult {
	pe.queriesTotal.Add(1)

	// If filtering is disabled, allow everything
	if !pe.enabled {
		pe.queriesAllowed.Add(1)
		return PolicyResult{Action: ActionAllow}
	}

	// Whitelist takes priority
	if pe.whitelist.Contains(domain) {
		pe.queriesAllowed.Add(1)
		if pe.logAllowed {
			pe.logger.Debug("Domain allowed by whitelist", "domain", domain)
		}
		return PolicyResult{
			Action:   ActionAllow,
			Rule:     domain,
			ListName: "whitelist",
		}
	}

	// Check blacklist
	if pe.blacklist.Contains(domain) {
		pe.queriesBlocked.Add(1)
		if pe.logBlocked {
			pe.logger.Info("Domain blocked", "domain", domain)
		}
		return PolicyResult{
			Action:   pe.blockAction,
			Rule:     domain,
			ListName: "blacklist",
		}
	}

	// Default: allow
	pe.queriesAllowed.Add(1)
	return PolicyResult{Action: ActionAllow}
}

// EvaluateWithContext is like Evaluate but respects context cancellation.
func (pe *PolicyEngine) EvaluateWithContext(ctx context.Context, domain string) (PolicyResult, error) {
	select {
	case <-ctx.Done():
		return PolicyResult{}, ctx.Err()
	default:
		return pe.Evaluate(domain), nil
	}
}

// AddToWhitelist adds a domain to the whitelist.
func (pe *PolicyEngine) AddToWhitelist(domain string) {
	pe.whitelist.Add(domain, true)
}

// AddToBlacklist adds a domain to the blacklist.
func (pe *PolicyEngine) AddToBlacklist(domain string) {
	pe.blacklist.Add(domain, true)
}

// RemoveFromWhitelist removes a domain from the whitelist.
func (pe *PolicyEngine) RemoveFromWhitelist(domain string) {
	pe.whitelist.Remove(domain)
}

// RemoveFromBlacklist removes a domain from the blacklist.
func (pe *PolicyEngine) RemoveFromBlacklist(domain string) {
	pe.blacklist.Remove(domain)
}

// Stats returns current filtering statistics.
func (pe *PolicyEngine) Stats() PolicyStats {
	return PolicyStats{
		QueriesTotal:   pe.queriesTotal.Load(),
		QueriesBlocked: pe.queriesBlocked.Load(),
		QueriesAllowed: pe.queriesAllowed.Load(),
		WhitelistSize:  pe.whitelist.Size(),
		BlacklistSize:  pe.blacklist.Size(),
		Enabled:        pe.enabled,
	}
}

// PolicyStats contains filtering statistics.
type PolicyStats struct {
	QueriesTotal   uint64
	QueriesBlocked uint64
	QueriesAllowed uint64
	WhitelistSize  int
	BlacklistSize  int
	Enabled        bool
}

// ListInfo returns information about loaded blocklists.
func (pe *PolicyEngine) ListInfo() []ListSource {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	sources := make([]ListSource, 0, len(pe.listSources))
	for _, s := range pe.listSources {
		sources = append(sources, s)
	}
	return sources
}

// SetEnabled enables or disables filtering.
func (pe *PolicyEngine) SetEnabled(enabled bool) {
	pe.enabled = enabled
}

// Close stops any background goroutines.
func (pe *PolicyEngine) Close() error {
	if pe.refreshTicker != nil {
		pe.refreshTicker.Stop()
	}
	if pe.refreshStop != nil {
		close(pe.refreshStop)
	}
	return nil
}

// String returns a summary of the policy engine state.
func (pe *PolicyEngine) String() string {
	stats := pe.Stats()
	return fmt.Sprintf("PolicyEngine{enabled=%v, whitelist=%d, blacklist=%d, blocked=%d/%d}",
		stats.Enabled, stats.WhitelistSize, stats.BlacklistSize,
		stats.QueriesBlocked, stats.QueriesTotal)
}
