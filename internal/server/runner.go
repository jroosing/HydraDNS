package server

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"syscall"
	"time"

	"github.com/jroosing/hydradns/internal/config"
	"github.com/jroosing/hydradns/internal/filtering"
	"github.com/jroosing/hydradns/internal/resolvers"
	"github.com/jroosing/hydradns/internal/zone"
)

// Runner orchestrates the DNS server startup, configuration, and shutdown.
type Runner struct {
	logger *slog.Logger
}

// NewRunner creates a new server runner with the given logger.
func NewRunner(logger *slog.Logger) *Runner {
	return &Runner{logger: logger}
}

// Run starts the DNS server with the given configuration.
//
// Server lifecycle:
//  1. Configure runtime (GOMAXPROCS based on workers setting)
//  2. Load zone files for local resolution
//  3. Build resolver chain (zones -> forwarding)
//  4. Start UDP and optionally TCP servers
//  5. Wait for shutdown signal (SIGINT/SIGTERM)
//  6. Gracefully stop servers with timeout
func (r *Runner) Run(cfg *config.Config) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	ctx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()

	// Configure GOMAXPROCS based on worker settings
	desiredProcs := r.configureRuntime(cfg)

	// Calculate concurrency limits
	maxConc := r.calculateMaxConcurrency(cfg, desiredProcs)
	upPool := r.calculateUpstreamPoolSize(cfg, maxConc)

	// Load zone files
	zones := r.loadZones(cfg)

	// Build resolver chain
	resolver := r.buildResolverChain(cfg, zones, upPool)
	defer resolver.Close()

	// Create server components
	h := &QueryHandler{Logger: r.logger, Resolver: resolver, Timeout: 4 * time.Second}
	limiter := NewRateLimiter(RateLimitSettings{
		CleanupSeconds:   cfg.RateLimit.CleanupSeconds,
		MaxIPEntries:     cfg.RateLimit.MaxIPEntries,
		MaxPrefixEntries: cfg.RateLimit.MaxPrefixEntries,
		GlobalQPS:        cfg.RateLimit.GlobalQPS,
		GlobalBurst:      cfg.RateLimit.GlobalBurst,
		PrefixQPS:        cfg.RateLimit.PrefixQPS,
		PrefixBurst:      cfg.RateLimit.PrefixBurst,
		IPQPS:            cfg.RateLimit.IPQPS,
		IPBurst:          cfg.RateLimit.IPBurst,
	})

	addr := net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port))
	r.logStartup(cfg, addr, maxConc, upPool)

	// Start servers
	udp := &UDPServer{Logger: r.logger, Handler: h, Limiter: limiter, WorkersPerSocket: maxConc}
	var tcp *TCPServer
	if cfg.Server.EnableTCP {
		tcp = &TCPServer{Logger: r.logger, Handler: h}
	}

	errCh := make(chan error, 2)
	go func() { errCh <- udp.Run(ctx, addr) }()
	if tcp != nil {
		go func() { errCh <- tcp.Run(ctx, addr) }()
	}

	// Wait for shutdown or error
	select {
	case <-ctx.Done():
		// shutdown requested via signal
	case err := <-errCh:
		if err != nil {
			cancelRun()
			return err
		}
	}

	// Graceful shutdown
	stopTimeout := 5 * time.Second
	_ = udp.Stop(stopTimeout)
	if tcp != nil {
		_ = tcp.Stop(stopTimeout)
	}
	return nil
}

// configureRuntime sets GOMAXPROCS based on worker configuration.
// Workers can reduce but never increase parallelism beyond the default.
func (r *Runner) configureRuntime(cfg *config.Config) int {
	baseProcs := runtime.GOMAXPROCS(0)
	if baseProcs <= 0 {
		baseProcs = 1
	}
	desiredProcs := baseProcs

	if cfg.Server.Workers.Mode == config.WorkersFixed {
		w := cfg.Server.Workers.Value
		if w <= 0 {
			w = 1
		}
		if w < desiredProcs {
			desiredProcs = w
		}
	}

	prev := runtime.GOMAXPROCS(desiredProcs)
	actual := runtime.GOMAXPROCS(0)
	if r.logger != nil {
		r.logger.Info("runtime", "gomaxprocs", actual, "prev", prev, "base", baseProcs)
	}
	return actual
}

// calculateMaxConcurrency determines the maximum concurrent request handlers.
func (r *Runner) calculateMaxConcurrency(cfg *config.Config, procs int) int {
	maxConc := cfg.Server.MaxConcurrency
	if maxConc <= 0 {
		c := procs
		if c <= 0 {
			c = 1
		}
		maxConc = c * 256
		if maxConc > 2048 {
			maxConc = 2048
		}
		if maxConc < 1 {
			maxConc = 1
		}
	}
	return maxConc
}

// calculateUpstreamPoolSize determines the UDP connection pool size for upstream queries.
func (r *Runner) calculateUpstreamPoolSize(cfg *config.Config, maxConc int) int {
	upPool := cfg.Server.UpstreamSocketPoolSize
	if upPool <= 0 {
		upPool = maxConc
		if upPool < 64 {
			upPool = 64
		}
		if upPool > 1024 {
			upPool = 1024
		}
	}
	return upPool
}

// loadZones discovers and loads zone files from the configured location.
func (r *Runner) loadZones(cfg *config.Config) []*zone.Zone {
	zoneFiles := discoverZoneFiles(cfg.Zones.Directory, cfg.Zones.Files)
	zones := make([]*zone.Zone, 0, len(zoneFiles))

	for _, p := range zoneFiles {
		z, err := zone.LoadFile(p)
		if err != nil {
			if r.logger != nil {
				r.logger.Warn("failed to load zone file", "path", p, "err", err)
			}
			continue
		}
		zones = append(zones, z)
	}

	if len(zones) > 0 && r.logger != nil {
		r.logger.Info("zones enabled", "count", len(zones), "files", zoneFiles)
	}
	return zones
}

// buildResolverChain creates the resolver chain: filtering -> zones (if any) -> forwarding.
func (r *Runner) buildResolverChain(cfg *config.Config, zones []*zone.Zone, upPool int) resolvers.Resolver {
	resList := make([]resolvers.Resolver, 0, 2)

	if len(zones) > 0 {
		resList = append(resList, resolvers.NewZoneResolver(zones))
	}

	fwd := resolvers.NewForwardingResolver(cfg.Upstream.Servers, upPool, 0, cfg.Server.TCPFallback)
	resList = append(resList, fwd)

	var chain resolvers.Resolver = &resolvers.Chained{Resolvers: resList}

	// Wrap with filtering if enabled
	if cfg.Filtering.Enabled {
		policy := r.buildFilteringPolicy(cfg)
		chain = resolvers.NewFilteringResolver(policy, chain)
		if r.logger != nil {
			r.logger.Info("filtering enabled",
				"whitelist_count", len(cfg.Filtering.WhitelistDomains),
				"blacklist_count", len(cfg.Filtering.BlacklistDomains),
				"blocklists", len(cfg.Filtering.Blocklists),
			)
		}
	}

	return chain
}

// buildFilteringPolicy creates a PolicyEngine from the configuration.
func (r *Runner) buildFilteringPolicy(cfg *config.Config) *filtering.PolicyEngine {
	// Convert blocklist configs to BlocklistURLs
	blocklists := make([]filtering.BlocklistURL, 0, len(cfg.Filtering.Blocklists))
	for _, bl := range cfg.Filtering.Blocklists {
		format := filtering.FormatAuto
		switch bl.Format {
		case "adblock":
			format = filtering.FormatAdblock
		case "hosts":
			format = filtering.FormatHosts
		case "domains":
			format = filtering.FormatDomains
		}
		blocklists = append(blocklists, filtering.BlocklistURL{
			Name:   bl.Name,
			URL:    bl.URL,
			Format: format,
		})
	}

	// Parse refresh interval
	refreshInterval := 24 * time.Hour
	if cfg.Filtering.RefreshInterval != "" {
		if d, err := time.ParseDuration(cfg.Filtering.RefreshInterval); err == nil {
			refreshInterval = d
		}
	}

	return filtering.NewPolicyEngine(filtering.PolicyEngineConfig{
		Enabled:          cfg.Filtering.Enabled,
		BlockAction:      filtering.ActionBlock,
		LogBlocked:       cfg.Filtering.LogBlocked,
		LogAllowed:       cfg.Filtering.LogAllowed,
		WhitelistDomains: cfg.Filtering.WhitelistDomains,
		BlacklistDomains: cfg.Filtering.BlacklistDomains,
		BlocklistURLs:    blocklists,
		RefreshInterval:  refreshInterval,
	})
}

// logStartup logs server configuration at startup.
func (r *Runner) logStartup(cfg *config.Config, addr string, maxConc, upPool int) {
	if r.logger != nil {
		r.logger.Info(
			"dns listening",
			"addr", addr,
			"udp", true,
			"tcp", cfg.Server.EnableTCP,
			"upstreams", cfg.Upstream.Servers,
			"max_concurrency", maxConc,
			"upstream_pool", upPool,
		)
	}
}

// discoverZoneFiles returns zone files to load, either from explicit config
// or by scanning the zones directory.
func discoverZoneFiles(zonesDir string, explicit []string) []string {
	// Use explicit list if provided
	if len(explicit) > 0 {
		out := make([]string, 0, len(explicit))
		for _, p := range explicit {
			p = filepath.Clean(p)
			if p != "" {
				out = append(out, p)
			}
		}
		return out
	}

	// Otherwise scan directory
	if zonesDir == "" {
		zonesDir = "zones"
	}
	entries, err := os.ReadDir(zonesDir)
	if err != nil {
		return nil
	}

	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "" {
			continue
		}
		files = append(files, filepath.Join(zonesDir, name))
	}
	sort.Strings(files)
	return files
}
