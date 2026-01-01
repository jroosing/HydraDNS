package resolvers

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jroosing/hydradns/internal/dns"
)

// mockResolver is a test resolver that can be configured to succeed or fail.
type mockResolver struct {
	result Result
	err    error
	closed bool
}

func (m *mockResolver) Resolve(ctx context.Context, req dns.Packet, reqBytes []byte) (Result, error) {
	if m.err != nil {
		return Result{}, m.err
	}
	return m.result, nil
}

func (m *mockResolver) Close() error {
	m.closed = true
	return nil
}

func TestChainedResolveFirstSucceeds(t *testing.T) {
	r1 := &mockResolver{result: Result{ResponseBytes: []byte{1, 2, 3}, Source: "r1"}}
	r2 := &mockResolver{result: Result{ResponseBytes: []byte{4, 5, 6}, Source: "r2"}}

	chain := &Chained{Resolvers: []Resolver{r1, r2}}

	req := dns.Packet{Header: dns.Header{ID: 1234}}
	res, err := chain.Resolve(context.Background(), req, nil)
	require.NoError(t, err)
	assert.Equal(t, "r1", res.Source)
}

func TestChainedResolveFirstFailsSecondSucceeds(t *testing.T) {
	r1 := &mockResolver{err: errors.New("first failed")}
	r2 := &mockResolver{result: Result{ResponseBytes: []byte{4, 5, 6}, Source: "r2"}}

	chain := &Chained{Resolvers: []Resolver{r1, r2}}

	req := dns.Packet{Header: dns.Header{ID: 1234}}
	res, err := chain.Resolve(context.Background(), req, nil)
	require.NoError(t, err)
	assert.Equal(t, "r2", res.Source)
}

func TestChainedResolveAllFail(t *testing.T) {
	r1 := &mockResolver{err: errors.New("first failed")}
	r2 := &mockResolver{err: errors.New("second failed")}

	chain := &Chained{Resolvers: []Resolver{r1, r2}}

	req := dns.Packet{Header: dns.Header{ID: 1234}}
	_, err := chain.Resolve(context.Background(), req, nil)
	require.Error(t, err)
	assert.Equal(t, "second failed", err.Error())
}

func TestChainedResolveNoResolvers(t *testing.T) {
	chain := &Chained{Resolvers: nil}

	req := dns.Packet{Header: dns.Header{ID: 1234}}
	_, err := chain.Resolve(context.Background(), req, nil)
	require.Error(t, err)
}

func TestChainedResolveContextCancelled(t *testing.T) {
	r1 := &mockResolver{err: errors.New("first failed")}
	r2 := &mockResolver{result: Result{Source: "r2"}}

	chain := &Chained{Resolvers: []Resolver{r1, r2}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := dns.Packet{Header: dns.Header{ID: 1234}}
	_, err := chain.Resolve(ctx, req, nil)
	assert.Equal(t, context.Canceled, err)
}

func TestChainedClose(t *testing.T) {
	r1 := &mockResolver{}
	r2 := &mockResolver{}

	chain := &Chained{Resolvers: []Resolver{r1, r2}}
	err := chain.Close()
	require.NoError(t, err)

	assert.True(t, r1.closed)
	assert.True(t, r2.closed)
}

// mockFailingCloser returns an error on Close
type mockFailingCloser struct {
	mockResolver
	closeErr error
}

func (m *mockFailingCloser) Close() error {
	return m.closeErr
}

func TestChainedCloseWithError(t *testing.T) {
	r1 := &mockFailingCloser{closeErr: errors.New("close failed")}
	r2 := &mockResolver{}

	chain := &Chained{Resolvers: []Resolver{r1, r2}}
	err := chain.Close()
	require.Error(t, err)
	// Should still close r2
	assert.True(t, r2.closed)
}
