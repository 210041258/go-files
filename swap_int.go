// Package testutils provides utilities for swapping integers in tests,
// including atomic operations and simple swapping functions.
package testutils

import (
    "sync"
    "sync/atomic"
)

// --------------------------------------------------------------------
// SwapInt – atomic integer for testing.
// --------------------------------------------------------------------

// SwapInt provides atomic operations on an int64 for testing concurrent code.
type SwapInt struct {
    value int64
}

// NewSwapInt creates a new SwapInt with an initial value.
func NewSwapInt(initial int64) *SwapInt {
    return &SwapInt{value: initial}
}

// Load returns the current value.
func (s *SwapInt) Load() int64 {
    return atomic.LoadInt64(&s.value)
}

// Store sets the value.
func (s *SwapInt) Store(val int64) {
    atomic.StoreInt64(&s.value, val)
}

// Swap atomically sets to new and returns the old.
func (s *SwapInt) Swap(new int64) int64 {
    return atomic.SwapInt64(&s.value, new)
}

// CompareAndSwap executes the compare-and-swap operation.
func (s *SwapInt) CompareAndSwap(old, new int64) bool {
    return atomic.CompareAndSwapInt64(&s.value, old, new)
}

// Add adds delta and returns the new value.
func (s *SwapInt) Add(delta int64) int64 {
    return atomic.AddInt64(&s.value, delta)
}

// --------------------------------------------------------------------
// MockSwapInt – records calls for testing.
// --------------------------------------------------------------------

// MockSwapInt implements a mock version of SwapInt that records all operations.
type MockSwapInt struct {
    mu         sync.Mutex
    value      int64
    loadCalls  int
    storeCalls []int64
    swapCalls  []int64
    casCalls   []struct{ old, new int64 }
    addCalls   []int64
}

// NewMockSwapInt creates a new mock with an initial value.
func NewMockSwapInt(initial int64) *MockSwapInt {
    return &MockSwapInt{value: initial}
}

// Load records the call and returns current value.
func (m *MockSwapInt) Load() int64 {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.loadCalls++
    return m.value
}

// Store records the call and sets value.
func (m *MockSwapInt) Store(val int64) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.storeCalls = append(m.storeCalls, val)
    m.value = val
}

// Swap records the call, sets new, returns old.
func (m *MockSwapInt) Swap(new int64) int64 {
    m.mu.Lock()
    defer m.mu.Unlock()
    old := m.value
    m.swapCalls = append(m.swapCalls, new)
    m.value = new
    return old
}

// CompareAndSwap records the call and performs CAS.
func (m *MockSwapInt) CompareAndSwap(old, new int64) bool {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.casCalls = append(m.casCalls, struct{ old, new int64 }{old, new})
    if m.value == old {
        m.value = new
        return true
    }
    return false
}

// Add records the call and adds delta.
func (m *MockSwapInt) Add(delta int64) int64 {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.addCalls = append(m.addCalls, delta)
    m.value += delta
    return m.value
}

// CallCounts returns the number of calls to each method.
func (m *MockSwapInt) CallCounts() (load, store, swap, cas, add int) {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.loadCalls, len(m.storeCalls), len(m.swapCalls), len(m.casCalls), len(m.addCalls)
}

// Reset clears recorded calls and optionally resets value.
func (m *MockSwapInt) Reset(value ...int64) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.loadCalls = 0
    m.storeCalls = nil
    m.swapCalls = nil
    m.casCalls = nil
    m.addCalls = nil
    if len(value) > 0 {
        m.value = value[0]
    }
}

// --------------------------------------------------------------------
// Simple swap functions.
// --------------------------------------------------------------------

// SwapInts swaps the values of two ints.
func SwapInts(a, b *int) {
    *a, *b = *b, *a
}

// SwapInt64s swaps the values of two int64s.
func SwapInt64s(a, b *int64) {
    *a, *b = *b, *a
}