package server

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/jroosing/hydradns/internal/pool"
)

// lenBufPool reduces allocations for TCP length prefix reads/writes.
// Each buffer is exactly 2 bytes for the DNS-over-TCP length field.
var lenBufPool = pool.New(func() *[]byte {
	buf := make([]byte, 2)
	return &buf
})

// TCP server configuration constants.
const (
	maxTCPMessageSize        = 65535            // Maximum DNS message size over TCP
	tcpReadTimeout           = 10 * time.Second // Read timeout per message
	tcpConnectionIdleTimeout = 30 * time.Second // Idle timeout for connection
	maxTCPConnectionsPerIP   = 10               // Max concurrent connections per IP
	maxQueriesPerConnection  = 100              // Max queries before closing connection
)

// TCPServer handles DNS queries over TCP with connection pipelining.
//
// Features:
//   - Per-IP connection limiting to prevent resource exhaustion
//   - Connection pipelining (multiple queries per connection)
//   - Idle timeout to free unused connections
//   - Graceful shutdown with timeout
//
// TCP DNS message format (RFC 1035 section 4.2.2):
// Each message is prefixed with a 2-byte big-endian length field.
type TCPServer struct {
	Logger  *slog.Logger  // Optional logger
	Handler *QueryHandler // Query processor

	ln net.Listener // The TCP listener

	wg sync.WaitGroup // Tracks active connections

	mu        sync.Mutex     // Protects connPerIP
	connPerIP map[string]int // Connection count per IP address
}

// Run starts the TCP server, listening on the given address.
func (s *TCPServer) Run(ctx context.Context, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.ln = ln
	defer ln.Close()

	s.mu.Lock()
	if s.connPerIP == nil {
		s.connPerIP = map[string]int{}
	}
	s.mu.Unlock()

	// Close listener when context is cancelled
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		c, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			return err
		}

		remoteIP := remoteIPString(c.RemoteAddr())

		// Enforce per-IP connection limit
		if !s.tryAcquireConn(remoteIP) {
			if s.Logger != nil {
				s.Logger.Warn("tcp connection limit exceeded", "ip", remoteIP)
			}
			_ = c.Close()
			continue
		}

		s.wg.Add(1)
		go s.handleConnection(ctx, c, remoteIP)
	}

	return nil
}

// handleConnection processes DNS queries on a single TCP connection.
// Supports pipelining: multiple queries can be sent on the same connection.
func (s *TCPServer) handleConnection(ctx context.Context, conn net.Conn, ip string) {
	defer s.wg.Done()
	defer s.releaseConn(ip)
	defer conn.Close()

	// Set initial idle timeout
	_ = conn.SetDeadline(time.Now().Add(tcpConnectionIdleTimeout))

	for range maxQueriesPerConnection {
		if ctx.Err() != nil {
			return
		}

		msg, ok := s.readMessage(conn)
		if !ok {
			return
		}
		if len(msg) == 0 {
			continue // empty message, try next
		}

		// Reset idle timeout after activity
		_ = conn.SetDeadline(time.Now().Add(tcpConnectionIdleTimeout))

		if s.Handler == nil {
			return
		}

		res := s.Handler.Handle(ctx, "tcp", conn.RemoteAddr().String(), msg)
		if len(res.ResponseBytes) == 0 {
			continue
		}

		if !s.writeMessage(conn, res.ResponseBytes) {
			return
		}
	}
}

// readMessage reads a length-prefixed DNS message from the connection.
// Returns nil, false on error or if the message is too large.
//
// Wire format:
//
//	+--+--+
//	|Length| 2 bytes, big-endian
//	+--+--+
//	| DNS  | Length bytes
//	+------+
func (s *TCPServer) readMessage(conn net.Conn) ([]byte, bool) {
	// Read 2-byte length prefix using pooled buffer
	_ = conn.SetReadDeadline(time.Now().Add(tcpReadTimeout))
	lenBufPtr := lenBufPool.Get()
	lenBuf := *lenBufPtr
	_, err := io.ReadFull(conn, lenBuf)
	if err != nil {
		lenBufPool.Put(lenBufPtr)
		return nil, false
	}
	msgLen := int(binary.BigEndian.Uint16(lenBuf))
	lenBufPool.Put(lenBufPtr)

	// Validate message length
	if msgLen == 0 {
		return nil, true // empty message
	}
	if msgLen > maxTCPMessageSize {
		return nil, false // message too large
	}

	// Read message body
	_ = conn.SetReadDeadline(time.Now().Add(tcpReadTimeout))
	msg := make([]byte, msgLen)
	if _, err := io.ReadFull(conn, msg); err != nil {
		return nil, false
	}
	return msg, true
}

// writeMessage writes a length-prefixed DNS message to the connection.
// Uses two writes to avoid allocating a combined buffer.
// Returns false on error.
func (s *TCPServer) writeMessage(conn net.Conn, response []byte) bool {
	respLen := len(response)
	if respLen > maxTCPMessageSize {
		return false
	}

	_ = conn.SetWriteDeadline(time.Now().Add(tcpReadTimeout))

	// Write length prefix and message body using writev (net.Buffers)
	lenBufPtr := lenBufPool.Get()
	lenBuf := *lenBufPtr
	binary.BigEndian.PutUint16(lenBuf, uint16(respLen))

	bufs := net.Buffers{lenBuf, response}
	_, err := bufs.WriteTo(conn)

	lenBufPool.Put(lenBufPtr)
	return err == nil
}

// Stop gracefully shuts down the TCP server.
// Waits up to the specified timeout for connections to close.
func (s *TCPServer) Stop(timeout time.Duration) error {
	if s.ln != nil {
		_ = s.ln.Close()
	}
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
		return errors.New("tcp server: timeout waiting for connections")
	}
}

// remoteIPString extracts the IP address from a network address.
// Used for per-IP connection tracking.
func remoteIPString(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	// Parse as host:port to extract just the IP
	host, _, err := net.SplitHostPort(addr.String())
	if err == nil {
		return host
	}
	return addr.String()
}

// tryAcquireConn attempts to increment the connection count for an IP.
// Returns false if the limit would be exceeded.
func (s *TCPServer) tryAcquireConn(ip string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	cur := s.connPerIP[ip]
	if cur >= maxTCPConnectionsPerIP {
		return false
	}
	s.connPerIP[ip] = cur + 1
	return true
}

// releaseConn decrements the connection count for an IP.
func (s *TCPServer) releaseConn(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cur := s.connPerIP[ip]
	if cur <= 1 {
		delete(s.connPerIP, ip)
		return
	}
	s.connPerIP[ip] = cur - 1
}
