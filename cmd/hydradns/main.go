package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jroosing/hydradns/internal/api"
	"github.com/jroosing/hydradns/internal/api/handlers"
	"github.com/jroosing/hydradns/internal/cluster"
	"github.com/jroosing/hydradns/internal/config"
	"github.com/jroosing/hydradns/internal/database"
	"github.com/jroosing/hydradns/internal/logging"
	"github.com/jroosing/hydradns/internal/server"
)

const (
	// DefaultDatabasePath is the default location for the HydraDNS database.
	DefaultDatabasePath = "hydradns.db"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// cliFlags holds parsed command-line flag values.
type cliFlags struct {
	dbPath         string
	host           string
	port           int
	workers        int
	noTCP          bool
	jsonLogs       bool
	debug          bool
	clusterMode    string
	clusterPrimary string
	clusterSecret  string
	clusterNodeID  string
}

// parseFlags parses command-line flags and returns the values.
func parseFlags() cliFlags {
	var f cliFlags
	flag.StringVar(&f.dbPath, "db", DefaultDatabasePath, "Path to SQLite database file")
	flag.StringVar(&f.host, "host", "", "Override DNS server bind host")
	flag.IntVar(&f.port, "port", 0, "Override DNS server bind port")
	flag.IntVar(&f.workers, "workers", -1, "Clamp GOMAXPROCS (can only reduce; -1 means default/auto)")
	flag.BoolVar(&f.noTCP, "no-tcp", false, "Disable TCP server")
	flag.BoolVar(&f.jsonLogs, "json-logs", false, "Enable JSON structured logging")
	flag.BoolVar(&f.debug, "debug", false, "Enable debug logging")
	flag.StringVar(&f.clusterMode, "cluster-mode", "", "Cluster mode: standalone, primary, or secondary")
	flag.StringVar(&f.clusterPrimary, "cluster-primary", "", "Primary node URL for secondary mode")
	flag.StringVar(&f.clusterSecret, "cluster-secret", "", "Shared secret for cluster authentication")
	flag.StringVar(&f.clusterNodeID, "cluster-node-id", "", "Unique node ID (auto-generated if empty)")
	flag.Parse()
	return f
}

// applyCLIOverrides applies command-line overrides to the config.
func applyCLIOverrides(cfg *config.Config, f cliFlags) {
	if f.host != "" {
		cfg.Server.Host = f.host
	}
	if f.port != 0 {
		cfg.Server.Port = f.port
	}
	if f.workers >= 0 {
		cfg.Server.Workers.Mode = 1 // WorkersFixed
		cfg.Server.Workers.Value = f.workers
	}
	if f.noTCP {
		cfg.Server.EnableTCP = false
	}
	if f.jsonLogs {
		cfg.Logging.Structured = true
		cfg.Logging.StructuredFormat = "json"
	}
	if f.debug {
		cfg.Logging.Level = "DEBUG"
	}
	if f.clusterMode != "" {
		cfg.Cluster.Mode = config.ClusterMode(f.clusterMode)
	}
	if f.clusterPrimary != "" {
		cfg.Cluster.PrimaryURL = f.clusterPrimary
	}
	if f.clusterSecret != "" {
		cfg.Cluster.SharedSecret = f.clusterSecret
	}
	if f.clusterNodeID != "" {
		cfg.Cluster.NodeID = f.clusterNodeID
	}
	if cfg.Cluster.NodeID == "" {
		cfg.Cluster.NodeID = uuid.New().String()[:8]
	}
}

func run() error {
	flags := parseFlags()

	// Open database (creates with defaults if new)
	db, err := database.Open(flags.dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Export database config to config.Config for compatibility
	cfg, err := db.ExportToConfig(context.Background())
	if err != nil {
		return fmt.Errorf("failed to load config from database: %w", err)
	}

	// Apply command-line overrides (these don't persist to database)
	applyCLIOverrides(cfg, flags)

	logger := logging.Configure(logging.Config{
		Level:            cfg.Logging.Level,
		Structured:       cfg.Logging.Structured,
		StructuredFormat: cfg.Logging.StructuredFormat,
		IncludePID:       cfg.Logging.IncludePID,
		ExtraFields:      cfg.Logging.ExtraFields,
	})
	logger.Info("HydraDNS starting",
		"database", flags.dbPath,
		"host", cfg.Server.Host,
		"port", cfg.Server.Port,
		"workers", cfg.Server.Workers.String(),
		"tcp", cfg.Server.EnableTCP,
	)
	logger.Info("rate limits", "effective", server.FormatRateLimitsLog(server.RateLimitSettings{
		CleanupSeconds:   cfg.RateLimit.CleanupSeconds,
		MaxIPEntries:     cfg.RateLimit.MaxIPEntries,
		MaxPrefixEntries: cfg.RateLimit.MaxPrefixEntries,
		GlobalQPS:        cfg.RateLimit.GlobalQPS,
		GlobalBurst:      cfg.RateLimit.GlobalBurst,
		PrefixQPS:        cfg.RateLimit.PrefixQPS,
		PrefixBurst:      cfg.RateLimit.PrefixBurst,
		IPQPS:            cfg.RateLimit.IPQPS,
		IPBurst:          cfg.RateLimit.IPBurst,
	}))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Build shared filtering policy engine (even if disabled) for API + DNS path
	policy := server.BuildPolicyEngine(cfg, logger)

	// Create runner early to get DNS stats collector
	runner := server.NewRunner(logger)
	runner.SetPolicyEngine(policy)

	// API server is always enabled (web UI is mandatory)
	apiSrv := api.New(cfg, db, logger)
	apiSrv.Handler().SetPolicyEngine(policy)

	// Wire DNS stats from runner to API handler
	dnsStats := runner.DNSStats()
	apiSrv.Handler().SetDNSStatsFunc(func() handlers.DNSStatsSnapshot {
		snapshot := dnsStats.Snapshot()
		return handlers.DNSStatsSnapshot{
			QueriesTotal: snapshot.QueriesTotal,
			QueriesUDP:   snapshot.QueriesUDP,
			QueriesTCP:   snapshot.QueriesTCP,
			ResponsesNX:  snapshot.ResponsesNX,
			ResponsesErr: snapshot.ResponsesErr,
			AvgLatencyMs: snapshot.AvgLatencyMs,
		}
	})

	logger.Info("web UI and API starting", "addr", apiSrv.Addr())

	go func() {
		serveErr := apiSrv.ListenAndServe()
		if serveErr == nil || errors.Is(serveErr, http.ErrServerClosed) {
			return
		}
		logger.Error("API server error", "err", serveErr)
		cancel()
	}()

	// Start cluster syncer if in secondary mode
	var syncer *cluster.Syncer
	if cfg.Cluster.Mode == config.ClusterModeSecondary {
		syncer = startClusterSyncer(ctx, cfg, db, logger, apiSrv.Handler())
	} else if cfg.Cluster.Mode != "" && cfg.Cluster.Mode != config.ClusterModeStandalone {
		logger.Info("cluster mode", "mode", cfg.Cluster.Mode, "node_id", cfg.Cluster.NodeID)
	}

	err = runner.RunWithContext(ctx, cfg)

	// Stop cluster syncer if running
	if syncer != nil {
		syncer.Stop()
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	_ = apiSrv.Shutdown(shutdownCtx)
	shutdownCancel()
	logger.Info("web UI and API stopped")

	if err != nil {
		return fmt.Errorf("server exited with error: %w", err)
	}
	return nil
}

// startClusterSyncer initializes and starts the cluster syncer for secondary mode.
func startClusterSyncer(
	ctx context.Context,
	cfg *config.Config,
	db *database.DB,
	logger *slog.Logger,
	h *handlers.Handler,
) *cluster.Syncer {
	logger.InfoContext(ctx, "starting cluster syncer",
		"primary_url", cfg.Cluster.PrimaryURL,
		"node_id", cfg.Cluster.NodeID,
		"sync_interval", cfg.Cluster.SyncInterval,
	)

	// Import function: imports config from primary into local database
	importFunc := func(data *cluster.ExportData) error {
		if err := db.ImportFromCluster(ctx, data); err != nil {
			return err
		}
		// Update local version to match primary
		return db.SetVersion(ctx, data.Version)
	}

	// Reload function: refreshes runtime components after config import
	reloadFunc := func() error {
		// TODO: Trigger filtering engine reload, custom DNS reload, etc.
		// For now, this is a no-op; full reload requires server restart
		logger.DebugContext(ctx, "config imported, runtime reload pending")
		return nil
	}

	// Version function: returns local config version
	versionFunc := func() (int64, error) {
		return db.GetVersion(ctx)
	}

	syncer, err := cluster.NewSyncer(&cfg.Cluster, logger, importFunc, reloadFunc, versionFunc)
	if err != nil {
		logger.ErrorContext(ctx, "failed to create cluster syncer", "err", err)
		return nil
	}

	// Set syncer on handler for API access
	h.SetClusterSyncer(syncer)

	if err := syncer.Start(ctx); err != nil {
		logger.ErrorContext(ctx, "failed to start cluster syncer", "err", err)
		return nil
	}

	return syncer
}
