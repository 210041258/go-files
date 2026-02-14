// Package testutils provides mock and simple in-memory priority queues for testing.
package testutils

import (
    "container/heap"
    "sync"
)

// PriorityQueue is the interface for a priority queue used in tests.
type PriorityQueue interface {
    // Push adds an item with given priority (lower number = higher priority).
    Push(item interface{}, priority int)
    // Pop removes and returns the highest priority item.
    Pop() interface{}
    // Len returns the number of items in the queue.
    Len() int
}

// --------------------------------------------------------------------
// MockPriorityQueue – a test double that records pushes and pops.
// --------------------------------------------------------------------

// MockPriorityQueue implements PriorityQueue for unit tests.
type MockPriorityQueue struct {
    mu      sync.Mutex
    pushes  []struct {
        Item     interface{}
        Priority int
    }
    pops    []interface{} // recorded popped items
    popFunc func() interface{} // optional custom pop behavior
}

// NewMockPriorityQueue creates a new mock priority queue.
func NewMockPriorityQueue() *MockPriorityQueue {
    return &MockPriorityQueue{}
}

// SetPopFunc sets a function that will be called for each Pop() to return a value.
// This allows simulating different queue states.
func (m *MockPriorityQueue) SetPopFunc(fn func() interface{}) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.popFunc = fn
}

// Push records the push.
func (m *MockPriorityQueue) Push(item interface{}, priority int) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.pushes = append(m.pushes, struct {
        Item     interface{}
        Priority int
    }{item, priority})
}

// Pop records the pop and returns the result of popFunc if set, otherwise returns nil.
func (m *MockPriorityQueue) Pop() interface{} {
    m.mu.Lock()
    defer m.mu.Unlock()
    var val interface{}
    if m.popFunc != nil {
        val = m.popFunc()
    }
    m.pops = append(m.pops, val)
    return val
}

// Len returns the number of recorded pushes (as a simplistic mock, not accurate for actual queue length).
func (m *MockPriorityQueue) Len() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return len(m.pushes) - len(m.pops)
}

// Pushes returns the recorded push calls.
func (m *MockPriorityQueue) Pushes() []struct {
    Item     interface{}
    Priority int
} {
    m.mu.Lock()
    defer m.mu.Unlock()
    cp := make([]struct {
        Item     interface{}
        Priority int
    }, len(m.pushes))
    copy(cp, m.pushes)
    return cp
}

// Pops returns the recorded pop results.
func (m *MockPriorityQueue) Pops() []interface{} {
    m.mu.Lock()
    defer m.mu.Unlock()
    cp := make([]interface{}, len(m.pops))
    copy(cp, m.pops)
    return cp
}

// Reset clears recorded pushes and pops.
func (m *MockPriorityQueue) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.pushes = nil
    m.pops = nil
}

// --------------------------------------------------------------------
// InMemoryPriorityQueue – a simple heap-based priority queue for integration tests.
// --------------------------------------------------------------------

// InMemoryPriorityQueue implements PriorityQueue using a min-heap.
type InMemoryPriorityQueue struct {
    mu    sync.Mutex
    heap  *priorityHeap
}

type priorityItem struct {
    value    interface{}
    priority int
    index    int // for heap.Interface
}

// priorityHeap implements heap.Interface for priorityItem (min-heap based on priority).
type priorityHeap []*priorityItem

func (h priorityHeap) Len() int { return len(h) }
func (h priorityHeap) Less(i, j int) bool {
    // Lower priority number = higher priority
    return h[i].priority < h[j].priority
}
func (h priorityHeap) Swap(i, j int) {
    h[i], h[j] = h[j], h[i]
    h[i].index = i
    h[j].index = j
}
func (h *priorityHeap) Push(x interface{}) {
    item := x.(*priorityItem)
    item.index = len(*h)
    *h = append(*h, item)
}
func (h *priorityHeap) Pop() interface{} {
    old := *h
    n := len(old)
    item := old[n-1]
    item.index = -1 // for safety
    *h = old[0 : n-1]
    return item
}

// NewInMemoryPriorityQueue creates a new empty in-memory priority queue.
func NewInMemoryPriorityQueue() *InMemoryPriorityQueue {
    return &InMemoryPriorityQueue{
        heap: &priorityHeap{},
    }
}

// Push adds an item with the given priority.
func (q *InMemoryPriorityQueue) Push(item interface{}, priority int) {
    q.mu.Lock()
    defer q.mu.Unlock()
    heap.Push(q.heap, &priorityItem{
        value:    item,
        priority: priority,
    })
}

// Pop removes and returns the highest priority item (lowest priority number).
// Returns nil if queue is empty.
func (q *InMemoryPriorityQueue) Pop() interface{} {
    q.mu.Lock()
    defer q.mu.Unlock()
    if q.heap.Len() == 0 {
        return nil
    }
    item := heap.Pop(q.heap).(*priorityItem)
    return item.value
}

// Len returns the number of items in the queue.
func (q *InMemoryPriorityQueue) Len() int {
    q.mu.Lock()
    defer q.mu.Unlock()
    return q.heap.Len()
}