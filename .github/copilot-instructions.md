# HydraDNS - Copilot Instructions

This document provides coding agents with essential information about the HydraDNS repository to work efficiently and maintain code quality.

## Project Overview

HydraDNS is a high-performance, production-ready DNS server written in Go 1.24+. It supports both authoritative zone serving and recursive forwarding with intelligent caching, rate limiting, and domain filtering.

### Key Features
- **Protocol Support**: UDP + TCP, EDNS0, automatic TCP fallback
- **Performance**: Concurrent I/O with goroutines, buffer pooling, singleflight deduplication
- **Caching**: TTL-aware LRU cache with negative caching
- **Security**: 3-tier rate limiting, domain filtering, response validation
- **Management**: REST API with Swagger documentation

### Architecture

The codebase follows a clean, layered architecture:

```
cmd/hydradns/           Main entry point
internal/
  ├── api/              REST management API (Gin-based)
  ├── config/           Configuration loading (Viper)
  ├── dns/              DNS wire format codec
  ├── filtering/        Domain filtering engine
  ├── helpers/          Utility functions
  ├── logging/          Structured logging (slog)
  ├── pool/             Buffer pooling
  ├── resolvers/        Resolver chain (zone → filtering → forwarding)
  ├── server/           UDP/TCP servers, rate limiting, query handling
  └── zone/             RFC 1035 zone file parser
```

## Building and Testing

### Essential Commands

```bash
# Run all tests
go test ./...

# Format code (ALWAYS run before committing)
go fmt ./...

# Lint code
go vet ./...

# Full quality check (format + vet + test)
make check

# Build all binaries
make build

# Build specific binary
go build ./cmd/hydradns

# Generate API documentation
make docs
```

### CI Pipeline

The CI runs on every push and PR:
- **Tests**: `go test ./...`
- **Formatting**: `gofmt -l .` (must produce no output)
- **Linting**: `go vet ./...`
- **Docker**: Build and smoke test

Use `make check` locally before pushing to catch issues early.

### Linting

This project uses a strict `.golangci.yml` configuration. Key linters enabled:
- `errcheck` - Check all errors
- `govet` - Go's official linter (with all analyzers except `fieldalignment`)
- `staticcheck` - Advanced static analysis
- `gosec` - Security issues
- `revive` - Comprehensive style checks
- `gocritic` - Performance and style
- Many others (see `.golangci.yml` for full list)

**Note**: The linter is strict but configured to ignore some false positives. Run `go vet ./...` as a minimum check.

## Go Version and Module

- **Go version**: 1.24.0+ (see `go.mod`)
- **Module path**: `github.com/jroosing/hydradns`
- **Go 1.24 features**: This project uses Go 1.24, which includes `WaitGroup.Go()` method. Use this method instead of the classic `Add`/`Done` pattern for launching goroutines.

Example:
```go
var wg sync.WaitGroup
wg.Go(task1)
wg.Go(task2)
wg.Wait()
```

## Project-Specific Conventions

### Package Organization

- **`cmd/`**: Main applications (only `hydradns` currently)
- **`internal/`**: All internal packages (not importable by external projects)
- **No `pkg/`**: All code is in `internal/` to prevent external usage
- **Test files**: Use `_test.go` suffix, placed next to code (white-box testing)
- **Package comments**: All packages have descriptive comments explaining their purpose

### Naming Conventions

Follow standard Go naming conventions strictly:

1. **Packages**: Single-word, lowercase (e.g., `server`, `resolvers`, `filtering`)
2. **Exported symbols**: Start with capital letter
3. **Unexported symbols**: Start with lowercase letter
4. **Interfaces**: Use `-er` suffix when possible (e.g., `Resolver`)
5. **Avoid stuttering**: Use `dns.Packet` not `dns.DNSPacket`

### Error Handling Patterns

```go
// ✅ Good: Check errors immediately, wrap with context
result, err := resolver.Resolve(ctx, req, reqBytes)
if err != nil {
    return Result{}, fmt.Errorf("resolve failed: %w", err)
}

// ✅ Good: Early return to reduce nesting
if len(req.Questions) == 0 {
    return Result{}, errors.New("no questions")
}

// ❌ Bad: Ignoring errors
result, _ := resolver.Resolve(ctx, req, reqBytes)

// ❌ Bad: Not wrapping errors
return Result{}, err
```

### Struct Patterns

```go
// ✅ Good: Clear, documented struct with struct tags
type ServerConfig struct {
    Host       string `yaml:"host" json:"host"`           // Bind address
    Port       int    `yaml:"port" json:"port"`           // Port number
    EnableTCP  bool   `yaml:"enable_tcp" json:"enable_tcp"` // Enable TCP server
}

// ✅ Good: Use pointer receivers for methods that modify state
func (s *Server) Start() error { ... }

// ✅ Good: Use value receivers for small, immutable structs
func (p Packet) IsQuery() bool { ... }
```

### Context Usage

```go
// ✅ Good: Always pass context as first parameter
func (r *ForwardingResolver) Resolve(ctx context.Context, req dns.Packet, reqBytes []byte) (Result, error)

// ✅ Good: Check context cancellation in loops
for _, resolver := range c.Resolvers {
    if ctx.Err() != nil {
        return Result{}, ctx.Err()
    }
    // ... process resolver
}

// ✅ Good: Use context-aware logging
logger.InfoContext(ctx, "message", "key", value)
```

### Logging Patterns

This project uses `log/slog` for structured logging:

```go
// ✅ Good: Use structured logging with key-value pairs
logger.Info("server starting", 
    "host", cfg.Host,
    "port", cfg.Port,
    "tcp", cfg.EnableTCP,
)

// ✅ Good: Use context-aware logging methods
logger.DebugContext(ctx, "dns request",
    "transport", "udp",
    "qname", qname,
    "qtype", qtype,
)

// ❌ Bad: Don't use global loggers (will fail sloglint)
slog.Info("message")  // Use logger.Info() instead

// ❌ Bad: Don't use fmt.Printf for logging in production code
fmt.Printf("Error: %v\n", err)  // Use logger.Error() instead
```

### Concurrency Patterns

```go
// ✅ Good: Use WaitGroup.Go() for launching goroutines (Go 1.24+)
var wg sync.WaitGroup
wg.Go(func() {
    // task 1
})
wg.Go(func() {
    // task 2
})
wg.Wait()

// ✅ Good: Always handle goroutine cleanup
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

go func() {
    select {
    case <-ctx.Done():
        return
    case result := <-resultCh:
        // process result
    }
}()

// ✅ Good: Use channels for communication
resCh := make(chan Result, 1)
go func() {
    res, err := compute()
    resCh <- Result{res, err}
}()
result := <-resCh
```

## Key Dependencies

### Core Dependencies

- **github.com/gin-gonic/gin**: HTTP framework for REST API
  - Use for building HTTP endpoints
  - Middleware for logging, auth, etc.
  
- **github.com/spf13/viper**: Configuration management
  - Loads YAML config files
  - Binds environment variables with `HYDRADNS_` prefix
  - Provides defaults for all settings

- **github.com/stretchr/testify**: Testing utilities
  - Use `assert` for test assertions
  - Use `require` when test should stop on failure
  - Example: `assert.Equal(t, expected, actual, "message")`

- **github.com/swaggo/swag**: API documentation generation
  - Generates Swagger/OpenAPI docs from code annotations
  - Run `make docs` to regenerate
  - Annotations live in `internal/api/handlers/`

### Standard Library Usage

This project heavily uses the Go standard library:

- **`log/slog`**: Structured logging (not `log` or `fmt` for production logging)
- **`context.Context`**: Cancellation and timeouts (always first parameter)
- **`net/netip`**: IP address parsing and manipulation (not `net.IP`)
- **`sync`**: WaitGroup, Mutex, RWMutex, Once, Pool
- **`time`**: Duration, Timer (not sleep loops)
- **`errors`**: Error wrapping with `fmt.Errorf("%w", err)`

## Testing Patterns

### Table-Driven Tests

```go
func TestRateLimiter_Scenarios(t *testing.T) {
    tests := []struct {
        name      string
        settings  RateLimitSettings
        requests  int
        expectOK  int
    }{
        {
            name: "allows within limit",
            settings: RateLimitSettings{IPQPS: 10, IPBurst: 5},
            requests: 5,
            expectOK: 5,
        },
        // ... more test cases
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            limiter := NewRateLimiter(tt.settings)
            // ... test logic
        })
    }
}
```

### Test Helpers

```go
// ✅ Good: Mark helper functions with t.Helper()
func assertNoError(t *testing.T, err error) {
    t.Helper()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
}

// ✅ Good: Use require for preconditions, assert for checks
require.NoError(t, err, "setup should not fail")
assert.Equal(t, expected, actual, "result should match")
```

### Test Organization

- **White-box testing**: Tests in same package (e.g., `package server_test`)
- **File naming**: `*_test.go` next to the code being tested
- **Subtests**: Use `t.Run()` for organizing related tests
- **Cleanup**: Use `t.Cleanup()` for deferred cleanup (not `defer` in tests)

## DNS-Specific Patterns

### DNS Packet Handling

```go
// ✅ Good: Parse request, check errors
parsed, err := dns.ParseRequestBounded(reqBytes)
if err != nil {
    return handleParseError(reqBytes)
}

// ✅ Good: Always check question count before accessing
if len(parsed.Questions) == 0 {
    return Result{}, errors.New("no questions")
}
qname := parsed.Questions[0].Name

// ✅ Good: Build error responses with proper codes
resp := dns.BuildErrorResponse(req, uint16(dns.RCodeServFail))
```

### Resolver Chain Pattern

Resolvers implement the `Resolver` interface and can be chained:

```go
type Resolver interface {
    Resolve(ctx context.Context, req dns.Packet, reqBytes []byte) (Result, error)
    Close() error
}
```

Chain order: **Filtering → Zone → Forwarding**

Each resolver either:
1. Returns a response (chain stops)
2. Returns an error (next resolver is tried)
3. Passes context cancellation up the chain

## Performance Considerations

### Buffer Pooling

```go
// ✅ Good: Use sync.Pool for frequently allocated buffers
var bufPool = sync.Pool{
    New: func() any {
        b := make([]byte, 4096)
        return &b
    },
}

buf := bufPool.Get().(*[]byte)
defer bufPool.Put(buf)
```

### Pre-allocation

```go
// ✅ Good: Pre-allocate slices when size is known
records := make([]Record, 0, len(zone.Records))

// ✅ Good: Estimate capacity for output buffers
estimatedSize := HeaderSize + len(questions)*50
out := make([]byte, 0, estimatedSize)
```

### Memory Efficiency

```go
// ✅ Good: Use value receivers for small structs (< 64 bytes)
func (h Header) IsQuery() bool { ... }

// ✅ Good: Use pointer receivers for large structs
func (c *Cache) Get(key string) ([]byte, bool) { ... }
```

## Configuration Patterns

Configuration uses Viper with 4-tier precedence:

1. **Command-line flags** (highest priority)
2. **Environment variables** (`HYDRADNS_<SECTION>_<KEY>`)
3. **YAML config file**
4. **Defaults** (in `config.setDefaults()`)

```go
// ✅ Good: Always provide defaults
v.SetDefault("server.port", 1053)

// ✅ Good: Use environment variable naming convention
// server.port -> HYDRADNS_SERVER_PORT
// upstream.servers -> HYDRADNS_UPSTREAM_SERVERS
```

## API Development

### HTTP Handlers

```go
// ✅ Good: Use Gin context for HTTP handlers
func (h *Handler) GetHealth(c *gin.Context) {
    c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

// ✅ Good: Use proper status codes
c.JSON(http.StatusOK, response)           // 200
c.JSON(http.StatusBadRequest, err)        // 400
c.JSON(http.StatusNotFound, err)          // 404
c.JSON(http.StatusInternalServerError, err) // 500
```

### Swagger Annotations

```go
// @Summary      Get server health
// @Description  Returns health check status
// @Tags         health
// @Produce      json
// @Success      200  {object}  models.StatusResponse
// @Router       /api/v1/health [get]
func (h *Handler) GetHealth(c *gin.Context) { ... }
```

## Common Pitfalls

### DNS-Specific

1. **Not checking question count**: Always check `len(req.Questions) > 0` before accessing
2. **Forgetting TC flag**: Set truncated flag for UDP responses > 512 bytes
3. **Not preserving request ID**: Response must have same ID as request
4. **Incorrect RCODE**: Use proper response codes (NXDOMAIN, SERVFAIL, etc.)

### Go-Specific

1. **Shadowing err**: Be careful with `:=` in if statements
   ```go
   // ❌ Bad: Shadows outer err
   result, err := firstCall()
   if result != nil {
       data, err := secondCall()  // Shadows err!
   }
   
   // ✅ Good: Reuse err
   result, err := firstCall()
   if result != nil {
       var data []byte
       data, err = secondCall()
   }
   ```

2. **Goroutine leaks**: Always ensure goroutines can exit
   ```go
   // ❌ Bad: Goroutine may leak if channel is never closed
   go func() {
       for msg := range ch {
           process(msg)
       }
   }()
   
   // ✅ Good: Use context for cancellation
   go func() {
       for {
           select {
           case <-ctx.Done():
               return
           case msg := <-ch:
               process(msg)
           }
       }
   }()
   ```

3. **Not closing resources**: Always close files, connections, response bodies
   ```go
   // ✅ Good: Use defer for cleanup
   resp, err := http.Get(url)
   if err != nil {
       return err
   }
   defer resp.Body.Close()
   ```

4. **Race conditions**: Use mutexes or channels for shared state
   ```go
   // ❌ Bad: Concurrent map access
   go func() { m["key"] = "value" }()
   go func() { _ = m["key"] }()
   
   // ✅ Good: Use sync.RWMutex
   var mu sync.RWMutex
   go func() { mu.Lock(); m["key"] = "value"; mu.Unlock() }()
   go func() { mu.RLock(); _ = m["key"]; mu.RUnlock() }()
   ```

### Project-Specific

1. **Not using slog**: Don't use `log` or `fmt.Printf` for logging
2. **Using net.IP**: Use `net/netip.Addr` instead (more efficient)
3. **Not wrapping errors**: Always use `fmt.Errorf("context: %w", err)`
4. **Ignoring context**: Pass and check `ctx.Err()` in long operations
5. **Direct config access**: Use config loading functions, not direct Viper calls outside `config` package

## Security Considerations

1. **Rate limiting**: Use the 3-tier rate limiter (global → prefix → IP)
2. **Input validation**: Validate all DNS queries and API inputs
3. **Error messages**: Don't leak sensitive info in error responses
4. **TLS**: Use TLS for API if exposed to networks (not implemented yet)
5. **API keys**: Respect `X-API-Key` header in API handlers

## Making Changes

### Before Starting

1. Read the relevant code and tests
2. Understand the architecture and data flow
3. Check for similar patterns in existing code
4. Review this document for conventions

### While Working

1. **Format**: Run `go fmt ./...` frequently
2. **Test**: Run `go test ./...` after changes
3. **Lint**: Run `go vet ./...` to catch issues
4. **Document**: Add comments for exported symbols
5. **Error handling**: Check all errors, wrap with context

### Before Committing

1. Run `make check` (format + vet + test)
2. Ensure all tests pass
3. Add tests for new functionality
4. Update documentation if needed
5. Check `git diff` to avoid committing unintended changes

## Need Help?

- **README.md**: Architecture overview, features, configuration
- **internal/zone/README.md**: Zone file parsing details
- **hydradns.example.yaml**: Full configuration reference
- **CI workflow**: `.github/workflows/ci.yml`
- **Linter config**: `.golangci.yml`

## Key Takeaways

1. **Follow idiomatic Go**: Simple, clear code over clever solutions
2. **Use standard library**: Prefer `log/slog`, `context`, `net/netip`, `sync`
3. **Test everything**: Table-driven tests with subtests
4. **Handle errors**: Check, wrap, and propagate with context
5. **Document code**: Clear comments for all exported symbols
6. **Run checks**: `make check` before every commit
7. **Use Go 1.24 features**: `WaitGroup.Go()` for launching goroutines
8. **Structured logging**: Use `log/slog` with context, never global loggers
9. **Context everywhere**: Always pass and check context cancellation
10. **Performance matters**: Use buffer pools, pre-allocation, and efficient algorithms
