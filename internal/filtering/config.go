package filtering

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config represents the filtering configuration for HydraDNS.
type Config struct {
	// Enabled determines if DNS filtering is active.
	Enabled bool `yaml:"enabled"`

	// Whitelist contains domains and sources that should always be allowed.
	Whitelist ListConfig `yaml:"whitelist"`

	// Blacklist contains domains and sources that should be blocked.
	Blacklist ListConfig `yaml:"blacklist"`

	// BlockResponse configures how blocked queries are answered.
	BlockResponse BlockResponseConfig `yaml:"block_response"`

	// Logging configures filtering-related logging.
	Logging FilterLoggingConfig `yaml:"logging"`

	// Refresh configures automatic blocklist updates.
	Refresh RefreshConfig `yaml:"refresh"`
}

// ListConfig contains domain lists and sources for filtering.
type ListConfig struct {
	// Domains is a list of domains to include.
	Domains []string `yaml:"domains"`

	// Sources is a list of remote blocklists to fetch.
	Sources []SourceConfig `yaml:"sources"`
}

// SourceConfig represents a remote blocklist source.
type SourceConfig struct {
	// Name is a friendly name for the source.
	Name string `yaml:"name"`

	// URL is the URL to fetch the blocklist from.
	URL string `yaml:"url"`

	// Format specifies the blocklist format (auto, domains, hosts, adblock).
	Format string `yaml:"format"`
}

// BlockResponseConfig configures how blocked queries are answered.
type BlockResponseConfig struct {
	// Type is the response type: "nxdomain", "nodata", or "address".
	Type string `yaml:"type"`

	// IPv4 is the IPv4 address to return for "address" type (for A queries).
	IPv4 string `yaml:"ipv4"`

	// IPv6 is the IPv6 address to return for "address" type (for AAAA queries).
	IPv6 string `yaml:"ipv6"`
}

// FilterLoggingConfig configures filtering-related logging.
type FilterLoggingConfig struct {
	// LogBlocked enables logging of blocked queries.
	LogBlocked bool `yaml:"log_blocked"`

	// LogAllowed enables logging of allowed queries (verbose).
	LogAllowed bool `yaml:"log_allowed"`
}

// RefreshConfig configures automatic blocklist updates.
type RefreshConfig struct {
	// Enabled determines if automatic refresh is active.
	Enabled bool `yaml:"enabled"`

	// Interval is how often to refresh blocklists.
	Interval time.Duration `yaml:"interval"`
}

// DefaultConfig returns the default filtering configuration.
func DefaultConfig() Config {
	return Config{
		Enabled: false, // Disabled by default, user must opt-in
		BlockResponse: BlockResponseConfig{
			Type: "nxdomain",
			IPv4: "0.0.0.0",
			IPv6: "::",
		},
		Logging: FilterLoggingConfig{
			LogBlocked: true,
			LogAllowed: false,
		},
		Refresh: RefreshConfig{
			Enabled:  true,
			Interval: 24 * time.Hour,
		},
	}
}

// Validate validates the configuration and returns an error if invalid.
func (c *Config) Validate() error {
	// Validate block response type
	switch c.BlockResponse.Type {
	case "", "nxdomain", "nodata", "address":
		// valid
	default:
		return fmt.Errorf("invalid block_response.type: %q (must be nxdomain, nodata, or address)", c.BlockResponse.Type)
	}

	// Validate sources
	for i, source := range c.Whitelist.Sources {
		if err := source.Validate(); err != nil {
			return fmt.Errorf("whitelist.sources[%d]: %w", i, err)
		}
	}

	for i, source := range c.Blacklist.Sources {
		if err := source.Validate(); err != nil {
			return fmt.Errorf("blacklist.sources[%d]: %w", i, err)
		}
	}

	return nil
}

// Validate validates a source configuration.
func (s *SourceConfig) Validate() error {
	if s.URL == "" {
		return fmt.Errorf("url is required")
	}

	switch strings.ToLower(s.Format) {
	case "", "auto", "domains", "hosts", "adblock":
		// valid
	default:
		return fmt.Errorf("invalid format: %q (must be auto, domains, hosts, or adblock)", s.Format)
	}

	return nil
}

// ToListFormat converts the format string to a ListFormat.
func (s *SourceConfig) ToListFormat() ListFormat {
	switch strings.ToLower(s.Format) {
	case "domains":
		return FormatDomains
	case "hosts":
		return FormatHosts
	case "adblock":
		return FormatAdblock
	default:
		return FormatAuto
	}
}

// ToPolicyEngineConfig converts the Config to a PolicyEngineConfig.
func (c *Config) ToPolicyEngineConfig() PolicyEngineConfig {
	blockAction := ActionBlock

	cfg := PolicyEngineConfig{
		Enabled:          c.Enabled,
		BlockAction:      blockAction,
		LogBlocked:       c.Logging.LogBlocked,
		LogAllowed:       c.Logging.LogAllowed,
		WhitelistDomains: c.Whitelist.Domains,
		BlacklistDomains: c.Blacklist.Domains,
		BlocklistURLs:    make([]BlocklistURL, 0),
	}

	// Add blacklist sources
	for _, source := range c.Blacklist.Sources {
		cfg.BlocklistURLs = append(cfg.BlocklistURLs, BlocklistURL{
			Name:   source.Name,
			URL:    source.URL,
			Format: source.ToListFormat(),
		})
	}

	// Add refresh interval if enabled
	if c.Refresh.Enabled && c.Refresh.Interval > 0 {
		cfg.RefreshInterval = c.Refresh.Interval
	}

	return cfg
}

// ConfigFromEnv creates a Config from environment variables.
// This allows overriding YAML configuration via environment.
func ConfigFromEnv(base Config) Config {
	cfg := base

	// HYDRADNS_FILTERING_ENABLED
	if v := os.Getenv("HYDRADNS_FILTERING_ENABLED"); v != "" {
		cfg.Enabled = strings.EqualFold(v, "true") || v == "1"
	}

	// HYDRADNS_FILTERING_LOG_BLOCKED
	if v := os.Getenv("HYDRADNS_FILTERING_LOG_BLOCKED"); v != "" {
		cfg.Logging.LogBlocked = strings.EqualFold(v, "true") || v == "1"
	}

	// HYDRADNS_FILTERING_LOG_ALLOWED
	if v := os.Getenv("HYDRADNS_FILTERING_LOG_ALLOWED"); v != "" {
		cfg.Logging.LogAllowed = strings.EqualFold(v, "true") || v == "1"
	}

	// HYDRADNS_FILTERING_BLOCK_TYPE
	if v := os.Getenv("HYDRADNS_FILTERING_BLOCK_TYPE"); v != "" {
		cfg.BlockResponse.Type = v
	}

	return cfg
}

// ExampleConfig returns an example configuration for documentation.
func ExampleConfig() Config {
	return Config{
		Enabled: true,
		Whitelist: ListConfig{
			Domains: []string{
				"example.com",
				"safe.example.org",
			},
		},
		Blacklist: ListConfig{
			Domains: []string{
				"malware.example.com",
				"ads.example.net",
			},
			Sources: []SourceConfig{
				{
					Name:   "hagezi-light",
					URL:    "https://cdn.jsdelivr.net/gh/hagezi/dns-blocklists@latest/domains/light.txt",
					Format: "domains",
				},
				{
					Name:   "hagezi-adblock",
					URL:    "https://cdn.jsdelivr.net/gh/hagezi/dns-blocklists@latest/adblock/light.txt",
					Format: "adblock",
				},
				{
					Name:   "stevenblack",
					URL:    "https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts",
					Format: "hosts",
				},
			},
		},
		BlockResponse: BlockResponseConfig{
			Type: "nxdomain",
		},
		Logging: FilterLoggingConfig{
			LogBlocked: true,
			LogAllowed: false,
		},
		Refresh: RefreshConfig{
			Enabled:  true,
			Interval: 24 * time.Hour,
		},
	}
}
