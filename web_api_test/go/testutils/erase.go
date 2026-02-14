// Package testutils provides mock and composite erasers for cleaning up test state.
package testutils

import (
    "sync"
)

// Eraser is the interface for wiping or resetting state.
// Useful for cleaning up between tests or resetting mocks.
type Eraser interface {
    // Erase performs the cleanup operation.
    Erase() error
}

// --------------------------------------------------------------------
// MockEraser – a test double that records calls.
// --------------------------------------------------------------------

// MockEraser implements Eraser for unit tests.
type MockEraser struct {
    mu    sync.Mutex
    calls int
    err   error // programmed error to return
}

// NewMockEraser creates a new mock eraser.
func NewMockEraser() *MockEraser {
    return &MockEraser{}
}

// SetError programs the error that Erase will return.
func (m *MockEraser) SetError(err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.err = err
}

// Erase records the call and returns the programmed error.
func (m *MockEraser) Erase() error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.calls++
    return m.err
}

// Calls returns the number of times Erase was called.
func (m *MockEraser) Calls() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.calls
}

// Reset clears the call count and error.
func (m *MockEraser) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.calls = 0
    m.err = nil
}

// --------------------------------------------------------------------
// CallbackEraser – invokes a provided function on Erase.
// --------------------------------------------------------------------

// CallbackEraser implements Eraser by calling a user‑supplied function.
type CallbackEraser struct {
    fn func() error
}

// NewCallbackEraser creates an eraser that runs the given function.
func NewCallbackEraser(fn func() error) *CallbackEraser {
    return &CallbackEraser{fn: fn}
}

// Erase calls the wrapped function.
func (c *CallbackEraser) Erase() error {
    if c.fn == nil {
        return nil
    }
    return c.fn()
}

// --------------------------------------------------------------------
// MultiEraser – composes multiple erasers into one.
// --------------------------------------------------------------------

// MultiEraser runs multiple erasers sequentially. The first error stops execution.
type MultiEraser struct {
    erasers []Eraser
}

// NewMultiEraser creates an eraser that runs all provided erasers.
func NewMultiEraser(erasers ...Eraser) *MultiEraser {
    return &MultiEraser{erasers: erasers}
}

// Erase calls Erase on each contained eraser in order.
// Returns the first error encountered, or nil if all succeed.
func (m *MultiEraser) Erase() error {
    for _, e := range m.erasers {
        if err := e.Erase(); err != nil {
            return err
        }
    }
    return nil
}

// Append adds additional erasers to the multi‑eraser.
func (m *MultiEraser) Append(erasers ...Eraser) {
    m.erasers = append(m.erasers, erasers...)
}

// --------------------------------------------------------------------
// NoOpEraser – does nothing.
// --------------------------------------------------------------------

// NoOpEraser implements Eraser with a no‑op.
type NoOpEraser struct{}

// Erase does nothing and returns nil.
func (NoOpEraser) Erase() error { return nil }