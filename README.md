
# HydraDNS

A high-performance, production-ready DNS server written in Go.

HydraDNS is designed for speed, reliability, and ease of deployment. It supports both authoritative zone serving and recursive forwarding with intelligent caching.

## Features

### Protocol Support
- **UDP + TCP** — Full RFC 1035 compliance
- **EDNS0** — Larger UDP payloads up to 4096 bytes (RFC 6891)
- **Automatic TCP fallback** — Retries truncated UDP responses over TCP

### Performance
- **Concurrent I/O** — Goroutines with non-blocking socket operations
- **Buffer pooling** — Reuses memory allocations for reduced GC pressure
- **Singleflight deduplication** — Prevents thundering herd on cache misses
- **O(1) zone lookups** — Indexed zone data for fast authoritative responses

### Caching
- **TTL-aware LRU cache** — Respects DNS record TTLs with configurable caps
- **Negative caching** — Caches NXDOMAIN and NODATA responses (RFC 2308)
- **SERVFAIL caching** — Short-term caching of upstream failures

### Security
- **3-tier rate limiting** — Global, per-prefix (/24), and per-IP token buckets
- **Domain filtering** — Trie-based whitelist/blacklist with remote blocklist support
- **Response validation** — Verifies upstream responses match requests
- **Hardened Docker deployment** — Non-root user, minimal attack surface

### Operations
- **Zone file support** — Standard RFC 1035 master file format
- **Strict-order failover** — Primary upstream with automatic fallback
- **Structured logging** — JSON or key-value format for log aggregation
- **Graceful shutdown** — Drains in-flight requests before stopping

### Management API
- **REST API** — Gin-based HTTP API for runtime configuration
- **OpenAPI/Swagger** — Interactive API documentation at `/swagger/`
- **Runtime control** — Toggle filtering, add domains, view stats without restart
- **API key auth** — Optional header-based authentication

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         HydraDNS                                │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐    ┌─────────────┐                             │
│  │ UDP Server  │    │ TCP Server  │     Transport Layer         │
│  │ (pooled buf)│    │ (pipelined) │                             │
│  └──────┬──────┘    └──────┬──────┘                             │
│         │                  │                                    │
│         └────────┬─────────┘                                    │
│                  ▼                                              │
│         ┌────────────────┐                                      │
│         │  Rate Limiter  │          Pre-parse Protection        │
│         │ (token bucket) │                                      │
│         └───────┬────────┘                                      │
│                 ▼                                               │
│         ┌────────────────┐                                      │
│         │ Query Handler  │          Parse + Dispatch            │
│         └───────┬────────┘                                      │
│                 ▼                                               │
│         ┌────────────────┐                                      │
│         │ Chained        │          Resolver Chain              │
│         │ Resolver       │                                      │
│         │  ├─ Filtering  │◄─── Domain whitelist/blacklist       │
│         │  ├─ Zone       │◄─── Local authoritative zones        │
│         │  └─ Forwarding │◄─── Upstream DNS servers             │
│         └───────┬────────┘                                      │
│                 │                                               │
│    ┌────────────┴────────────┐                                  │
│    ▼                         ▼                                  │
│  ┌──────────┐        ┌──────────────┐                           │
│  │  Cache   │        │ Singleflight │   Deduplication           │
│  │ (LRU+TTL)│        │   Groups     │                           │
│  └──────────┘        └──────────────┘                           │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                    REST API (Gin)                       │    │
│  │  /api/v1/health, /config, /zones, /filtering, /stats   │    │
│  │  Swagger UI: /swagger/                                  │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Component Overview

| Component | Location | Purpose |
|-----------|----------|---------|
| **UDP Server** | `internal/server/udp_server.go` | Handles UDP queries with buffer pooling and concurrency control |
| **TCP Server** | `internal/server/tcp_server.go` | Handles TCP queries with connection pipelining and per-IP limits |
| **Rate Limiter** | `internal/server/rate_limit.go` | 3-tier token bucket rate limiting (global → /24 prefix → IP) |
| **Query Handler** | `internal/server/query_handler.go` | Parses requests, dispatches to resolvers, handles timeouts |
| **Chained Resolver** | `internal/resolvers/chained.go` | Tries resolvers in order (filtering → zone → forwarding) |
| **Filtering Resolver** | `internal/resolvers/filtering_resolver.go` | Domain filtering with whitelist/blacklist support |
| **Zone Resolver** | `internal/resolvers/zone_resolver.go` | Serves authoritative responses from loaded zones |
| **Forwarding Resolver** | `internal/resolvers/forwarding_resolver.go` | Forwards to upstream servers with caching and failover |
| **Cache** | `internal/resolvers/cache.go` | TTL-aware LRU cache with negative caching support |
| **DNS Codec** | `internal/dns/` | Wire format parsing and serialization |
| **Zone Parser** | `internal/zone/` | RFC 1035 master file format parser |
| **REST API** | `internal/api/` | HTTP management API with Swagger docs |

### Request Flow

1. **Receive** — UDP/TCP server receives DNS query
2. **Rate Limit** — Token bucket check (drops if exceeded)
3. **Parse** — Decode DNS wire format into structured packet
4. **Resolve** — Try resolvers in chain order:
   - **Zone Resolver**: Check local zones (O(1) indexed lookup)
   - **Forwarding Resolver**: Check cache → singleflight → upstream
5. **Respond** — Serialize and send response (truncate for UDP if needed)

---

## Requirements

- Go 1.24+

---

## Quick Start

```bash
# Run tests
go test ./...

# Start server with default config
go run ./cmd/hydradns

# Start with explicit config
go run ./cmd/hydradns --config hydradns.example.yaml

# Start with debug logging
go run ./cmd/hydradns --debug
```

---

## Configuration

HydraDNS uses YAML configuration with the following priority:

1. Command-line arguments
2. YAML config file
3. Environment variables
4. Default values

### Example Configuration

```yaml
server:
  host: "0.0.0.0"
  port: 1053
  workers: auto
  enable_tcp: true
  tcp_fallback: true

upstream:
  servers:
    - "9.9.9.9"   # Primary: Quad9
    - "1.1.1.1"   # Fallback: Cloudflare
    - "8.8.8.8"   # Fallback: Google

zones:
  directory: "zones"

logging:
  level: "INFO"
  structured: false
```

See [hydradns.example.yaml](hydradns.example.yaml) for full configuration options.

### Command-Line Options

| Flag | Description |
|------|-------------|
| `--config` | Path to YAML config file |
| `--host` | Override bind address |
| `--port` | Override bind port |
| `--workers` | Set worker count (clamps GOMAXPROCS) |
| `--no-tcp` | Disable TCP server |
| `--json-logs` | Enable JSON structured logging |
| `--debug` | Enable debug logging |

---

## Tools

HydraDNS includes utility commands for debugging and testing:

```bash
# Print parsed zone file
go run ./cmd/print-zone --file zones/example.zone

# Query the server
go run ./cmd/dnsquery --server 127.0.0.1:1053 --name example.com --qtype 1

# Benchmark
go run ./cmd/bench --server 127.0.0.1:1053 --name example.com \
    --concurrency 200 --requests 20000
```

---

## Docker

```bash
# Build and run
docker compose up --build

# The container includes a health check using dnsquery
```

The Docker image runs as a non-root user with minimal dependencies.

---

## Zone Files

HydraDNS supports RFC 1035 master file format:

```zone
$ORIGIN example.com.
$TTL 3600

@       IN  SOA   ns1.example.com. admin.example.com. (
                  2024010101  ; Serial
                  3600        ; Refresh
                  900         ; Retry
                  604800      ; Expire
                  86400       ; Minimum TTL
                  )

@       IN  NS    ns1.example.com.
@       IN  A     192.0.2.1
www     IN  A     192.0.2.2
mail    IN  MX    10 mail.example.com.
```

Place zone files in the `zones/` directory (or configure a custom path).

---

## Performance Optimizations

HydraDNS includes several performance optimizations for high-throughput DNS serving:

- **Buffer pooling** — UDP receive buffers and TCP length prefixes are pooled
- **Capacity pre-allocation** — Slices are sized based on expected content
- **Indexed zone lookups** — Zone names are indexed for O(1) access
- **Singleflight** — Concurrent identical queries share a single upstream request
- **Two-write TCP** — Avoids allocation by writing length prefix and body separately

---

## Rate Limiting

HydraDNS implements 3-tier token bucket rate limiting to protect against abuse while allowing legitimate high-volume traffic. Rate limiting is applied **before** query parsing, minimizing CPU overhead for dropped requests.

### How It Works

Each incoming query must pass all three rate limit tiers:

1. **Global** — Total queries per second across all clients
2. **Prefix** — Queries per second from each /24 subnet (IPv4) or /48 (IPv6)
3. **Per-IP** — Queries per second from each individual IP address

The token bucket algorithm allows sustained throughput at the configured QPS rate, with burst capacity to absorb short spikes.

### Default Limits

| Tier | QPS | Burst | Purpose |
|------|-----|-------|--------|
| **Global** | 100,000 | 100,000 | Total server capacity |
| **Prefix** (/24) | 10,000 | 20,000 | Limit per subnet |
| **Per-IP** | 3,000 | 6,000 | Limit per client |

The default per-IP limit of **3,000 QPS** is suitable for home/small office use. Your actual measured throughput will be slightly lower than the configured QPS due to rate limiter overhead.

### Tuning for Higher Throughput

HydraDNS can handle significantly higher QPS with adjusted rate limits. For high-performance deployments:

```bash
# High-performance configuration (10k+ QPS per client)
export HYDRADNS_RL_IP_QPS=10000
export HYDRADNS_RL_IP_BURST=20000

# Or disable rate limiting entirely for internal/trusted networks
export HYDRADNS_RL_IP_QPS=0
```

**Recommended settings by use case:**

| Use Case | IP QPS | IP Burst | Notes |
|----------|--------|----------|-------|
| Home/Small Office | 3,000 | 6,000 | Default, good protection |
| Enterprise/Internal | 10,000 | 20,000 | Higher for trusted clients |
| Behind Load Balancer | 50,000+ | 100,000 | Single source IP for many clients |
| Development/Testing | 0 (disabled) | — | No limits for benchmarking |

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `HYDRADNS_RL_GLOBAL_QPS` | 100000 | Global queries per second limit |
| `HYDRADNS_RL_GLOBAL_BURST` | 100000 | Global burst capacity |
| `HYDRADNS_RL_PREFIX_QPS` | 10000 | Per /24 subnet QPS limit |
| `HYDRADNS_RL_PREFIX_BURST` | 20000 | Per /24 subnet burst capacity |
| `HYDRADNS_RL_IP_QPS` | 3000 | Per-IP QPS limit (0 = disabled) |
| `HYDRADNS_RL_IP_BURST` | 6000 | Per-IP burst capacity |
| `HYDRADNS_RL_MAX_IP_ENTRIES` | 65536 | Max tracked IP addresses |
| `HYDRADNS_RL_MAX_PREFIX_ENTRIES` | 16384 | Max tracked /24 prefixes |
| `HYDRADNS_RL_CLEANUP_SECONDS` | 60 | Stale entry cleanup interval |

### Docker Configuration

```yaml
# docker-compose.yml
services:
  hydradns:
    environment:
      - HYDRADNS_RL_IP_QPS=10000    # Increase for higher throughput
      - HYDRADNS_RL_IP_BURST=20000
```

### Performance Notes

- Rate limiting uses `netip.Addr` internally to avoid string allocations
- Token buckets are per-tier with O(1) lookup via map
- Stale entries are cleaned up periodically to bound memory usage
- When rate limited, queries are dropped before parsing (minimal CPU impact)

---

## Domain Filtering

HydraDNS includes a high-performance domain filtering system for ad blocking, malware protection, and custom access control. Filtering uses a trie-based data structure for O(k) lookups where k is the number of domain labels.

### Features

- **Whitelist priority** — Whitelisted domains always allowed, even if on blacklist
- **Wildcard subdomains** — Blocking `ads.example.com` also blocks `tracker.ads.example.com`
- **Multiple blocklist formats** — Adblock Plus, hosts file, and plain domain lists
- **Remote blocklists** — Fetch from URLs with automatic periodic refresh
- **~86ns lookups** — High-performance trie with 10,000+ domains

### Quick Start

Enable filtering with environment variables:

```bash
# Enable filtering with Hagezi blocklist
export HYDRADNS_FILTERING_ENABLED=true
export HYDRADNS_FILTERING_BLOCKLIST_URL="https://cdn.jsdelivr.net/gh/hagezi/dns-blocklists@latest/adblock/light.txt"

go run ./cmd/hydradns
```

### Configuration

```yaml
filtering:
  # Enable domain filtering (default: false)
  enabled: true

  # Log blocked queries (default: true)
  log_blocked: true

  # Custom whitelist (always allowed)
  whitelist_domains:
    - "safe.example.com"

  # Custom blacklist (always blocked)
  blacklist_domains:
    - "ads.example.com"
    - "tracker.example.com"

  # Remote blocklists
  blocklists:
    - name: "hagezi-light"
      url: "https://cdn.jsdelivr.net/gh/hagezi/dns-blocklists@latest/adblock/light.txt"
      format: "adblock"
    - name: "stevenblack"
      url: "https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts"
      format: "hosts"

  # Refresh interval for remote blocklists (default: 24h)
  refresh_interval: "24h"
```

### Supported Blocklist Formats

| Format | Description | Example |
|--------|-------------|---------|
| `adblock` | Adblock Plus format | `\|\|ads.example.com^` |
| `hosts` | Hosts file format | `0.0.0.0 ads.example.com` |
| `domains` | Plain domain list | `ads.example.com` |
| `auto` | Auto-detect format | — |

### Popular Blocklists

| List | Format | Description |
|------|--------|-------------|
| [Hagezi Light](https://cdn.jsdelivr.net/gh/hagezi/dns-blocklists@latest/adblock/light.txt) | adblock | Balanced blocking, minimal false positives |
| [Hagezi Normal](https://cdn.jsdelivr.net/gh/hagezi/dns-blocklists@latest/adblock/multi.txt) | adblock | More aggressive blocking |
| [StevenBlack](https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts) | hosts | Ads + malware |
| [OISD](https://abp.oisd.nl/) | adblock | Comprehensive blocking |

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `HYDRADNS_FILTERING_ENABLED` | `false` | Enable/disable filtering |
| `HYDRADNS_FILTERING_LOG_BLOCKED` | `true` | Log blocked queries |
| `HYDRADNS_FILTERING_WHITELIST` | — | Comma-separated whitelist domains |
| `HYDRADNS_FILTERING_BLACKLIST` | — | Comma-separated blacklist domains |
| `HYDRADNS_FILTERING_BLOCKLIST_URL` | — | Single blocklist URL (auto-detect format) |

### How It Works

1. **Query received** — Domain extracted from DNS question
2. **Whitelist check** — If domain matches whitelist, allow immediately
3. **Blacklist check** — If domain matches blacklist/blocklists, return NXDOMAIN
4. **Default allow** — Unmatched domains pass to resolver chain

The filtering resolver sits at the front of the resolver chain, before zone and forwarding resolvers.

---

## REST API

HydraDNS includes an optional REST API for runtime management and monitoring. The API is built with Gin and includes interactive Swagger documentation.

### Enabling the API

```yaml
api:
  enabled: true
  host: "127.0.0.1"  # Bind address (use 0.0.0.0 for all interfaces)
  port: 8080
  api_key: "your-secret-key"  # Optional, leave empty for no auth
```

Or via environment variables:

```bash
export HYDRADNS_API_ENABLED=true
export HYDRADNS_API_HOST=127.0.0.1
export HYDRADNS_API_PORT=8080
export HYDRADNS_API_KEY=your-secret-key
```

### Swagger UI

When the API is enabled, interactive documentation is available at:

```
http://localhost:8080/swagger/index.html
```

### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/health` | GET | Health check |
| `/api/v1/stats` | GET | Server statistics (uptime, memory, goroutines) |
| `/api/v1/config` | GET | Current configuration (sensitive fields redacted) |
| `/api/v1/zones` | GET | List all loaded zones |
| `/api/v1/zones/:name` | GET | Get zone details with records |
| `/api/v1/filtering/stats` | GET | Filtering statistics |
| `/api/v1/filtering/enabled` | PUT | Enable/disable filtering at runtime |
| `/api/v1/filtering/whitelist` | GET | List whitelist domains |
| `/api/v1/filtering/whitelist` | POST | Add domains to whitelist |
| `/api/v1/filtering/blacklist` | GET | List blacklist domains |
| `/api/v1/filtering/blacklist` | POST | Add domains to blacklist |

### Authentication

If `api_key` is configured, all requests must include the `X-API-Key` header:

```bash
curl -H "X-API-Key: your-secret-key" http://localhost:8080/api/v1/health
```

### Example Usage

```bash
# Health check
curl http://localhost:8080/api/v1/health

# Get server stats
curl -H "X-API-Key: secret" http://localhost:8080/api/v1/stats

# Add domains to blacklist
curl -X POST -H "X-API-Key: secret" -H "Content-Type: application/json" \
  -d '{"domains": ["ads.example.com", "tracker.example.com"]}' \
  http://localhost:8080/api/v1/filtering/blacklist

# Toggle filtering on/off
curl -X PUT -H "X-API-Key: secret" -H "Content-Type: application/json" \
  -d '{"enabled": false}' \
  http://localhost:8080/api/v1/filtering/enabled
```

### Generating API Docs

Swagger documentation is generated from source code annotations:

```bash
make docs
```

---

## License

See [LICENSE](LICENSE) for details.