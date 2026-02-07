# GitHub Copilot Instructions — Go DNS Forwarding Server (Go 1.25)

These instructions guide GitHub Copilot when generating, modifying, or reviewing code in this repository.

## Context & Philosophy

This project is a **DNS forwarding server** with **simple custom DNS support** for **homelab and small self-hosted environments**, running on **bare metal, LXC, or VMs**.

The server:
- **Primary function**: Forward DNS queries to upstream servers
- **Secondary function**: Answer simple custom A/AAAA/CNAME records (dnsmasq-style)
- **Standards-compliant**: EDNS(0), DNSSEC-aware forwarding
- **Transport**: UDP with TCP fallback
- **Scaling**: SO_REUSEPORT and multi-CPU support

### Architecture Summary
- **Forwarding resolver**: Query upstream DNS servers with caching
- **Custom DNS resolver**: Simple A/AAAA/CNAME records from YAML config
- **Chained resolver**: Try custom DNS first, then forward upstream
- **No full zone file support**: Removed to keep codebase simple

### Core Values (Highest Priority)
1. **Clean, readable, idiomatic Go**
2. **Correctness and standards compliance**
3. **Safety and maintainability**
4. **Concurrency and scalability**
5. Performance (important, but not at the expense of clarity)

Extra allocations, slightly slower paths, or simpler designs are acceptable if they make the code **easier to understand, audit, and maintain**.

---

## 1. Go Language Standards

### Go Version
- Target **Go 1.25**
- Use only features available in Go 1.25

### Code Style
- All code must be `gofmt` formatted
- Follow *Effective Go*
- Prefer explicit, boring, readable code
- Avoid clever tricks and unnecessary abstractions

### Clean Code Rules
- Functions should do **one thing**
- Keep functions and files reasonably small
- Prefer clear names over short names
- Avoid deeply nested logic
- Avoid boolean parameter traps
- Document non-obvious decisions

### Error Handling
- Never ignore errors
- Wrap errors with context using `fmt.Errorf(\"...: %w\", err)`
- Panics are allowed only for unrecoverable startup failures

---

## 2. Concurrency & Parallelism (Required)

Concurrency is **not optional**.

- The server **must scale with available CPUs**
- Designed for multi-core environments using:
  - Goroutines
  - `SO_REUSEPORT`
- Correctness and clarity are more important than lock-free designs

### Rules
- Prefer goroutines with **clear ownership and lifecycle**
- Always document goroutine startup and shutdown behavior
- Avoid shared mutable state when possible
- When shared state is required:
  - Use explicit synchronization (mutexes, atomics)
  - Keep critical sections small
  - Document why sharing is safe

---

## 3. DNS Design Principles

### Protocol Correctness
- Always follow DNS RFCs
- Never emit malformed responses
- Correctly preserve and set DNS flags:
  - QR, AA, RD, RA, AD, CD, TC

### Forwarding vs Authoritative Behavior
- Authoritative zones:
  - Must **not** forward queries
  - Must set **AA**
- Forwarded queries:
  - Preserve RD
  - Preserve EDNS(0)
  - Respect upstream TTLs

---

## 4. Type-Oriented DNS Modeling (Strongly Preferred)

Use a **type-oriented (“top-like”) design**.

### Guidelines
- Prefer **explicit Go types per DNS concept**
  - `ARecord`
  - `AAAARecord`
  - `CNAMERecord`
  - `MXRecord`
  - `SOARecord`
  - etc.
- Avoid large generic structs with type switches
- Avoid `map[string]interface{}`

Each record type should:
- Validate itself
- Know how to serialize to wire format
- Clearly express its semantics

Strong typing is preferred over flexibility.

---

## 5. Transport Behavior

### UDP (Primary)
- UDP is the default transport
- Respect EDNS(0) payload size
- Handle truncation correctly

### TCP Fallback
Retry over TCP when:
- TC flag is set
- Query exceeds UDP limits

TCP handling must:
- Enforce read/write timeouts
- Be safe for concurrent connections

### SO_REUSEPORT
- Enable for UDP and TCP listeners
- Allow multiple sockets per address
- Designed to distribute load across CPUs

---

## 6. EDNS(0)

- Correctly parse and emit OPT records
- Respect advertised UDP payload size
- Preserve EDNS options when forwarding
- Unsupported EDNS versions must result in `FORMERR` (RFC 6891)

---

## 7. DNSSEC Awareness

The server is **DNSSEC-aware for forwarding**, not a validator.

### Required Behavior
- Preserve DO, AD, and CD flags when forwarding
- Forward DNSSEC-related records intact as opaque data:
  - RRSIG
  - DNSKEY
  - DS
  - NSEC / NSEC3
- Custom DNS responses **never set AD flag** (not DNSSEC-signed)

---

## 8. Custom DNS Configuration

Custom DNS is **not a full authoritative zone implementation**:
- Simple YAML configuration (hosts + cnames maps)
- Supports only A, AAAA, and CNAME records
- No SOA, NS, MX, SRV, CAA, TXT support
- No zone transfers or DNSSEC signing
- Designed for homelab hostname resolution

Example:
```yaml
custom_dns:
  hosts:
    homelab.local:
      - 192.168.1.10
      - 2001:db8::1
  cnames:
    www.homelab.local: homelab.local
```

---

## 9. Performance Guidelines (Balanced)

- Avoid premature optimization
- Concurrency must not be sacrificed for simplicity
- Extra allocations are acceptable if they improve clarity
- Avoid reflection in hot paths unless it meaningfully simplifies code
- Avoid leaking optimizations into public APIs

---

## 10. Security & Robustness

- Validate all external input
- Enforce reasonable size limits on:
  - DNS messages
  - TCP reads
- Protect against:
  - Amplification attacks
  - Resource exhaustion
- Never trust upstream responses blindly

---

## 11. Logging & Observability

- Use structured logging
- Do not log full DNS payloads by default
- Log:
  - Startup/shutdown
  - Upstream failures
  - Protocol violations

---

## 12. Testing

- Unit tests for:
  - Message parsing
  - Flag handling
  - Zone matching
- Integration tests for:
  - UDP/TCP behavior
  - Truncation and TCP fallback
  - EDNS(0) edge cases
- Tests must be deterministic and race-free

---

## 13. What Copilot Must NOT Do

- Do not introduce non-standard DNS behavior
- Do not ignore RFC edge cases
- Do not add heavy dependencies without justification
- Do not optimize at the cost of clarity
- Do not use untyped or generic data models where strong typing is possible

---

**Summary:**  
Prefer clean, correct, boring Go code that scales safely across CPUs and is easy to understand and maintain in a homelab environment.