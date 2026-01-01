package server

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetipAddrFromUDPAddr(t *testing.T) {
	tests := []struct {
		name     string
		addr     *net.UDPAddr
		expectOK bool
		expectIP string
	}{
		{
			name:     "IPv4",
			addr:     &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 12345},
			expectOK: true,
			expectIP: "192.168.1.1",
		},
		{
			name:     "IPv6",
			addr:     &net.UDPAddr{IP: net.ParseIP("2001:db8::1"), Port: 53},
			expectOK: true,
			expectIP: "2001:db8::1",
		},
		{
			name:     "IPv4-mapped IPv6",
			addr:     &net.UDPAddr{IP: net.ParseIP("::ffff:192.168.1.1"), Port: 12345},
			expectOK: true,
			expectIP: "192.168.1.1", // Should be unmapped to IPv4
		},
		{
			name:     "nil address",
			addr:     nil,
			expectOK: false,
		},
		{
			name:     "nil IP in address",
			addr:     &net.UDPAddr{IP: nil, Port: 12345},
			expectOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip, ok := netipAddrFromUDPAddr(tt.addr)
			assert.Equal(t, tt.expectOK, ok)
			if ok {
				assert.Equal(t, tt.expectIP, ip.String())
			}
		})
	}
}

func TestUDPServer_RunOnConn(t *testing.T) {
	// Create a UDP connection for testing
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	require.NoError(t, err, "ResolveUDPAddr failed")

	conn, err := net.ListenUDP("udp", addr)
	require.NoError(t, err, "ListenUDP failed")
	defer conn.Close()

	s := &UDPServer{
		WorkersPerSocket: 2, // Small for testing
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Run in goroutine
	done := make(chan error, 1)
	go func() {
		done <- s.RunOnConn(ctx, conn)
	}()

	// Wait for context to expire
	<-ctx.Done()

	select {
	case err := <-done:
		assert.NoError(t, err, "RunOnConn returned error")
	case <-time.After(time.Second):
		t.Error("timeout waiting for RunOnConn to finish")
	}
}

func TestUDPServer_Stop_NoConnections(t *testing.T) {
	s := &UDPServer{}

	// Should not panic or hang with no connections
	err := s.Stop(100 * time.Millisecond)
	assert.NoError(t, err, "Stop with no connections should not error")
}

func TestUDPServer_Stop_ZeroTimeout(t *testing.T) {
	s := &UDPServer{}

	// Should wait indefinitely with 0 timeout
	err := s.Stop(0)
	assert.NoError(t, err, "Stop with zero timeout should not error")
}

func TestUDPServer_DefaultWorkersPerSocket(t *testing.T) {
	s := &UDPServer{}

	// WorkersPerSocket defaults to 1024 in Run/RunOnConn
	assert.Equal(t, 0, s.WorkersPerSocket, "initial WorkersPerSocket should be 0 (unset)")

	// Create a connection and immediately cancel to test default setting
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	conn, _ := net.ListenUDP("udp", addr)
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	go s.RunOnConn(ctx, conn)
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, 1024, s.WorkersPerSocket, "WorkersPerSocket should default to 1024")
}

func TestUDPServer_HandlePacket_NilHandler(t *testing.T) {
	s := &UDPServer{
		Handler: nil, // No handler
	}

	// Should not panic with nil handler
	bufPtr := new([]byte)
	*bufPtr = make([]byte, 100)

	p := packet{
		bufPtr: bufPtr,
		n:      12,
		peer:   &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345},
	}

	// This should not panic
	ctx := context.Background()
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	conn, _ := net.ListenUDP("udp", addr)
	defer conn.Close()

	s.handlePacket(ctx, conn, p)
	// Test passes if no panic
}

func TestListenReusePort(t *testing.T) {
	// Test that listenReusePort creates a valid connection
	conn, err := listenReusePort("127.0.0.1:0")
	require.NoError(t, err, "listenReusePort failed")
	defer conn.Close()

	// Verify it's listening
	addr := conn.LocalAddr()
	assert.NotNil(t, addr, "expected non-nil local address")
}

func TestListenReusePort_InvalidAddress(t *testing.T) {
	_, err := listenReusePort("invalid:address::")
	assert.Error(t, err, "expected error for invalid address")
}

func TestListenReusePort_MultipleOnSamePort(t *testing.T) {
	// First connection
	conn1, err := listenReusePort("127.0.0.1:0")
	require.NoError(t, err, "first listenReusePort failed")
	defer conn1.Close()

	// Get the port
	port := conn1.LocalAddr().(*net.UDPAddr).Port

	// Second connection on same port should work due to SO_REUSEPORT
	addr := "127.0.0.1:" + itoa(port)
	conn2, err := listenReusePort(addr)
	if err != nil {
		// This might fail on some systems - that's okay
		t.Skipf("SO_REUSEPORT may not be fully supported: %v", err)
	}
	if conn2 != nil {
		defer conn2.Close()
	}
}

// itoa converts an int to a string without importing strconv
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits [20]byte
	i := len(digits)
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		digits[i] = '-'
	}
	return string(digits[i:])
}
