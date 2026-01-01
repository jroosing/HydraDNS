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
	"github.com/jroosing/hydradns/internal/config"
	"github.com/jroosing/hydradns/internal/logging"
	"github.com/jroosing/hydradns/internal/server"
)

func main() {
	var (
		configPath = flag.String("config", "", "Path to YAML configuration file (or set HYDRADNS_CONFIG)")
		host       = flag.String("host", "", "Override bind host")
		port       = flag.Int("port", 0, "Override bind port")
		workers    = flag.Int("workers", -1, "Clamp GOMAXPROCS (can only reduce; -1 means default/auto)")
		noTCP      = flag.Bool("no-tcp", false, "Disable TCP server")
		jsonLogs   = flag.Bool("json-logs", false, "Enable JSON structured logging")
		debug      = flag.Bool("debug", false, "Enable debug logging")
	)
	flag.Parse()

	cfg, err := config.Load(config.ResolveConfigPath(*configPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	if *host != "" {
		cfg.Server.Host = *host
	}
	if *port != 0 {
		cfg.Server.Port = *port
	}
	if *workers >= 0 {
		cfg.Server.Workers = config.WorkerSetting{Mode: config.WorkersFixed, Value: *workers}
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

	var apiSrv *api.Server
	if cfg.API.Enabled {
		apiSrv = api.New(cfg, logger)
		logger.Info("management API starting", "addr", apiSrv.Addr())

		go func() {
			err := apiSrv.ListenAndServe()
			if err == nil || errors.Is(err, http.ErrServerClosed) {
				return
			}
			logger.Error("management API error", "err", err)
			cancel()
		}()
	}

	runner := server.NewRunner(logger)
	err = runner.RunWithContext(ctx, cfg)

	if apiSrv != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = apiSrv.Shutdown(shutdownCtx)
		shutdownCancel()
		logger.Info("management API stopped")
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "server exited with error: %v\n", err)
		os.Exit(1)
	}
}
