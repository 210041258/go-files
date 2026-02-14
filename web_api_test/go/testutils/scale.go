// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"math/rand"
	"sync"
	"time"
)

// ScaleDuration multiplies a duration by a floating‑point factor.
// The result is truncated to the nearest nanosecond.
func ScaleDuration(d time.Duration, factor float64) time.Duration {
	return time.Duration(float64(d) * factor)
}

// ScaleInt multiplies an integer by a floating‑point factor and returns
// the nearest integer (rounded to the nearest int, with .5 rounding up).
func ScaleInt(x int, factor float64) int {
	return int(float64(x)*factor + 0.5)
}

// JitterConfig holds parameters for adding randomness to a duration.
type JitterConfig struct {
	// Percent defines the maximum jitter as a percentage of the base duration.
	// For example, 0.1 means ±10%. Must be >= 0.
	Percent float64
	// Random source; if nil, the global rand is used.
	Rand *rand.Rand
}

// AddJitter returns a duration that is the base value plus or minus a random
// amount up to the given percentage. The resulting duration is always non‑negative.
func AddJitter(base time.Duration, cfg *JitterConfig) time.Duration {
	if cfg == nil || cfg.Percent <= 0 {
		return base
	}
	maxDelta := float64(base) * cfg.Percent
	delta := (cfg.randFloat64()*2 - 1) * maxDelta // between -maxDelta and +maxDelta
	result := float64(base) + delta
	if result < 0 {
		result = 0
	}
	return time.Duration(result)
}

// randFloat64 returns a random float64 in [0.0,1.0) using the configured source.
func (c *JitterConfig) randFloat64() float64 {
	if c.Rand != nil {
		return c.Rand.Float64()
	}
	return rand.Float64()
}

// ------------------------------------------------------------------------
// Parallel execution helpers
// ------------------------------------------------------------------------

// ParallelConfig controls the behaviour of RunParallel.
type ParallelConfig struct {
	// Workers is the number of concurrent goroutines to use.
	Workers int
	// TotalCalls is the total number of times to execute the function.
	// If TotalCalls <= 0, the function runs indefinitely (until the test stops).
	TotalCalls int
	// RateLimit, if > 0, limits the overall number of calls per second.
	// It is implemented by inserting delays between calls across all workers.
	RateLimit float64 // calls per second
}

// RunParallel executes the given function fn concurrently according to the
// provided configuration. It blocks until all calls have completed or, if
// TotalCalls <= 0, it runs forever (the caller must stop it externally).
// If Workers <= 0, it defaults to 1. If TotalCalls > 0, the function is
// called exactly that many times in total.
func RunParallel(cfg *ParallelConfig, fn func()) {
	if cfg.Workers <= 0 {
		cfg.Workers = 1
	}

	var wg sync.WaitGroup
	wg.Add(cfg.Workers)

	// Channel to distribute work if TotalCalls is limited.
	var workChan chan struct{}
	if cfg.TotalCalls > 0 {
		workChan = make(chan struct{}, cfg.TotalCalls)
		for i := 0; i < cfg.TotalCalls; i++ {
			workChan <- struct{}{}
		}
		close(workChan)
	}

	// Rate limiting: compute minimum interval between calls.
	var ticker *time.Ticker
	if cfg.RateLimit > 0 {
		interval := time.Duration(float64(time.Second) / cfg.RateLimit)
		ticker = time.NewTicker(interval)
	}

	for i := 0; i < cfg.Workers; i++ {
		go func() {
			defer wg.Done()
			if workChan != nil {
				// Finite number of calls
				for range workChan {
					if ticker != nil {
						<-ticker.C
					}
					fn()
				}
			} else {
				// Infinite calls
				for {
					if ticker != nil {
						<-ticker.C
					}
					fn()
				}
			}
		}()
	}

	wg.Wait()
	if ticker != nil {
		ticker.Stop()
	}
}