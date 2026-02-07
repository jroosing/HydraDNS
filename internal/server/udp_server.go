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

// DefaultWorkersPerSocket is the default number of worker goroutines per UDP socket.
const DefaultWorkersPerSocket = 1024

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
//
// Goroutine Lifecycle:
//
// For each CPU core, Run() spawns:
//   - 1 receiver goroutine: Reads incoming UDP packets from socket
//   - N worker goroutines: Process packets and write responses (N = WorkersPerSocket)
//
// All goroutines share the same context and exit when it is cancelled.
// Graceful shutdown waits up to 5 seconds for in-flight queries.
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
//
// Goroutine Behavior:
//   - Spawns 1 receiver + N workers per CPU core (total = NumCPU * (1 + WorkersPerSocket))
//   - All goroutines read context and exit when ctx is cancelled
//   - Close() or context cancellation triggers graceful shutdown
//
// Returns error only if socket creation fails. Otherwise blocks until shutdown.
func (s *UDPServer) Run(ctx context.Context, addr string) error {
	if s.WorkersPerSocket <= 0 {
		s.WorkersPerSocket = DefaultWorkersPerSocket
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
		c := conn
		ch := packetCh

		// Receiver goroutine (never blocks on worker availability)
		s.wg.Go(func() {
			s.recvLoop(ctx, c, ch)
		})

		// Fixed worker pool for this socket
		for range s.WorkersPerSocket {
			s.wg.Go(func() {
				s.workerLoop(ctx, c, ch)
			})
		}
	}

	<-ctx.Done()
	return s.Stop(5 * time.Second)
}

// RunOnConn runs the server on an existing UDP connection.
// This is useful for testing and when the caller manages the socket.
func (s *UDPServer) RunOnConn(ctx context.Context, conn *net.UDPConn) error {
	if s.WorkersPerSocket <= 0 {
		s.WorkersPerSocket = DefaultWorkersPerSocket
	}

	s.conns = []*net.UDPConn{conn}
	packetCh := make(chan packet, s.WorkersPerSocket)
	c := conn
	ch := packetCh

	s.wg.Go(func() {
		s.recvLoop(ctx, c, ch)
	})

	for range s.WorkersPerSocket {
		s.wg.Go(func() {
			s.workerLoop(ctx, c, ch)
		})
	}

	<-ctx.Done()
	return nil
}

// recvLoop reads packets from the socket and dispatches to workers.
// Never blocks on worker availability; drops packets if all workers are busy.
//
// Goroutine lifecycle: Started in Run() for each UDP socket, exits when:
// - Context is cancelled (server shutdown)
// - Socket is closed
// Cleanup: Returns buffers to pool, socket closed by caller.
func (s *UDPServer) recvLoop(ctx context.Context, conn *net.UDPConn, out chan<- packet) {
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
//
// Goroutine lifecycle: WorkersPerSocket instances started per UDP socket in Run().
// Exits when:
// - Context is cancelled (server shutdown)
// - Packet channel is closed
// Cleanup: Returns packet buffers to pool after processing.
func (s *UDPServer) workerLoop(ctx context.Context, conn *net.UDPConn, in <-chan packet) {
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
	// Extract IP from peer address to avoid String() allocation
	peerIP := p.peer.IP.String()
	res := s.Handler.Handle(ctx, "udp", peerIP, payload)
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
//
// SO_REUSEPORT Scalability:
//
// SO_REUSEPORT is a Linux kernel feature that allows multiple sockets to bind
// to the same address and port. The kernel distributes incoming packets across
// all bound sockets, load-balancing without requiring userspace coordination.
//
// Benefits:
//   - Each CPU core can have its own socket (no lock contention on single socket)
//   - Kernel handles packet distribution fairly
//   - Eliminates thundering herd (all goroutines waking on single packet)
//   - Achieves near-linear scaling with CPU cores
//
// Implementation:
//
// HydraDNS creates one UDP socket per CPU core, each with WorkersPerSocket
// goroutines handling packets. This gives optimal throughput on multi-core systems.
//
// Large Socket Buffers:
//
// Each socket has 4MB send and receive buffers for burst handling. This allows
// the kernel to queue incoming packets while userspace is busy processing.
func listenReusePort(addr string) (*net.UDPConn, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	lc := net.ListenConfig{
		Control: func(_, _ string, c syscall.RawConn) error {
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
