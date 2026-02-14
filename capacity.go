package testutils

import (
	"sync"
)

// Stack is a Last-In-First-Out (LIFO) collection of generic items.
// It is thread-safe and suitable for concurrent use.
// The zero value is ready to use.
type Stack[T any] struct {
	items []T
	mu    sync.RWMutex
}

// NewStack creates a new empty Stack.
func NewStack[T any]() *Stack[T] {
	return &Stack[T]{}
}

// NewStackWithCapacity creates a Stack with a pre-allocated backing array.
func NewStackWithCapacity[T any](capacity int) *Stack[T] {
	return &Stack[T]{
		items: make([]T, 0, capacity),
	}
}

// Push adds one or more items to the top of the stack.
// The last argument becomes the topmost item.
func (s *Stack[T]) Push(v ...T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, v...)
}

// PushAll exists for compatibility. Use Push.
func (s *Stack[T]) PushAll(values []T) {
	s.Push(values...)
}

// Pop removes and returns the top item.
// Returns zero, false if the stack is empty.
func (s *Stack[T]) Pop() (T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.items) == 0 {
		var zero T
		return zero, false
	}

	index := len(s.items) - 1
	item := s.items[index]
	var zero T
	s.items[index] = zero
	s.items = s.items[:index]
	return item, true
}

// PopN removes and returns the top n items.
// Returns fewer than n if the stack has less than n items.
func (s *Stack[T]) PopN(n int) []T {
	s.mu.Lock()
	defer s.mu.Unlock()

	length := len(s.items)
	if length == 0 || n <= 0 {
		return nil
	}
	if n > length {
		n = length
	}

	start := length - n
	topN := make([]T, n)
	copy(topN, s.items[start:])

	var zero T
	for i := start; i < length; i++ {
		s.items[i] = zero
	}
	s.items = s.items[:start]
	return topN
}

// Peek returns the top item without removing it.
func (s *Stack[T]) Peek() (T, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.items) == 0 {
		var zero T
		return zero, false
	}
	return s.items[len(s.items)-1], true
}

// Len returns the number of items.
func (s *Stack[T]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

// Cap returns the capacity of the underlying slice.
func (s *Stack[T]) Cap() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cap(s.items)
}

// IsEmpty returns true if the stack has no items.
func (s *Stack[T]) IsEmpty() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items) == 0
}

// Clear removes all items and zeros the backing storage.
func (s *Stack[T]) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	var zero T
	for i := range s.items {
		s.items[i] = zero
	}
	s.items = s.items[:0]
}

// Values returns a defensive copy of all items.
func (s *Stack[T]) Values() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]T, len(s.items))
	copy(items, s.items)
	return items
}

// --------------------------------------------------------------------
// CAPACITY MANAGEMENT
// --------------------------------------------------------------------

// Grow ensures that the stack has at least the specified capacity.
// If the current capacity is less than n, it reallocates.
// This does not change the stack length.
func (s *Stack[T]) Grow(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if n <= cap(s.items) {
		return
	}
	newItems := make([]T, len(s.items), n)
	copy(newItems, s.items)
	s.items = newItems
}

// Reserve is an alias for Grow. It ensures enough capacity.
func (s *Stack[T]) Reserve(n int) {
	s.Grow(n)
}

// Shrink reduces the capacity to the current length.
// This frees unused memory. It is a no‑op if the stack is empty.
func (s *Stack[T]) Shrink() {
	s.mu.Lock()
	defer s.mu.Unlock()

	l := len(s.items)
	if l == 0 {
		// Replace with nil to release the backing array entirely.
		s.items = nil
		return
	}
	if l == cap(s.items) {
		return
	}
	newItems := make([]T, l)
	copy(newItems, s.items)
	s.items = newItems
}

// SetCapacity ensures that the capacity is exactly n.
// If n < length, the stack is truncated (old items beyond n are lost).
// If n > current capacity, the stack grows to at least n.
// This is a combination of Truncate and Grow/Shrink.
func (s *Stack[T]) SetCapacity(n int) {
	if n < 0 {
		panic("SetCapacity: negative capacity")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// Truncate if necessary
	if n < len(s.items) {
		var zero T
		for i := n; i < len(s.items); i++ {
			s.items[i] = zero
		}
		s.items = s.items[:n]
	}
	// Reallocate if capacity differs
	if cap(s.items) != n {
		newItems := make([]T, len(s.items), n)
		copy(newItems, s.items)
		s.items = newItems
	}
}

// Truncate reduces the stack length to n.
// If n >= current length, it does nothing.
// Removed elements are zeroed to allow GC.
func (s *Stack[T]) Truncate(n int) {
	if n < 0 {
		panic("Truncate: negative length")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	length := len(s.items)
	if n >= length {
		return
	}
	var zero T
	for i := n; i < length; i++ {
		s.items[i] = zero
	}
	s.items = s.items[:n]
}

// =============================================================================
// CAPACITY STRATEGIES & BEST PRACTICES
// =============================================================================

/*
┌─────────────────────────────────────────────────────────────────┐
│                      CAPACITY MANAGEMENT                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│ 1. Preallocation:                                              │
│    Use NewStackWithCapacity(c) or s.Grow(c) when the maximum   │
│    stack depth is known. This avoids repeated slice growth.    │
│                                                                 │
│ 2. Memory Release:                                             │
│    After a large stack is cleared, call s.Shrink() to free the │
│    backing array. This is important in long‑running services.  │
│                                                                 │
│ 3. Truncation:                                                 │
│    s.Truncate(n) is faster than PopN(len-n) because it         │
│    zeroes and discards in one operation with a single lock.    │
│                                                                 │
│ 4. Exact Capacity:                                             │
│    s.SetCapacity(n) ensures the underlying slice has exactly   │
│    n slots. Useful for pooling or when memory must be tightly  │
│    controlled.                                                 │
│                                                                 │
│ 5. Zero‑Value Usability:                                       │
│    var s Stack[T] – items is nil, Len=0, Cap=0.                │
│    Push, Grow, etc. all work without initialization.           │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
*/
