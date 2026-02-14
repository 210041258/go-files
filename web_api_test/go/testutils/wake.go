// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"sync"
	"time"
)

// Waiter defines the interface for something that can block until woken.
type Waiter interface {
	// Wait blocks until the Wake method is called.
	// If the wake‑up has already occurred, Wait returns immediately.
	Wait()
}

// Waker defines the interface for something that can wake up waiters.
type Waker interface {
	// Wake unblocks all goroutines waiting on this Waker.
	// Subsequent calls to Wake have no effect.
	Wake()
}

// Wakeable combines both Waiter and Waker.
type Wakeable interface {
	Waiter
	Waker
}

// ------------------------------------------------------------------------
// wakeable – implementation using a channel
// ------------------------------------------------------------------------

type wakeable struct {
	once sync.Once
	ch   chan struct{}
}

// NewWakeable creates a new Wakeable that starts in the non‑woken state.
func NewWakeable() *wakeable {
	return &wakeable{
		ch: make(chan struct{}),
	}
}

// Wait blocks until Wake is called. If Wake has already been called,
// Wait returns immediately.
func (w *wakeable) Wait() {
	<-w.ch
}

// Wake unblocks all goroutines waiting on this Wakeable.
// It is safe to call Wake multiple times; only the first call has any effect.
func (w *wakeable) Wake() {
	w.once.Do(func() {
		close(w.ch)
	})
}

// Reset re‑arms the Wakeable, discarding any previous wake‑up.
// After Reset, Wait will block again until the next Wake.
// Use with caution; not safe for concurrent use with Wait or Wake.
func (w *wakeable) Reset() {
	w.ch = make(chan struct{})
	w.once = sync.Once{}
}

// ------------------------------------------------------------------------
// WakeAfter returns a Wakeable that wakes after the given duration.
// This is similar to time.After but returns a Wakeable that can be waited on.
func WakeAfter(d time.Duration) Wakeable {
	w := NewWakeable()
	time.AfterFunc(d, w.Wake)
	return w
}

// ------------------------------------------------------------------------
// WakeFunc – functional adapter for Waiter
// ------------------------------------------------------------------------

// WakeFunc is a function type that implements Waiter.
type WakeFunc func()

// Wait calls the underlying function.
func (f WakeFunc) Wait() {
	f()
}