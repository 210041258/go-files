// Package store provides a common interface and in‑memory implementation
// for a JSON‑based key‑value store with TTL support.
package testutils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"
)

// ErrNotFound is returned when a key is not found in the store.
var ErrNotFound = errors.New("key not found")

// Store defines the common interface for all storage backends.
type Store interface {
	// Close releases any resources held by the store.
	Close() error

	// Set stores a value under the given key with an optional TTL.
	// The value is marshaled as JSON. If ttl <= 0, no expiration is set.
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// Get retrieves the value for the given key and unmarshals it into dest.
	// Returns ErrNotFound if the key does not exist or has expired.
	Get(ctx context.Context, key string, dest interface{}) error

	// Delete removes the key from the store.
	Delete(ctx context.Context, key string) error

	// List returns all keys matching the given pattern.
	// Pattern semantics are implementation‑dependent.
	List(ctx context.Context, pattern string) ([]string, error)
}

// ----------------------------------------------------------------------
// In‑Memory Store
// ----------------------------------------------------------------------

// item represents a stored value with optional expiration.
type item struct {
	data      []byte
	expiresAt *time.Time
}

// MemoryStore implements the Store interface using an in‑memory map.
type MemoryStore struct {
	mu      sync.RWMutex
	items   map[string]item
	done    chan struct{}
	wg      sync.WaitGroup
	closeMu sync.Once
}

// MemoryStoreOption configures the MemoryStore.
type MemoryStoreOption func(*MemoryStore)

// WithCleanupInterval sets the interval at which expired entries are
// automatically removed from the store. The default is 1 minute.
// A zero or negative value disables automatic cleanup.
func WithCleanupInterval(d time.Duration) MemoryStoreOption {
	return func(s *MemoryStore) {
		if d > 0 {
			s.startCleanup(d)
		}
	}
}

// NewMemoryStore creates a new in‑memory store with optional configuration.
// By default, automatic cleanup runs every minute. Use WithCleanupInterval(0)
// to disable it.
func NewMemoryStore(opts ...MemoryStoreOption) *MemoryStore {
	s := &MemoryStore{
		items: make(map[string]item),
		done:  make(chan struct{}),
	}
	// Default cleanup interval: 1 minute.
	s.startCleanup(1 * time.Minute)

	for _, opt := range opts {
		opt(s)
	}
	return s
}

// startCleanup launches the background goroutine that periodically removes
// expired entries.
func (s *MemoryStore) startCleanup(interval time.Duration) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-s.done:
				return
			case <-ticker.C:
				s.deleteExpired()
			}
		}
	}()
}

// deleteExpired removes all expired entries from the store.
func (s *MemoryStore) deleteExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, v := range s.items {
		if v.expiresAt != nil && v.expiresAt.Before(now) {
			delete(s.items, k)
		}
	}
}

// Close stops the background cleanup and releases resources.
func (s *MemoryStore) Close() error {
	s.closeMu.Do(func() {
		close(s.done)
		s.wg.Wait()
	})
	return nil
}

// Set stores a JSON‑marshaled value under the given key with optional TTL.
func (s *MemoryStore) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal value: %w", err)
	}

	var expiresAt *time.Time
	if ttl > 0 {
		t := time.Now().Add(ttl)
		expiresAt = &t
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key] = item{
		data:      data,
		expiresAt: expiresAt,
	}
	return nil
}

// Get retrieves the value for the given key and unmarshals it into dest.
// If the key has expired, it is removed and ErrNotFound is returned.
func (s *MemoryStore) Get(ctx context.Context, key string, dest interface{}) error {
	s.mu.RLock()
	it, ok := s.items[key]
	s.mu.RUnlock()
	if !ok {
		return ErrNotFound
	}

	// Check expiration.
	if it.expiresAt != nil && it.expiresAt.Before(time.Now()) {
		// Delete expired key in background (do not block the read).
		go s.Delete(context.Background(), key)
		return ErrNotFound
	}

	if err := json.Unmarshal(it.data, dest); err != nil {
		return fmt.Errorf("unmarshal value: %w", err)
	}
	return nil
}

// Delete removes the key from the store.
func (s *MemoryStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, key)
	return nil
}

// List returns all keys that match the given pattern and are not expired.
// Pattern syntax follows filepath.Match: '*' matches any sequence,
// '?' matches any single character.
func (s *MemoryStore) List(ctx context.Context, pattern string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var keys []string
	now := time.Now()
	for k, it := range s.items {
		// Skip expired.
		if it.expiresAt != nil && it.expiresAt.Before(now) {
			continue
		}
		// Check pattern match.
		matched, err := filepath.Match(pattern, k)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern: %w", err)
		}
		if matched {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

// CleanupExpired manually triggers removal of all expired entries.
// This is a synchronous operation; it blocks until cleanup is complete.
func (s *MemoryStore) CleanupExpired(ctx context.Context) error {
	s.deleteExpired()
	return nil
}