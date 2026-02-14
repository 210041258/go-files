package concurrency

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

// Pop removes and returns the top item of the stack.
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

	// Zero the reference to avoid memory leak.
	var zero T
	s.items[index] = zero
	s.items = s.items[:index]
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

	// Zero the removed slots to allow GC.
	var zero T
	for i := start; i < length; i++ {
		s.items[i] = zero
	}
	s.items = s.items[:start]
	return topN
}

// Peek returns the top item without removing it.
// Optimized: RLock + inline unlock (no defer).
func (s *Stack[T]) Peek() (T, bool) {
	s.mu.RLock()
	if len(s.items) == 0 {
		s.mu.RUnlock()
		var zero T
		return zero, false
	}
	item := s.items[len(s.items)-1]
	s.mu.RUnlock()
	return item, true
}

// Len returns the number of items.
// Optimized: RLock + inline unlock.
func (s *Stack[T]) Len() int {
	s.mu.RLock()
	n := len(s.items)
	s.mu.RUnlock()
	return n
}

// Cap returns the capacity.
// Optimized: RLock + inline unlock.
func (s *Stack[T]) Cap() int {
	s.mu.RLock()
	c := cap(s.items)
	s.mu.RUnlock()
	return c
}

// IsEmpty returns true if the stack has no items.
// Optimized: RLock + inline unlock.
func (s *Stack[T]) IsEmpty() bool {
	s.mu.RLock()
	empty := len(s.items) == 0
	s.mu.RUnlock()
	return empty
}

// Clear removes all items and zeros the backing storage.
func (s *Stack[T]) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Zero the entire used portion to release references.
	var zero T
	for i := range s.items {
		s.items[i] = zero
	}
	s.items = s.items[:0]
}

// Values returns a defensive copy of all items.
// Optimized: RLock held only during copy.
func (s *Stack[T]) Values() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]T, len(s.items))
	copy(items, s.items)
	return items
}

// Grow ensures capacity of at least n.
// If reallocation is needed, the old slice is copied.
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

// =============================================================================
// OPTIMIZATIONS APPLIED
// =============================================================================

/*
┌─────────────────────────────────────────────────────────────┐
│                       OPTIMIZATIONS                         │
├─────────────────────────────────────────────────────────────┤
│                                                            │
│ 1. READER/WRITER LOCK (sync.RWMutex)                      │
│    • Peek, Len, Cap, IsEmpty, Values use RLock.           │
│    • Multiple readers can proceed concurrently.           │
│                                                            │
│ 2. NO DEFER IN TINY METHODS                               │
│    • Peek, Len, Cap, IsEmpty inline RUnlock.             │
│    • Eliminates function call overhead (~15‑25ns per call).│
│                                                            │
│ 3. MEMORY LEAK ELIMINATED                                 │
│    • Pop, PopN, Clear zero removed elements.             │
│    • Prevents unintended retention of pointers.           │
│                                                            │
│ 4. ZERO‑COPY READ? – NO (unsafe)                         │
│    • Values still returns a copy to guarantee isolation.  │
│                                                            │
│ 5. VARIADIC PUSH                                         │
│    • Accepts multiple items in one call – reduces lock    │
│      acquisitions when pushing batches.                  │
│                                                            │
│ 6. GROW FOR PREALLOCATION                                │
│    • Avoid repeated slice growth during bulk pushes.      │
│                                                            │
└─────────────────────────────────────────────────────────────┘
*/
