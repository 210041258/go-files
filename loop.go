// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"fmt"
	"testing"
	"time"
)

// RetryConfig holds configuration for the Retry function.
type RetryConfig struct {
	Attempts int           // maximum number of attempts (including the first)
	Delay    time.Duration // delay between attempts
	Backoff  func(int) time.Duration // optional custom backoff; if nil, fixed delay is used
}

// DefaultRetryConfig returns a sensible default retry configuration.
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		Attempts: 3,
		Delay:    100 * time.Millisecond,
		Backoff:  nil,
	}
}

// Retry executes the given function repeatedly until it succeeds (returns nil)
// or the maximum number of attempts is reached. The function is passed the
// current attempt number (starting from 1). If all attempts fail, the last
// error is returned.
func Retry(cfg *RetryConfig, fn func(attempt int) error) error {
	if cfg == nil {
		cfg = DefaultRetryConfig()
	}
	var lastErr error
	for attempt := 1; attempt <= cfg.Attempts; attempt++ {
		if err := fn(attempt); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt < cfg.Attempts {
			delay := cfg.Delay
			if cfg.Backoff != nil {
				delay = cfg.Backoff(attempt)
			}
			time.Sleep(delay)
		}
	}
	return fmt.Errorf("after %d attempts: %w", cfg.Attempts, lastErr)
}

// RetryTest is a convenience wrapper for Retry that fails the test if the
// function does not succeed within the configured attempts.
func RetryTest(t testing.TB, cfg *RetryConfig, fn func(attempt int) error) {
	t.Helper()
	if err := Retry(cfg, fn); err != nil {
		t.Fatal(err)
	}
}

// PollConfig holds configuration for the Poll function.
type PollConfig struct {
	Interval   time.Duration // how often to check the condition
	Timeout    time.Duration // maximum time to keep polling
	Immediate  bool          // if true, check condition immediately before waiting
}

// DefaultPollConfig returns a sensible default polling configuration.
func DefaultPollConfig() *PollConfig {
	return &PollConfig{
		Interval:  100 * time.Millisecond,
		Timeout:   5 * time.Second,
		Immediate: true,
	}
}

// Poll repeatedly executes the given function until it returns true or the
// timeout is reached. The function receives the current attempt number
// (starting from 1). It returns nil if the condition became true within the
// timeout, otherwise an error.
func Poll(cfg *PollConfig, fn func(attempt int) bool) error {
	if cfg == nil {
		cfg = DefaultPollConfig()
	}
	deadline := time.Now().Add(cfg.Timeout)
	attempt := 0
	if cfg.Immediate {
		attempt++
		if fn(attempt) {
			return nil
		}
	}
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("polling timed out after %v", cfg.Timeout)
			}
			attempt++
			if fn(attempt) {
				return nil
			}
		}
	}
}

// PollTest is a convenience wrapper for Poll that fails the test if the
// condition does not become true within the timeout.
func PollTest(t testing.TB, cfg *PollConfig, fn func(attempt int) bool) {
	t.Helper()
	if err := Poll(cfg, fn); err != nil {
		t.Fatal(err)
	}
}

// Until is an alias for Poll (kept for backward compatibility).
var Until = Poll

// ------------------------------------------------------------------------
// Backoff generators
// ------------------------------------------------------------------------

// ExponentialBackoff returns a backoff function that increases the delay
// exponentially with the attempt number, using the given base and optional
// max delay.
func ExponentialBackoff(base time.Duration, max time.Duration) func(int) time.Duration {
	return func(attempt int) time.Duration {
		// attempt starts from 1; we want first backoff to be base^1
		delay := base * (1 << uint(attempt-1))
		if max > 0 && delay > max {
			return max
		}
		return delay
	}
}

// LinearBackoff returns a backoff function that increases the delay linearly
// with the attempt number: delay = base + increment*(attempt-1)
func LinearBackoff(base, increment time.Duration) func(int) time.Duration {
	return func(attempt int) time.Duration {
		return base + increment*time.Duration(attempt-1)
	}
}