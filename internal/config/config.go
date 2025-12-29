package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type WorkersMode int

const (
	WorkersAuto WorkersMode = iota
	WorkersFixed
)

type WorkerSetting struct {
	Mode  WorkersMode
	Value int
}

func (w WorkerSetting) String() string {
	if w.Mode == WorkersAuto {
		return "auto"
	}
	return strconv.Itoa(w.Value)
}

type ServerConfig struct {
	Host                   string        `yaml:"host"`
	Port                   int           `yaml:"port"`
	Workers                WorkerSetting `yaml:"-"`
	WorkersRaw             string        `yaml:"workers"`
	MaxConcurrency         int           `yaml:"max_concurrency"`
	UpstreamSocketPoolSize int           `yaml:"upstream_socket_pool_size"`
	EnableTCP              bool          `yaml:"enable_tcp"`
	TCPFallback            bool          `yaml:"tcp_fallback"`
}

type UpstreamConfig struct {
	Servers []string `yaml:"servers"`
}

type ZonesConfig struct {
	Directory string   `yaml:"directory"`
	Files     []string `yaml:"files"`
}

type LoggingConfig struct {
	Level            string            `yaml:"level"`
	Structured       bool              `yaml:"structured"`
	StructuredFormat string            `yaml:"structured_format"`
	IncludePID       bool              `yaml:"include_pid"`
	ExtraFields      map[string]string `yaml:"extra_fields"`
}

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Upstream UpstreamConfig `yaml:"upstream"`
	Zones    ZonesConfig    `yaml:"zones"`
	Logging  LoggingConfig  `yaml:"logging"`
}

func ResolveConfigPath(flagValue string) string {
	if strings.TrimSpace(flagValue) != "" {
		return flagValue
	}
	if v := strings.TrimSpace(os.Getenv("HYDRADNS_CONFIG")); v != "" {
		return v
	}
	return ""
}

func Load(path string) (*Config, error) {
	cfg := defaultConfig()
	if strings.TrimSpace(path) != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(b, cfg); err != nil {
			return nil, err
		}
	}

	applyEnvOverrides(cfg)
	if err := normalize(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:                   "0.0.0.0",
			Port:                   1053,
			Workers:                WorkerSetting{Mode: WorkersAuto},
			MaxConcurrency:         0,
			UpstreamSocketPoolSize: 0,
			EnableTCP:              true,
			TCPFallback:            true,
		},
		Upstream: UpstreamConfig{Servers: []string{"8.8.8.8"}},
		Zones: ZonesConfig{
			Directory: "zones",
			Files:     nil,
		},
		Logging: LoggingConfig{
			Level:            "INFO",
			Structured:       false,
			StructuredFormat: "json",
			IncludePID:       false,
			ExtraFields:      map[string]string{},
		},
	}
}

func applyEnvOverrides(cfg *Config) {
	if v := strings.TrimSpace(os.Getenv("HYDRADNS_HOST")); v != "" {
		cfg.Server.Host = v
	}
	if v := strings.TrimSpace(os.Getenv("HYDRADNS_PORT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("HYDRADNS_WORKERS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Server.Workers = WorkerSetting{Mode: WorkersFixed, Value: n}
		}
	}
	if v := strings.TrimSpace(os.Getenv("HYDRADNS_UPSTREAM_SERVERS")); v != "" {
		parts := strings.Split(v, ",")
		servers := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			// allow host:port but ignore port (python forces 53)
			host := p
			if h, _, ok := strings.Cut(p, ":"); ok {
				host = h
			}
			servers = append(servers, host)
		}
		if len(servers) > 0 {
			cfg.Upstream.Servers = servers
		}
	}
	if v := strings.TrimSpace(os.Getenv("HYDRADNS_ZONES_DIR")); v != "" {
		cfg.Zones.Directory = v
	}
	if v := strings.TrimSpace(os.Getenv("HYDRADNS_ENABLE_TCP")); v != "" {
		cfg.Server.EnableTCP = envBool(v, cfg.Server.EnableTCP)
	}
	if v := strings.TrimSpace(os.Getenv("HYDRADNS_TCP_FALLBACK")); v != "" {
		cfg.Server.TCPFallback = envBool(v, cfg.Server.TCPFallback)
	}
	if v := strings.TrimSpace(os.Getenv("LOG_LEVEL")); v != "" {
		cfg.Logging.Level = strings.ToUpper(v)
	}
}

func normalize(cfg *Config) error {
	if strings.TrimSpace(cfg.Server.WorkersRaw) != "" && cfg.Server.Workers.Mode == WorkersAuto {
		w := strings.TrimSpace(cfg.Server.WorkersRaw)
		if w == "auto" {
			cfg.Server.Workers = WorkerSetting{Mode: WorkersAuto}
		} else {
			n, err := strconv.Atoi(w)
			if err != nil {
				return fmt.Errorf("invalid server.workers: %q", w)
			}
			cfg.Server.Workers = WorkerSetting{Mode: WorkersFixed, Value: n}
		}
	}
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return errors.New("server.port must be 1..65535")
	}
	if len(cfg.Upstream.Servers) == 0 {
		cfg.Upstream.Servers = []string{"8.8.8.8"}
	}
	// match python: max 3 strict-order failover
	if len(cfg.Upstream.Servers) > 3 {
		cfg.Upstream.Servers = cfg.Upstream.Servers[:3]
	}
	if cfg.Zones.Directory == "" {
		cfg.Zones.Directory = "zones"
	}
	cfg.Zones.Directory = filepath.Clean(cfg.Zones.Directory)
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "INFO"
	}
	if cfg.Logging.StructuredFormat == "" {
		cfg.Logging.StructuredFormat = "json"
	}
	return nil
}

func envBool(raw string, def bool) bool {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return def
	}
}
