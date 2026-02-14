package testutils

import (
	"context"
	"errors"
	"sync"
)

// ErrQueueClosed is returned when operations are performed on a closed queue.
var ErrQueueClosed = errors.New("queue: closed")

// Queue is a generic, thread-safe FIFO queue with optional capacity.
// Zero or negative capacity means unbounded.
type Queue[T any] struct {
	mu       sync.Mutex
	notEmpty *sync.Cond // Signaled when queue becomes non-empty
	notFull  *sync.Cond // Signaled when queue becomes non-full (only if bounded)
	data     []T
	cap      int
	closed   bool
}

// NewQueue creates a new Queue. If capacity ≤ 0, the queue is unbounded.
func NewQueue[T any](capacity int) *Queue[T] {
	if capacity < 0 {
		capacity = 0
	}
	q := &Queue[T]{
		cap:  capacity,
		data: make([]T, 0, capacity), // Pre-allocate if bounded
	}
	q.notEmpty = sync.NewCond(&q.mu)
	q.notFull = sync.NewCond(&q.mu)
	return q
}

// Enqueue adds an item to the queue. Blocks if the queue is bounded and full.
// Returns ErrQueueClosed if the queue is closed.
func (q *Queue[T]) Enqueue(item T) error {
	return q.EnqueueContext(context.Background(), item)
}

// EnqueueContext adds an item with context cancellation support.
// It blocks if the queue is bounded and full, or until the context is done.
func (q *Queue[T]) EnqueueContext(ctx context.Context, item T) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return ErrQueueClosed
	}

	// Wait for space if bounded and full
	for q.cap > 0 && len(q.data) >= q.cap && !q.closed {
		if err := q.waitCond(ctx, q.notFull); err != nil {
			return err // context cancelled or deadline exceeded
		}
	}

	// Check closed status again after waking
	if q.closed {
		return ErrQueueClosed
	}

	q.data = append(q.data, item)
	q.notEmpty.Signal() // wake one dequeuer
	return nil
}

// Dequeue removes and returns the oldest item. Blocks if the queue is empty.
// Returns ErrQueueClosed if the queue is closed and empty.
func (q *Queue[T]) Dequeue() (T, error) {
	return q.DequeueContext(context.Background())
}

// DequeueContext removes an item with context cancellation support.
func (q *Queue[T]) DequeueContext(ctx context.Context) (T, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	var zero T

	for len(q.data) == 0 && !q.closed {
		if err := q.waitCond(ctx, q.notEmpty); err != nil {
			return zero, err // context cancelled or deadline exceeded
		}
	}

	if len(q.data) == 0 && q.closed {
		return zero, ErrQueueClosed
	}

	item := q.data[0]
	q.data = q.data[1:]

	// Optimization: Reset slice to zero length to free underlying array if empty
	if len(q.data) == 0 {
		q.data = q.data[:0]
	}

	if q.cap > 0 {
		q.notFull.Signal() // wake one enqueuer (bounded queue)
	}
	return item, nil
}

// TryEnqueue attempts to add an item without blocking.
// Returns false immediately if the queue is bounded and full, or if closed.
func (q *Queue[T]) TryEnqueue(item T) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed || (q.cap > 0 && len(q.data) >= q.cap) {
		return false
	}

	q.data = append(q.data, item)
	q.notEmpty.Signal()
	return true
}

// TryDequeue attempts to remove an item without blocking.
// Returns (zero, false) if empty.
func (q *Queue[T]) TryDequeue() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	var zero T
	if len(q.data) == 0 {
		return zero, false
	}

	item := q.data[0]
	q.data = q.data[1:]

	if len(q.data) == 0 {
		q.data = q.data[:0]
	}

	if q.cap > 0 {
		q.notFull.Signal()
	}
	return item, true
}

// Peek returns the oldest item without removing it.
// Returns (zero, false) if empty.
func (q *Queue[T]) Peek() (T, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	var zero T
	if len(q.data) == 0 {
		return zero, false
	}
	return q.data[0], true
}

// Len returns the current number of items in the queue.
func (q *Queue[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.data)
}

// Cap returns the capacity of the queue (0 = unbounded).
func (q *Queue[T]) Cap() int {
	return q.cap // cap is immutable, technically no lock needed
}

// Clear removes all items from the queue and notifies waiting enqueuers.
func (q *Queue[T]) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Reset length to 0
	q.data = q.data[:0]

	if q.cap > 0 {
		q.notFull.Broadcast() // wake all enqueuers waiting for space
	}
	// Dequeuers waiting on empty queue stay waiting (unless also closed)
}

// Close marks the queue as closed and wakes all waiters.
// Subsequent Enqueue calls return ErrQueueClosed.
// Dequeue calls continue until the queue is empty, then return ErrQueueClosed.
func (q *Queue[T]) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		q.closed = true
		q.notEmpty.Broadcast() // wake all dequeuers to check empty/closed status
		if q.cap > 0 {
			q.notFull.Broadcast() // wake all enqueuers to check closed status
		}
	}
}

// IsClosed returns whether the queue is closed.
func (q *Queue[T]) IsClosed() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.closed
}

// waitCond waits on a condition variable with context cancellation.
// It spawns a detached goroutine to handle cancellation, which interacts
// safely with cond.Broadcast() to wake up the main waiting thread.
// This avoids deadlock that occurs when a select statement waiting for both
// cond.Wait() and ctx.Done() holds the lock (which Wait() doesn't like).
func (q *Queue[T]) waitCond(ctx context.Context, cond *sync.Cond) error {
	// Create a channel to signal that the context has been handled
	stop := make(chan struct{})
	go func() {
		defer close(stop)
		<-ctx.Done()

		// Acquire lock to broadcast
		q.mu.Lock()
		defer q.mu.Unlock()
		cond.Broadcast()
	}()

	// Block on condition (this releases the mutex internally)
	cond.Wait()

	// Determine why we woke up
	select {
	case <-stop:
		return ctx.Err()
	default:
		return nil
	}
}
