// Package testutils provides mock and simple bandwidth limiting and monitoring utilities for testing.
package testutils

import (
    "context"
    "errors"
    "sync"
    "time"
)

// BandwidthLimiter is the interface for controlling bandwidth usage.
type BandwidthLimiter interface {
    // WaitN blocks until n bytes can be transmitted, respecting the context.
    WaitN(ctx context.Context, n int) error
}

// --------------------------------------------------------------------
// MockBandwidthLimiter – a test double that records calls and can be programmed.
// --------------------------------------------------------------------

// MockBandwidthLimiter implements BandwidthLimiter for unit tests.
type MockBandwidthLimiter struct {
    mu       sync.Mutex
    waitN    func(ctx context.Context, n int) error // optional custom behavior
    calls    []struct{ N int }                       // recorded WaitN calls
    waitErrs map[int]error                            // per‑call error injection
    callCount int
}

// NewMockBandwidthLimiter creates a new mock limiter.
func NewMockBandwidthLimiter() *MockBandwidthLimiter {
    return &MockBandwidthLimiter{
        waitErrs: make(map[int]error),
    }
}

// SetWaitN overrides the WaitN method with custom behavior.
func (m *MockBandwidthLimiter) SetWaitN(fn func(ctx context.Context, n int) error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.waitN = fn
}

// InjectWaitError makes the nth call to WaitN (1‑based) return the given error.
func (m *MockBandwidthLimiter) InjectWaitError(callNumber int, err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.waitErrs[callNumber] = err
}

// WaitN records the call and delegates to custom function or returns stored error.
func (m *MockBandwidthLimiter) WaitN(ctx context.Context, n int) error {
    m.mu.Lock()
    m.callCount++
    call := m.callCount
    m.calls = append(m.calls, struct{ N int }{n})
    if err, ok := m.waitErrs[call]; ok {
        delete(m.waitErrs, call)
        m.mu.Unlock()
        return err
    }
    if m.waitN != nil {
        fn := m.waitN
        m.mu.Unlock()
        return fn(ctx, n)
    }
    m.mu.Unlock()
    return nil
}

// Calls returns a copy of the recorded WaitN arguments.
func (m *MockBandwidthLimiter) Calls() []int {
    m.mu.Lock()
    defer m.mu.Unlock()
    cp := make([]int, len(m.calls))
    for i, call := range m.calls {
        cp[i] = call.N
    }
    return cp
}

// Reset clears recorded calls and injected errors.
func (m *MockBandwidthLimiter) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.calls = nil
    m.waitErrs = make(map[int]error)
    m.callCount = 0
    m.waitN = nil
}

// --------------------------------------------------------------------
// TokenBucketLimiter – a simple token bucket for bandwidth (bytes/second).
// --------------------------------------------------------------------

// TokenBucketLimiter implements a token bucket rate limiter for bandwidth.
type TokenBucketLimiter struct {
    mu        sync.Mutex
    tokens    int
    capacity  int
    rate      int           // bytes per second
    lastRefill time.Time
    stopChan  chan struct{}
    wg        sync.WaitGroup
}

// NewTokenBucketLimiter creates a limiter that allows up to `capacity` bytes burst,
// refilling at `rate` bytes per second.
// If rate <= 0, no limiting is applied (WaitN returns immediately).
func NewTokenBucketLimiter(rate, capacity int) *TokenBucketLimiter {
    if rate <= 0 {
        rate = 0 // unlimited
    }
    tb := &TokenBucketLimiter{
        tokens:    capacity,
        capacity:  capacity,
        rate:      rate,
        lastRefill: time.Now(),
        stopChan:  make(chan struct{}),
    }
    if rate > 0 {
        tb.wg.Add(1)
        go tb.refillLoop()
    }
    return tb
}

// refillLoop periodically adds tokens.
func (tb *TokenBucketLimiter) refillLoop() {
    defer tb.wg.Done()
    ticker := time.NewTicker(100 * time.Millisecond) // small granularity
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            tb.mu.Lock()
            now := time.Now()
            elapsed := now.Sub(tb.lastRefill)
            tb.lastRefill = now
            // Add tokens based on elapsed time and rate
            add := int(elapsed.Seconds() * float64(tb.rate))
            if add > 0 {
                tb.tokens += add
                if tb.tokens > tb.capacity {
                    tb.tokens = tb.capacity
                }
            }
            tb.mu.Unlock()
        case <-tb.stopChan:
            return
        }
    }
}

// WaitN blocks until n tokens are available or context is cancelled.
func (tb *TokenBucketLimiter) WaitN(ctx context.Context, n int) error {
    if tb.rate <= 0 {
        // unlimited
        return nil
    }
    if n <= 0 {
        return nil
    }

    for {
        // Try to consume tokens
        tb.mu.Lock()
        if tb.tokens >= n {
            tb.tokens -= n
            tb.mu.Unlock()
            return nil
        }
        // Not enough tokens; calculate wait time
        deficit := n - tb.tokens
        waitTime := time.Duration(deficit) * time.Second / time.Duration(tb.rate)
        tb.mu.Unlock()

        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(waitTime):
            // loop and try again
        }
    }
}

// Stop stops the refill goroutine.
func (tb *TokenBucketLimiter) Stop() {
    if tb.rate > 0 {
        close(tb.stopChan)
        tb.wg.Wait()
    }
}

// --------------------------------------------------------------------
// SlidingWindowLimiter – a more precise bandwidth limiter using a sliding window.
// --------------------------------------------------------------------

// SlidingWindowLimiter implements a rate limiter using a sliding window of time.
type SlidingWindowLimiter struct {
    mu       sync.Mutex
    rate     int           // bytes per second
    window   time.Duration // e.g., 1 second
    buckets  map[int64]int // timestamp truncated to window granularity -> bytes
}

// NewSlidingWindowLimiter creates a limiter that allows `rate` bytes per second,
// using a sliding window of `window` duration (e.g., 1s). Smaller windows give
// more accurate limiting but more memory.
func NewSlidingWindowLimiter(rate int, window time.Duration) *SlidingWindowLimiter {
    if window <= 0 {
        window = time.Second
    }
    return &SlidingWindowLimiter{
        rate:    rate,
        window:  window,
        buckets: make(map[int64]int),
    }
}

// WaitN blocks until n bytes can be sent within the rate limit.
func (l *SlidingWindowLimiter) WaitN(ctx context.Context, n int) error {
    if l.rate <= 0 {
        return nil
    }
    for {
        wait, err := l.tryConsume(n)
        if err != nil {
            return err
        }
        if wait == 0 {
            return nil
        }
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(wait):
            // continue
        }
    }
}

// tryConsume attempts to consume n bytes; returns wait duration if over limit.
func (l *SlidingWindowLimiter) tryConsume(n int) (time.Duration, error) {
    l.mu.Lock()
    defer l.mu.Unlock()
    now := time.Now()
    // Truncate to window granularity
    currentSlot := now.UnixNano() / int64(l.window)
    // Clean up old slots (older than 2 windows)
    for slot := range l.buckets {
        if slot < currentSlot-1 { // keep previous slot for sliding calculation
            delete(l.buckets, slot)
        }
    }
    // Calculate total bytes in current window (current + previous slot weighted)
    var total int
    prevSlot := currentSlot - 1
    if val, ok := l.buckets[prevSlot]; ok {
        // Weight by how much of previous slot is still in window
        prevSlotStart := time.Unix(0, prevSlot*int64(l.window))
        overlap := l.window - now.Sub(prevSlotStart.Add(l.window))
        if overlap > 0 {
            weight := float64(overlap) / float64(l.window)
            total += int(float64(val) * weight)
        }
    }
    total += l.buckets[currentSlot]

    if total+n > l.rate {
        // Need to wait until next slot or later
        // Calculate time until current slot ends
        slotEnd := time.Unix(0, (currentSlot+1)*int64(l.window))
        wait := slotEnd.Sub(now)
        return wait, nil
    }
    // Consume
    l.buckets[currentSlot] += n
    return 0, nil
}

// --------------------------------------------------------------------
// BandwidthMonitor – records bandwidth usage over time.
// --------------------------------------------------------------------

// BandwidthMonitor tracks bytes transferred and can report current rate.
type BandwidthMonitor struct {
    mu        sync.Mutex
    total     int64
    startTime time.Time
    samples   []sample
}

type sample struct {
    time  time.Time
    bytes int64
}

// NewBandwidthMonitor creates a new monitor.
func NewBandwidthMonitor() *BandwidthMonitor {
    return &BandwidthMonitor{
        startTime: time.Now(),
    }
}

// Add records that n bytes were transferred.
func (m *BandwidthMonitor) Add(n int) {
    m.mu.Lock()
    defer m.mu.Unlock()
    now := time.Now()
    m.total += int64(n)
    m.samples = append(m.samples, sample{time: now, bytes: int64(n)})
}

// Total returns total bytes transferred.
func (m *BandwidthMonitor) Total() int64 {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.total
}

// Rate returns the average bytes per second over the entire lifetime.
func (m *BandwidthMonitor) Rate() float64 {
    m.mu.Lock()
    defer m.mu.Unlock()
    elapsed := time.Since(m.startTime).Seconds()
    if elapsed == 0 {
        return 0
    }
    return float64(m.total) / elapsed
}

// RateLast returns the average bytes per second over the last d duration.
func (m *BandwidthMonitor) RateLast(d time.Duration) float64 {
    m.mu.Lock()
    defer m.mu.Unlock()
    cutoff := time.Now().Add(-d)
    var total int64
    var found bool
    for _, s := range m.samples {
        if s.time.After(cutoff) {
            total += s.bytes
            found = true
        }
    }
    if !found {
        return 0
    }
    return float64(total) / d.Seconds()
}

// Reset clears all recorded data.
func (m *BandwidthMonitor) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.total = 0
    m.startTime = time.Now()
    m.samples = nil
}