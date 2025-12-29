
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
- **Response validation** — Verifies upstream responses match requests
- **Hardened Docker deployment** — Non-root user, minimal attack surface

### Operations
- **Zone file support** — Standard RFC 1035 master file format
- **Strict-order failover** — Primary upstream with automatic fallback
- **Structured logging** — JSON or key-value format for log aggregation
- **Graceful shutdown** — Drains in-flight requests before stopping

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
└─────────────────────────────────────────────────────────────────┘
```

### Component Overview

| Component | Location | Purpose |
|-----------|----------|---------|
| **UDP Server** | `internal/server/udp_server.go` | Handles UDP queries with buffer pooling and concurrency control |
| **TCP Server** | `internal/server/tcp_server.go` | Handles TCP queries with connection pipelining and per-IP limits |
| **Rate Limiter** | `internal/server/rate_limit.go` | 3-tier token bucket rate limiting (global → /24 prefix → IP) |
| **Query Handler** | `internal/server/query_handler.go` | Parses requests, dispatches to resolvers, handles timeouts |
| **Chained Resolver** | `internal/resolvers/chained.go` | Tries resolvers in order (zone → forwarding) |
| **Zone Resolver** | `internal/resolvers/zone_resolver.go` | Serves authoritative responses from loaded zones |
| **Forwarding Resolver** | `internal/resolvers/forwarding_resolver.go` | Forwards to upstream servers with caching and failover |
| **Cache** | `internal/resolvers/cache.go` | TTL-aware LRU cache with negative caching support |
| **DNS Codec** | `internal/dns/` | Wire format parsing and serialization |
| **Zone Parser** | `internal/zone/` | RFC 1035 master file format parser |

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

## License

See [LICENSE](LICENSE) for details.