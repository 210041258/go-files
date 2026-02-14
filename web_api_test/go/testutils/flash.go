// Package flash provides a fast ephemeral storage for buffering, caching, and deduplication.
package testutils

import (
    "errors"
    "sync"
    "time"
)

// Common errors.
var (
    ErrKeyNotFound = errors.New("key not found")
    ErrKeyExpired  = errors.New("key expired")
)

// Store is the interface for flash storage.
type Store interface {
    // Set stores a value with an optional TTL. If ttl <= 0, the item never expires.
    Set(key string, value []byte, ttl time.Duration) error
    // Get retrieves a value. Returns ErrKeyNotFound or ErrKeyExpired if not found/expired.
    Get(key string) ([]byte, error)
    // Delete removes a key.
    Delete(key string) error
    // Exists checks if a key exists and is not expired.
    Exists(key string) (bool, error)
    // Close shuts down the store.
    Close() error
}

// In-memory implementation with TTL.
type memoryStore struct {
    mu    sync.RWMutex
    items map[string]*item
}

type item struct {
    value      []byte
    expiration int64 // nanoseconds since epoch; 0 means no expiration
}

// NewMemoryStore creates an in-memory flash store.
func NewMemoryStore() Store {
    return &memoryStore{
        items: make(map[string]*item),
    }
}

func (m *memoryStore) Set(key string, value []byte, ttl time.Duration) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    var exp int64
    if ttl > 0 {
        exp = time.Now().Add(ttl).UnixNano()
    }
    m.items[key] = &item{
        value:      append([]byte(nil), value...), // copy
        expiration: exp,
    }
    return nil
}

func (m *memoryStore) Get(key string) ([]byte, error) {
    m.mu.RLock()
    it, ok := m.items[key]
    m.mu.RUnlock()
    if !ok {
        return nil, ErrKeyNotFound
    }
    if it.expiration > 0 && time.Now().UnixNano() > it.expiration {
        // lazy expiration: delete on access
        m.mu.Lock()
        delete(m.items, key)
        m.mu.Unlock()
        return nil, ErrKeyExpired
    }
    return append([]byte(nil), it.value...), nil // copy
}

func (m *memoryStore) Delete(key string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    delete(m.items, key)
    return nil
}

func (m *memoryStore) Exists(key string) (bool, error) {
    m.mu.RLock()
    it, ok := m.items[key]
    m.mu.RUnlock()
    if !ok {
        return false, nil
    }
    if it.expiration > 0 && time.Now().UnixNano() > it.expiration {
        m.mu.Lock()
        delete(m.items, key)
        m.mu.Unlock()
        return false, nil
    }
    return true, nil
}

func (m *memoryStore) Close() error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.items = nil
    return nil
}

// Queue-like wrapper for buffering.
// Buffer is a FIFO queue backed by a flash store (could be in-memory or persistent).
type Buffer struct {
    store   Store
    headKey string // e.g., "queue:head"
    tailKey string // e.g., "queue:tail"
    mu      sync.Mutex
}

// NewBuffer creates a new buffer using the given store.
func NewBuffer(store Store) *Buffer {
    return &Buffer{store: store}
}

// Push adds an item to the end of the buffer.
func (b *Buffer) Push(data []byte) error {
    b.mu.Lock()
    defer b.mu.Unlock()
    // Generate a unique ID (e.g., timestamp + counter)
    id := time.Now().UnixNano()
    key := b.itemKey(id)
    if err := b.store.Set(key, data, 0); err != nil {
        return err
    }
    // Update tail pointer
    tail, _ := b.store.Get(b.tailKey)
    if tail == nil {
        // first item, also set head
        b.store.Set(b.headKey, []byte(key), 0)
    } else {
        // link previous tail to this new item (optional, for ordered traversal)
    }
    b.store.Set(b.tailKey, []byte(key), 0)
    return nil
}

// Pop retrieves and removes the oldest item from the buffer.
func (b *Buffer) Pop() ([]byte, error) {
    b.mu.Lock()
    defer b.mu.Unlock()
    head, err := b.store.Get(b.headKey)
    if err != nil {
        return nil, ErrKeyNotFound // empty
    }
    key := string(head)
    data, err := b.store.Get(key)
    if err != nil {
        // inconsistency, but handle
        return nil, err
    }
    b.store.Delete(key)
    // Update head to next item (simplified)
    // In a real implementation, you'd have a linked structure.
    // For simplicity, we set head to the next item's key.
    // If no next, set head to empty.
    // This example is incomplete; proper queue requires ordering.
    return data, nil
}

func (b *Buffer) itemKey(id int64) string {
    return "item:" + strconv.FormatInt(id, 10)
}