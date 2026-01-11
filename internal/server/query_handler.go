// Package server implements DNS protocol servers for UDP and TCP.
//
// Goroutine Model:
//
// The server spawns multiple goroutines for handling incoming queries:
//   - UDPServer: 1 receiver + N workers per CPU core
//   - TCPServer: 1 listener per CPU core + 1 handler per active connection
//
// All goroutines are coordinated through a shared context:
//   - Context is cancelled on shutdown signal (SIGINT/SIGTERM)
//   - All goroutines check context regularly and exit cleanly
//   - No long-lived blocking operations without context awareness
//
// Error Handling:
//
// Errors are wrapped with context using fmt.Errorf("...: %w", err) throughout.
// This preserves error chains while adding operational context.
package server

import (
	"context"
	"log/slog"
	"time"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/jroosing/hydradns/internal/resolvers"
)

// QueryHandler processes DNS queries through a resolver and handles
// timeouts and error conditions.
type QueryHandler struct {
	Logger   *slog.Logger       // Optional logger for debug output
	Resolver resolvers.Resolver // The resolver chain to process queries
	Timeout  time.Duration      // Maximum time for query resolution (default: 4s)
}

// HandleResult contains the outcome of query processing.
type HandleResult struct {
	ResponseBytes []byte     // Serialized DNS response
	Source        string     // Origin of response (cache, upstream, error type)
	Parsed        dns.Packet // Parsed request (if ParsedOK is true)
	ParsedOK      bool       // Whether the request was successfully parsed
}

// Handle processes a DNS request and returns a response.
//
// Processing steps:
//  1. Parse the raw request bytes
//  2. Forward to resolver with timeout
//  3. Handle errors (parse, timeout, resolver failure) with SERVFAIL
//  4. Log request details at debug level
//
// The context is checked for cancellation (e.g., server shutdown).
func (h *QueryHandler) Handle(ctx context.Context, transport string, src string, reqBytes []byte) HandleResult {
	// Step 1: Parse request
	parsed, err := dns.ParseRequestBounded(reqBytes)
	if err != nil {
		return h.handleParseError(reqBytes)
	}

	// Extract question info for logging
	qname, qtype := extractQuestionInfo(parsed)

	// Step 2: Resolve with timeout
	result := h.resolveWithTimeout(ctx, parsed, reqBytes)

	// Step 3: Log at debug level
	h.logRequest(ctx, transport, src, parsed, qname, qtype, len(reqBytes), result.Source)

	return HandleResult{
		ResponseBytes: result.ResponseBytes,
		Source:        result.Source,
		Parsed:        parsed,
		ParsedOK:      true,
	}
}

// handleParseError attempts to build an error response from a malformed request.
// Returns FORMERR if the header/question could be extracted, or nil if not.
func (h *QueryHandler) handleParseError(reqBytes []byte) HandleResult {
	resp := tryBuildErrorFromRaw(reqBytes, uint16(dns.RCodeFormErr))
	if resp == nil {
		return HandleResult{ResponseBytes: nil, Source: "parse-error", ParsedOK: false}
	}
	return HandleResult{ResponseBytes: resp, Source: "formerr", ParsedOK: false}
}

// extractQuestionInfo extracts the QNAME and QTYPE from a parsed request.
func extractQuestionInfo(parsed dns.Packet) (string, int) {
	qname := "<no-question>"
	qtype := -1
	if len(parsed.Questions) > 0 {
		qname = parsed.Questions[0].Name
		qtype = int(parsed.Questions[0].Type)
	}
	return qname, qtype
}

// resolveWithTimeout runs the resolver with a timeout.
// Returns SERVFAIL on timeout, cancellation, or resolver error.
//
// Design note: This spawns a goroutine per query to enforce timeout without blocking
// the worker pool. An alternative design would make resolvers context-aware and timeout
// internally, but that would require all resolver implementations to handle context
// cancellation correctly. The current approach keeps timeout enforcement isolated here.
//
// Goroutine lifecycle: Spawned per query, exits when:
// - Resolver completes (success or error)
// - Context cancelled (server shutdown)
// - Timeout expires
// Cleanup: Channel closed automatically on goroutine exit, no cleanup needed.
func (h *QueryHandler) resolveWithTimeout(ctx context.Context, parsed dns.Packet, reqBytes []byte) resolvers.Result {
	// Start resolver in background
	resCh := make(chan struct {
		res resolvers.Result
		err error
	}, 1)
	go func() {
		res, err := h.Resolver.Resolve(ctx, parsed, reqBytes)
		resCh <- struct {
			res resolvers.Result
			err error
		}{res: res, err: err}
	}()

	// Set up timeout
	timeout := h.Timeout
	if timeout <= 0 {
		timeout = 4 * time.Second
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	// Wait for result, timeout, or cancellation
	select {
	case <-ctx.Done():
		return h.buildErrorResult(parsed, "shutdown", dns.RCodeServFail)
	case <-timer.C:
		return h.buildErrorResult(parsed, "timeout", dns.RCodeServFail)
	case r := <-resCh:
		if r.err != nil {
			return h.buildErrorResult(parsed, "servfail", dns.RCodeServFail)
		}
		return r.res
	}
}

// buildErrorResult builds an error response for a given parsed packet.
func (h *QueryHandler) buildErrorResult(parsed dns.Packet, source string, rcode dns.RCode) resolvers.Result {
	return resolvers.Result{
		ResponseBytes: mustMarshal(dns.BuildErrorResponse(parsed, uint16(rcode))),
		Source:        source,
	}
}

// logRequest logs DNS request details at debug level.
func (h *QueryHandler) logRequest(
	ctx context.Context,
	transport, src string,
	parsed dns.Packet,
	qname string,
	qtype int,
	reqLen int,
	source string,
) {
	if h.Logger == nil || !h.Logger.Enabled(ctx, slog.LevelDebug) {
		return
	}
	h.Logger.DebugContext(
		ctx,
		"dns request",
		"transport", transport,
		"src", src,
		"id", int(parsed.Header.ID),
		"qname", qname,
		"qtype", qtype,
		"bytes", reqLen,
		"source", source,
	)
}

// mustMarshal serializes a DNS packet, returning nil on error.
func mustMarshal(p dns.Packet) []byte {
	b, err := p.Marshal()
	if err != nil {
		return nil
	}
	return b
}

// tryBuildErrorFromRaw attempts to construct an error response from raw bytes.
// This is used when request parsing fails but we can still extract enough
// information (transaction ID, question) to build a valid error response.
//
// Returns nil if even the header cannot be parsed.
func tryBuildErrorFromRaw(reqBytes []byte, rcode uint16) []byte {
	off := 0
	h, err := dns.ParseHeader(reqBytes, &off)
	if err != nil {
		return nil
	}

	// Try to include the question in the error response
	var questions []dns.Question
	if h.QDCount > 0 {
		q, err := dns.ParseQuestion(reqBytes, &off)
		if err == nil {
			questions = make([]dns.Question, 1)
			questions[0] = q
		}
	}

	p := dns.Packet{Header: dns.Header{ID: h.ID, Flags: h.Flags}, Questions: questions}
	b, _ := dns.BuildErrorResponse(p, rcode).Marshal()
	return b
}
