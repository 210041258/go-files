// Package testutils provides mock and simple in-memory checkpointing for testing.
package testutils

import (
    "errors"
    "sync"
)

// ErrKeyNotFound is returned when a checkpoint key does not exist.
var ErrKeyNotFound = errors.New("checkpoint key not found")

// Checkpointer is the interface for saving and loading progress state.
type Checkpointer interface {
    // Save stores a value for the given key. Overwrites any existing value.
    Save(key string, value []byte) error
    // Load retrieves the value for the given key. Returns ErrKeyNotFound if the key does not exist.
    Load(key string) ([]byte, error)
    // Delete removes a key. No error if the key does not exist.
    Delete(key string) error
}

// --------------------------------------------------------------------
// MockCheckpointer – a test double that records all calls and can be programmed.
// --------------------------------------------------------------------

// MockCheckpointer implements Checkpointer for unit tests.
type MockCheckpointer struct {
    mu         sync.Mutex
    data       map[string][]byte          // stored values
    saveCalls  []string                   // keys saved
    loadCalls  []string                   // keys loaded
    deleteCalls []string                   // keys deleted
    loadFunc   func(string) ([]byte, error) // optional custom behavior for Load
}

// NewMockCheckpointer creates a new mock checkpointer.
func NewMockCheckpointer() *MockCheckpointer {
    return &MockCheckpointer{
        data: make(map[string][]byte),
    }
}

// SetLoadFunc overrides the Load method with custom behavior.
func (m *MockCheckpointer) SetLoadFunc(fn func(key string) ([]byte, error)) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.loadFunc = fn
}

// Save records the key and stores the value.
func (m *MockCheckpointer) Save(key string, value []byte) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.saveCalls = append(m.saveCalls, key)
    // Store a copy to prevent modification after return.
    cp := make([]byte, len(value))
    copy(cp, value)
    m.data[key] = cp
    return nil
}

// Load returns the stored value or calls loadFunc if set.
func (m *MockCheckpointer) Load(key string) ([]byte, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.loadCalls = append(m.loadCalls, key)
    if m.loadFunc != nil {
        return m.loadFunc(key)
    }
    val, ok := m.data[key]
    if !ok {
        return nil, ErrKeyNotFound
    }
    cp := make([]byte, len(val))
    copy(cp, val)
    return cp, nil
}

// Delete records the key and removes it from storage.
func (m *MockCheckpointer) Delete(key string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.deleteCalls = append(m.deleteCalls, key)
    delete(m.data, key)
    return nil
}

// SaveCalls returns the list of keys passed to Save.
func (m *MockCheckpointer) SaveCalls() []string {
    m.mu.Lock()
    defer m.mu.Unlock()
    cp := make([]string, len(m.saveCalls))
    copy(cp, m.saveCalls)
    return cp
}

// LoadCalls returns the list of keys passed to Load.
func (m *MockCheckpointer) LoadCalls() []string {
    m.mu.Lock()
    defer m.mu.Unlock()
    cp := make([]string, len(m.loadCalls))
    copy(cp, m.loadCalls)
    return cp
}

// DeleteCalls returns the list of keys passed to Delete.
func (m *MockCheckpointer) DeleteCalls() []string {
    m.mu.Lock()
    defer m.mu.Unlock()
    cp := make([]string, len(m.deleteCalls))
    copy(cp, m.deleteCalls)
    return cp
}

// Reset clears all recorded calls and stored data.
func (m *MockCheckpointer) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.data = make(map[string][]byte)
    m.saveCalls = nil
    m.loadCalls = nil
    m.deleteCalls = nil
    m.loadFunc = nil
}

// --------------------------------------------------------------------
// InMemoryCheckpointer – a simple map-based checkpoint for integration tests.
// --------------------------------------------------------------------

// InMemoryCheckpointer implements Checkpointer with an in-memory map.
type InMemoryCheckpointer struct {
    mu   sync.RWMutex
    data map[string][]byte
}

// NewInMemoryCheckpointer creates a new empty in-memory checkpointer.
func NewInMemoryCheckpointer() *InMemoryCheckpointer {
    return &InMemoryCheckpointer{
        data: make(map[string][]byte),
    }
}

// Save stores a value for the key.
func (c *InMemoryCheckpointer) Save(key string, value []byte) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    cp := make([]byte, len(value))
    copy(cp, value)
    c.data[key] = cp
    return nil
}

// Load retrieves a value for the key.
func (c *InMemoryCheckpointer) Load(key string) ([]byte, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    val, ok := c.data[key]
    if !ok {
        return nil, ErrKeyNotFound
    }
    cp := make([]byte, len(val))
    copy(cp, val)
    return cp, nil
}

// Delete removes a key.
func (c *InMemoryCheckpointer) Delete(key string) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    delete(c.data, key)
    return nil
}