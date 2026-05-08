package ringbuf

import "sync"

type RingBuffer[T any] struct {
	mu    sync.RWMutex
	items []T
	cap   int
	head  int // next write position
	size  int
}

func New[T any](capacity int) *RingBuffer[T] {
	return &RingBuffer[T]{
		items: make([]T, capacity),
		cap:   capacity,
	}
}

func (r *RingBuffer[T]) Push(item T) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.items[r.head] = item
	r.head = (r.head + 1) % r.cap
	if r.size < r.cap {
		r.size++
	}
}

func (r *RingBuffer[T]) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.size
}

func (r *RingBuffer[T]) Cap() int {
	return r.cap
}

// Get returns the item at logical index i (0 = oldest).
func (r *RingBuffer[T]) Get(i int) T {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var zero T
	if i < 0 || i >= r.size {
		return zero
	}
	actual := r.physicalIndex(i)
	return r.items[actual]
}

// Slice returns a copy of items from logical index start to end (exclusive).
func (r *RingBuffer[T]) Slice(start, end int) []T {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if start < 0 {
		start = 0
	}
	if end > r.size {
		end = r.size
	}
	if start >= end {
		return nil
	}

	result := make([]T, end-start)
	for i := start; i < end; i++ {
		result[i-start] = r.items[r.physicalIndex(i)]
	}
	return result
}

// All returns a copy of all items in order (oldest first).
func (r *RingBuffer[T]) All() []T {
	return r.Slice(0, r.Len())
}

// ForEach iterates items in logical order (oldest first) without allocating.
// Returning false from fn stops iteration early.
func (r *RingBuffer[T]) ForEach(fn func(item T) bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for i := 0; i < r.size; i++ {
		if !fn(r.items[r.physicalIndex(i)]) {
			return
		}
	}
}

// Last returns the most recent item.
func (r *RingBuffer[T]) Last() (T, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var zero T
	if r.size == 0 {
		return zero, false
	}
	return r.items[r.physicalIndex(r.size-1)], true
}

func (r *RingBuffer[T]) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.head = 0
	r.size = 0
}

func (r *RingBuffer[T]) physicalIndex(logicalIndex int) int {
	if r.size < r.cap {
		return logicalIndex
	}
	return (r.head + logicalIndex) % r.cap
}
