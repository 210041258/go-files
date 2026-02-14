// Package testutils provides mock and simple in-memory throttlers for testing.
package testutils

import (
    "context"
    "sync"
    "time"
)

// Throttler is the interface for rate limiting operations.
type Throttler interface {
    // Allow returns true if an operation is permitted immediately.
    Allow() bool
    // Wait blocks until an operation is permitted or the context is cancelled.
    Wait(ctx context.Context) error
}

// --------------------------------------------------------------------
// MockThrottler – a test double that records all checks and can be programmed.
// --------------------------------------------------------------------

// MockThrottler implements Throttler for unit tests.
type MockThrottler struct {
    mu        sync.Mutex
    allow     bool   // programmed response for Allow()
    waitErr   error  // programmed response for Wait()
    allowCalls []string // optional key if you want to track with a label
    waitCalls  []string
}

// NewMockThrottler creates a new mock throttler with default allow=true and no wait error.
func NewMockThrottler() *MockThrottler {
    return &MockThrottler{
        allow:   true,
        waitErr: nil,
    }
}

// SetAllow programs the response for future Allow() calls.
func (m *MockThrottler) SetAllow(allow bool) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.allow = allow
}

// SetWaitError programs the error returned by Wait() (nil means success).
func (m *MockThrottler) SetWaitError(err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.waitErr = err
}

// Allow records the call and returns the programmed allow value.
func (m *MockThrottler) Allow() bool {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.allowCalls = append(m.allowCalls, "allow")
    return m.allow
}

// AllowWithKey records a call with a specific key (for more detailed tracking).
func (m *MockThrottler) AllowWithKey(key string) bool {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.allowCalls = append(m.allowCalls, key)
    return m.allow
}

// Wait records the call and returns the programmed waitErr.
func (m *MockThrottler) Wait(ctx context.Context) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.waitCalls = append(m.waitCalls, "wait")
    return m.waitErr
}

// WaitWithKey records a wait call with a specific key.
func (m *MockThrottler) WaitWithKey(ctx context.Context, key string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.waitCalls = append(m.waitCalls, key)
    return m.waitErr
}

// AllowCalls returns the list of recorded Allow call identifiers.
func (m *MockThrottler) AllowCalls() []string {
    m.mu.Lock()
    defer m.mu.Unlock()
    cp := make([]string, len(m.allowCalls))
    copy(cp, m.allowCalls)
    return cp
}

// WaitCalls returns the list of recorded Wait call identifiers.
func (m *MockThrottler) WaitCalls() []string {
    m.mu.Lock()
    defer m.mu.Unlock()
    cp := make([]string, len(m.waitCalls))
    copy(cp, m.waitCalls)
    return cp
}

// Reset clears all call history.
func (m *MockThrottler) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.allowCalls = nil
    m.waitCalls = nil
}

// --------------------------------------------------------------------
// TokenBucket – a simple rate limiter for integration tests.
// --------------------------------------------------------------------

// TokenBucket implements a token bucket rate limiter.
type TokenBucket struct {
    mu        sync.Mutex
    tokens    int
    capacity  int
    refill    int           // tokens per refill interval
    interval  time.Duration
    lastRefill time.Time
    stopChan  chan struct{}
    wg        sync.WaitGroup
}

// NewTokenBucket creates a token bucket that refills 'refill' tokens every 'interval',
// up to 'capacity'.
// If refill <= 0, no automatic refill occurs; tokens must be added manually.
func NewTokenBucket(capacity, refill int, interval time.Duration) *TokenBucket {
    tb := &TokenBucket{
        tokens:    capacity,
        capacity:  capacity,
        refill:    refill,
        interval:  interval,
        lastRefill: time.Now(),
        stopChan:  make(chan struct{}),
    }
    if refill > 0 && interval > 0 {
        tb.wg.Add(1)
        go tb.refillLoop()
    }
    return tb
}

// refillLoop periodically adds tokens.
func (tb *TokenBucket) refillLoop() {
    defer tb.wg.Done()
    ticker := time.NewTicker(tb.interval)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            tb.mu.Lock()
            tb.tokens += tb.refill
            if tb.tokens > tb.capacity {
                tb.tokens = tb.capacity
            }
            tb.lastRefill = time.Now()
            tb.mu.Unlock()
        case <-tb.stopChan:
            return
        }
    }
}

// Allow consumes one token if available, returns true if successful.
func (tb *TokenBucket) Allow() bool {
    tb.mu.Lock()
    defer tb.mu.Unlock()
    if tb.tokens > 0 {
        tb.tokens--
        return true
    }
    return false
}

// Wait blocks until a token is available or the context is cancelled.
func (tb *TokenBucket) Wait(ctx context.Context) error {
    for {
        if tb.Allow() {
            return nil
        }
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(1 * time.Millisecond): // small sleep to avoid busy loop
        }
    }
}

// AddTokens manually adds tokens (useful for tests).
func (tb *TokenBucket) AddTokens(n int) {
    tb.mu.Lock()
    defer tb.mu.Unlock()
    tb.tokens += n
    if tb.tokens > tb.capacity {
        tb.tokens = tb.capacity
    }
}

// Tokens returns the current number of available tokens.
func (tb *TokenBucket) Tokens() int {
    tb.mu.Lock()
    defer tb.mu.Unlock()
    return tb.tokens
}

// Stop stops the automatic refill goroutine.
func (tb *TokenBucket) Stop() {
    if tb.refill > 0 && tb.interval > 0 {
        close(tb.stopChan)
        tb.wg.Wait()
    }
}