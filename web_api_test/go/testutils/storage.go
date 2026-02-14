// Package storage provides a unified factory for creating Store backends
// from URL strings. It supports Redis, PostgreSQL, SQLite, and in‑memory stores.
package testutils

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	// Import the concrete store implementations.
	// Adjust import paths to match your module.
	"github.com/yourproject/store" // common interface and memory store
	_ "github.com/yourproject/store/redis"   // if you separate packages, otherwise just ensure they are imported
	_ "github.com/yourproject/store/postgres"
	_ "github.com/yourproject/store/sqlite"
	// In a real project you'd have these as separate packages or combined.
	// For simplicity, we assume all implementations are in package "store"
	// and are accessible via their constructors.
)

// NewStoreFromURL creates a new Store based on the provided URL.
// Supported schemes:
//   - redis://[password@]host:port[/db][?dial_timeout=5s]
//   - postgres://user:pass@host:port/dbname?sslmode=...
//   - sqlite:///path/to/file.db[?mode=memory&cache=shared]
//   - memory://[?cleanup=10s]
//
// The context is used for any initial connection or setup that may block.
func NewStoreFromURL(ctx context.Context, rawURL string) (store.Store, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid storage URL: %w", err)
	}

	switch u.Scheme {
	case "redis":
		return newRedisStore(ctx, u)
	case "postgres", "postgresql":
		return newPostgresStore(ctx, u)
	case "sqlite", "sqlite3":
		return newSQLiteStore(ctx, u)
	case "memory":
		return newMemoryStore(ctx, u)
	default:
		return nil, fmt.Errorf("unsupported storage scheme: %s", u.Scheme)
	}
}

// newRedisStore creates a Redis store from a URL.
func newRedisStore(ctx context.Context, u *url.URL) (store.Store, error) {
	// Reconstruct DSN without the scheme prefix.
	// For redis, we can use the URL directly (go-redis supports redis:// URLs).
	// But to allow extra query parameters, we might need to build options.
	// Here we simply pass the URL string to a helper.
	// In practice, you'd use redis.ParseURL and then NewRedisStore.
	redisURL := u.String()
	// The memory store import is assumed to have NewRedisStoreFromURL or similar.
	// We'll call a constructor that accepts a URL.
	return store.NewRedisStoreFromURL(redisURL) // this must exist in your redis_store.go
}

// newPostgresStore creates a PostgreSQL store from a URL.
func newPostgresStore(ctx context.Context, u *url.URL) (store.Store, error) {
	// PostgreSQL connection string is the URL's opaque part.
	// pgxpool.ParseConfig accepts a DSN, which can be the URL string.
	dsn := u.String()
	return store.NewPostgresStore(ctx, dsn)
}

// newSQLiteStore creates a SQLite store from a URL.
func newSQLiteStore(ctx context.Context, u *url.URL) (store.Store, error) {
	// SQLite expects a file path; the URL path holds it.
	// We can also handle query parameters like ?mode=memory.
	filePath := u.Path
	if filePath == "" && u.Opaque != "" {
		// Handle cases like "sqlite:file.db" (no //)
		filePath = u.Opaque
	}
	if filePath == "" {
		return nil, fmt.Errorf("missing database file path in SQLite URL")
	}
	// Optionally, we could parse query parameters for mode, cache, etc.
	// For now, just pass the file path.
	return store.NewSQLiteStore(filePath) // from sqlite_store.go
}

// newMemoryStore creates an in‑memory store with optional cleanup interval.
func newMemoryStore(ctx context.Context, u *url.URL) (store.Store, error) {
	// Parse query parameters.
	q := u.Query()
	cleanup := q.Get("cleanup")
	var opts []store.MemoryStoreOption
	if cleanup != "" {
		d, err := time.ParseDuration(cleanup)
		if err != nil {
			return nil, fmt.Errorf("invalid cleanup duration: %w", err)
		}
		opts = append(opts, store.WithCleanupInterval(d))
	}
	return store.NewMemoryStore(opts...), nil
}

// ----------------------------------------------------------------------
// Example usage:
//   store, err := storage.NewStoreFromURL(ctx, "redis://localhost:6379/0")
//   if err != nil { ... }
//   defer store.Close()
// ----------------------------------------------------------------------