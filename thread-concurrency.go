// Package thread provides advanced concurrency primitives for the gateway.
package testutils

import (
    "context"
    "sync"
    "time"
)

// --------------------------------------------------------------------
// Semaphore
// --------------------------------------------------------------------

// Semaphore is a counting semaphore that limits concurrent access.
type Semaphore struct {
    tickets chan struct{}
}

// NewSemaphore creates a semaphore with the given maximum concurrency.
func NewSemaphore(max int) *Semaphore {
    return &Semaphore{
        tickets: make(chan struct{}, max),
    }
}

// Acquire blocks until a ticket is available.
func (s *Semaphore) Acquire() {
    s.tickets <- struct{}{}
}

// TryAcquire attempts to acquire a ticket without blocking.
func (s *Semaphore) TryAcquire() bool {
    select {
    case s.tickets <- struct{}{}:
        return true
    default:
        return false
    }
}

// AcquireContext acquires a ticket, respecting context cancellation.
func (s *Semaphore) AcquireContext(ctx context.Context) error {
    select {
    case s.tickets <- struct{}{}:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

// Release returns a ticket.
func (s *Semaphore) Release() {
    <-s.tickets
}

// --------------------------------------------------------------------
// KeyedMutex – synchronize per key
// --------------------------------------------------------------------

// KeyedMutex provides exclusive access per key.
type KeyedMutex struct {
    mu   sync.Mutex
    locks map[string]*sync.Mutex
    refs  map[string]int
}

// NewKeyedMutex creates a new keyed mutex.
func NewKeyedMutex() *KeyedMutex {
    return &KeyedMutex{
        locks: make(map[string]*sync.Mutex),
        refs:  make(map[string]int),
    }
}

// Lock acquires the lock for a specific key.
func (km *KeyedMutex) Lock(key string) {
    km.mu.Lock()
    if _, ok := km.locks[key]; !ok {
        km.locks[key] = &sync.Mutex{}
    }
    km.refs[key]++
    m := km.locks[key]
    km.mu.Unlock()
    m.Lock()
}

// Unlock releases the lock for a specific key.
func (km *KeyedMutex) Unlock(key string) {
    km.mu.Lock()
    defer km.mu.Unlock()
    m := km.locks[key]
    km.refs[key]--
    if km.refs[key] == 0 {
        delete(km.locks, key)
        delete(km.refs, key)
    }
    m.Unlock()
}

// --------------------------------------------------------------------
// RateLimiter – token bucket style
// --------------------------------------------------------------------

// RateLimiter implements a token bucket rate limiter.
type RateLimiter struct {
    tokens    chan struct{}
    closeOnce sync.Once
    done      chan struct{}
}

// NewRateLimiter creates a rate limiter that allows 'rate' operations per second.
func NewRateLimiter(rate int) *RateLimiter {
    rl := &RateLimiter{
        tokens: make(chan struct{}, rate),
        done:   make(chan struct{}),
    }
    // Start a goroutine to refill tokens at the given rate.
    go rl.refill(rate)
    return rl
}

func (rl *RateLimiter) refill(rate int) {
    ticker := time.NewTicker(time.Second / time.Duration(rate))
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            select {
            case rl.tokens <- struct{}{}:
            default:
                // bucket full, discard
            }
        case <-rl.done:
            return
        }
    }
}

// Allow returns true if a token is available (non‑blocking).
func (rl *RateLimiter) Allow() bool {
    select {
    case <-rl.tokens:
        return true
    default:
        return false
    }
}

// Wait blocks until a token is available.
func (rl *RateLimiter) Wait(ctx context.Context) error {
    select {
    case <-rl.tokens:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

// Stop shuts down the rate limiter's refill goroutine.
func (rl *RateLimiter) Stop() {
    rl.closeOnce.Do(func() {
        close(rl.done)
    })
}

// --------------------------------------------------------------------
// Pipeline – stages with bounded concurrency
// --------------------------------------------------------------------

// Pipeline represents a processing pipeline with multiple stages.
type Pipeline struct {
    stages []*pipelineStage
}

type pipelineStage struct {
    name        string
    concurrency int
    process     func(interface{}) (interface{}, error)
}

// NewPipeline creates an empty pipeline.
func NewPipeline() *Pipeline {
    return &Pipeline{}
}

// AddStage appends a processing stage with the given concurrency.
func (p *Pipeline) AddStage(name string, concurrency int, process func(interface{}) (interface{}, error)) {
    p.stages = append(p.stages, &pipelineStage{
        name:        name,
        concurrency: concurrency,
        process:     process,
    })
}

// Run executes the pipeline on an input channel and returns an output channel.
func (p *Pipeline) Run(ctx context.Context, input <-chan interface{}) <-chan interface{} {
    if len(p.stages) == 0 {
        // No stages, just pass through.
        return input
    }

    // Start with the input channel.
    var in <-chan interface{} = input
    for _, stage := range p.stages {
        out := p.runStage(ctx, stage, in)
        in = out
    }
    return in
}

func (p *Pipeline) runStage(ctx context.Context, stage *pipelineStage, in <-chan interface{}) <-chan interface{} {
    out := make(chan interface{}, stage.concurrency*2) // buffer
    var wg sync.WaitGroup

    for i := 0; i < stage.concurrency; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for {
                select {
                case val, ok := <-in:
                    if !ok {
                        return
                    }
                    res, err := stage.process(val)
                    if err != nil {
                        // In a real system, you'd handle errors (log, dead‑letter, etc.)
                        continue
                    }
                    select {
                    case out <- res:
                    case <-ctx.Done():
                        return
                    }
                case <-ctx.Done():
                    return
                }
            }
        }()
    }

    // Close output channel when all workers are done.
    go func() {
        wg.Wait()
        close(out)
    }()

    return out
}

// --------------------------------------------------------------------
// CircuitBreaker – simple state machine
// --------------------------------------------------------------------

// CircuitBreaker protects calls to an external service.
type CircuitBreaker struct {
    mu           sync.Mutex
    state        int // 0=closed, 1=open, 2=half‑open
    failures     int
    threshold    int
    timeout      time.Duration
    lastFailure  time.Time
}

const (
    stateClosed = iota
    stateOpen
    stateHalfOpen
)

// NewCircuitBreaker creates a circuit breaker that opens after 'threshold' failures.
func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
    return &CircuitBreaker{
        threshold: threshold,
        timeout:   timeout,
    }
}

// Execute runs the given function if the circuit is closed or half‑open.
func (cb *CircuitBreaker) Execute(fn func() error) error {
    cb.mu.Lock()
    switch cb.state {
    case stateOpen:
        if time.Since(cb.lastFailure) > cb.timeout {
            cb.state = stateHalfOpen
        } else {
            cb.mu.Unlock()
            return ErrCircuitOpen
        }
    case stateHalfOpen:
        // allow one trial
    }
    cb.mu.Unlock()

    err := fn()

    cb.mu.Lock()
    defer cb.mu.Unlock()
    if err != nil {
        cb.failures++
        cb.lastFailure = time.Now()
        if cb.failures >= cb.threshold {
            cb.state = stateOpen
        }
        return err
    }
    // success
    cb.failures = 0
    if cb.state == stateHalfOpen {
        cb.state = stateClosed
    }
    return nil
}

var ErrCircuitOpen = errors.New("circuit breaker is open")