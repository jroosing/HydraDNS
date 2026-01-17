package server

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/jroosing/hydradns/internal/config"
	"github.com/jroosing/hydradns/internal/filtering"
	"github.com/jroosing/hydradns/internal/resolvers"
)

// Runner orchestrates the DNS server startup, configuration, and shutdown.
type Runner struct {
	logger       *slog.Logger
	policyEngine *filtering.PolicyEngine
}

// NewRunner creates a new server runner with the given logger.
func NewRunner(logger *slog.Logger) *Runner {
	return &Runner{logger: logger}
}

// SetPolicyEngine injects a shared policy engine for both DNS resolution and the API.
// If nil, RunWithContext will build one from the current config.
func (r *Runner) SetPolicyEngine(pe *filtering.PolicyEngine) {
	r.policyEngine = pe
}

// Run starts the DNS server with the given configuration.
//
// Server lifecycle:
//  1. Configure runtime (GOMAXPROCS based on workers setting)
//  2. Initialize custom DNS resolver (if configured)
//  3. Build resolver chain (custom DNS -> forwarding)
//  4. Start UDP and optionally TCP servers
//  5. Wait for shutdown signal (SIGINT/SIGTERM)
//  6. Gracefully stop servers with timeout
func (r *Runner) Run(cfg *config.Config) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	return r.RunWithContext(ctx, cfg)
}

// RunWithContext starts the DNS server and blocks until ctx is canceled or a server error occurs.
//
// This enables callers (e.g. a management API) to share the same shutdown signal.
//
// Goroutine lifecycle: Spawns two long-lived goroutines:
// 1. UDP server (udp.Run) - exits when context cancelled
// 2. TCP server (tcp.Run) - exits when context cancelled (if enabled)
// Both servers spawn their own worker goroutines internally.
// Cleanup: Resolvers closed, rate limiter cleanup triggered by context cancellation.
func (r *Runner) RunWithContext(ctx context.Context, cfg *config.Config) error {
	ctx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()

	// Configure GOMAXPROCS based on worker settings
	desiredProcs := r.configureRuntime(cfg)

	// Calculate concurrency limits
	maxConc := r.calculateMaxConcurrency(cfg, desiredProcs)
	upPool := r.calculateUpstreamPoolSize(cfg, maxConc)

	// Initialize custom DNS resolver
	customResolver := r.initCustomDNS(cfg)

	// Build or reuse filtering policy
	policy := r.policyEngine
	if policy == nil {
		policy = BuildPolicyEngine(cfg, r.logger)
		r.policyEngine = policy
	}

	// Build resolver chain
	resolver := r.buildResolverChain(cfg, customResolver, upPool, policy)
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
		maxConc = max(min(c*256, 2048), 1)
	}
	return maxConc
}

// calculateUpstreamPoolSize determines the UDP connection pool size for upstream queries.
func (r *Runner) calculateUpstreamPoolSize(cfg *config.Config, maxConc int) int {
	upPool := cfg.Server.UpstreamSocketPoolSize
	if upPool <= 0 {
		upPool = min(max(maxConc, 64), 1024)
	}
	return upPool
}

// initCustomDNS creates a custom DNS resolver from configuration.
func (r *Runner) initCustomDNS(cfg *config.Config) *resolvers.CustomDNSResolver {
	if len(cfg.CustomDNS.Hosts) == 0 && len(cfg.CustomDNS.CNAMEs) == 0 {
		return nil
	}

	customResolver, err := resolvers.NewCustomDNSResolver(cfg.CustomDNS.Hosts, cfg.CustomDNS.CNAMEs)
	if err != nil {
		if r.logger != nil {
			r.logger.Error("failed to initialize custom DNS", "err", err)
		}
		return nil
	}

	if r.logger != nil {
		r.logger.Info("custom DNS enabled",
			"hosts", len(cfg.CustomDNS.Hosts),
			"cnames", len(cfg.CustomDNS.CNAMEs),
		)
	}
	return customResolver
}

// buildResolverChain creates the resolver chain: filtering -> custom DNS (if any) -> forwarding.
func (r *Runner) buildResolverChain(cfg *config.Config, customResolver *resolvers.CustomDNSResolver, upPool int, policy *filtering.PolicyEngine) resolvers.Resolver {
	resList := make([]resolvers.Resolver, 0, 2)

	if customResolver != nil {
		resList = append(resList, customResolver)
	}

	// Parse upstream timeouts
	udpTimeout, _ := time.ParseDuration(cfg.Upstream.UDPTimeout)
	tcpTimeout, _ := time.ParseDuration(cfg.Upstream.TCPTimeout)

	fwd := resolvers.NewForwardingResolver(
		cfg.Upstream.Servers,
		upPool,
		0,
		cfg.Server.TCPFallback,
		udpTimeout,
		tcpTimeout,
		cfg.Upstream.MaxRetries,
	)
	resList = append(resList, fwd)

	var chain resolvers.Resolver = &resolvers.Chained{Resolvers: resList}

	// Always wrap with filtering; the policy's enabled flag controls behavior.
	if policy != nil {
		chain = resolvers.NewFilteringResolver(policy, chain)
		if r.logger != nil {
			r.logger.Info("filtering configured",
				"enabled", cfg.Filtering.Enabled,
				"whitelist_count", len(cfg.Filtering.WhitelistDomains),
				"blacklist_count", len(cfg.Filtering.BlacklistDomains),
				"blocklists", len(cfg.Filtering.Blocklists),
			)
		}
	}

	return chain
}

// BuildPolicyEngine constructs a filtering policy engine from the config.
// The returned engine may be disabled based on cfg.Filtering.Enabled but remains usable for stats and toggling.
func BuildPolicyEngine(cfg *config.Config, logger *slog.Logger) *filtering.PolicyEngine {
	if cfg == nil {
		return nil
	}

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

	refreshInterval := 24 * time.Hour
	if cfg.Filtering.RefreshInterval != "" {
		if d, err := time.ParseDuration(cfg.Filtering.RefreshInterval); err == nil {
			refreshInterval = d
		}
	}

	return filtering.NewPolicyEngine(filtering.PolicyEngineConfig{
		Logger:           logger,
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
