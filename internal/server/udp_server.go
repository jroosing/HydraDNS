package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/jroosing/hydradns/internal/dns"
)

// bufferPool reduces allocations for incoming UDP packets.
// Each buffer is sized for the maximum expected DNS message.
var bufferPool = sync.Pool{
	New: func() any {
		buf := make([]byte, dns.MaxIncomingDNSMessageSize)
		return &buf
	},
}

// UDPServer handles DNS queries over UDP.
//
// Features:
//   - Buffer pooling to reduce GC pressure under load
//   - Semaphore-based concurrency limiting
//   - Rate limiting per source IP
//   - EDNS-aware response truncation
//   - Graceful shutdown with timeout
type UDPServer struct {
	Logger         *slog.Logger  // Optional logger
	Handler        *QueryHandler // Query processor
	Limiter        *RateLimiter  // Optional per-IP rate limiter
	MaxConcurrency int           // Maximum concurrent request handlers

	conn *net.UDPConn   // The UDP socket
	wg   sync.WaitGroup // Tracks in-flight requests
	sem  chan struct{}  // Concurrency semaphore
}

// Run starts the UDP server, listening on the given address.
func (s *UDPServer) Run(ctx context.Context, addr string) error {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}
	return s.RunOnConn(ctx, conn)
}

// RunOnConn runs the server on an existing UDP connection.
// This is useful for testing and when the caller manages the socket.
//
// Request processing flow:
//  1. Read packet from socket (with 1s timeout for shutdown checks)
//  2. Apply rate limiting (drop if exceeded)
//  3. Acquire semaphore slot (drop if at max concurrency)
//  4. Process request in goroutine
//  5. Truncate response if needed for EDNS buffer size
//  6. Send response
func (s *UDPServer) RunOnConn(ctx context.Context, conn *net.UDPConn) error {
	s.conn = conn
	defer conn.Close()

	maxConc := s.MaxConcurrency
	if maxConc <= 0 {
		maxConc = 1
	}
	s.sem = make(chan struct{}, maxConc)

	for {
		if ctx.Err() != nil {
			break
		}

		packet, remote, ok := s.receivePacket(ctx, conn)
		if !ok {
			continue
		}

		// Apply rate limiting
		if s.Limiter != nil && !s.Limiter.Allow(remote.IP.String()) {
			continue
		}

		// Try to acquire semaphore (non-blocking)
		if !s.tryAcquireSemaphore() {
			continue // at max concurrency, drop request
		}

		s.wg.Add(1)
		go s.handleRequest(ctx, conn, packet, remote)
	}

	return nil
}

// receivePacket reads a UDP packet using a pooled buffer.
// Returns the packet data and source address, or ok=false if no packet was received.
func (s *UDPServer) receivePacket(ctx context.Context, conn *net.UDPConn) ([]byte, *net.UDPAddr, bool) {
	bufPtr := bufferPool.Get().(*[]byte)
	buf := *bufPtr
	defer bufferPool.Put(bufPtr)

	_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	n, remote, err := conn.ReadFromUDP(buf)
	if err != nil {
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			return nil, nil, false // timeout, check context and retry
		}
		if ctx.Err() != nil {
			return nil, nil, false // server shutting down
		}
		return nil, nil, false
	}
	if remote == nil {
		return nil, nil, false
	}

	// Copy data out of pooled buffer
	data := make([]byte, n)
	copy(data, buf[:n])
	return data, remote, true
}

// tryAcquireSemaphore attempts to acquire a concurrency slot.
// Returns false if the server is at maximum concurrency.
func (s *UDPServer) tryAcquireSemaphore() bool {
	select {
	case s.sem <- struct{}{}:
		return true
	default:
		return false
	}
}

// handleRequest processes a single DNS request.
func (s *UDPServer) handleRequest(ctx context.Context, conn *net.UDPConn, payload []byte, peer *net.UDPAddr) {
	defer s.wg.Done()
	defer func() { <-s.sem }()

	if s.Handler == nil {
		return
	}

	res := s.Handler.Handle(ctx, "udp", peer.String(), payload)
	if len(res.ResponseBytes) == 0 {
		return
	}

	// Apply EDNS-aware truncation if we have EDNS info
	if res.ParsedOK {
		maxSize := s.calculateMaxResponseSize(res.Parsed)
		resBytes := truncateUDPResponse(res.ResponseBytes, maxSize)
		_, _ = conn.WriteToUDP(resBytes, peer)
		return
	}
	_, _ = conn.WriteToUDP(res.ResponseBytes, peer)
}

// calculateMaxResponseSize determines the maximum UDP response size
// based on the client's EDNS buffer size advertisement.
func (s *UDPServer) calculateMaxResponseSize(parsed dns.Packet) int {
	maxSize := dns.ClientMaxUDPSize(parsed)
	if maxSize > dns.EDNSMaxUDPPayloadSize {
		maxSize = dns.EDNSMaxUDPPayloadSize
	}
	return maxSize
}

// Stop gracefully shuts down the UDP server.
// Waits up to the specified timeout for in-flight requests to complete.
// Returns an error if the timeout is exceeded.
func (s *UDPServer) Stop(timeout time.Duration) error {
	if s.conn == nil {
		return nil
	}
	_ = s.conn.Close()

	if timeout <= 0 {
		s.wg.Wait()
		return nil
	}

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return errors.New("udp server: timeout waiting for in-flight requests")
	}
}
