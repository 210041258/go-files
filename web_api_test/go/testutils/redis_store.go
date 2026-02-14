// redis_store.go
package testutils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore implements the Store interface using Redis.
type RedisStore struct {
	client *redis.Client
}

// NewRedisStore creates a new Redis store.
// Options can be provided via redis.Options struct.
func NewRedisStore(opts *redis.Options) (*RedisStore, error) {
	client := redis.NewClient(opts)

	// Test connection.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("connect to redis: %w", err)
	}

	return &RedisStore{client: client}, nil
}

// NewRedisStoreFromURL creates a new Redis store using a Redis URL.
// Example: "redis://<user>:<pass>@localhost:6379/<db>"
func NewRedisStoreFromURL(redisURL string) (*RedisStore, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}
	return NewRedisStore(opt)
}

// Close closes the Redis client.
func (s *RedisStore) Close() error {
	return s.client.Close()
}

// Set stores a value under the given key with optional TTL.
// The value is marshaled as JSON.
func (s *RedisStore) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal value: %w", err)
	}

	if err := s.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}
	return nil
}

// Get retrieves the value for the given key and unmarshals it into dest.
// Returns ErrNotFound if the key does not exist.
func (s *RedisStore) Get(ctx context.Context, key string, dest interface{}) error {
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrNotFound
		}
		return fmt.Errorf("redis get: %w", err)
	}

	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("unmarshal value: %w", err)
	}
	return nil
}

// Delete removes the key from the store.
func (s *RedisStore) Delete(ctx context.Context, key string) error {
	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("redis del: %w", err)
	}
	return nil
}

// List returns all keys matching the given pattern (Redis glob style).
// Use with caution on large datasets; consider SCAN for production.
func (s *RedisStore) List(ctx context.Context, pattern string) ([]string, error) {
	keys, err := s.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("redis keys: %w", err)
	}
	return keys, nil
}

// SetHash stores multiple fields in a Redis hash.
func (s *RedisStore) SetHash(ctx context.Context, key string, fields map[string]interface{}) error {
	if err := s.client.HSet(ctx, key, fields).Err(); err != nil {
		return fmt.Errorf("redis hset: %w", err)
	}
	return nil
}

// GetHash retrieves all fields from a Redis hash.
func (s *RedisStore) GetHash(ctx context.Context, key string) (map[string]string, error) {
	return s.client.HGetAll(ctx, key).Result()
}

// Expire sets a timeout on a key.
func (s *RedisStore) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return s.client.Expire(ctx, key, ttl).Err()
}