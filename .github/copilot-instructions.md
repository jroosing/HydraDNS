# HydraDNS - Copilot Development Instructions

This document provides guidance for working efficiently with the HydraDNS codebase. HydraDNS is a high-performance DNS server written in Go 1.25+, designed for speed, reliability, and production use.

## Project Overview

HydraDNS is a production-ready DNS server with the following key features:

- **Protocol Support**: UDP + TCP with full RFC 1035 compliance, EDNS0, automatic TCP fallback
- **Performance**: Concurrent I/O with goroutines, buffer pooling, singleflight deduplication, O(1) zone lookups
- **Caching**: TTL-aware LRU cache with negative caching (NXDOMAIN, NODATA, SERVFAIL)
- **Security**: 3-tier rate limiting, domain filtering with trie-based whitelist/blacklist
- **Operations**: Zone file support, strict-order failover, structured logging, graceful shutdown
- **Management API**: REST API with Gin framework, OpenAPI/Swagger documentation, runtime configuration

### Architecture

```
Transport Layer: UDP + TCP servers with buffer pooling
Rate Limiter: 3-tier token bucket (global → /24 prefix → IP)
Query Handler: Parse + dispatch to resolver chain
Resolver Chain:
  ├─ Filtering Resolver: Domain whitelist/blacklist
  ├─ Zone Resolver: Local authoritative zones
  └─ Forwarding Resolver: Upstream DNS servers with caching
Cache: TTL-aware LRU + Singleflight deduplication
REST API: Gin-based HTTP API with Swagger docs
```

### Key Components

| Component | Location | Purpose |
|-----------|----------|---------|
| **Main Entry** | `cmd/hydradns/main.go` | Application entry point with config loading and server lifecycle |
| **UDP Server** | `internal/server/udp_server.go` | Handles UDP queries with buffer pooling, SO_REUSEPORT |
| **TCP Server** | `internal/server/tcp_server.go` | Handles TCP queries with connection pipelining |
| **Rate Limiter** | `internal/server/rate_limit.go` | 3-tier token bucket using netip.Addr |
| **Query Handler** | `internal/server/query_handler.go` | Request parsing, resolver dispatch, timeouts |
| **Chained Resolver** | `internal/resolvers/chained.go` | Tries resolvers in order |
| **Filtering Resolver** | `internal/resolvers/filtering_resolver.go` | Domain filtering with trie structure |
| **Zone Resolver** | `internal/resolvers/zone_resolver.go` | Authoritative responses from loaded zones |
| **Forwarding Resolver** | `internal/resolvers/forwarding_resolver.go` | Upstream forwarding with caching |
| **Cache** | `internal/resolvers/cache.go` | TTL-aware LRU cache (generics-based) |
| **DNS Codec** | `internal/dns/` | Wire format parsing and serialization |
| **Zone Parser** | `internal/zone/` | RFC 1035 master file format parser |
| **REST API** | `internal/api/` | HTTP management API with Swagger |
| **Config** | `internal/config/` | Configuration loading with Viper |
| **Logging** | `internal/logging/` | Structured logging setup with slog |

## Development Setup

### Prerequisites

- **Go 1.25.5+** (as specified in `go.mod`)
- Git for version control
- Docker (optional, for containerized testing)

### Getting Started

```bash
# Clone the repository (if not already cloned)
git clone https://github.com/jroosing/hydradns.git
cd hydradns

# Download dependencies
go mod download

# Run tests
go test ./...

# Format code
go fmt ./...

# Run linters
go vet ./...

# Build the main server
go build ./cmd/hydradns

# Run the server with default config
./hydradns

# Run with custom config
./hydradns --config hydradns.example.yaml

# Run with debug logging
./hydradns --debug
```

### Using the Makefile

The project includes a Makefile with common development tasks:

```bash
make test          # Run all tests
make fmt           # Format Go code
make vet           # Run go vet
make build         # Build all binaries (hydradns, dnsquery, print-zone, bench)
make check         # Run fmt + vet + test
make docs          # Generate Swagger/OpenAPI docs
make docker-build  # Build Docker image
make docker-run    # Run with docker-compose
make clean         # Remove build artifacts
```

## Testing

### Test Organization

- Tests live in `*_test.go` files next to the code they test
- Uses standard Go testing package with `github.com/stretchr/testify` for assertions
- Test package naming:
  - Same package (white-box): `package mypackage`
  - Separate package (black-box): `package mypackage_test`

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests in a specific package
go test ./internal/dns

# Run specific test
go test -run TestPacket_MarshalAndParse_SimpleQuery ./internal/dns

# Verbose output
go test -v ./...
```

### Test Patterns

- Use table-driven tests for multiple test cases
- Use `testify/assert` and `testify/require` for assertions
- Name tests descriptively: `Test_FunctionName_Scenario`
- Use subtests with `t.Run()` for better organization
- Mark helper functions with `t.Helper()`

Example:
```go
func TestCache_Get_ExpiredEntry(t *testing.T) {
    cache := NewTTLCache[string, string](100)
    // test implementation
}
```

## Building and Running

### Main Application

```bash
# Build server binary
go build ./cmd/hydradns

# Run with different configs
./hydradns                                    # Uses default config or HYDRADNS_CONFIG env var
./hydradns --config hydradns.example.yaml    # Explicit config file
./hydradns --host 0.0.0.0 --port 1053        # Override config values
./hydradns --debug                            # Enable debug logging
./hydradns --json-logs                        # JSON structured logging
./hydradns --no-tcp                           # Disable TCP server
./hydradns --workers 4                        # Set worker count
```

### Utility Commands

```bash
# Query the DNS server
go run ./cmd/dnsquery --server 127.0.0.1:1053 --name example.com --qtype 1

# Print parsed zone file
go run ./cmd/print-zone --file zones/example.zone

# Benchmark the server
go run ./cmd/bench --server 127.0.0.1:1053 --name example.com \
    --concurrency 200 --requests 20000
```

### Docker

```bash
# Build and run with docker-compose
docker compose up --build

# Build image only
docker build -t hydradns:latest .

# Run container
docker run -d --name hydradns -p 1053:1053/udp -p 1053:1053/tcp hydradns:latest
```

## Code Conventions

### General Go Practices

Follow idiomatic Go conventions as documented in `.github/instructions/go.instructions.md`. Key points:

- Write simple, clear, idiomatic Go code
- Keep the happy path left-aligned (minimize indentation)
- Return early to reduce nesting
- Use meaningful variable names
- Document exported types, functions, and methods
- Check all errors immediately
- Use `context.Context` for cancellation and timeouts

### Project-Specific Patterns

#### 1. Logging

Always use `log/slog` structured logging (never `log` package):

```go
logger.Info("message", "key1", value1, "key2", value2)
logger.Error("error occurred", "err", err, "context", additionalInfo)
logger.Debug("debug info", "details", data)
```

#### 2. Rate Limiting

- Use `netip.Addr` for IP addresses (zero-allocation)
- Apply rate limiting before parsing to minimize CPU overhead
- Token bucket algorithm with burst capacity

```go
// Good: netip.Addr for rate limiting
addr := netip.MustParseAddr("192.0.2.1")
if !limiter.Allow(addr) {
    return // rate limited
}

// Bad: string-based IP handling
if !limiter.Allow("192.0.2.1") { // avoid string allocations
    return
}
```

#### 3. Buffer Pooling

Reuse buffers to reduce GC pressure in hot paths:

```go
// Using pool package
var bufferPool = pool.New(func() *[]byte {
    buf := make([]byte, 4096)
    return &buf
})

buf := bufferPool.Get()
defer bufferPool.Put(buf)
```

#### 4. Generics Usage

Use Go generics where appropriate (Go 1.18+):

```go
// Cache uses generics for type safety
cache := NewTTLCache[string, *dns.Packet](10000)
```

#### 5. Concurrency

- Use `sync.WaitGroup.Go()` method (Go 1.25+) for launching goroutines:
  ```go
  var wg sync.WaitGroup
  wg.Go(task1)
  wg.Go(task2)
  wg.Wait()
  ```
- Always ensure goroutines can exit (no leaks)
- Use `context.Context` for cancellation
- Protect shared state with `sync.Mutex` or `sync.RWMutex`
- Use channels for communication between goroutines

#### 6. Error Handling

- Wrap errors with context using `fmt.Errorf` with `%w`:
  ```go
  return fmt.Errorf("failed to load config: %w", err)
  ```
- Keep error messages lowercase, no trailing punctuation
- Name error variables `err`
- Don't both log AND return errors (choose one)

#### 7. Configuration

- Use Viper for configuration loading
- Support config file, environment variables, and CLI flags
- Environment variable pattern: `HYDRADNS_<SECTION>_<KEY>`
- Always validate configuration after loading

#### 8. DNS Wire Format

- Use `internal/dns` package for all DNS packet operations
- Handle EDNS0 for UDP buffers > 512 bytes
- Implement automatic TCP fallback for truncated responses
- Always validate DNS packets from untrusted sources

#### 9. Performance Optimizations

Key performance patterns used in the codebase:

- **SO_REUSEPORT**: Multiple UDP sockets for kernel-level load balancing
- **Buffer pooling**: Reduce allocations for UDP receive buffers
- **Fixed worker pools**: Avoid spawning goroutines per request
- **Singleflight**: Deduplicate concurrent identical queries
- **Indexed zones**: O(1) lookups using maps
- **netip.Addr**: Zero-allocation IP address handling
- **Pre-allocated slices**: Size slices based on expected content

## CI/CD

### GitHub Actions Workflows

Located in `.github/workflows/ci.yml`, the CI runs:

1. **Test Job**: Downloads dependencies and runs `go test ./...`
2. **Lint Job**: Runs `gofmt -l .` and `go vet ./...`
3. **Docker Job**: Builds Docker image and runs smoke test with `dnsquery`

### Pre-commit Checks

Before committing, run:

```bash
make check  # Runs fmt, vet, and tests
```

### Linting

The project uses `golangci-lint` with strict configuration (`.golangci.yml`):

```bash
# If golangci-lint is installed
golangci-lint run

# The config enables many linters including:
# - errcheck, govet, staticcheck
# - gosec (security)
# - revive, gocritic (style)
# - And many more...
```

Note: CI currently uses `gofmt` and `go vet`, but `golangci-lint` config is available for local development.

## API Documentation

### Generating Swagger Docs

The REST API uses Swag for OpenAPI/Swagger documentation:

```bash
# Generate docs (updates internal/api/docs/)
make docs

# Or manually:
go run github.com/swaggo/swag/cmd/swag@latest init \
    -g internal/api/handlers/base.go \
    -o internal/api/docs \
    --parseDependency --parseInternal
```

### Accessing Swagger UI

When the API is enabled (`api.enabled: true` in config):

```
http://localhost:8080/swagger/index.html
```

### API Annotations

Use Swag annotations in handler files:

```go
// GetHealth godoc
// @Summary Health check endpoint
// @Description Returns server health status
// @Tags health
// @Produce json
// @Success 200 {object} models.HealthResponse
// @Router /api/v1/health [get]
func (h *HealthHandler) GetHealth(c *gin.Context) {
    // implementation
}
```

## Common Tasks

### Adding a New DNS Record Type

1. Add the type constant to `internal/dns/enums.go`
2. Implement parsing in `internal/dns/parsing.go`
3. Implement marshaling in `internal/dns/record.go`
4. Add zone file support in `internal/zone/zone.go`
5. Write tests in `internal/dns/dns_test.go` and `internal/zone/zone_test.go`

### Adding a New API Endpoint

1. Define request/response models in `internal/api/models/`
2. Add handler function in `internal/api/handlers/`
3. Register route in `internal/api/routes.go`
4. Add Swagger annotations to handler
5. Regenerate docs with `make docs`
6. Write tests in `internal/api/handlers/handlers_test.go`

### Adding a New Resolver

1. Implement the `Resolver` interface from `internal/resolvers/types.go`
2. Add to resolver chain in `internal/resolvers/chained.go`
3. Update configuration types in `internal/config/types.go`
4. Write tests in `internal/resolvers/resolvers_test.go`

### Modifying Configuration

1. Update YAML struct in `internal/config/types.go`
2. Update example config `hydradns.example.yaml`
3. Update README.md configuration section
4. Handle in `cmd/hydradns/main.go` if needed
5. Write tests in `internal/config/config_test.go`

## Dependencies

### Core Dependencies

- **gin-gonic/gin**: HTTP web framework for REST API
- **spf13/viper**: Configuration management (YAML, env vars)
- **stretchr/testify**: Testing assertions and mocks
- **swaggo**: Swagger/OpenAPI documentation generation
- **golang.org/x/sys**: Low-level system calls (SO_REUSEPORT, etc.)

### Dependency Management

```bash
# Add a new dependency
go get github.com/example/package

# Update dependencies
go get -u ./...

# Tidy up go.mod and go.sum
go mod tidy

# Verify dependencies
go mod verify
```

## Troubleshooting

### Build Issues

```bash
# Clean build cache
go clean -cache

# Re-download modules
rm -rf $GOPATH/pkg/mod
go mod download

# Check for module issues
go mod verify
go mod tidy
```

### Test Failures

```bash
# Run tests with verbose output
go test -v ./...

# Run specific test
go test -run TestName ./path/to/package

# Enable race detector
go test -race ./...
```

### DNS Query Testing

```bash
# Test with dig
dig @127.0.0.1 -p 1053 example.com

# Test with built-in dnsquery tool
go run ./cmd/dnsquery --server 127.0.0.1:1053 --name example.com --qtype 1

# Check Docker container logs
docker logs hydradns
```

## Best Practices Summary

1. **Always run tests** before committing (`make check`)
2. **Use structured logging** with slog, not fmt.Println or log package
3. **Handle all errors** explicitly
4. **Use context.Context** for cancellation and timeouts
5. **Avoid allocations** in hot paths (use buffer pools, netip.Addr)
6. **Document exported APIs** with clear godoc comments
7. **Write tests** for new features and bug fixes
8. **Follow Go conventions** as specified in `.github/instructions/go.instructions.md`
9. **Use generics** for type-safe collections (Go 1.18+)
10. **Leverage Go 1.25 features** like `WaitGroup.Go()` method

## Performance Considerations

When working on performance-critical code:

- **Profile first**: Use `pprof` to identify bottlenecks
- **Benchmark**: Use `testing.B` for benchmarks
- **Minimize allocations**: Reuse buffers, use `sync.Pool`
- **Avoid string operations**: Use `[]byte` or `netip.Addr` when possible
- **Consider concurrency**: But avoid over-parallelization
- **Use efficient data structures**: Maps for O(1) lookups, tries for prefix matching
- **Batch operations**: Where possible to reduce syscall overhead

## Security Considerations

- **Validate all input**: Especially from network (DNS packets, HTTP requests)
- **Rate limit aggressively**: Protect against DoS attacks
- **Use context timeouts**: Prevent resource exhaustion
- **Sanitize logs**: Don't log sensitive data (API keys, user data)
- **Run as non-root**: Docker container runs as non-root user
- **Keep dependencies updated**: Regularly check for security patches

## Additional Resources

- [Go Documentation](https://go.dev/doc/)
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [DNS RFC 1035](https://www.rfc-editor.org/rfc/rfc1035)
- [EDNS0 RFC 6891](https://www.rfc-editor.org/rfc/rfc6891)
- Project README: `README.md`
- Go Instructions: `.github/instructions/go.instructions.md`
