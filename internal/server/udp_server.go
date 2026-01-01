package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"runtime"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/jroosing/hydradns/internal/dns"
	"github.com/jroosing/hydradns/internal/pool"
)

// Socket buffer sizes for high throughput (4MB each).
const (
	socketRecvBufferSize = 4 * 1024 * 1024
	socketSendBufferSize = 4 * 1024 * 1024
)

// bufferPool reduces allocations for incoming UDP packets.
// Each buffer is sized for the maximum expected DNS message.
var bufferPool = pool.New(func() *[]byte {
	buf := make([]byte, dns.MaxIncomingDNSMessageSize)
	return &buf
})

// UDPServer handles DNS queries over UDP.
//
// Features:
//   - Multiple sockets with SO_REUSEPORT for kernel-level load balancing
//   - Fixed worker pool per socket (no goroutine spawn per packet)
//   - Buffer pooling to reduce GC pressure under load
//   - Non-blocking receive path (drops packets if workers are busy)
//   - Rate limiting per source IP (using netip.Addr to avoid allocations)
//   - EDNS-aware response truncation
//   - Graceful shutdown with timeout
//   - Large socket buffers for burst handling
type UDPServer struct {
	Logger           *slog.Logger  // Optional logger
	Handler          *QueryHandler // Query processor
	Limiter          *RateLimiter  // Optional per-IP rate limiter
	WorkersPerSocket int           // Worker goroutines per socket (default 1024)

	conns []*net.UDPConn // UDP sockets (one per CPU core)
	wg    sync.WaitGroup // Tracks receiver and worker goroutines
}

// packet represents a received UDP packet pending processing.
type packet struct {
	bufPtr *[]byte
	n      int
	peer   *net.UDPAddr
}

// Run starts the UDP server with multiple sockets using SO_REUSEPORT.
// Each socket has its own fixed pool of worker goroutines.
func (s *UDPServer) Run(ctx context.Context, addr string) error {
	if s.WorkersPerSocket <= 0 {
		s.WorkersPerSocket = 1024
	}

	socketCount := runtime.NumCPU()
	s.conns = make([]*net.UDPConn, 0, socketCount)

	for range socketCount {
		conn, err := listenReusePort(addr)
		if err != nil {
			// Close any already-opened sockets
			for _, c := range s.conns {
				_ = c.Close()
			}
			return err
		}

		// Set large socket buffers for burst handling
		_ = conn.SetReadBuffer(socketRecvBufferSize)
		_ = conn.SetWriteBuffer(socketSendBufferSize)

		s.conns = append(s.conns, conn)

		// Buffered channel for packet handoff (2x workers for headroom)
		packetCh := make(chan packet, s.WorkersPerSocket*2)

		// Receiver goroutine (never blocks on worker availability)
		s.wg.Add(1)
		go s.recvLoop(ctx, conn, packetCh)

		// Fixed worker pool for this socket
		for range s.WorkersPerSocket {
			s.wg.Add(1)
			go s.workerLoop(ctx, conn, packetCh)
		}
	}

	<-ctx.Done()
	return s.Stop(5 * time.Second)
}

// RunOnConn runs the server on an existing UDP connection.
// This is useful for testing and when the caller manages the socket.
func (s *UDPServer) RunOnConn(ctx context.Context, conn *net.UDPConn) error {
	if s.WorkersPerSocket <= 0 {
		s.WorkersPerSocket = 1024
	}

	s.conns = []*net.UDPConn{conn}
	packetCh := make(chan packet, s.WorkersPerSocket)

	s.wg.Add(1)
	go s.recvLoop(ctx, conn, packetCh)

	for range s.WorkersPerSocket {
		s.wg.Add(1)
		go s.workerLoop(ctx, conn, packetCh)
	}

	<-ctx.Done()
	return nil
}

// recvLoop reads packets from the socket and dispatches to workers.
// Never blocks on worker availability; drops packets if all workers are busy.
func (s *UDPServer) recvLoop(ctx context.Context, conn *net.UDPConn, out chan<- packet) {
	defer s.wg.Done()

	for {
		bufPtr := bufferPool.Get()
		buf := *bufPtr

		n, peer, err := conn.ReadFromUDP(buf)
		if err != nil {
			bufferPool.Put(bufPtr)
			// Check if we're shutting down
			if ctx.Err() != nil {
				return
			}
			// Socket closed or other error
			return
		}

		// Apply rate limiting using netip.Addr to avoid string allocation
		if s.Limiter != nil {
			ip, ok := netipAddrFromUDPAddr(peer)
			if !ok || !s.Limiter.AllowAddr(ip) {
				bufferPool.Put(bufPtr)
				continue
			}
		}

		// Non-blocking dispatch to worker pool
		select {
		case out <- packet{bufPtr, n, peer}:
			// Successfully queued
		default:
			// All workers busy, drop packet to keep receive path fast
			bufferPool.Put(bufPtr)
		}
	}
}

// workerLoop processes packets from the channel.
func (s *UDPServer) workerLoop(ctx context.Context, conn *net.UDPConn, in <-chan packet) {
	defer s.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case pkt, ok := <-in:
			if !ok {
				return
			}
			s.handlePacket(ctx, conn, pkt)
		}
	}
}

// handlePacket processes a single DNS request.
func (s *UDPServer) handlePacket(ctx context.Context, conn *net.UDPConn, p packet) {
	defer bufferPool.Put(p.bufPtr)

	if s.Handler == nil {
		return
	}

	payload := (*p.bufPtr)[:p.n]
	res := s.Handler.Handle(ctx, "udp", p.peer.String(), payload)
	if len(res.ResponseBytes) == 0 {
		return
	}

	// Apply EDNS-aware truncation if we have EDNS info
	resp := res.ResponseBytes
	if res.ParsedOK {
		maxSize := min(dns.ClientMaxUDPSize(res.Parsed), dns.EDNSMaxUDPPayloadSize)
		resp = truncateUDPResponse(resp, maxSize)
	}

	_, _ = conn.WriteToUDP(resp, p.peer)
}

// Stop gracefully shuts down the UDP server.
// Closes all sockets and waits up to the specified timeout for goroutines to exit.
func (s *UDPServer) Stop(timeout time.Duration) error {
	// Close all sockets to unblock receive loops
	for _, c := range s.conns {
		_ = c.Close()
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
		return errors.New("udp server: timeout waiting for goroutines to exit")
	}
}

// netipAddrFromUDPAddr extracts a netip.Addr from a net.UDPAddr without allocation.
func netipAddrFromUDPAddr(addr *net.UDPAddr) (netip.Addr, bool) {
	if addr == nil {
		return netip.Addr{}, false
	}
	ip, ok := netip.AddrFromSlice(addr.IP)
	if !ok {
		return netip.Addr{}, false
	}
	return ip.Unmap(), true
}

// listenReusePort creates a UDP socket with SO_REUSEPORT enabled.
// This allows multiple sockets to bind to the same address, with the kernel
// distributing incoming packets across them for better multi-core scalability.
func listenReusePort(addr string) (*net.UDPConn, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				_ = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
			})
		},
	}

	pc, err := lc.ListenPacket(context.Background(), "udp", udpAddr.String())
	if err != nil {
		return nil, err
	}

	return pc.(*net.UDPConn), nil
}
