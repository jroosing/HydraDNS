package pool

import "sync"

// Pool is a generic wrapper around sync.Pool.
type Pool[T any] struct {
	internal sync.Pool
}

// New creates a new Pool with the given constructor.
func New[T any](newFn func() T) *Pool[T] {
	return &Pool[T]{
		internal: sync.Pool{
			New: func() any {
				return newFn()
			},
		},
	}
}

// Get retrieves an item from the pool.
func (p *Pool[T]) Get() T {
	return p.internal.Get().(T)
}

// Put returns an item to the pool.
func (p *Pool[T]) Put(item T) {
	p.internal.Put(item)
}
