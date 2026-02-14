// Package testutils provides load generation utilities for testing.
package testutils

import (
    "context"
    "sync"
    "sync/atomic"
    "time"
)

// LoadGenerator is the interface for generating load in tests.
type LoadGenerator interface {
    // Start begins generating load. It may be non‑blocking.
    Start(ctx context.Context) error
    // Stop gracefully stops load generation.
    Stop() error
    // Count returns the total number of messages generated so far.
    Count() uint64
}

// LoadProfile defines how load should be generated.
type LoadProfile struct {
    // Rate is the number of messages per second. If 0, generate as fast as possible.
    Rate int
    // Duration limits how long to generate. Zero means run until stopped.
    Duration time.Duration
    // Payload is a function that returns the next message to send.
    // If nil, a default payload (e.g., current timestamp) will be used.
    Payload func() interface{}
    // Receiver is called for each generated message. Must be set.
    Receiver func(ctx context.Context, msg interface{})
}

// --------------------------------------------------------------------
// MockLoadGenerator – a test double that records control calls.
// --------------------------------------------------------------------

// MockLoadGenerator implements LoadGenerator for unit tests.
type MockLoadGenerator struct {
    mu        sync.Mutex
    started   bool
    stopped   bool
    startErr  error
    stopErr   error
    count     uint64
    startCalls int
    stopCalls  int
}

// NewMockLoadGenerator creates a new mock load generator.
func NewMockLoadGenerator() *MockLoadGenerator {
    return &MockLoadGenerator{}
}

// SetStartError programs the error to be returned by Start.
func (m *MockLoadGenerator) SetStartError(err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.startErr = err
}

// SetStopError programs the error to be returned by Stop.
func (m *MockLoadGenerator) SetStopError(err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.stopErr = err
}

// SetCount sets the count returned by Count().
func (m *MockLoadGenerator) SetCount(n uint64) {
    atomic.StoreUint64(&m.count, n)
}

// Start records the call and returns the programmed error.
func (m *MockLoadGenerator) Start(ctx context.Context) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.startCalls++
    m.started = true
    return m.startErr
}

// Stop records the call and returns the programmed error.
func (m *MockLoadGenerator) Stop() error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.stopCalls++
    m.stopped = true
    return m.stopErr
}

// Count returns the mock count.
func (m *MockLoadGenerator) Count() uint64 {
    return atomic.LoadUint64(&m.count)
}

// StartCalls returns how many times Start was called.
func (m *MockLoadGenerator) StartCalls() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.startCalls
}

// StopCalls returns how many times Stop was called.
func (m *MockLoadGenerator) StopCalls() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.stopCalls
}

// Reset clears all state and counters.
func (m *MockLoadGenerator) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.started = false
    m.stopped = false
    m.startErr = nil
    m.stopErr = nil
    m.startCalls = 0
    m.stopCalls = 0
    atomic.StoreUint64(&m.count, 0)
}

// --------------------------------------------------------------------
// SimpleLoadGenerator – a real load generator for integration tests.
// --------------------------------------------------------------------

// SimpleLoadGenerator implements LoadGenerator using a ticker.
type SimpleLoadGenerator struct {
    profile LoadProfile
    cancel  context.CancelFunc
    wg      sync.WaitGroup
    count   uint64
}

// NewSimpleLoadGenerator creates a new load generator with the given profile.
// The profile.Receiver must be set.
func NewSimpleLoadGenerator(profile LoadProfile) *SimpleLoadGenerator {
    return &SimpleLoadGenerator{
        profile: profile,
    }
}

// Start begins generating load in a background goroutine.
// If profile.Duration > 0, generation stops automatically after that duration.
// Otherwise, run until Stop is called or the context is cancelled.
func (g *SimpleLoadGenerator) Start(ctx context.Context) error {
    if g.profile.Receiver == nil {
        panic("LoadProfile.Receiver must be set")
    }
    ctx, cancel := context.WithCancel(ctx)
    g.cancel = cancel

    g.wg.Add(1)
    go func() {
        defer g.wg.Done()
        g.run(ctx)
    }()
    return nil
}

// Stop stops load generation and waits for the generator goroutine to exit.
func (g *SimpleLoadGenerator) Stop() error {
    if g.cancel != nil {
        g.cancel()
    }
    g.wg.Wait()
    return nil
}

// Count returns the number of messages generated so far.
func (g *SimpleLoadGenerator) Count() uint64 {
    return atomic.LoadUint64(&g.count)
}

// run is the main generation loop.
func (g *SimpleLoadGenerator) run(ctx context.Context) {
    var ticker *time.Ticker
    if g.profile.Rate > 0 {
        interval := time.Second / time.Duration(g.profile.Rate)
        ticker = time.NewTicker(interval)
        defer ticker.Stop()
    }

    // Optional duration timer
    var timer <-chan time.Time
    if g.profile.Duration > 0 {
        timer = time.After(g.profile.Duration)
    }

    for {
        select {
        case <-ctx.Done():
            return
        case <-timer:
            // Duration expired – stop automatically.
            return
        default:
            // If rate limited, wait for ticker.
            if ticker != nil {
                select {
                case <-ticker.C:
                case <-ctx.Done():
                    return
                }
            }
            // Generate a message.
            msg := g.nextPayload()
            atomic.AddUint64(&g.count, 1)
            g.profile.Receiver(ctx, msg)
            // If no rate limit, we loop immediately (no waiting).
            if ticker == nil {
                // Small sleep to avoid CPU spin; adjust as needed.
                time.Sleep(1 * time.Microsecond)
            }
        }
    }
}

// nextPayload returns the next message payload.
func (g *SimpleLoadGenerator) nextPayload() interface{} {
    if g.profile.Payload != nil {
        return g.profile.Payload()
    }
    // Default payload: a simple string with a counter.
    // In practice, you'd want something more meaningful.
    return "load-message"
}