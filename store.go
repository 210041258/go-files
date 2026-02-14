// common.go (optional – if you want to share the interface and error)
// Place this in a separate file or include it in each store file.
package store

import (
	"context"
	"errors"
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