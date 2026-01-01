// Package api provides the REST management API for HydraDNS.
// It exposes endpoints for health checks, statistics, configuration,
// zone management, and domain filtering control via a Gin-based HTTP server.
package api

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jroosing/hydradns/internal/api/handlers"
	"github.com/jroosing/hydradns/internal/api/middleware"
	"github.com/jroosing/hydradns/internal/config"
)

// Server is the management REST API server.
//
// This is scaffolding: endpoints are present but most write operations are stubbed.
// Wire this into cmd/hydradns (or internal/server.Runner) when you want it running.
//
// Security note: do not expose the API to untrusted networks without authentication.
type Server struct {
	cfg        *config.Config
	logger     *slog.Logger
	engine     *gin.Engine
	httpServer *http.Server
}

func New(cfg *config.Config, logger *slog.Logger) *Server {
	if cfg == nil {
		panic("api.New: cfg is nil")
	}

	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(middleware.SlogRequestLogger(logger))

	h := handlers.New(cfg, logger)
	RegisterRoutes(engine, h, cfg)

	addr := net.JoinHostPort(cfg.API.Host, strconv.Itoa(cfg.API.Port))
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return &Server{cfg: cfg, logger: logger, engine: engine, httpServer: httpServer}
}

func (s *Server) Addr() string {
	if s.httpServer == nil {
		return ""
	}
	return s.httpServer.Addr
}

func (s *Server) Engine() *gin.Engine {
	return s.engine
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
