// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"math"
	"math/rand"
	"time"
)

// LogarithmicBackoff implements a logarithmic backoff strategy with optional jitter.
// The delay grows logarithmically with the attempt number.
type LogarithmicBackoff struct {
	// InitialInterval is the base delay; it is always added.
	InitialInterval time.Duration
	// Scale is multiplied by log(attempt+1) to determine the additional delay.
	Scale time.Duration
	// MaxInterval caps the total delay; if zero, no cap.
	MaxInterval time.Duration
	// MaxElapsedTime caps the total time spent backing off; if zero, no cap.
	MaxElapsedTime time.Duration
	// Jitter, if true, applies full jitter: the actual delay is a random
	// value between 0 and the computed interval.
	Jitter bool
	// Random source for jitter; if nil, the global rand is used.
	Rand *rand.Rand

	// internal state
	attempt   int
	startTime time.Time
}

// NewLogarithmicBackoff creates a new logarithmic backoff with sensible defaults.
// Defaults: initial 100ms, scale 50ms, max 5s, no jitter, no time cap.
func NewLogarithmicBackoff() *LogarithmicBackoff {
	return &LogarithmicBackoff{
		InitialInterval: 100 * time.Millisecond,
		Scale:           50 * time.Millisecond,
		MaxInterval:     5 * time.Second,
		MaxElapsedTime:  0, // no limit
		Jitter:          false,
		Rand:            nil, // use global rand
	}
}

// Reset clears the internal state, making the backoff start from the beginning.
func (b *LogarithmicBackoff) Reset() {
	b.attempt = 0
	b.startTime = time.Time{}
}

// NextBackoff returns the next delay to wait, based on the current state.
// It returns -1 if the max elapsed time has been exceeded.
func (b *LogarithmicBackoff) NextBackoff() time.Duration {
	// Check elapsed time limit
	if b.MaxElapsedTime > 0 {
		if b.startTime.IsZero() {
			b.startTime = time.Now()
		} else if time.Since(b.startTime) > b.MaxElapsedTime {
			return -1
		}
	}

	// Increment attempt counter (first call yields attempt=1)
	b.attempt++

	// Compute base delay: initial + scale * log(1 + attempt)
	// Using natural logarithm; for attempt=1, log(2) ≈ 0.693
	logVal := math.Log1p(float64(b.attempt)) // log(1 + attempt)
	base := b.InitialInterval + time.Duration(float64(b.Scale)*logVal)

	// Apply cap
	if b.MaxInterval > 0 && base > b.MaxInterval {
		base = b.MaxInterval
	}

	// Apply jitter
	if b.Jitter {
		// Full jitter: random duration between 0 and base
		return time.Duration(b.randInt63n(int64(base)))
	}
	return base
}

// randInt63n returns a non-negative pseudo-random number in [0,n) using the
// backoff's random source, or the global rand if nil.
func (b *LogarithmicBackoff) randInt63n(n int64) int64 {
	if n <= 0 {
		return 0
	}
	if b.Rand != nil {
		return b.Rand.Int63n(n)
	}
	return rand.Int63n(n)
}

// ------------------------------------------------------------------------
// Retry with logarithmic backoff
// ------------------------------------------------------------------------

// RetryWithLogarithmicBackoff executes the given function with logarithmic backoff.
// It returns nil on success, or the last error if all attempts fail.
// The function may return a special error to stop retrying immediately
// by implementing a Stop() bool method (if Stop() returns true).
func RetryWithLogarithmicBackoff(backoff *LogarithmicBackoff, fn func() error) error {
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