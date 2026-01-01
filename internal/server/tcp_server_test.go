package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTCPServer_remoteIPString(t *testing.T) {
	tests := []struct {
		name     string
		addr     net.Addr
		expected string
	}{
		{
			name:     "TCP address",
			addr:     &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 12345},
			expected: "192.168.1.1",
		},
		{
			name:     "IPv6 TCP address",
			addr:     &net.TCPAddr{IP: net.ParseIP("::1"), Port: 12345},
			expected: "::1",
		},
		{
			name:     "nil address",
			addr:     nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := remoteIPString(tt.addr)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestTCPServer_tryAcquireConn(t *testing.T) {
	s := &TCPServer{
		connPerIP: map[string]int{},
	}

	ip := "192.168.1.1"

	// Should be able to acquire up to max connections
	for i := 0; i < maxTCPConnectionsPerIP; i++ {
		assert.True(t, s.tryAcquireConn(ip), "should be able to acquire connection %d", i+1)
	}

	// Should not be able to acquire one more
	assert.False(t, s.tryAcquireConn(ip), "should not be able to exceed max connections per IP")
}

func TestTCPServer_releaseConn(t *testing.T) {
	s := &TCPServer{
		connPerIP: map[string]int{"192.168.1.1": 5},
	}

	ip := "192.168.1.1"

	// Release connections
	s.releaseConn(ip)
	assert.Equal(t, 4, s.connPerIP[ip], "expected 4 connections after release")

	// Release all
	for i := 0; i < 4; i++ {
		s.releaseConn(ip)
	}

	// Should be removed from map when count reaches 0
	_, exists := s.connPerIP[ip]
	assert.False(t, exists, "IP should be removed from map when count reaches 0")
}

func TestTCPServer_readMessage(t *testing.T) {
	s := &TCPServer{}

	// Test with a valid DNS-over-TCP message
	dnsMsg := []byte{0x12, 0x34, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint16(len(dnsMsg)))
	buf.Write(dnsMsg)

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		client.Write(buf.Bytes())
	}()

	msg, ok := s.readMessage(server)
	require.True(t, ok, "readMessage returned not ok")
	assert.Equal(t, dnsMsg, msg, "message mismatch")
}

func TestTCPServer_readMessage_EmptyMessage(t *testing.T) {
	s := &TCPServer{}

	// Length 0
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint16(0))

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		client.Write(buf.Bytes())
	}()

	msg, ok := s.readMessage(server)
	assert.True(t, ok, "readMessage should return ok=true for empty message")
	assert.Nil(t, msg, "expected nil message for empty")
}

func TestTCPServer_readMessage_TooLarge(t *testing.T) {
	s := &TCPServer{}

	// Message larger than max (65535+1)
	// We can't actually send a message this large, but we can test the length check
	var buf bytes.Buffer
	// maxTCPMessageSize is 65535, so any message with length > 65535 should fail
	// Since we're testing readMessage which checks the length, just verify it handles
	// the case where the connection is closed before full read
	binary.Write(&buf, binary.BigEndian, uint16(100))

	client, server := net.Pipe()
	defer server.Close()

	go func() {
		client.Write(buf.Bytes()) // only write length, not body
		client.Close()            // close before body is written
	}()

	_, ok := s.readMessage(server)
	assert.False(t, ok, "readMessage should return ok=false when body read fails")
}

func TestTCPServer_writeMessage(t *testing.T) {
	s := &TCPServer{}

	response := []byte{0x12, 0x34, 0x81, 0x80, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00}

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	done := make(chan []byte, 1)
	go func() {
		// Read length prefix
		lenBuf := make([]byte, 2)
		io.ReadFull(client, lenBuf)
		msgLen := binary.BigEndian.Uint16(lenBuf)

		// Read message body
		msg := make([]byte, msgLen)
		io.ReadFull(client, msg)
		done <- msg
	}()

	ok := s.writeMessage(server, response)
	assert.True(t, ok, "writeMessage returned false")

	select {
	case msg := <-done:
		assert.Equal(t, response, msg, "message mismatch")
	case <-time.After(time.Second):
		t.Error("timeout waiting for message")
	}
}

func TestTCPServer_Stop_NoListener(t *testing.T) {
	s := &TCPServer{}

	// Should not panic with nil listener
	err := s.Stop(100 * time.Millisecond)
	assert.NoError(t, err, "Stop with no listener should not error")
}

func TestTCPServer_Stop_ZeroTimeout(t *testing.T) {
	s := &TCPServer{}

	// Should wait indefinitely with 0 timeout
	// Just verify it doesn't hang or panic when there are no connections
	err := s.Stop(0)
	assert.NoError(t, err, "Stop with zero timeout should not error")
}

func TestTCPServer_Run_InvalidAddress(t *testing.T) {
	s := &TCPServer{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Invalid address should fail
	err := s.Run(ctx, "invalid:address:format::")
	assert.Error(t, err, "expected error for invalid address")
}
