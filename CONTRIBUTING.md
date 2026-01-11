# Contributing to HydraDNS

Thank you for your interest in contributing to HydraDNS! This document provides guidelines and instructions for contributing.

## Code of Conduct

Be respectful and constructive. We're building a welcoming community for everyone.

## Getting Started

### Prerequisites

- **Go 1.25+** — [Install Go](https://golang.org/doc/install)
- **Git** — For version control
- **Make** — For running common tasks
- **golangci-lint** — For linting (`go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`)

### Fork and Clone

```bash
# Fork on GitHub, then clone your fork
git clone https://github.com/YOUR_USERNAME/HydraDNS.git
cd HydraDNS

# Add upstream remote for syncing
git remote add upstream https://github.com/original/HydraDNS.git
```

### Set Up Development Environment

```bash
# Install dependencies
go mod download

# Run tests
go test ./...

# Run linter
make lint

# Build the binary
make build
```

## Making Changes

### Branch Naming

Use descriptive branch names:

```bash
git checkout -b feature/add-dnssec-validation
git checkout -b fix/tcp-connection-leak
git checkout -b docs/improve-contributing-guide
```

### Code Style

HydraDNS follows [Effective Go](https://golang.org/doc/effective_go) and the [Go Code Review Comments](https://golang.org/doc/review).

#### Key Principles

1. **Clarity over cleverness** — Write code that's easy to understand
2. **Explicit error handling** — Never ignore errors
3. **Simple, boring code** — Avoid unnecessary abstractions
4. **Concurrency-aware** — Consider goroutine safety and resource cleanup
5. **Type-oriented design** — Prefer explicit types over generic interfaces

#### Formatting and Linting

```bash
# Format code
go fmt ./...

# Run linter before committing
make lint

# Run tests
make test
```

All code must pass linting and tests before submission.

### Commit Messages

Write clear, descriptive commit messages:

```
feat: add DNSSEC validation support

- Implement DNSSEC signature validation
- Add AD flag handling for validated responses
- Include unit tests for signature verification

Fixes #123
```

#### Commit Message Format

```
<type>: <subject>

<body>

<footer>
```

**Types:**
- `feat` — New feature
- `fix` — Bug fix
- `refactor` — Code reorganization
- `test` — Test additions or changes
- `docs` — Documentation changes
- `perf` — Performance improvements
- `chore` — Build, CI, dependencies

**Subject:**
- Imperative mood ("add" not "adds")
- Don't capitalize
- No period at the end
- Under 50 characters

**Body (optional):**
- Explain *why*, not *what*
- Wrap at 72 characters
- Separate from subject with blank line

**Footer (optional):**
- Reference issues: `Fixes #123`, `Closes #456`

### Testing

All code must include tests.

#### Running Tests

```bash
# Run all tests
go test ./...

# Run specific package tests
go test ./internal/dns

# Run with coverage
go test -cover ./...

# Run with race detector (essential for concurrency)
go test -race ./...
```

#### Writing Tests

- Test files go in `*_test.go` in the same package
- Table-driven tests are preferred for multiple cases
- Mock external dependencies (upstream servers, files, etc.)
- Test both happy path and error cases

#### Example Test

```go
func TestParseQuery(t *testing.T) {
    tests := []struct {
        name    string
        input   []byte
        want    *dns.Query
        wantErr bool
    }{
        {
            name:  "simple A record query",
            input: simpleAQuery,
            want:  &dns.Query{Name: "example.com", Type: dns.TypeA},
        },
        {
            name:    "truncated packet",
            input:   []byte{0x00},
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := dns.ParseQuery(tt.input)
            if (err != nil) != tt.wantErr {
                t.Fatalf("ParseQuery() error = %v, wantErr %v", err, tt.wantErr)
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("ParseQuery() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Documentation

Document your changes:

1. **Code comments** — Explain *why*, not *what*
2. **Package docs** — Add package-level comments to new packages
3. **Exported symbols** — All exported functions/types need comments
4. **README updates** — Update README.md if adding features
5. **Changelog** — Consider mentioning significant changes

#### Example Documentation

```go
// CustomDNSResolver handles queries against local custom DNS records.
// It tries to match the query name against configured hosts and CNAMEs,
// returning authoritative responses for matches.
type CustomDNSResolver struct {
    // hosts maps domain names to their IP addresses
    hosts map[string][]net.IP
    // cnames maps aliases to their canonical names
    cnames map[string]string
    // mu protects hosts and cnames during reloads
    mu sync.RWMutex
}

// Resolve attempts to resolve the query using custom DNS records.
// If no match is found, it returns ErrNotFound (not NXDOMAIN).
func (r *CustomDNSResolver) Resolve(ctx context.Context, q *dns.Query) (*dns.Response, error) {
    // ...
}
```

## Submitting Changes

### Create a Pull Request

1. **Push your branch:**
   ```bash
   git push origin feature/your-feature
   ```

2. **Open a PR** on GitHub with a clear title and description
3. **Reference issues** in the PR description (e.g., "Fixes #123")
4. **Describe your changes** — What problem does this solve? How?

### PR Guidelines

- **One feature per PR** — Keep PRs focused
- **Tests included** — All code changes need tests
- **Passes CI** — GitHub Actions must pass
- **No large refactors in feature PRs** — Separate refactoring PRs
- **Documented** — Update README/docs if needed

### PR Description Template

```markdown
## Description
Brief description of what this PR does.

## Motivation
Why is this change needed? What problem does it solve?

## Changes
- Item 1
- Item 2
- Item 3

## Testing
How was this tested? What edge cases are covered?

## Checklist
- [ ] Tests added/updated
- [ ] Code follows style guidelines
- [ ] Documentation updated
- [ ] No breaking changes
- [ ] Tested with race detector: `go test -race ./...`

Fixes #123
```

## DNS Standards Compliance

When modifying DNS handling, ensure compliance with relevant RFCs:

- **RFC 1035** — DNS Protocol
- **RFC 1034** — DNS Concepts and Facilities
- **RFC 2308** — Negative Caching of DNS Queries
- **RFC 6891** — EDNS(0)
- **RFC 4034** — DNSSEC Signatures and Keys
- **RFC 4035** — DNSSEC Protocol

## Architecture Decisions

HydraDNS is designed with these principles in mind:

1. **Clarity first** — Easy to understand, even if slightly less efficient
2. **Type-safe** — Explicit types over generic interfaces
3. **Concurrent by design** — Safe goroutine usage throughout
4. **Homelab-friendly** — Minimal dependencies, easy deployment
5. **Standards-compliant** — Follows DNS RFCs strictly

When proposing architectural changes, consider these principles.

## Building and Testing

```bash
# Full build and test
make test build

# Run benchmarks
make benchmark

# Build Docker image
make docker-build

# Run linter
make lint

# View all available tasks
make help
```

## Questions or Need Help?

- **GitHub Issues** — For bug reports and feature requests
- **GitHub Discussions** — For questions and ideas (if enabled)
- **Pull Request Comments** — Ask questions during review

## License

By contributing to HydraDNS, you agree that your contributions will be licensed under the MIT License. See [LICENSE](LICENSE) for details.

---

Thank you for contributing to HydraDNS! 🎉
