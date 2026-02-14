// Package ratelimit provides rate limiting primitives and middleware
// for HTTP, gRPC, and WebSocket servers. It supports token bucket,
// sliding window, and per‑key limiters with optional Redis backend.
package testutils

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ----------------------------------------------------------------------
// Limiter interface
// ----------------------------------------------------------------------

// Limiter defines the interface that all rate limiters must implement.
type Limiter interface {
	// Allow checks whether a single event is permitted.
	Allow() bool

	// Wait blocks until an event is permitted or the context is cancelled.
	Wait(ctx context.Context) error

	// Reserve returns a Reservation that indicates how long to wait.
	// Reserve(ctx context.Context) Reservation // optional, can be added if needed
}

// Reservation represents a future token reservation.
// Simplified version; full implementation would include delay, ok, etc.
type Reservation interface {
	// Delay returns the wait time before the reservation can be used.
	Delay() time.Duration
	// OK returns whether the reservation is valid.
	OK() bool
}

// ----------------------------------------------------------------------
// Token bucket (using x/time/rate)
// ----------------------------------------------------------------------

// TokenBucket is a rate limiter that uses the token bucket algorithm.
// It wraps golang.org/x/time/rate.Limiter.
type TokenBucket struct {
	limiter *rate.Limiter
}

// NewTokenBucket creates a new token bucket limiter.
// rate: number of tokens generated per second.
// burst: maximum token accumulation.
func NewTokenBucket(rateLimit rate.Limit, burst int) *TokenBucket {
	return &TokenBucket{
		limiter: rate.NewLimiter(rateLimit, burst),
	}
}

// NewTokenBucketPerSec creates a limiter with the given number of
// operations per second and burst equal to the rate (common pattern).
func NewTokenBucketPerSec(opsPerSec int) *TokenBucket {
	return NewTokenBucket(rate.Limit(opsPerSec), opsPerSec)
}

// Allow implements Limiter.
func (t *TokenBucket) Allow() bool {
	return t.limiter.Allow()
}

// Wait implements Limiter.
func (t *TokenBucket) Wait(ctx context.Context) error {
	return t.limiter.Wait(ctx)
}

// Reserve returns a reservation.
func (t *TokenBucket) Reserve(ctx context.Context) *rate.Reservation {
	return t.limiter.Reserve()
}

// ----------------------------------------------------------------------
// Sliding window log (in‑memory)
// ----------------------------------------------------------------------

// SlidingWindow is an in‑memory sliding window log limiter.
type SlidingWindow struct {
	mu       sync.Mutex
	window   time.Duration
	max      int
	log      map[string][]time.Time // per‑key log
	nowFunc  func() time.Time       // for testing
}

// NewSlidingWindow creates a sliding window log limiter that allows
// up to max requests in the last window duration.
// This limiter is per‑key; use PerKeyLimiter for multiple clients.
func NewSlidingWindow(window time.Duration, max int) *SlidingWindow {
	return &SlidingWindow{
		window:  window,
		max:     max,
		log:     make(map[string][]time.Time),
		nowFunc: time.Now,
	}
}

// Allow checks if the event identified by key is allowed.
func (s *SlidingWindow) Allow(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.nowFunc()
	cutoff := now.Add(-s.window)

	// Clean old entries and get current count.
	entries := s.log[key]
	valid := entries[:0]
	for _, t := range entries {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	count := len(valid)

	if count < s.max {
		// Allow: add new timestamp.
		s.log[key] = append(valid, now)
		return true
	}
	s.log[key] = valid // update with cleaned slice
	return false
}

// ----------------------------------------------------------------------
// Per‑key limiter (factory)
// ----------------------------------------------------------------------

// PerKeyLimiter manages a separate limiter for each key (e.g., IP, user ID).
// It automatically cleans up unused limiters after a period of inactivity.
type PerKeyLimiter struct {
	mu          sync.Mutex
	limiters    map[string]*entry
	newLimiter  func() Limiter
	ttl         time.Duration
	cleanupTick time.Duration
	stop        chan struct{}
	wg          sync.WaitGroup
}

type entry struct {
	limiter   Limiter
	lastSeen  time.Time
}

// NewPerKeyLimiter creates a per‑key limiter factory.
// newLimiter is a function that returns a new Limiter instance for each key.
// ttl is the idle time after which a limiter is removed.
// cleanupInterval specifies how often the cleanup runs.
func NewPerKeyLimiter(newLimiter func() Limiter, ttl, cleanupInterval time.Duration) *PerKeyLimiter {
	p := &PerKeyLimiter{
		limiters:    make(map[string]*entry),
		newLimiter:  newLimiter,
		ttl:         ttl,
		cleanupTick: cleanupInterval,
		stop:        make(chan struct{}),
	}
	p.wg.Add(1)
	go p.cleanupLoop()
	return p
}

// Stop terminates the cleanup goroutine.
func (p *PerKeyLimiter) Stop() {
	close(p.stop)
	p.wg.Wait()
}

// Allow checks whether the event for the given key is allowed.
func (p *PerKeyLimiter) Allow(key string) bool {
	return p.getLimiter(key).Allow()
}

// Wait blocks until the limiter for the given key permits an event.
func (p *PerKeyLimiter) Wait(ctx context.Context, key string) error {
	return p.getLimiter(key).Wait(ctx)
}

// getLimiter retrieves or creates the limiter for a key.
func (p *PerKeyLimiter) getLimiter(key string) Limiter {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.limiters[key]
	if !ok {
		e = &entry{
			limiter:  p.newLimiter(),
			lastSeen: time.Now(),
		}
		p.limiters[key] = e
	} else {
		e.lastSeen = time.Now()
	}
	return e.limiter
}

// cleanupLoop removes limiters that haven't been used for more than ttl.
func (p *PerKeyLimiter) cleanupLoop() {
	defer p.wg.Done()
	ticker := time.NewTicker(p.cleanupTick)
	defer ticker.Stop()
	for {
		select {
		case <-p.stop:
			return
		case <-ticker.C:
			p.cleanup()
		}
	}
}

func (p *PerKeyLimiter) cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()
	cutoff := time.Now().Add(-p.ttl)
	for key, e := range p.limiters {
		if e.lastSeen.Before(cutoff) {
			delete(p.limiters, key)
		}
	}
}

// ----------------------------------------------------------------------
// Distributed rate limiter (Redis)
// ----------------------------------------------------------------------

// RedisSlidingWindow implements a sliding window log using Redis.
type RedisSlidingWindow struct {
	client *redis.Client
	window time.Duration
	max    int
	prefix string
}

// NewRedisSlidingWindow creates a distributed rate limiter using Redis.
// It stores sorted sets per key with timestamps as scores.
func NewRedisSlidingWindow(client *redis.Client, window time.Duration, max int, prefix string) *RedisSlidingWindow {
	return &RedisSlidingWindow{
		client: client,
		window: window,
		max:    max,
		prefix: prefix,
	}
}

// Allow checks if the event for the given key is allowed.
func (r *RedisSlidingWindow) Allow(ctx context.Context, key string) (bool, error) {
	key = r.prefix + ":" + key
	now := time.Now().UnixMilli()
	cutoff := now - r.window.Milliseconds()

	pipe := r.client.Pipeline()
	// Remove old entries.
	pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(cutoff, 10))
	// Count current entries.
	countCmd := pipe.ZCard(ctx, key)
	// Add current request timestamp.
	pipe.ZAdd(ctx, key, redis.Z{Score: float64(now), Member: now})
	// Set expiration on the key to avoid memory leaks.
	pipe.Expire(ctx, key, r.window*2)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}

	count := countCmd.Val()
	if count < int64(r.max) {
		return true, nil
	}
	// Rollback the added entry if limit exceeded.
	r.client.ZRem(ctx, key, now)
	return false, nil
}

// ----------------------------------------------------------------------
// HTTP middleware
// ----------------------------------------------------------------------

// HTTPConfig configures the HTTP rate limiter middleware.
type HTTPConfig struct {
	// Limiter is the rate limiter to use.
	Limiter Limiter
	// KeyFunc extracts a rate limiting key from the request (e.g., IP, user ID).
	// If nil, the client IP is used.
	KeyFunc func(r *http.Request) string
	// OnLimitExceeded is called when rate limit is exceeded.
	// If nil, a default 429 response is sent.
	OnLimitExceeded http.HandlerFunc
}

// HTTPMiddleware creates a rate limiting middleware for HTTP servers.
func HTTPMiddleware(cfg HTTPConfig) func(http.Handler) http.Handler {
	if cfg.KeyFunc == nil {
		cfg.KeyFunc = func(r *http.Request) string {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				return r.RemoteAddr
			}
			return ip
		}
	}
	if cfg.OnLimitExceeded == nil {
		cfg.OnLimitExceeded = func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-RateLimit-Reset", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"too many requests"}`))
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := cfg.KeyFunc(r)
			// If the limiter is per-key, we need to adapt.
			// For simplicity, we assume the limiter is already per-key or global.
			// For PerKeyLimiter, we need to call Allow(key).
			if pk, ok := cfg.Limiter.(interface {
				Allow(string) bool
			}); ok {
				if !pk.Allow(key) {
					cfg.OnLimitExceeded(w, r)
					return
				}
			} else {
				if !cfg.Limiter.Allow() {
					cfg.OnLimitExceeded(w, r)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ----------------------------------------------------------------------
// gRPC interceptors
// ----------------------------------------------------------------------

// UnaryInterceptor returns a grpc.UnaryServerInterceptor that limits
// incoming RPCs based on the provided limiter and key extractor.
func UnaryInterceptor(limiter interface {
	Allow(string) bool
}, keyFunc func(ctx context.Context, fullMethod string) string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		key := keyFunc(ctx, info.FullMethod)
		if !limiter.Allow(key) {
			return nil, status.Errorf(codes.ResourceExhausted, "rate limit exceeded")
		}
		return handler(ctx, req)
	}
}

// StreamInterceptor returns a grpc.StreamServerInterceptor.
func StreamInterceptor(limiter interface {
	Allow(string) bool
}, keyFunc func(ctx context.Context, fullMethod string) string) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		key := keyFunc(ss.Context(), info.FullMethod)
		if !limiter.Allow(key) {
			return status.Errorf(codes.ResourceExhausted, "rate limit exceeded")
		}
		return handler(srv, ss)
	}
}

// ----------------------------------------------------------------------
// WebSocket / TCP connection limiter
// ----------------------------------------------------------------------

// ConnLimiter limits the number of concurrent connections per key.
type ConnLimiter struct {
	mu      sync.Mutex
	conns   map[string]int
	max     int
}

// NewConnLimiter creates a limiter that restricts concurrent connections.
func NewConnLimiter(max int) *ConnLimiter {
	return &ConnLimiter{
		conns: make(map[string]int),
		max:   max,
	}
}

// Acquire increments the connection count for the given key.
// Returns true if the limit has not been exceeded.
func (c *ConnLimiter) Acquire(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conns[key] >= c.max {
		return false
	}
	c.conns[key]++
	return true
}

// Release decrements the connection count.
func (c *ConnLimiter) Release(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if val := c.conns[key]; val > 0 {
		c.conns[key]--
		if c.conns[key] == 0 {
			delete(c.conns, key)
		}
	}
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------

// func exampleHTTP() {
//     // Global token bucket: 10 req/s, burst 10
//     limiter := ratelimit.NewTokenBucketPerSec(10)
//     middleware := ratelimit.HTTPMiddleware(ratelimit.HTTPConfig{
//         Limiter: limiter,
//     })
//
//     mux := http.NewServeMux()
//     mux.Handle("/api", middleware(http.HandlerFunc(apiHandler)))
//     http.ListenAndServe(":8080", mux)
// }

// func examplePerIP() {
//     // Per IP token bucket: each IP gets 5 req/s, burst 5
//     factory := func() ratelimit.Limiter {
//         return ratelimit.NewTokenBucketPerSec(5)
//     }
//     perIP := ratelimit.NewPerKeyLimiter(factory, 10*time.Minute, 1*time.Minute)
//     defer perIP.Stop()
//
//     middleware := ratelimit.HTTPMiddleware(ratelimit.HTTPConfig{
//         Limiter: perIP, // PerKeyLimiter implements Allow(string) bool
//     })
//
//     http.ListenAndServe(":8080", middleware(http.DefaultServeMux))
// }

// func exampleGRPC() {
//     perIP := ratelimit.NewPerKeyLimiter(...)
//     interceptor := ratelimit.UnaryInterceptor(perIP, func(ctx context.Context, method string) string {
//         // extract IP from peer info
//         p, _ := peer.FromContext(ctx)
//         return p.Addr.String()
//     })
//     s := grpc.NewServer(grpc.UnaryInterceptor(interceptor))
// }