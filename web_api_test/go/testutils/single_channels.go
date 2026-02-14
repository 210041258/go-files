// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"sync"
)

// ------------------------------------------------------------------------
// SingleChannel – a channel that holds at most one value
// ------------------------------------------------------------------------

// SingleChannel is a container that can hold a single value.
// It is useful for passing a result from one goroutine to many waiters,
// or for simulating a future/promise in tests.
type SingleChannel[T any] struct {
	mu      sync.Mutex
	done    chan struct{}
	value   T
	has     bool
	closed  bool
}

// NewSingleChannel creates a new SingleChannel with no value set.
func NewSingleChannel[T any]() *SingleChannel[T] {
	return &SingleChannel[T]{
		done: make(chan struct{}),
	}
}

// Send attempts to store a value. It returns true if the value was stored,
// false if a value was already set or the channel was closed.
// Send never blocks.
func (sc *SingleChannel[T]) Send(val T) bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if sc.has || sc.closed {
		return false
	}
	sc.value = val
	sc.has = true
	close(sc.done)
	return true
}

// Get blocks until a value is available or the channel is closed.
// If a value was sent, it returns the value and true.
// If the channel was closed without a value, it returns the zero value and false.
func (sc *SingleChannel[T]) Get() (T, bool) {
	<-sc.done
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if !sc.has {
		var zero T
		return zero, false
	}
	return sc.value, true
}

// TryGet returns the value immediately if available, otherwise it returns
// the zero value and false. It never blocks.
func (sc *SingleChannel[T]) TryGet() (T, bool) {
	select {
	case <-sc.done:
		sc.mu.Lock()
		defer sc.mu.Unlock()
		if !sc.has {
			var zero T
			return zero, false
		}
		return sc.value, true
	default:
		var zero T
		return zero, false
	}
}

// Close marks the channel as closed, meaning no value will ever be sent.
// Any current or future call to Get will return the zero value and false.
// Close returns true if the channel was not already closed or set.
func (sc *SingleChannel[T]) Close() bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if sc.closed || sc.has {
		return false
	}
	sc.closed = true
	close(sc.done)
	return true
}

// Done returns a channel that is closed when a value has been set or the
// channel is closed. This allows use in select statements.
func (sc *SingleChannel[T]) Done() <-chan struct{} {
	return sc.done
}

// ------------------------------------------------------------------------
// Signal – a one‑shot boolean event
// ------------------------------------------------------------------------

// Signal is a one‑time event that can be waited on by multiple goroutines.
// It is like a sync.Cond but simpler and specialised for a single notification.
type Signal struct {
	ch chan struct{}
	once sync.Once
}

// NewSignal creates a new signal in the unsignaled state.
func NewSignal() *Signal {
	return &Signal{
		ch: make(chan struct{}),
	}
}

// Notify triggers the signal. Any goroutine waiting on Wait will unblock.
// Subsequent calls to Notify have no effect.
func (s *Signal) Notify() {
	s.once.Do(func() {
		close(s.ch)
	})
}

// Wait blocks until the signal is notified.
func (s *Signal) Wait() {
	<-s.ch
}

// TryWait returns true immediately if the signal has been notified,
// otherwise it returns false (non‑blocking).
func (s *Signal) TryWait() bool {
	select {
	case <-s.ch:
		return true
	default:
		return false
	}
}

// Done returns a channel that is closed when the signal is notified.
// This allows use in select statements.
func (s *Signal) Done() <-chan struct{} {
	return s.ch
}

// Reset re‑arms the signal. After Reset, it returns to the unsignaled state.
// Use with caution; not safe for concurrent use with Wait or Notify.
func (s *Signal) Reset() {
	s.ch = make(chan struct{})
	s.once = sync.Once{}
}