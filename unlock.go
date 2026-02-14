// Package testutils provides mock and simple in‑memory unlockers for testing.
// It allows tests to simulate unlocking operations and verify that locks are
// released correctly.
package testutils

import (
    "errors"
    "sync"
    "time"
)

// --------------------------------------------------------------------
// Unlocker – interface for releasing a lock.
// --------------------------------------------------------------------

// Unlocker defines a method to release a lock.
type Unlocker interface {
    // Unlock releases the lock. It may return an error if the lock is already
    // unlocked or if there is a problem.
    Unlock() error
}

// --------------------------------------------------------------------
// MockUnlocker – a test double that records calls and can be programmed.
// --------------------------------------------------------------------

// MockUnlocker implements Unlocker for unit tests.
type MockUnlocker struct {
    mu         sync.Mutex
    unlockFunc func() error // optional custom behavior
    calls      int
    errors     map[int]error // per‑call error (1‑based)
}

// NewMockUnlocker creates a new mock unlocker with no programmed errors.
func NewMockUnlocker() *MockUnlocker {
    return &MockUnlocker{
        errors: make(map[int]error),
    }
}

// SetUnlockFunc overrides the Unlock method with custom behavior.
func (m *MockUnlocker) SetUnlockFunc(fn func() error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.unlockFunc = fn
}

// InjectError makes the nth call to Unlock return the given error.
func (m *MockUnlocker) InjectError(callNumber int, err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.errors[callNumber] = err
}

// Unlock records the call and returns programmed error or calls custom function.
func (m *MockUnlocker) Unlock() error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.calls++
    if err, ok := m.errors[m.calls]; ok {
        delete(m.errors, m.calls)
        return err
    }
    if m.unlockFunc != nil {
        return m.unlockFunc()
    }
    return nil
}

// CallCount returns the number of times Unlock was called.
func (m *MockUnlocker) CallCount() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.calls
}

// Reset clears recorded calls and injected errors.
func (m *MockUnlocker) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.calls = 0
    m.errors = make(map[int]error)
    m.unlockFunc = nil
}

// --------------------------------------------------------------------
// InMemoryUnlocker – a simple unlocker that tracks state.
// --------------------------------------------------------------------

// InMemoryUnlocker implements Unlocker with a boolean to detect multiple unlocks.
type InMemoryUnlocker struct {
    mu      sync.Mutex
    unlocked bool
    err      error // error to return (if any)
}

// NewInMemoryUnlocker creates a new unlocker that has not been unlocked yet.
func NewInMemoryUnlocker() *InMemoryUnlocker {
    return &InMemoryUnlocker{}
}

// SetError makes Unlock return the given error (regardless of state).
func (u *InMemoryUnlocker) SetError(err error) {
    u.mu.Lock()
    defer u.mu.Unlock()
    u.err = err
}

// Unlock releases the lock. If already unlocked, it returns an error
// (unless overridden by SetError).
func (u *InMemoryUnlocker) Unlock() error {
    u.mu.Lock()
    defer u.mu.Unlock()
    if u.err != nil {
        return u.err
    }
    if u.unlocked {
        return errors.New("in‑memory unlocker: already unlocked")
    }
    u.unlocked = true
    return nil
}

// IsUnlocked returns true if Unlock has been called successfully.
func (u *InMemoryUnlocker) IsUnlocked() bool {
    u.mu.Lock()
    defer u.mu.Unlock()
    return u.unlocked
}

// Reset resets the unlocked state and clears any programmed error.
func (u *InMemoryUnlocker) Reset() {
    u.mu.Lock()
    defer u.mu.Unlock()
    u.unlocked = false
    u.err = nil
}

// --------------------------------------------------------------------
// UnlockFunc – adapter to turn a function into an Unlocker.
// --------------------------------------------------------------------

// UnlockFunc is a function type that implements Unlocker.
type UnlockFunc func() error

// Unlock calls the underlying function.
func (f UnlockFunc) Unlock() error {
    return f()
}

// --------------------------------------------------------------------
// UnlockAssertions – helper functions for testing with Unlocker.
// --------------------------------------------------------------------

type testingT interface {
    Error(args ...interface{})
    Errorf(format string, args ...interface{})
}

// UnlockAssertions provides convenience methods for verifying unlocker behavior.
type UnlockAssertions struct {
    t testingT
}

// NewUnlockAssertions creates a new assertion helper.
func NewUnlockAssertions(t testingT) *UnlockAssertions {
    return &UnlockAssertions{t: t}
}

// AssertUnlocked asserts that the mock unlocker was called at least once.
func (a *UnlockAssertions) AssertUnlocked(m *MockUnlocker) {
    if m.CallCount() == 0 {
        a.t.Error("expected Unlock to be called, but it wasn't")
    }
}

// AssertNotUnlocked asserts that the mock unlocker was not called.
func (a *UnlockAssertions) AssertNotUnlocked(m *MockUnlocker) {
    if count := m.CallCount(); count > 0 {
        a.t.Errorf("expected Unlock not to be called, but it was called %d times", count)
    }
}

// AssertUnlockCount asserts the exact number of Unlock calls.
func (a *UnlockAssertions) AssertUnlockCount(m *MockUnlocker, expected int) {
    if count := m.CallCount(); count != expected {
        a.t.Errorf("expected %d Unlock calls, got %d", expected, count)
    }
}