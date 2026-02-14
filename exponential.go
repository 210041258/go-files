// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"math"
	"math/rand"
	"time"
)

// JitterType defines the kind of jitter applied to backoff intervals.
type JitterType int

const (
	// NoJitter means the delay is exactly the computed backoff.
	NoJitter JitterType = iota
	// FullJitter randomizes the delay between 0 and the computed backoff.
	FullJitter
	// EqualJitter splits the computed backoff into two halves:
	// one half is fixed, the other is randomized.
	EqualJitter
	// DecorrelatedJitter uses a formula that produces more varied intervals.
	DecorrelatedJitter
)

// ExponentialBackoff implements an exponential backoff strategy with optional jitter.
// It is safe for concurrent use if the random source is not shared.
type ExponentialBackoff struct {
	// InitialInterval is the first delay; must be > 0.
	InitialInterval time.Duration
	// Multiplier is the factor by which the interval grows; must be >= 1.
	Multiplier float64
	// MaxInterval caps the interval; if zero, no cap.
	MaxInterval time.Duration
	// MaxElapsedTime caps the total time spent backing off; if zero, no cap.
	MaxElapsedTime time.Duration
	// Jitter specifies the kind of jitter to apply.
	Jitter JitterType
	// Random source for jitter; if nil, the global rand is used.
	Rand *rand.Rand

	// internal state
	currentInterval time.Duration
	startTime       time.Time
}

// NewExponentialBackoff creates a new backoff with sensible defaults.
func NewExponentialBackoff() *ExponentialBackoff {
	return &ExponentialBackoff{
		InitialInterval: 100 * time.Millisecond,
		Multiplier:      2.0,
		MaxInterval:     10 * time.Second,
		MaxElapsedTime:  0, // no limit
		Jitter:          FullJitter,
		Rand:            nil, // use global rand
	}
}

// Reset clears the internal state, making the backoff start from the beginning.
func (b *ExponentialBackoff) Reset() {
	b.currentInterval = 0
	b.startTime = time.Time{}
}

// NextBackoff returns the next delay to wait, based on the current state.
// It returns -1 if the max elapsed time has been exceeded.
func (b *ExponentialBackoff) NextBackoff() time.Duration {
	if b.MaxElapsedTime > 0 {
		if b.startTime.IsZero() {
			b.startTime = time.Now()
		} else if time.Since(b.startTime) > b.MaxElapsedTime {
			return -1
		}
	}

	if b.currentInterval == 0 {
		b.currentInterval = b.InitialInterval
	} else {
		// Apply multiplier
		next := float64(b.currentInterval) * b.Multiplier
		if b.MaxInterval > 0 && next > float64(b.MaxInterval) {
			next = float64(b.MaxInterval)
		}
		b.currentInterval = time.Duration(next)
	}

	// Apply jitter
	switch b.Jitter {
	case NoJitter:
		return b.currentInterval
	case FullJitter:
		return time.Duration(b.randInt63n(int64(b.currentInterval)))
	case EqualJitter:
		half := int64(b.currentInterval) / 2
		return time.Duration(half + b.randInt63n(half))
	case DecorrelatedJitter:
		// Formula: min(max, random(initial, last*3))
		last := b.currentInterval
		min := b.InitialInterval
		max := b.MaxInterval
		if max == 0 {
			max = time.Duration(math.MaxInt64)
		}
		// random between min and min(3*last, max)
		upper := 3 * last
		if upper > max {
			upper = max
		}
		if upper < min {
			upper = min
		}
		// pick random duration in [min, upper]
		delta := int64(upper - min)
		if delta < 0 {
			delta = 0
		}
		b.currentInterval = min + time.Duration(b.randInt63n(delta))
		return b.currentInterval
	default:
		return b.currentInterval
	}
}

// randInt63n returns a non-negative pseudo-random number in [0,n) using the
// backoff's random source, or the global rand if nil.
func (b *ExponentialBackoff) randInt63n(n int64) int64 {
	if n <= 0 {
		return 0
	}
	if b.Rand != nil {
		return b.Rand.Int63n(n)
	}
	return rand.Int63n(n)
}

// ------------------------------------------------------------------------
// Retry with exponential backoff
// ------------------------------------------------------------------------

// RetryWithBackoff executes the given function with exponential backoff.
// It returns nil on success, or the last error if all attempts fail.
// The function may return a special error to stop retrying immediately.
func RetryWithBackoff(backoff *ExponentialBackoff, fn func() error) error {
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