// Package item provides a generic item with key, value, and optional expiration.
// It is useful for building caches, storage layers, or priority queues.
package testutils

import (
	"time"
)

// Item represents a stored item with a key, value, and optional expiration.
// The key must be comparable, the value can be any type.
type Item[K comparable, V any] struct {
	Key       K
	Value     V
	CreatedAt time.Time
	ExpiresAt *time.Time // nil means no expiration
}

// New creates a new item with the given key and value and no expiration.
func New[K comparable, V any](key K, value V) *Item[K, V] {
	return &Item[K, V]{
		Key:       key,
		Value:     value,
		CreatedAt: time.Now(),
		ExpiresAt: nil,
	}
}

// NewWithTTL creates a new item with a time‑to‑live duration.
func NewWithTTL[K comparable, V any](key K, value V, ttl time.Duration) *Item[K, V] {
	exp := time.Now().Add(ttl)
	return &Item[K, V]{
		Key:       key,
		Value:     value,
		CreatedAt: time.Now(),
		ExpiresAt: &exp,
	}
}

// IsExpired reports whether the item has expired (if expiration is set).
func (i *Item[K, V]) IsExpired() bool {
	if i.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*i.ExpiresAt)
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     item := item.New("user:1", "Alice")
//     fmt.Println(item.Key, item.Value) // "user:1", "Alice"
//
//     itemWithTTL := item.NewWithTTL("session:abc", 12345, 30*time.Minute)
//     fmt.Println(itemWithTTL.IsExpired()) // false
// }