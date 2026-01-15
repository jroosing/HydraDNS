package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jroosing/hydradns/internal/api"
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

func run() error {
	var (
		dbPath   = flag.String("db", DefaultDatabasePath, "Path to SQLite database file")
		host     = flag.String("host", "", "Override DNS server bind host")
		port     = flag.Int("port", 0, "Override DNS server bind port")
		workers  = flag.Int("workers", -1, "Clamp GOMAXPROCS (can only reduce; -1 means default/auto)")
		noTCP    = flag.Bool("no-tcp", false, "Disable TCP server")
		jsonLogs = flag.Bool("json-logs", false, "Enable JSON structured logging")
		debug    = flag.Bool("debug", false, "Enable debug logging")
	)
	flag.Parse()

	// Open database (creates with defaults if new)
	db, err := database.Open(*dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Export database config to config.Config for compatibility
	cfg, err := db.ExportToConfig()
	if err != nil {
		return fmt.Errorf("failed to load config from database: %w", err)
	}

	// Apply command-line overrides (these don't persist to database)
	if *host != "" {
		cfg.Server.Host = *host
	}
	if *port != 0 {
		cfg.Server.Port = *port
	}
	if *workers >= 0 {
		cfg.Server.Workers.Mode = 1 // WorkersFixed
		cfg.Server.Workers.Value = *workers
	}
	if *noTCP {
		cfg.Server.EnableTCP = false
	}
	if *jsonLogs {
		cfg.Logging.Structured = true
		cfg.Logging.StructuredFormat = "json"
	}
	if *debug {
		cfg.Logging.Level = "DEBUG"
	}

	logger := logging.Configure(logging.Config{
		Level:            cfg.Logging.Level,
		Structured:       cfg.Logging.Structured,
		StructuredFormat: cfg.Logging.StructuredFormat,
		IncludePID:       cfg.Logging.IncludePID,
		ExtraFields:      cfg.Logging.ExtraFields,
	})
	logger.Info("HydraDNS starting",
		"database", *dbPath,
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

	// API server is always enabled (web UI is mandatory)
	apiSrv := api.New(cfg, db, logger)
	logger.Info("web UI and API starting", "addr", apiSrv.Addr())

	go func() {
		serveErr := apiSrv.ListenAndServe()
		if serveErr == nil || errors.Is(serveErr, http.ErrServerClosed) {
			return
		}
		logger.Error("API server error", "err", serveErr)
		cancel()
	}()

	runner := server.NewRunner(logger)
	err = runner.RunWithContext(ctx, cfg)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	_ = apiSrv.Shutdown(shutdownCtx)
	shutdownCancel()
	logger.Info("web UI and API stopped")

	if err != nil {
		return fmt.Errorf("server exited with error: %w", err)
	}
	return nil
}
