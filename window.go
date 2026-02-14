// Package testutils provides mock and simple in-memory windowing utilities for testing.
package testutils

import (
    "sync"
    "time"
)

// Window is the interface for time‑based window operations.
type Window interface {
    // Add inserts an item with the current time or a given timestamp.
    Add(key string, value interface{})
    // AddAt inserts an item with a specific timestamp.
    AddAt(timestamp time.Time, key string, value interface{})
    // Get returns all items in the window that fall within the time range [start, end).
    // If start or end is zero, the window's natural boundaries are used (e.g., earliest/latest).
    Get(start, end time.Time) []WindowItem
    // Clear removes all items.
    Clear()
}

// WindowItem represents an item stored in a window.
type WindowItem struct {
    Timestamp time.Time
    Key       string
    Value     interface{}
}

// --------------------------------------------------------------------
// MockWindow – a test double that records all adds and can be programmed.
// --------------------------------------------------------------------

// MockWindow implements Window for unit tests.
type MockWindow struct {
    mu    sync.Mutex
    items []WindowItem
    getFunc func(start, end time.Time) []WindowItem // optional custom behavior
}

// NewMockWindow creates a new mock window.
func NewMockWindow() *MockWindow {
    return &MockWindow{}
}

// SetGetFunc overrides the Get method with custom behavior.
func (m *MockWindow) SetGetFunc(fn func(start, end time.Time) []WindowItem) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.getFunc = fn
}

// Add records an item with the current time.
func (m *MockWindow) Add(key string, value interface{}) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.items = append(m.items, WindowItem{
        Timestamp: time.Now(),
        Key:       key,
        Value:     value,
    })
}

// AddAt records an item with the given timestamp.
func (m *MockWindow) AddAt(timestamp time.Time, key string, value interface{}) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.items = append(m.items, WindowItem{
        Timestamp: timestamp,
        Key:       key,
        Value:     value,
    })
}

// Get returns the recorded items if no custom function is set, otherwise calls the custom function.
func (m *MockWindow) Get(start, end time.Time) []WindowItem {
    m.mu.Lock()
    defer m.mu.Unlock()
    if m.getFunc != nil {
        return m.getFunc(start, end)
    }
    // Default: return all items (ignore time range)
    cp := make([]WindowItem, len(m.items))
    copy(cp, m.items)
    return cp
}

// Clear removes all recorded items.
func (m *MockWindow) Clear() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.items = nil
}

// Items returns a copy of all items added (for inspection).
func (m *MockWindow) Items() []WindowItem {
    m.mu.Lock()
    defer m.mu.Unlock()
    cp := make([]WindowItem, len(m.items))
    copy(cp, m.items)
    return cp
}

// Reset clears all items and resets the custom function.
func (m *MockWindow) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.items = nil
    m.getFunc = nil
}

// --------------------------------------------------------------------
// InMemoryWindow – a simple time‑window for integration tests.
// --------------------------------------------------------------------

// InMemoryWindow implements a real in‑memory window that stores items with timestamps.
// It supports querying by time range.
type InMemoryWindow struct {
    mu    sync.RWMutex
    items []WindowItem
}

// NewInMemoryWindow creates a new empty window.
func NewInMemoryWindow() *InMemoryWindow {
    return &InMemoryWindow{}
}

// Add inserts an item with the current time.
func (w *InMemoryWindow) Add(key string, value interface{}) {
    w.AddAt(time.Now(), key, value)
}

// AddAt inserts an item with the given timestamp.
func (w *InMemoryWindow) AddAt(timestamp time.Time, key string, value interface{}) {
    w.mu.Lock()
    defer w.mu.Unlock()
    w.items = append(w.items, WindowItem{
        Timestamp: timestamp,
        Key:       key,
        Value:     value,
    })
}

// Get returns all items with timestamps in [start, end).
// If start.IsZero(), the earliest item's time is used.
// If end.IsZero(), the latest item's time plus 1 ns is used.
func (w *InMemoryWindow) Get(start, end time.Time) []WindowItem {
    w.mu.RLock()
    defer w.mu.RUnlock()

    // Determine effective range
    effectiveStart := start
    effectiveEnd := end
    if effectiveStart.IsZero() && len(w.items) > 0 {
        effectiveStart = w.items[0].Timestamp
    }
    if effectiveEnd.IsZero() && len(w.items) > 0 {
        effectiveEnd = w.items[len(w.items)-1].Timestamp.Add(1 * time.Nanosecond)
    }

    var result []WindowItem
    for _, item := range w.items {
        if !effectiveStart.IsZero() && item.Timestamp.Before(effectiveStart) {
            continue
        }
        if !effectiveEnd.IsZero() && !item.Timestamp.Before(effectiveEnd) {
            continue
        }
        result = append(result, item)
    }
    return result
}

// Clear removes all items.
func (w *InMemoryWindow) Clear() {
    w.mu.Lock()
    defer w.mu.Unlock()
    w.items = nil
}

// Len returns the number of items in the window.
func (w *InMemoryWindow) Len() int {
    w.mu.RLock()
    defer w.mu.RUnlock()
    return len(w.items)
}