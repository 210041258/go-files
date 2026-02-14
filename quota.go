// Package concurrency provides reusable concurrency primitives including
// a token bucket rate limiter and a blocking queue.
package testutils

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ------------------------------------------------------------------------
// 1. Rate Limiter Interface
// ------------------------------------------------------------------------

// Limiter defines the contract for rate limiting algorithms.
type Limiter interface {
	// Allow checks if a request is permitted immediately and consumes one token.
	Allow() bool

	// Wait blocks until a token is available or the context is cancelled.
	Wait(ctx context.Context) error

	// WaitN blocks until n tokens are available or the context is cancelled.
	WaitN(ctx context.Context, n int) error
}

// ------------------------------------------------------------------------
// 2. TokenBucket – Correct Implementation
// ------------------------------------------------------------------------

// TokenBucket implements a token bucket rate limiter with floating‑point
// refill precision and condition‑variable based waiting.
type TokenBucket struct {
	capacity float64
	tokens   float64
	rate     float64 // tokens per second

	mu         sync.Mutex
	cond       *sync.Cond
	lastRefill time.Time
}

// NewTokenBucket creates a new token bucket.
// capacity: maximum tokens (burst size)
// rate: how many tokens are added per second
func NewTokenBucket(capacity int, rate float64) *TokenBucket {
	if capacity <= 0 {
		panic("capacity must be positive")
	}
	if rate <= 0 {
		panic("rate must be positive")
	}
	tb := &TokenBucket{
		capacity:   float64(capacity),
		tokens:     float64(capacity), // start full
		rate:       rate,
		lastRefill: time.Now(),
	}
	tb.cond = sync.NewCond(&tb.mu)
	return tb
}

// refill adds tokens based on elapsed time.
// Must be called with the mutex held.
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * tb.rate
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
	tb.lastRefill = now
}

// Allow consumes one token if available.
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.refill()
	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

// Wait blocks until a token is available.
func (tb *TokenBucket) Wait(ctx context.Context) error {
	return tb.WaitN(ctx, 1)
}

// WaitN blocks until n tokens are available.
// It returns an error if n exceeds capacity, if the context is cancelled,
// or if the bucket is closed (if we add close later).
func (tb *TokenBucket) WaitN(ctx context.Context, n int) error {
	if n <= 0 {
		return nil
	}
	need := float64(n)
	tb.mu.Lock()
	defer tb.mu.Unlock()

	// Check if n exceeds capacity – can never be satisfied
	if need > tb.capacity {
		return fmt.Errorf("WaitN: requested %d tokens exceeds capacity %d",
			n, int(tb.capacity))
	}

	for {
		tb.refill()
		if tb.tokens >= need {
			tb.tokens -= need
			return nil
		}
		// Not enough tokens; calculate wait time for the deficit.
		deficit := need - tb.tokens
		// time needed = deficit / rate (seconds)
		waitTime := time.Duration((deficit / tb.rate) * float64(time.Second))

		// Use a channel to unblock when the condition variable wakes us.
		// We'll create a timer and wait on the condition variable.
		timer := time.NewTimer(waitTime)
		defer timer.Stop()

		// Wait for either the condition broadcast or timer expiry.
		// Because we hold the lock, we must create a channel to unblock.
		waitChan := make(chan bool, 1)
		go func() {
			select {
			case <-timer.C:
				tb.cond.Broadcast() // wake up all waiters to recheck
			case <-ctx.Done():
				tb.cond.Broadcast()
			}
			waitChan <- true
		}()

		tb.cond.Wait() // releases lock, waits for Broadcast, reacquires lock
		<-waitChan

		// Check context error after waking
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
}

// ------------------------------------------------------------------------
// 3. Throttle Helper
// ------------------------------------------------------------------------

// Throttle executes fn under rate limiting. It waits for a token before each call.
func Throttle(ctx context.Context, limiter Limiter, fn func() error) error {
	if err := limiter.Wait(ctx); err != nil {
		return fmt.Errorf("throttle: %w", err)
	}
	return fn()
}

// ------------------------------------------------------------------------
// 4. Generic Blocking Queue with Context Support
// ------------------------------------------------------------------------

// Queue is a thread‑safe FIFO queue with optional capacity limit.
// It supports blocking Enqueue/Dequeue with context cancellation.
type Queue[T any] struct {
	items    []T
	capacity int
	mu       sync.Mutex
	notFull  *sync.Cond
	notEmpty *sync.Cond
	closed   bool
}

// NewQueue creates a new queue. If capacity ≤ 0, the queue is unbounded.
func NewQueue[T any](capacity int) *Queue[T] {
	q := &Queue[T]{
		items:    make([]T, 0, max(capacity, 0)),
		capacity: capacity,
	}
	q.notFull = sync.NewCond(&q.mu)
	q.notEmpty = sync.NewCond(&q.mu)
	return q
}

// Enqueue adds an item to the queue. It blocks if the queue is at capacity.
// Returns an error if the queue is closed.
func (q *Queue[T]) Enqueue(item T) error {
	return q.EnqueueContext(context.Background(), item)
}

// EnqueueContext adds an item with context cancellation.
func (q *Queue[T]) EnqueueContext(ctx context.Context, item T) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Wait for capacity or cancellation
	for q.capacity > 0 && len(q.items) >= q.capacity {
		if q.closed {
			return fmt.Errorf("enqueue on closed queue")
		}
		// Wait with context
		waitDone := make(chan struct{})
		go func() {
			q.notFull.Wait()
			close(waitDone)
		}()
		select {
		case <-ctx.Done():
			// Wake up the waiting goroutine by broadcasting
			q.notFull.Broadcast()
			return ctx.Err()
		case <-waitDone:
		}
	}

	if q.closed {
		return fmt.Errorf("enqueue on closed queue")
	}

	q.items = append(q.items, item)
	q.notEmpty.Signal()
	return nil
}

// Dequeue removes and returns an item from the queue.
// It blocks if the queue is empty. Returns false if the queue is closed and empty.
func (q *Queue[T]) Dequeue() (T, bool) {
	return q.DequeueContext(context.Background())
}

// DequeueContext removes an item with context cancellation.
func (q *Queue[T]) DequeueContext(ctx context.Context) (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.items) == 0 {
		if q.closed {
			var zero T
			return zero, false
		}
		waitDone := make(chan struct{})
		go func() {
			q.notEmpty.Wait()
			close(waitDone)
		}()
		select {
		case <-ctx.Done():
			q.notEmpty.Broadcast()
			var zero T
			return zero, false
		case <-waitDone:
		}
	}

	item := q.items[0]
	q.items = q.items[1:]
	q.notFull.Signal()
	return item, true
}

// Close prevents further enqueues. Dequeues continue until the queue is empty.
func (q *Queue[T]) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.notFull.Broadcast()
	q.notEmpty.Broadcast()
}

// Len returns the number of items currently in the queue.
func (q *Queue[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

// Cap returns the capacity of the queue (0 means unbounded).
func (q *Queue[T]) Cap() int {
	return q.capacity
}

// IsClosed returns true if the queue has been closed.
func (q *Queue[T]) IsClosed() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.closed
}

// max returns the larger of two ints.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
