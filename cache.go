// Package cache provides a generic, in-memory, thread-safe cache with TTL support.
// It uses Go generics, requires no external dependencies, and offers both
// simple and configurable constructors.
package testutils

import (
	"sync"
	"sync/atomic"
	"time"
)

// ----------------------------------------------------------------------------
// Item
// ----------------------------------------------------------------------------

// Item represents a cached value with an optional expiration time.
type Item[V any] struct {
	value      V
	expiration time.Time // zero means never expires
}

// Expired returns true if the item has expired.
func (i *Item[V]) Expired(now time.Time) bool {
	if i.expiration.IsZero() {
		return false
	}
	return now.After(i.expiration)
}

// ----------------------------------------------------------------------------
// Stats
// ----------------------------------------------------------------------------

// Stats holds cache usage counters, using atomic counters for concurrency.
type Stats struct {
	Hits   atomic.Uint64
	Misses atomic.Uint64
}

// ----------------------------------------------------------------------------
// Cache
// ----------------------------------------------------------------------------

// Cache is a generic, thread-safe, in-memory cache with expiration.
type Cache[K comparable, V any] struct {
	mu          sync.RWMutex
	items       map[K]*Item[V]
	defaultTTL  time.Duration
	stopClean   chan struct{}
	cleanupOnce sync.Once
	stats       Stats
}

// Config holds optional cache parameters.
type Config struct {
	DefaultTTL      time.Duration // zero means items never expire by default
	CleanupInterval time.Duration // zero disables background cleanup
}

// New creates a new cache with the given default TTL and cleanup interval.
func New[K comparable, V any](defaultTTL, cleanupInterval time.Duration) *Cache[K, V] {
	return NewWithConfig[K, V](Config{
		DefaultTTL:      defaultTTL,
		CleanupInterval: cleanupInterval,
	})
}

// NewWithConfig creates a new cache with the provided configuration.
func NewWithConfig[K comparable, V any](cfg Config) *Cache[K, V] {
	c := &Cache[K, V]{
		items:      make(map[K]*Item[V]),
		defaultTTL: cfg.DefaultTTL,
		stopClean:  make(chan struct{}),
	}
	if cfg.CleanupInterval > 0 {
		go c.runCleanup(cfg.CleanupInterval)
	}
	return c
}

// ----------------------------------------------------------------------------
// Core operations
// ----------------------------------------------------------------------------

// Set stores a value with an optional TTL.
// ttl == 0 uses the cache default TTL, ttl < 0 means never expires.
func (c *Cache[K, V]) Set(key K, value V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = &Item[V]{
		value:      value,
		expiration: c.computeExpiration(ttl),
	}
}

// Get retrieves a value. Returns (zero, false) if not found or expired.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.getNoLock(key)
}

// Delete removes an item from the cache.
func (c *Cache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// DeleteExpired removes all expired items.
func (c *Cache[K, V]) DeleteExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for k, item := range c.items {
		if item.Expired(now) {
			delete(c.items, k)
		}
	}
}

// Clear removes all items.
func (c *Cache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[K]*Item[V])
}

// Len returns the total number of items (including expired items).
func (c *Cache[K, V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Keys returns all keys (including expired items).
func (c *Cache[K, V]) Keys() []K {
	c.mu.RLock()
	defer c.mu.RUnlock()
	keys := make([]K, 0, len(c.items))
	for k := range c.items {
		keys = append(keys, k)
	}
	return keys
}

// Stats returns a snapshot of cache statistics.
func (c *Cache[K, V]) Stats() Stats {
	return c.stats
}

// ResetStats resets hit/miss counters.
func (c *Cache[K, V]) ResetStats() {
	c.stats.Hits.Store(0)
	c.stats.Misses.Store(0)
}

// ----------------------------------------------------------------------------
// GetOrSet with optimized locking
// ----------------------------------------------------------------------------

// GetOrSet atomically returns the value if present; otherwise, it calls fn() to compute it.
func (c *Cache[K, V]) GetOrSet(key K, ttl time.Duration, fn func() (V, error)) (V, error) {
	// Fast read path
	c.mu.RLock()
	item, found := c.items[key]
	now := time.Now()
	if found && !item.Expired(now) {
		c.stats.Hits.Add(1)
		val := item.value
		c.mu.RUnlock()
		return val, nil
	}
	c.mu.RUnlock()

	// Compute value outside lock
	val, err := fn()
	if err != nil {
		var zero V
		return zero, err
	}

	// Write lock to store
	c.mu.Lock()
	defer c.mu.Unlock()
	item, found = c.items[key]
	if found && !item.Expired(now) {
		c.stats.Hits.Add(1)
		return item.value, nil
	}
	c.items[key] = &Item[V]{
		value:      val,
		expiration: c.computeExpiration(ttl),
	}
	return val, nil
}

// getNoLock assumes caller holds lock.
func (c *Cache[K, V]) getNoLock(key K) (V, bool) {
	item, found := c.items[key]
	if !found {
		c.stats.Misses.Add(1)
		var zero V
		return zero, false
	}
	now := time.Now()
	if item.Expired(now) {
		delete(c.items, key)
		c.stats.Misses.Add(1)
		var zero V
		return zero, false
	}
	c.stats.Hits.Add(1)
	return item.value, true
}

// computeExpiration calculates expiration time based on ttl.
func (c *Cache[K, V]) computeExpiration(ttl time.Duration) time.Time {
	switch {
	case ttl < 0:
		return time.Time{}
	case ttl == 0 && c.defaultTTL > 0:
		return time.Now().Add(c.defaultTTL)
	case ttl > 0:
		return time.Now().Add(ttl)
	default:
		return time.Time{}
	}
}

// ----------------------------------------------------------------------------
// Background cleanup
// ----------------------------------------------------------------------------

func (c *Cache[K, V]) runCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.DeleteExpired()
		case <-c.stopClean:
			return
		}
	}
}

// StopCleanup stops background cleanup safely.
func (c *Cache[K, V]) StopCleanup() {
	c.cleanupOnce.Do(func() {
		close(c.stopClean)
	})
}

// Close is an alias for StopCleanup.
func (c *Cache[K, V]) Close() {
	c.StopCleanup()
}

// ----------------------------------------------------------------------------
// NullCache – no-op implementation for testing
// ----------------------------------------------------------------------------

type NullCache[K comparable, V any] struct{}

func (n NullCache[K, V]) Set(key K, value V, ttl time.Duration) {}
func (n NullCache[K, V]) Get(key K) (V, bool)                   { var zero V; return zero, false }
func (n NullCache[K, V]) Delete(key K)                          {}
func (n NullCache[K, V]) DeleteExpired()                        {}
func (n NullCache[K, V]) Clear()                                {}
func (n NullCache[K, V]) Len() int                              { return 0 }
func (n NullCache[K, V]) Keys() []K                             { return nil }
func (n NullCache[K, V]) Stats() Stats                          { return Stats{} }
func (n NullCache[K, V]) ResetStats()                           {}
func (n NullCache[K, V]) StopCleanup()                          {}
func (n NullCache[K, V]) Close()                                {}
func (n NullCache[K, V]) GetOrSet(key K, ttl time.Duration, fn func() (V, error)) (V, error) {
	return fn()
}
