// sqlite_store.go
package testutils

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore implements the Store interface using SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite store.
// filepath: path to the SQLite database file (e.g., "data.db").
// Use ":memory:" for an in-memory database.
func NewSQLiteStore(filepath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", filepath+"?_foreign_keys=on&_journal=WAL")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Set connection pool limits.
	db.SetMaxOpenConns(1) // SQLite allows only one writer concurrently.
	db.SetMaxIdleConns(1)

	// Create table if not exists.
	query := `
		CREATE TABLE IF NOT EXISTS kv_store (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL, -- JSON stored as text
			expires_at DATETIME NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_expires_at ON kv_store(expires_at);
	`
	if _, err := db.Exec(query); err != nil {
		db.Close()
		return nil, fmt.Errorf("create table: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// Close closes the database.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// Set stores a value under the given key with optional TTL.
// The value is marshaled as JSON.
func (s *SQLiteStore) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
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
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			expires_at = excluded.expires_at,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err = s.db.ExecContext(ctx, query, key, string(data), expiresAt)
	if err != nil {
		return fmt.Errorf("execute upsert: %w", err)
	}
	return nil
}

// Get retrieves the value for the given key and unmarshals it into dest.
// Returns ErrNotFound if the key does not exist or has expired.
func (s *SQLiteStore) Get(ctx context.Context, key string, dest interface{}) error {
	var value string
	var expiresAt sql.NullTime

	query := `SELECT value, expires_at FROM kv_store WHERE key = ?`
	err := s.db.QueryRowContext(ctx, query, key).Scan(&value, &expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		return fmt.Errorf("query row: %w", err)
	}

	// Check expiration.
	if expiresAt.Valid && expiresAt.Time.Before(time.Now()) {
		// Delete expired key in background.
		go s.Delete(context.Background(), key)
		return ErrNotFound
	}

	if err := json.Unmarshal([]byte(value), dest); err != nil {
		return fmt.Errorf("unmarshal value: %w", err)
	}
	return nil
}

// Delete removes the key from the store.
func (s *SQLiteStore) Delete(ctx context.Context, key string) error {
	query := `DELETE FROM kv_store WHERE key = ?`
	_, err := s.db.ExecContext(ctx, query, key)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	return nil
}

// List returns all keys matching the given pattern (using SQL LIKE).
func (s *SQLiteStore) List(ctx context.Context, pattern string) ([]string, error) {
	query := `SELECT key FROM kv_store WHERE key LIKE ? AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)`
	rows, err := s.db.QueryContext(ctx, query, pattern)
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
func (s *SQLiteStore) CleanupExpired(ctx context.Context) error {
	query := `DELETE FROM kv_store WHERE expires_at < CURRENT_TIMESTAMP`
	_, err := s.db.ExecContext(ctx, query)
	return err
}