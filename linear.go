// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"math/rand"
	"time"
)

// LinearBackoff implements a linear backoff strategy with optional jitter.
// The delay increases by a fixed increment after each attempt.
type LinearBackoff struct {
	// InitialInterval is the first delay; must be > 0.
	InitialInterval time.Duration
	// Increment is added to the delay after each attempt.
	Increment time.Duration
	// MaxInterval caps the delay; if zero, no cap.
	MaxInterval time.Duration
	// MaxElapsedTime caps the total time spent backing off; if zero, no cap.
	MaxElapsedTime time.Duration
	// Jitter, if true, applies full jitter: the actual delay is a random
	// value between 0 and the computed interval.
	Jitter bool
	// Random source for jitter; if nil, the global rand is used.
	Rand *rand.Rand

	// internal state
	currentInterval time.Duration
	startTime       time.Time
}

// NewLinearBackoff creates a new linear backoff with sensible defaults.
// Defaults: initial 100ms, increment 100ms, max 5s, no jitter, no time cap.
func NewLinearBackoff() *LinearBackoff {
	return &LinearBackoff{
		InitialInterval: 100 * time.Millisecond,
		Increment:       100 * time.Millisecond,
		MaxInterval:     5 * time.Second,
		MaxElapsedTime:  0, // no limit
		Jitter:          false,
		Rand:            nil, // use global rand
	}
}

// Reset clears the internal state, making the backoff start from the beginning.
func (b *LinearBackoff) Reset() {
	b.currentInterval = 0
	b.startTime = time.Time{}
}

// NextBackoff returns the next delay to wait, based on the current state.
// It returns -1 if the max elapsed time has been exceeded.
func (b *LinearBackoff) NextBackoff() time.Duration {
	// Check elapsed time limit
	if b.MaxElapsedTime > 0 {
		if b.startTime.IsZero() {
			b.startTime = time.Now()
		} else if time.Since(b.startTime) > b.MaxElapsedTime {
			return -1
		}
	}

	// Compute next interval
	if b.currentInterval == 0 {
		b.currentInterval = b.InitialInterval
	} else {
		next := b.currentInterval + b.Increment
		if b.MaxInterval > 0 && next > b.MaxInterval {
			next = b.MaxInterval
		}
		b.currentInterval = next
	}

	// Apply jitter
	if b.Jitter {
		// Full jitter: random duration between 0 and currentInterval
		return time.Duration(b.randInt63n(int64(b.currentInterval)))
	}
	return b.currentInterval
}

// randInt63n returns a non-negative pseudo-random number in [0,n) using the
// backoff's random source, or the global rand if nil.
func (b *LinearBackoff) randInt63n(n int64) int64 {
	if n <= 0 {
		return 0
	}
	if b.Rand != nil {
		return b.Rand.Int63n(n)
	}
	return rand.Int63n(n)
}

// ------------------------------------------------------------------------
// Retry with linear backoff
// ------------------------------------------------------------------------

// RetryWithLinearBackoff executes the given function with linear backoff.
// It returns nil on success, or the last error if all attempts fail.
// The function may return a special error to stop retrying immediately
// by implementing a Stop() bool method (if Stop() returns true).
func RetryWithLinearBackoff(backoff *LinearBackoff, fn func() error) error {
	backoff.Reset()
	for {
		err := fn()
		if err == nil {
			return nil
		}
		// Optionally check for a sentinel error to stop retrying
		if stop, ok := err.(interface{ Stop() bool }); ok && stop.Stop() {
			return err
		}
		delay := backoff.NextBackoff()
		if delay < 0 {
			return err // max elapsed time exceeded
		}
		time.Sleep(delay)
	}
}