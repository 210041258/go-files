package testutils

import (
    "math"
    "time"
)

// BackoffCalculator computes exponential backoff durations.
type BackoffCalculator struct {
    Initial    time.Duration // initial backoff duration
    Max        time.Duration // maximum backoff duration
    Multiplier float64       // multiplier per attempt
}

// NewBackoffCalculator creates a new BackoffCalculator with defaults.
func NewBackoffCalculator(initial, max time.Duration, multiplier float64) *BackoffCalculator {
    if multiplier <= 0 {
        multiplier = 2.0
    }
    return &BackoffCalculator{
        Initial:    initial,
        Max:        max,
        Multiplier: multiplier,
    }
}

// Duration returns the backoff duration for a given attempt number (1-based).
func (b *BackoffCalculator) Duration(attempt int) time.Duration {
    if attempt <= 0 {
        attempt = 1
    }
    backoff := float64(b.Initial) * math.Pow(b.Multiplier, float64(attempt-1))
    if time.Duration(backoff) > b.Max {
        return b.Max
    }
    return time.Duration(backoff)
}
