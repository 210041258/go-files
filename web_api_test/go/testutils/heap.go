// Package heap provides generic heap (priority queue) data structures.
package testutils

import (
    "container/heap"
    "sync"
)

// Lessable is an interface that items must implement to be comparable in a heap.
type Lessable interface {
    Less(other interface{}) bool // returns true if current item has higher priority (for min-heap)
}

// Heap is a generic min-heap (priority queue) of items that satisfy Lessable.
type Heap[T Lessable] struct {
    items []T
}

// New creates a new empty heap.
func New[T Lessable]() *Heap[T] {
    return &Heap[T]{}
}

// Len returns the number of items in the heap.
func (h *Heap[T]) Len() int { return len(h.items) }

// Push adds an item to the heap.
func (h *Heap[T]) Push(item T) {
    heap.Push(h, item)
}

// Pop removes and returns the highest priority (lowest) item.
func (h *Heap[T]) Pop() T {
    return heap.Pop(h).(T)
}

// Peek returns the highest priority item without removing it.
func (h *Heap[T]) Peek() T {
    return h.items[0]
}

// Internal methods required by container/heap.

func (h *Heap[T]) Less(i, j int) bool {
    return h.items[i].Less(h.items[j])
}

func (h *Heap[T]) Swap(i, j int) {
    h.items[i], h.items[j] = h.items[j], h.items[i]
}

func (h *Heap[T]) PushItem(x interface{}) {
    h.items = append(h.items, x.(T))
}

func (h *Heap[T]) PopItem() interface{} {
    n := len(h.items)
    item := h.items[n-1]
    h.items = h.items[:n-1]
    return item
}

// ThreadSafeHeap wraps a heap with a mutex for concurrent access.
type ThreadSafeHeap[T Lessable] struct {
    h  *Heap[T]
    mu sync.RWMutex
}

// NewThreadSafe creates a new thread‑safe heap.
func NewThreadSafe[T Lessable]() *ThreadSafeHeap[T] {
    return &ThreadSafeHeap[T]{h: New[T]()}
}

func (tsh *ThreadSafeHeap[T]) Push(item T) {
    tsh.mu.Lock()
    defer tsh.mu.Unlock()
    tsh.h.Push(item)
}

func (tsh *ThreadSafeHeap[T]) Pop() T {
    tsh.mu.Lock()
    defer tsh.mu.Unlock()
    return tsh.h.Pop()
}

func (tsh *ThreadSafeHeap[T]) Len() int {
    tsh.mu.RLock()
    defer tsh.mu.RUnlock()
    return tsh.h.Len()
}

func (tsh *ThreadSafeHeap[T]) Peek() T {
    tsh.mu.RLock()
    defer tsh.mu.RUnlock()
    return tsh.h.Peek()
}