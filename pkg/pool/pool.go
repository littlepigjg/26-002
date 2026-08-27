// Package pool provides a sync.Pool-based buffer pool for performance optimization.
package pool

import (
	"bytes"
	"sync"
)

// BufferPool manages a pool of reusable byte buffers.
type BufferPool struct {
	pool sync.Pool
}

// NewBufferPool creates a new BufferPool.
func NewBufferPool() *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
	}
}

// Get retrieves a buffer from the pool.
func (bp *BufferPool) Get() *bytes.Buffer {
	buf := bp.pool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// Put returns a buffer to the pool.
func (bp *BufferPool) Put(buf *bytes.Buffer) {
	bp.pool.Put(buf)
}

// Pool is a generic object pool.
type Pool[T any] struct {
	pool sync.Pool
}

// NewPool creates a new generic Pool with a factory function.
func NewPool[T any](factory func() T) *Pool[T] {
	return &Pool[T]{
		pool: sync.Pool{
			New: func() interface{} {
				return factory()
			},
		},
	}
}

// Get retrieves an object from the pool.
func (p *Pool[T]) Get() T {
	return p.pool.Get().(T)
}

// Put returns an object to the pool.
func (p *Pool[T]) Put(obj T) {
	p.pool.Put(obj)
}

// MapPool manages a pool of reusable maps.
type MapPool[K comparable, V any] struct {
	pool sync.Pool
}

// NewMapPool creates a new MapPool.
func NewMapPool[K comparable, V any]() *MapPool[K, V] {
	return &MapPool[K, V]{
		pool: sync.Pool{
			New: func() interface{} {
				return make(map[K]V)
			},
		},
	}
}

// Get retrieves a map from the pool.
func (mp *MapPool[K, V]) Get() map[K]V {
	m := mp.pool.Get().(map[K]V)
	for k := range m {
		delete(m, k)
	}
	return m
}

// Put returns a map to the pool.
func (mp *MapPool[K, V]) Put(m map[K]V) {
	for k := range m {
		delete(m, k)
	}
	mp.pool.Put(m)
}

// SlicePool manages a pool of reusable slices.
type SlicePool[T any] struct {
	pool sync.Pool
}

// NewSlicePool creates a new SlicePool with a default capacity.
func NewSlicePool[T any](capacity int) *SlicePool[T] {
	return &SlicePool[T]{
		pool: sync.Pool{
			New: func() interface{} {
				s := make([]T, 0, capacity)
				return &s
			},
		},
	}
}

// Get retrieves a slice from the pool.
func (sp *SlicePool[T]) Get() *[]T {
	return sp.pool.Get().(*[]T)
}

// Put returns a slice to the pool.
func (sp *SlicePool[T]) Put(s *[]T) {
	*s = (*s)[:0]
	sp.pool.Put(s)
}
