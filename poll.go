// Package poll provides utilities for polling conditions with backoff.
// It is useful for waiting for resources to become ready, retrying operations,
// or implementing simple polling loops with timeout and cancellation.
package testutils

import (
	"context"
	"errors"
	"time"
)

// ----------------------------------------------------------------------
// Core polling function
// ----------------------------------------------------------------------

// Condition is a function that reports whether a desired state has been reached.
// It may also return an error to indicate a permanent failure.
type Condition func() (bool, error)

// Wait polls the condition repeatedly until it returns true, the context is
// done, or a permanent error occurs. It uses a fixed delay between attempts.
// If the condition returns false and no error, it will be called again after
// the delay.
func Wait(ctx context.Context, delay time.Duration, cond Condition) error {
	if delay <= 0 {
		delay = 100 * time.Millisecond
	}
	ticker := time.NewTicker(delay)
	defer ticker.Stop()

	for {
		ok, err := cond()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// ----------------------------------------------------------------------
// Exponential backoff polling
// ----------------------------------------------------------------------

// BackoffConfig defines the parameters for exponential backoff polling.
type BackoffConfig struct {
	// Initial delay before the first retry.
	Initial time.Duration
	// Maximum delay between retries.
	Max time.Duration
	// Multiplier for exponential backoff (e.g., 2.0).
	Multiplier float64
	// Jitter adds randomness to the delay (0 to 1). If jitter > 0, the actual
	// delay will be randomly chosen between delay and delay*(1+jitter).
	Jitter float64
}

// DefaultBackoffConfig returns a sensible default configuration.
func DefaultBackoffConfig() BackoffConfig {
	return BackoffConfig{
		Initial:    100 * time.Millisecond,
		Max:        10 * time.Second,
		Multiplier: 2.0,
		Jitter:     0.1,
	}
}

// WaitWithBackoff polls the condition with exponential backoff.
// The delay between attempts starts at cfg.Initial and increases by
// cfg.Multiplier each time, up to cfg.Max. Jitter may be applied.
func WaitWithBackoff(ctx context.Context, cfg BackoffConfig, cond Condition) error {
	delay := cfg.Initial
	for {
		ok, err := cond()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		// Wait for the delay or context cancellation.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(jitteredDelay(delay, cfg.Jitter)):
		}
		// Increase delay for next iteration.
		delay = time.Duration(float64(delay) * cfg.Multiplier)
		if delay > cfg.Max {
			delay = cfg.Max
		}
	}
}

// jitteredDelay applies random jitter to the base delay if jitter > 0.
func jitteredDelay(delay time.Duration, jitter float64) time.Duration {
	if jitter <= 0 {
		return delay
	}
	// Add up to jitter*delay extra.
	maxExtra := float64(delay) * jitter
	extra := float64(time.Now().UnixNano()) / 1e9 // crude random seed
	extra = extra - float64(int(extra))           // fractional part
	return delay + time.Duration(extra*maxExtra)
}

// ----------------------------------------------------------------------
// Retry helpers
// ----------------------------------------------------------------------

// Retry runs the given function repeatedly until it returns nil, the context
// is cancelled, or a permanent error is returned (wrapped with ErrPermanent).
// It uses exponential backoff between attempts.
func Retry(ctx context.Context, cfg BackoffConfig, fn func() error) error {
	var lastErr error
	delay := cfg.Initial
	for {
		err := fn()
		if err == nil {
			return nil
		}
		if errors.Is(err, ErrPermanent) {
			return err
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(jitteredDelay(delay, cfg.Jitter)):
		}
		delay = time.Duration(float64(delay) * cfg.Multiplier)
		if delay > cfg.Max {
			delay = cfg.Max
		}
	}
}

// ErrPermanent is a sentinel error that can be wrapped to indicate that
// a retry should stop immediately. Use fmt.Errorf("...: %w", ErrPermanent)
// to wrap an existing error.
var ErrPermanent = errors.New("permanent failure")

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//     defer cancel()
//
//     // Poll for a file to exist.
//     err := poll.Wait(ctx, 500*time.Millisecond, func() (bool, error) {
//         _, err := os.Stat("/tmp/ready")
//         if err == nil {
//             return true, nil
//         }
//         if os.IsNotExist(err) {
//             return false, nil
//         }
//         return false, err // permanent error
//     })
//
//     // Retry a network operation with backoff.
//     err = poll.Retry(ctx, poll.DefaultBackoffConfig(), func() error {
//         resp, err := http.Get("https://example.com")
//         if err != nil {
//             return err
//         }
//         defer resp.Body.Close()
//         if resp.StatusCode >= 500 {
//             return fmt.Errorf("server error: %s", resp.Status)
//         }
//         if resp.StatusCode == 404 {
//             return fmt.Errorf("not found: %w", poll.ErrPermanent)
//         }
//         return nil
//     })
// }