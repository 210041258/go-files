// Package quit provides coordinated shutdown for goroutines.
package testutils

import (
    "context"
    "sync"
)

// C is a shutdown coordinator. It combines a quit channel with a wait group.
type C struct {
    quit chan struct{}
    once sync.Once
    wg   sync.WaitGroup
}

// New creates a new quit coordinator.
func New() *C {
    return &C{
        quit: make(chan struct{}),
    }
}

// Quit returns a channel that is closed when shutdown is initiated.
func (q *C) Quit() <-chan struct{} {
    return q.quit
}

// Shutdown signals all listeners to stop. It can be called multiple times safely.
func (q *C) Shutdown() {
    q.once.Do(func() {
        close(q.quit)
    })
}

// Add adds delta to the wait group. Use it before starting a goroutine.
func (q *C) Add(delta int) {
    q.wg.Add(delta)
}

// Done decrements the wait group. Call it when a goroutine finishes.
func (q *C) Done() {
    q.wg.Done()
}

// Wait blocks until all goroutines have called Done after Shutdown.
func (q *C) Wait() {
    q.wg.Wait()
}

// Go starts a function in a goroutine, automatically handling Add/Done.
func (q *C) Go(fn func()) {
    q.Add(1)
    go func() {
        defer q.Done()
        fn()
    }()
}

// GoWithQuit starts a function that receives the quit channel, handling Add/Done.
func (q *C) GoWithQuit(fn func(<-chan struct{})) {
    q.Add(1)
    go func() {
        defer q.Done()
        fn(q.quit)
    }()
}

// Context returns a context that is canceled when shutdown is initiated.
func (q *C) Context() context.Context {
    ctx, cancel := context.WithCancel(context.Background())
    q.GoWithQuit(func(<-chan struct{}) {
        <-q.quit
        cancel()
    })
    return ctx
}