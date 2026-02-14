// Package testutils provides mock and simple in-memory deduplicators for testing.
package testutils

import (
    "sync"
    "time"
)

// Deduplicator is the interface for duplicate detection.
type Deduplicator interface {
    // IsDuplicate returns true if the key has been seen before (and not expired).
    IsDuplicate(key string) bool
}

// --------------------------------------------------------------------
// MockDeduplicator – a test double that records all checks.
// --------------------------------------------------------------------

// MockDeduplicator implements Deduplicator for unit tests.
// It records every key checked and can be inspected or reset.
type MockDeduplicator struct {
    mu    sync.Mutex
    seen  map[string]bool
    calls []string
}

// NewMockDeduplicator creates a new mock deduplicator.
func NewMockDeduplicator() *MockDeduplicator {
    return &MockDeduplicator{
        seen: make(map[string]bool),
    }
}

// IsDuplicate records the key and returns true if it has been seen before.
func (m *MockDeduplicator) IsDuplicate(key string) bool {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.calls = append(m.calls, key)
    if m.seen[key] {
        return true
    }
    m.seen[key] = true
    return false
}

// Reset clears the seen set and call history.
func (m *MockDeduplicator) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.seen = make(map[string]bool)
    m.calls = nil
}

// Calls returns the list of keys that were checked, in order.
func (m *MockDeduplicator) Calls() []string {
    m.mu.Lock()
    defer m.mu.Unlock()
    cp := make([]string, len(m.calls))
    copy(cp, m.calls)
    return cp
}

// SeenCount returns the number of unique keys seen so far.
func (m *MockDeduplicator) SeenCount() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return len(m.seen)
}

// --------------------------------------------------------------------
// MemDedupe – a simple in-memory deduplicator with optional TTL.
// Suitable for integration tests or small-scale production use.
// --------------------------------------------------------------------

// MemDedupe implements Deduplicator with an in-memory map and time‑based expiration.
type MemDedupe struct {
    mu    sync.Mutex
    store map[string]time.Time
    ttl   time.Duration
}

// NewMemDedupe creates a deduplicator that remembers keys for the given TTL.
// If ttl <= 0, keys never expire.
func NewMemDedupe(ttl time.Duration) *MemDedupe {
    return &MemDedupe{
        store: make(map[string]time.Time),
        ttl:   ttl,
    }
}

// IsDuplicate returns true if the key is already stored and not expired.
// If the key is new or expired, it is stored (with a new expiration) and false is returned.
func (d *MemDedupe) IsDuplicate(key string) bool {
    d.mu.Lock()
    defer d.mu.Unlock()
    d.cleanup()
    if exp, ok := d.store[key]; ok {
        if d.ttl > 0 && time.Now().After(exp) {
            // Expired, treat as new
            delete(d.store, key)
        } else {
            return true
        }
    }
    // Store the key with new expiration
    var exp time.Time
    if d.ttl > 0 {
        exp = time.Now().Add(d.ttl)
    }
    d.store[key] = exp
    return false
}

// cleanup removes expired entries (called under lock).
func (d *MemDedupe) cleanup() {
    if d.ttl <= 0 {
        return
    }
    now := time.Now()
    for k, exp := range d.store {
        if now.After(exp) {
            delete(d.store, k)
        }
    }
}

// Len returns the current number of stored keys (after cleaning expired ones).
func (d *MemDedupe) Len() int {
    d.mu.Lock()
    defer d.mu.Unlock()
    d.cleanup()
    return len(d.store)
}