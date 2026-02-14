// Package rate provides a simple, self‑contained token bucket rate limiter.
// It implements the core token bucket algorithm without external dependencies.
package testutils

import (
	"context"
	"sync"
	"time"
)

// ----------------------------------------------------------------------
// Rate represents a frequency, e.g., "10 per second".
// ----------------------------------------------------------------------

// Rate expresses a number of events per time interval.
type Rate struct {
	Count int
	Per   time.Duration
}

// Every converts a rate to a duration between events.
func (r Rate) Every() time.Duration {
	if r.Count <= 0 {
		return 0
	}
	return r.Per / time.Duration(r.Count)
}

// String returns a human‑readable representation, e.g., "10/1s".
func (r Rate) String() string {
	return r.String()
}

// ----------------------------------------------------------------------
// TokenBucket is a thread‑safe token bucket limiter.
// ----------------------------------------------------------------------

// TokenBucket implements a token bucket rate limiter.
// Tokens are added at a fixed rate; each request consumes one token.
type TokenBucket struct {
	mu          sync.Mutex
	rate        Rate
	tokens      float64
	capacity    float64
	lastRefill  time.Time
}

// NewTokenBucket creates a token bucket that allows up to `rate.Count`
// events per `rate.Per` time period, with a maximum burst size of `burst`.
// If burst <= 0, it defaults to the rate count (common setting).
func NewTokenBucket(rate Rate, burst int) *TokenBucket {
	if burst <= 0 {
		burst = rate.Count
	}
	return &TokenBucket{
		rate:       rate,
		tokens:     float64(burst),
		capacity:   float64(burst),
		lastRefill: time.Now(),
	}
}

// refill adds tokens based on the elapsed time.
func (tb *TokenBucket) refill(now time.Time) {
	if now.Before(tb.lastRefill) {
		return // time went backwards; ignore
	}
	elapsed := now.Sub(tb.lastRefill)
	// Calculate tokens to add.
	add := float64(elapsed) / float64(tb.rate.Per) * float64(tb.rate.Count)
	tb.tokens += add
	if tb.tokens > tb.capacity {
		tb.tokens = tb.capacity
	}
	tb.lastRefill = now
}

// Allow reports whether one token is available at this moment.
func (tb *TokenBucket) Allow() bool {
	return tb.AllowN(1)
}

// AllowN reports whether n tokens are available at this moment.
func (tb *TokenBucket) AllowN(n int) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	now := time.Now()
	tb.refill(now)
	if tb.tokens >= float64(n) {
		tb.tokens -= float64(n)
		return true
	}
	return false
}

// Wait blocks until one token is available or the context is cancelled.
func (tb *TokenBucket) Wait(ctx context.Context) error {
	return tb.WaitN(ctx, 1)
}

// WaitN blocks until n tokens are available or the context is cancelled.
func (tb *TokenBucket) WaitN(ctx context.Context, n int) error {
	for {
		tb.mu.Lock()
		now := time.Now()
		tb.refill(now)
		if tb.tokens >= float64(n) {
			tb.tokens -= float64(n)
			tb.mu.Unlock()
			return nil
		}
		// How long until we have enough tokens?
		needed := float64(n) - tb.tokens
		// tokens added per second = rate.Count / rate.Per
		perSecond := float64(tb.rate.Count) / float64(tb.rate.Per)
		waitDuration := time.Duration(needed / perSecond * float64(time.Second))
		if waitDuration < time.Millisecond {
			waitDuration = time.Millisecond // avoid busy loop
		}
		tb.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitDuration):
			// continue loop
		}
	}
}

// SetRate dynamically adjusts the rate of the limiter.
func (tb *TokenBucket) SetRate(rate Rate) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.rate = rate
	// keep current token count and capacity unchanged
}

// ----------------------------------------------------------------------
// SimplePeriodicLimiter: a fixed‑window limiter using time.Ticker.
// ----------------------------------------------------------------------

// PeriodicLimiter enforces a limit by allowing up to max events per interval,
// resetting at fixed intervals. It is simpler than token bucket but may
// allow bursts at the window boundaries.
type PeriodicLimiter struct {
	ticker *time.Ticker
	ch     chan struct{}
	mu     sync.Mutex
	closed bool
}

// NewPeriodicLimiter creates a limiter that allows `max` events every `interval`.
// It uses a buffered channel to hold tokens.
func NewPeriodicLimiter(max int, interval time.Duration) *PeriodicLimiter {
	pl := &PeriodicLimiter{
		ch: make(chan struct{}, max),
	}
	// Fill channel initially.
	for i := 0; i < max; i++ {
		pl.ch <- struct{}{}
	}
	pl.ticker = time.NewTicker(interval)
	go pl.refillLoop()
	return pl
}

func (pl *PeriodicLimiter) refillLoop() {
	for range pl.ticker.C {
		pl.mu.Lock()
		if pl.closed {
			pl.mu.Unlock()
			return
		}
		// Refill to capacity.
		for len(pl.ch) < cap(pl.ch) {
			pl.ch <- struct{}{}
		}
		pl.mu.Unlock()
	}
}

// Allow checks if an event is allowed, consuming a token if so.
func (pl *PeriodicLimiter) Allow() bool {
	select {
	case <-pl.ch:
		return true
	default:
		return false
	}
}

// Wait blocks until a token is available or the context is done.
func (pl *PeriodicLimiter) Wait(ctx context.Context) error {
	select {
	case <-pl.ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop stops the refill goroutine and closes the limiter.
func (pl *PeriodicLimiter) Stop() {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	if !pl.closed {
		pl.closed = true
		pl.ticker.Stop()
		close(pl.ch)
	}
}

// ----------------------------------------------------------------------
// Utility: often used rates
// ----------------------------------------------------------------------

var (
	// PerSecond creates a rate of n per second.
	PerSecond = func(n int) Rate { return Rate{Count: n, Per: time.Second} }
	// PerMinute creates a rate of n per minute.
	PerMinute = func(n int) Rate { return Rate{Count: n, Per: time.Minute} }
	// PerHour creates a rate of n per hour.
	PerHour   = func(n int) Rate { return Rate{Count: n, Per: time.Hour} }
)

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     // Token bucket: 10 requests per second, burst 5.
//     limiter := rate.NewTokenBucket(rate.PerSecond(10), 5)
//
//     for i := 0; i < 20; i++ {
//         if limiter.Allow() {
//             fmt.Println("allowed", i)
//         } else {
//             fmt.Println("denied", i)
//         }
//         time.Sleep(50 * time.Millisecond)
//     }
//
//     // Periodic limiter: 5 requests every 10 seconds.
//     plim := rate.NewPeriodicLimiter(5, 10*time.Second)
//     defer plim.Stop()
//     ctx := context.Background()
//     if err := plim.Wait(ctx); err == nil {
//         // handle request
//     }
// }