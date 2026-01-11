package models

import "github.com/jroosing/hydradns/internal/config"

// APIConfigResponse is a redacted version of APIConfig (no api_key exposed).
type APIConfigResponse struct {
	Enabled bool   `json:"enabled"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
}

// ServerConfigResponse wraps ServerConfig with workers as string.
type ServerConfigResponse struct {
	Host                   string `json:"host"`
	Port                   int    `json:"port"`
	Workers                string `json:"workers"`
	MaxConcurrency         int    `json:"max_concurrency"`
	UpstreamSocketPoolSize int    `json:"upstream_socket_pool_size"`
	EnableTCP              bool   `json:"enable_tcp"`
	TCPFallback            bool   `json:"tcp_fallback"`
}

// ConfigResponse is the API response for GET /config.
type ConfigResponse struct {
	Server    ServerConfigResponse   `json:"server"`
	Upstream  config.UpstreamConfig  `json:"upstream"`
	CustomDNS config.CustomDNSConfig `json:"custom_dns"`
	Logging   config.LoggingConfig   `json:"logging"`
	Filtering config.FilteringConfig `json:"filtering"`
	RateLimit config.RateLimitConfig `json:"rate_limit"`
	API       APIConfigResponse      `json:"api"`
}
