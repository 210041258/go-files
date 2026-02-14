// postgres_store.go
package testutils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore implements the Store interface using PostgreSQL.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a new PostgreSQL store with connection pool.
// DSN example: "postgres://username:password@localhost:5432/dbname?sslmode=disable"
func NewPostgresStore(ctx context.Context, dsn string) (*PostgresStore, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse DSN: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}

	// Create table if not exists.
	query := `
		CREATE TABLE IF NOT EXISTS kv_store (
			key TEXT PRIMARY KEY,
			value JSONB NOT NULL,
			expires_at TIMESTAMPTZ NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_expires_at ON kv_store(expires_at);
	`
	if _, err := pool.Exec(ctx, query); err != nil {
		pool.Close()
		return nil, fmt.Errorf("create table: %w", err)
	}

	return &PostgresStore{pool: pool}, nil
}

// Close releases the connection pool.
func (s *PostgresStore) Close() error {
	s.pool.Close()
	return nil
}

// Set stores a value under the given key with optional TTL.
// The value is marshaled as JSON.
func (s *PostgresStore) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal value: %w", err)
	}

	var expiresAt *time.Time
	if ttl > 0 {
		t := time.Now().Add(ttl)
		expiresAt = &t
	}

	query := `
		INSERT INTO kv_store (key, value, expires_at, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (key) DO UPDATE SET
			value = EXCLUDED.value,
			expires_at = EXCLUDED.expires_at,
			updated_at = NOW()
	`
	_, err = s.pool.Exec(ctx, query, key, data, expiresAt)
	if err != nil {
		return fmt.Errorf("execute upsert: %w", err)
	}
	return nil
}

// Get retrieves the value for the given key and unmarshals it into dest.
// Returns ErrNotFound if the key does not exist or has expired.
func (s *PostgresStore) Get(ctx context.Context, key string, dest interface{}) error {
	var data []byte
	var expiresAt *time.Time

	query := `SELECT value, expires_at FROM kv_store WHERE key = $1`
	err := s.pool.QueryRow(ctx, query, key).Scan(&data, &expiresAt)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return ErrNotFound
		}
		return fmt.Errorf("query row: %w", err)
	}

	// Check expiration.
	if expiresAt != nil && expiresAt.Before(time.Now()) {
		// Delete expired key in background.
		go s.Delete(context.Background(), key)
		return ErrNotFound
	}

	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("unmarshal value: %w", err)
	}
	return nil
}

// Delete removes the key from the store.
func (s *PostgresStore) Delete(ctx context.Context, key string) error {
	query := `DELETE FROM kv_store WHERE key = $1`
	_, err := s.pool.Exec(ctx, query, key)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	return nil
}

// List returns all keys matching the given pattern (using SQL LIKE).
func (s *PostgresStore) List(ctx context.Context, pattern string) ([]string, error) {
	query := `SELECT key FROM kv_store WHERE key LIKE $1 AND (expires_at IS NULL OR expires_at > NOW())`
	rows, err := s.pool.Query(ctx, query, pattern)
	if err != nil {
		return nil, fmt.Errorf("list keys: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

// CleanupExpired removes expired entries from the database.
func (s *PostgresStore) CleanupExpired(ctx context.Context) error {
	query := `DELETE FROM kv_store WHERE expires_at < NOW()`
	_, err := s.pool.Exec(ctx, query)
	return err
}