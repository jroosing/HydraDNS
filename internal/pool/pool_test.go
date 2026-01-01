package pool

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPool_GetPut(t *testing.T) {
	callCount := 0
	p := New(func() *int {
		callCount++
		v := 42
		return &v
	})

	// First Get should create a new item
	item1 := p.Get()
	require.NotNil(t, item1, "expected non-nil item from Get")
	assert.Equal(t, 42, *item1)

	// Put the item back
	p.Put(item1)

	// Second Get might return the same item (pooled) or create new
	item2 := p.Get()
	require.NotNil(t, item2, "expected non-nil item from second Get")
}

func TestPool_ConcurrentAccess(t *testing.T) {
	p := New(func() []byte {
		return make([]byte, 1024)
	})

	var wg sync.WaitGroup
	const goroutines = 100
	const iterations = 100

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				buf := p.Get()
				assert.Len(t, buf, 1024)
				// Simulate some work
				buf[0] = byte(j)
				p.Put(buf)
			}
		}()
	}

	wg.Wait()
}

func TestPool_DifferentTypes(t *testing.T) {
	t.Run("string pool", func(t *testing.T) {
		p := New(func() string {
			return "default"
		})
		s := p.Get()
		assert.Equal(t, "default", s)
		p.Put("custom")
	})

	t.Run("struct pool", func(t *testing.T) {
		type Item struct {
			ID   int
			Name string
		}
		p := New(func() *Item {
			return &Item{ID: 0, Name: "new"}
		})
		item := p.Get()
		assert.Equal(t, "new", item.Name)
		item.ID = 123
		item.Name = "modified"
		p.Put(item)
	})
}
