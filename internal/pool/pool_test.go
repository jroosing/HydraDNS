package pool_test

import (
	"sync"
	"testing"

	"github.com/jroosing/hydradns/internal/pool"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Pool Basic Operations Tests
// =============================================================================

func TestPool_GetAndPut(t *testing.T) {
	// Create a pool of byte slices
	bufPool := pool.New(func() []byte {
		return make([]byte, 1024)
	})

	// Get an item
	buf := bufPool.Get()
	assert.NotNil(t, buf)
	assert.Len(t, buf, 1024)

	// Put it back
	bufPool.Put(buf)

	// Get again - may or may not be the same item
	buf2 := bufPool.Get()
	assert.NotNil(t, buf2)
	assert.Len(t, buf2, 1024)
}

func TestPool_ConstructorCalled(t *testing.T) {
	callCount := 0
	p := pool.New(func() int {
		callCount++
		return callCount
	})

	// First get should call constructor
	v1 := p.Get()
	assert.Equal(t, 1, v1)
	assert.Equal(t, 1, callCount)

	// Second get should also call constructor (nothing put back yet)
	v2 := p.Get()
	assert.Equal(t, 2, v2)
	assert.Equal(t, 2, callCount)
}

func TestPool_ReusesItems(t *testing.T) {
	type item struct {
		id int
	}

	p := pool.New(func() *item {
		return &item{id: 1}
	})

	// Get and modify
	i1 := p.Get()
	i1.id = 42
	p.Put(i1)

	// Get again - should get the same item (pool reuses)
	i2 := p.Get()
	// Note: i2 may or may not be i1 depending on GC, but if it is, id should be 42
	assert.NotNil(t, i2)
}

// =============================================================================
// Pool Type Tests
// =============================================================================

func TestPool_WithStructType(t *testing.T) {
	type response struct {
		Data   []byte
		Status int
	}

	p := pool.New(func() *response {
		return &response{
			Data:   make([]byte, 512),
			Status: 0,
		}
	})

	r := p.Get()
	assert.NotNil(t, r)
	assert.Len(t, r.Data, 512)

	r.Status = 200
	p.Put(r)
}

func TestPool_WithSliceType(t *testing.T) {
	p := pool.New(func() []string {
		return make([]string, 0, 10)
	})

	s := p.Get()
	assert.NotNil(t, s)
	assert.Equal(t, 0, len(s))
	assert.Equal(t, 10, cap(s))
}

func TestPool_WithMapType(t *testing.T) {
	p := pool.New(func() map[string]int {
		return make(map[string]int)
	})

	m := p.Get()
	assert.NotNil(t, m)
	m["key"] = 1
	p.Put(m)
}

// =============================================================================
// Pool Concurrency Tests
// =============================================================================

func TestPool_ConcurrentAccess(t *testing.T) {
	p := pool.New(func() []byte {
		return make([]byte, 256)
	})

	var wg sync.WaitGroup
	const goroutines = 100
	const iterations = 1000

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				buf := p.Get()
				assert.NotNil(t, buf)
				// Simulate work
				buf[0] = 1
				p.Put(buf)
			}
		}()
	}

	wg.Wait()
}

// =============================================================================
// Pool Benchmarks
// =============================================================================

func BenchmarkPool_GetPut(b *testing.B) {
	p := pool.New(func() []byte {
		return make([]byte, 1024)
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := p.Get()
		p.Put(buf)
	}
}

func BenchmarkPool_Parallel(b *testing.B) {
	p := pool.New(func() []byte {
		return make([]byte, 1024)
	})

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := p.Get()
			p.Put(buf)
		}
	})
}
