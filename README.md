
# HydraDNS

A high-performance, production-ready DNS server written in Go.

HydraDNS is designed for speed, reliability, and ease of deployment. It forwards DNS queries with intelligent caching and can answer simple custom A/AAAA/CNAME records for homelab-style hostnames.

> **Development status:** HydraDNS is still under active development and not yet ready for production or real-world use. Expect breaking changes and incomplete features.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.25%2B-blue.svg)](https://golang.org)
[![Made for Homelabs](https://img.shields.io/badge/Made%20for-Homelabs-orange.svg)](https://github.com)

## Features

### Protocol Support
- **UDP + TCP** — Full RFC 1035 compliance
- **EDNS0** — Larger UDP payloads up to 4096 bytes (RFC 6891)
- **Automatic TCP fallback** — Retries truncated UDP responses over TCP

### Performance
- **Concurrent I/O** — Goroutines with non-blocking socket operations
- **Buffer pooling** — Reuses memory allocations for reduced GC pressure
- **Singleflight deduplication** — Prevents thundering herd on cache misses
- **O(1) custom DNS lookups** — Indexed host mappings for fast local responses

### Caching
- **TTL-aware LRU cache** — Respects DNS record TTLs with configurable caps
- **Negative caching** — Caches NXDOMAIN and NODATA responses (RFC 2308)
- **SERVFAIL caching** — Short-term caching of upstream failures

### Security
- **3-tier rate limiting** — Global, per-prefix (/24), and per-IP token buckets
- **Domain filtering** — Trie-based whitelist/blacklist with remote blocklist support
- **Response validation** — Verifies upstream responses match requests
- **Hardened Docker deployment** — Non-root user, minimal attack surface

### Configuration & Management
- **SQLite database** — All configuration stored in a single database file
- **Web UI** — Built-in Angular-based management interface
- **REST API** — Gin-based HTTP API for runtime configuration
- **OpenAPI/Swagger** — Interactive API documentation at `/swagger/`
- **Zero-config startup** — Sensible defaults, just run the binary

### Operations
- **Custom DNS** — Simple hosts/CNAME configuration (dnsmasq-style)
- **Strict-order failover** — Primary upstream with automatic fallback
- **Structured logging** — JSON or key-value format for log aggregation
- **Graceful shutdown** — Drains in-flight requests before stopping

### DNS API
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
│         │  ├─ Custom DNS │◄─── Database hosts/CNAME records     │
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
│  │                    Web UI + REST API                    │    │
│  │  /api/v1/health, /config, /custom-dns, /filtering       │    │
│  │  Swagger UI: /swagger/                                  │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                    SQLite Database                      │    │
│  │  Configuration, Custom DNS, Filtering Rules             │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Request Flow:**
1. **Receive** — UDP/TCP packet arrives at transport layer
2. **Rate limit** — Token bucket check before parsing
3. **Parse** — Decode DNS wire format into structured packet
4. **Resolve** — Try resolvers in chain order:
  - **Custom DNS Resolver**: Check database-defined hosts and CNAMEs
  - **Forwarding Resolver**: Check cache → singleflight → upstream
5. **Respond** — Serialize and send response (truncate for UDP if needed)

---

## Requirements

- Go 1.25+

---

## Quick Start

```bash
# Run tests
go test ./...

# Start server (creates hydradns.db with defaults on first run via migrations)
go run ./cmd/hydradns

# Start with custom database path
go run ./cmd/hydradns --db /path/to/hydradns.db

# Start with debug logging
go run ./cmd/hydradns --debug

# Override DNS server settings (runtime only, not persisted)
go run ./cmd/hydradns --host 0.0.0.0 --port 53
```

On first run, HydraDNS creates a SQLite database (`hydradns.db`) with sensible defaults seeded by the migrations:
- DNS server: `0.0.0.0:1053` (UDP + TCP)
- Upstream servers: `9.9.9.9`, `1.1.1.1`, `8.8.8.8`
- Web UI + API: `0.0.0.0:8080`
- Filtering: Disabled by default

Access the web UI at **http://localhost:8080** to configure everything else.

---

## Configuration

HydraDNS stores all configuration in a SQLite database file. On first startup, the migrations create the schema and seed sensible defaults.

### Database Location

By default, the database is created as `hydradns.db` in the current directory. Override with:

```bash
# Command-line flag
./hydradns --db /var/lib/hydradns/config.db

# Docker volume mount
docker run -v hydradns-data:/data hydradns --db /data/hydradns.db
```

### Default Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| DNS Host | `0.0.0.0` | Bind address for DNS |
| DNS Port | `1053` | DNS port (UDP + TCP) |
| Workers | `auto` | Number of worker goroutines |
| TCP Enabled | `true` | Enable TCP server |
| TCP Fallback | `true` | Retry truncated responses over TCP |
| Upstream Servers | `9.9.9.9, 1.1.1.1, 8.8.8.8` | DNS forwarders |
| API Host | `0.0.0.0` | Web UI/API bind address |
| API Port | `8080` | Web UI/API port |
| Filtering | `false` | Domain filtering disabled |

### Command-Line Options

| Flag | Description |
|------|-------------|
| `--db` | Path to SQLite database file (default: `hydradns.db`) |
| `--host` | Override DNS bind address (runtime only) |
| `--port` | Override DNS bind port (runtime only) |
| `--workers` | Set worker count (clamps GOMAXPROCS) |
| `--no-tcp` | Disable TCP server |
| `--json-logs` | Enable JSON structured logging |
| `--debug` | Enable debug logging |

### Configuring via Web UI

After starting HydraDNS, open **http://localhost:8080** in your browser to:

- Add/remove custom DNS records (A, AAAA, CNAME)
- Configure upstream DNS servers
- Manage domain filtering (whitelist, blacklist, blocklists)
- View server statistics and health

All changes are persisted to the database immediately.

---

## Docker

### Quick Start

```bash
# Build and run with Docker Compose
docker compose up --build

# Or build manually
docker build -f Dockerfile.ui -t hydradns .
docker run -p 1053:1053/udp -p 1053:1053/tcp -p 8080:8080 \
  -v hydradns-data:/data hydradns
```

### Persistent Storage

The SQLite database should be stored on a volume:

```yaml
# docker-compose.yml
services:
  hydradns:
    volumes:
      - hydradns-data:/data
    environment:
      - HYDRADNS_DB=/data/hydradns.db

volumes:
  hydradns-data:
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `HYDRADNS_DB` | Path to SQLite database file |
| `HYDRADNS_LOGGING_LEVEL` | Log level (DEBUG, INFO, WARN, ERROR) |

The Docker image runs as a non-root user with minimal dependencies.

---

## Custom DNS

Add custom A/AAAA/CNAME records via the Web UI or REST API:

### Via Web UI

1. Open **http://localhost:8080**
2. Navigate to **Custom DNS**
3. Add hosts (hostname → IP) or CNAMEs (alias → target)

### Via REST API

```bash
# Add an A record
curl -X POST http://localhost:8080/api/v1/custom-dns/hosts \
  -H "Content-Type: application/json" \
  -d '{"hostname": "homelab.local", "ip_address": "192.168.1.10"}'

# Add a CNAME
curl -X POST http://localhost:8080/api/v1/custom-dns/cnames \
  -H "Content-Type: application/json" \
  -d '{"alias": "www.homelab.local", "target": "homelab.local"}'

# List all custom DNS records
curl http://localhost:8080/api/v1/custom-dns
```

- Queries that do not match custom DNS entries are forwarded upstream.
- Multiple IPs can be added for the same hostname (round-robin).

---

## Performance Optimizations

HydraDNS includes several performance optimizations for high-throughput DNS serving:

- **Buffer pooling** — UDP receive buffers and TCP length prefixes are pooled
- **Capacity pre-allocation** — Slices are sized based on expected content
- **Indexed custom DNS lookups** — Hostnames are indexed for O(1) access
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
| **Global** | 100000 | 100000 | Total server capacity |
| **Prefix** (/24) | 10000 | 20000 | Limit per subnet |
| **Per-IP** | 5000 | 10000 | Limit per client |

The default per-IP limit of **5000 QPS** is suitable for home/small office use. Your actual measured throughput will be slightly lower than the configured QPS due to rate limiter overhead.

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
| Home/Small Office | 5000 | 10000 | Default, good protection |
| Enterprise/Internal | 10000 | 20000 | Higher for trusted clients |
| Behind Load Balancer | 50000+ | 100000 | Single source IP for many clients |
| Development/Testing | 0 (disabled) | — | No limits for benchmarking |

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `HYDRADNS_RL_GLOBAL_QPS` | 100000 | Global queries per second limit |
| `HYDRADNS_RL_GLOBAL_BURST` | 100000 | Global burst capacity |
| `HYDRADNS_RL_PREFIX_QPS` | 10000 | Per /24 subnet QPS limit |
| `HYDRADNS_RL_PREFIX_BURST` | 20000 | Per /24 subnet burst capacity |
| `HYDRADNS_RL_IP_QPS` | 5000 | Per-IP QPS limit (0 = disabled) |
| `HYDRADNS_RL_IP_BURST` | 10000 | Per-IP burst capacity |
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

HydraDNS includes a domain filtering system for ad blocking, malware protection, and custom access control. Filtering uses a trie-based data structure for O(k) lookups where k is the number of domain labels.

### Features

- **Whitelist priority** — Whitelisted domains always allowed, even if on blacklist
- **Wildcard subdomains** — Blocking `ads.example.com` also blocks `tracker.ads.example.com`
- **Multiple blocklist formats** — Adblock Plus, hosts file, and plain domain lists
- **Remote blocklists** — Fetch from URLs with automatic periodic refresh
- **~86ns lookups** — High-performance trie with 10,000+ domains

### Quick Start

1. Open **http://localhost:8080**
2. Navigate to **Filtering**
3. Enable filtering
4. Add blocklists (e.g., Hagezi, StevenBlack)

Or via environment variables for initial setup:

```bash
# Enable filtering with Hagezi blocklist
export HYDRADNS_FILTERING_ENABLED=true
export HYDRADNS_FILTERING_BLOCKLIST_URL="https://cdn.jsdelivr.net/gh/hagezi/dns-blocklists@latest/adblock/light.txt"

go run ./cmd/hydradns
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

The filtering resolver sits at the front of the resolver chain, before custom DNS and forwarding resolvers.

---

## REST API

HydraDNS includes a REST API for runtime management and monitoring. The API is built with Gin and includes interactive Swagger documentation.

### Swagger UI

Interactive documentation is available at:

```
http://localhost:8080/swagger/index.html
```

### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/health` | GET | Health check |
| `/api/v1/stats` | GET | Server statistics (uptime, memory, goroutines) |
| `/api/v1/config` | GET | Current configuration (sensitive fields redacted) |
| `/api/v1/custom-dns` | GET | List custom DNS hosts and CNAMEs |
| `/api/v1/filtering/stats` | GET | Filtering statistics |
| `/api/v1/filtering/enabled` | PUT | Enable/disable filtering at runtime |
| `/api/v1/filtering/whitelist` | GET | List whitelist domains |
| `/api/v1/filtering/whitelist` | POST | Add domains to whitelist |
| `/api/v1/filtering/blacklist` | GET | List blacklist domains |
| `/api/v1/filtering/blacklist` | POST | Add domains to blacklist |

### Authentication

Configure an API key via the Web UI under **Settings** → **API**. Once set, include it in requests:

```bash
curl -H "X-Api-Key: your-secret-key" http://localhost:8080/api/v1/health
```

### Example Usage

```bash
# Health check
curl http://localhost:8080/api/v1/health

# Get server stats
curl -H "X-Api-Key: secret" http://localhost:8080/api/v1/stats

# Add domains to blacklist
curl -X POST -H "X-Api-Key: secret" -H "Content-Type: application/json" \
  -d '{"domains": ["ads.example.com", "tracker.example.com"]}' \
  http://localhost:8080/api/v1/filtering/blacklist

# Toggle filtering on/off
curl -X PUT -H "X-Api-Key: secret" -H "Content-Type: application/json" \
  -d '{"enabled": false}' \
  http://localhost:8080/api/v1/filtering/enabled
```

---

## AI Assistance

Using AI tools is fine for this project, but every AI output must be human-reviewed and validated before it ships:

- AI-generated code, docs, and examples are allowed as long as a human maintainer reviews and validates them before merge.
- AI-written comments are also fine, but treat them like code: **read and confirm** accuracy, clarity, and intent before accepting.
- When unsure, follow the project standards (Go 1.25, RFC correctness, clarity-first code) instead of AI suggestions.

---

## License

HydraDNS is licensed under the **MIT License** — a permissive, open-source license that allows you to:

- ✅ Use for any purpose (commercial or personal)
- ✅ Modify and distribute the software
- ✅ Fork and create derivative works
- ✅ Include in proprietary projects

**The only requirement:** include the original license and copyright notice.

See [LICENSE](LICENSE) for the full legal text.

---

## Contributing

Contributions are welcome! Whether it's bug reports, feature requests, or code contributions, please feel free to open an issue or submit a pull request.

### Development Setup

```bash
# Clone and enter the repository
git clone https://github.com/yourusername/HydraDNS.git
cd HydraDNS

# Run tests
go test ./...

# Run linter
golangci-lint run ./...

# Build the binary
make build
```

### Code Style

- Follow [Effective Go](https://golang.org/doc/effective_go)
- Run `gofmt` before committing
- All code must be tested
- See `.golangci.yml` for linter configuration

### Reporting Issues

Please include:
- Go version (`go version`)
- HydraDNS version or commit hash
- Steps to reproduce
- Expected vs. actual behavior
- Log output (with debug flag if possible)

---

## Acknowledgments

HydraDNS is built for the homelab community. Special thanks to all contributors and users who have tested, reported issues, and provided feedback.

---

## License

See [LICENSE](LICENSE) for details.