package testutils

import (
	"sync"
)

// Stack is a LIFO collection of generic items.
// The zero value is ready to use (no constructor required).
type Stack[T any] struct {
	items []T
	mu    sync.RWMutex
}

// NewStack creates a new empty Stack.
func NewStack[T any]() *Stack[T] {
	return &Stack[T]{}
}

// NewStackWithCapacity creates a Stack with a pre‑allocated backing array.
func NewStackWithCapacity[T any](capacity int) *Stack[T] {
	return &Stack[T]{
		items: make([]T, 0, capacity),
	}
}

// --------------------------------------------------------------------
// CORE OPERATIONS
// --------------------------------------------------------------------

// Push adds one or more items to the top.
// The last argument becomes the topmost item.
func (s *Stack[T]) Push(v ...T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, v...)
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

	idx := len(s.items) - 1
	item := s.items[idx]
	var zero T
	s.items[idx] = zero // prevent memory leak
	s.items = s.items[:idx]
	return item, true
}

// PopN removes and returns the top n items.
// Returns fewer than n if the stack has less than n items.
// The returned slice has the top item as the last element.
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
// Returns zero, false if the stack is empty.
func (s *Stack[T]) Peek() (T, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.items) == 0 {
		var zero T
		return zero, false
	}
	return s.items[len(s.items)-1], true
}

// Len returns the number of items in the stack.
func (s *Stack[T]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
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
// The first element is the bottom, the last is the top.
func (s *Stack[T]) Values() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()

	vals := make([]T, len(s.items))
	copy(vals, s.items)
	return vals
}

// --------------------------------------------------------------------
// CAPACITY MANAGEMENT
// --------------------------------------------------------------------

// Cap returns the current capacity of the underlying slice.
func (s *Stack[T]) Cap() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cap(s.items)
}

// Grow ensures that the capacity is at least n.
// If n is larger than the current capacity, a new backing slice is allocated.
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

// Reserve is an alias for Grow; it ensures enough capacity.
func (s *Stack[T]) Reserve(n int) {
	s.Grow(n)
}

// Shrink reduces the capacity to the current length.
// This frees unused memory. If the stack is empty, the backing array is released.
func (s *Stack[T]) Shrink() {
	s.mu.Lock()
	defer s.mu.Unlock()

	l := len(s.items)
	if l == 0 {
		s.items = nil // release entire backing array
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
// - If n < length, the stack is truncated (old items beyond n are lost).
// - If n > current capacity, the stack grows to at least n.
// - If n == current capacity, no allocation occurs.
func (s *Stack[T]) SetCapacity(n int) {
	if n < 0 {
		panic("SetCapacity: negative capacity")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// truncate if necessary
	if n < len(s.items) {
		var zero T
		for i := n; i < len(s.items); i++ {
			s.items[i] = zero
		}
		s.items = s.items[:n]
	}
	// reallocate if capacity differs
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

// --------------------------------------------------------------------
// COMPATIBILITY
// --------------------------------------------------------------------

// PushAll exists for backward compatibility. Use Push instead.
func (s *Stack[T]) PushAll(values []T) {
	s.Push(values...)
}
