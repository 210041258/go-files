// Package testutils provides mock and simple in‑memory performers for testing.
// It allows tests to define and verify operations that perform actions,
// similar to the Perform method in behavior.go but as a standalone interface.
package testutils

import (
    "sync"
    "time"
)

// --------------------------------------------------------------------
// Performer – interface for executing actions.
// --------------------------------------------------------------------

// Performer defines a method to perform a named action with optional arguments.
type Performer interface {
    // Perform executes the given action and returns a result or error.
    Perform(action string, args ...interface{}) (interface{}, error)
}

// --------------------------------------------------------------------
// MockPerformer – a test double that records calls and can be programmed.
// --------------------------------------------------------------------

// PerformCall records a single call to Perform.
type PerformCall struct {
    Action    string
    Args      []interface{}
    Timestamp time.Time
}

// MockPerformer implements Performer for unit tests.
type MockPerformer struct {
    mu           sync.Mutex
    performFunc  func(action string, args ...interface{}) (interface{}, error)
    calls        []PerformCall
    callCount    int
    returnValues map[int]struct {
        val interface{}
        err error
    }
}

// NewMockPerformer creates a new mock performer.
func NewMockPerformer() *MockPerformer {
    return &MockPerformer{
        returnValues: make(map[int]struct {
            val interface{}
            err error
        }),
    }
}

// SetPerformFunc overrides the Perform method with custom behavior.
func (m *MockPerformer) SetPerformFunc(fn func(action string, args ...interface{}) (interface{}, error)) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.performFunc = fn
}

// InjectReturnValue makes the nth call to Perform return the given value and error.
func (m *MockPerformer) InjectReturnValue(callNumber int, val interface{}, err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.returnValues[callNumber] = struct {
        val interface{}
        err error
    }{val, err}
}

// Perform records the call and returns the programmed result or calls the custom function.
func (m *MockPerformer) Perform(action string, args ...interface{}) (interface{}, error) {
    m.mu.Lock()
    m.callCount++
    call := m.callCount
    m.calls = append(m.calls, PerformCall{
        Action:    action,
        Args:      append([]interface{}{}, args...),
        Timestamp: time.Now(),
    })
    if ret, ok := m.returnValues[call]; ok {
        delete(m.returnValues, call)
        m.mu.Unlock()
        return ret.val, ret.err
    }
    if m.performFunc != nil {
        fn := m.performFunc
        m.mu.Unlock()
        return fn(action, args...)
    }
    m.mu.Unlock()
    return nil, nil
}

// Calls returns a copy of all recorded calls.
func (m *MockPerformer) Calls() []PerformCall {
    m.mu.Lock()
    defer m.mu.Unlock()
    cp := make([]PerformCall, len(m.calls))
    copy(cp, m.calls)
    return cp
}

// CallCount returns the number of times Perform was called.
func (m *MockPerformer) CallCount() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.callCount
}

// Reset clears recorded calls and injected return values.
func (m *MockPerformer) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.calls = nil
    m.callCount = 0
    m.returnValues = make(map[int]struct {
        val interface{}
        err error
    })
    m.performFunc = nil
}

// --------------------------------------------------------------------
// InMemoryPerformer – a simple performer that can be programmed per action.
// --------------------------------------------------------------------

// InMemoryPerformer implements Performer with a map of action handlers.
type InMemoryPerformer struct {
    mu      sync.RWMutex
    handlers map[string]func(args ...interface{}) (interface{}, error)
    defaultHandler func(action string, args ...interface{}) (interface{}, error)
}

// NewInMemoryPerformer creates a new empty performer.
func NewInMemoryPerformer() *InMemoryPerformer {
    return &InMemoryPerformer{
        handlers: make(map[string]func(args ...interface{}) (interface{}, error)),
    }
}

// RegisterAction registers a handler for a specific action.
func (p *InMemoryPerformer) RegisterAction(action string, handler func(args ...interface{}) (interface{}, error)) {
    p.mu.Lock()
    defer p.mu.Unlock()
    p.handlers[action] = handler
}

// SetDefaultHandler sets a function that is called when no action‑specific handler is found.
func (p *InMemoryPerformer) SetDefaultHandler(handler func(action string, args ...interface{}) (interface{}, error)) {
    p.mu.Lock()
    defer p.mu.Unlock()
    p.defaultHandler = handler
}

// Perform executes the appropriate handler for the action.
func (p *InMemoryPerformer) Perform(action string, args ...interface{}) (interface{}, error) {
    p.mu.RLock()
    handler, ok := p.handlers[action]
    defaultHandler := p.defaultHandler
    p.mu.RUnlock()

    if ok {
        return handler(args...)
    }
    if defaultHandler != nil {
        return defaultHandler(action, args...)
    }
    return nil, nil
}

// --------------------------------------------------------------------
// PerformerFunc – adapter to turn a function into a Performer.
// --------------------------------------------------------------------

// PerformerFunc is a function type that implements Performer.
type PerformerFunc func(action string, args ...interface{}) (interface{}, error)

// Perform calls the underlying function.
func (f PerformerFunc) Perform(action string, args ...interface{}) (interface{}, error) {
    return f(action, args...)
}

// --------------------------------------------------------------------
// PerformAssertions – helper functions for testing with Performer.
// --------------------------------------------------------------------

type testingT interface {
    Error(args ...interface{})
    Errorf(format string, args ...interface{})
}

// PerformAssertions provides convenience methods for verifying performer calls.
type PerformAssertions struct {
    t testingT
}

// NewPerformAssertions creates a new assertion helper.
func NewPerformAssertions(t testingT) *PerformAssertions {
    return &PerformAssertions{t: t}
}

// AssertCalled asserts that the performer was called with the given action.
func (a *PerformAssertions) AssertCalled(m *MockPerformer, action string) {
    for _, call := range m.Calls() {
        if call.Action == action {
            return
        }
    }
    a.t.Errorf("expected performer to be called with action %q, but it wasn't", action)
}

// AssertNotCalled asserts that the performer was not called with the given action.
func (a *PerformAssertions) AssertNotCalled(m *MockPerformer, action string) {
    for _, call := range m.Calls() {
        if call.Action == action {
            a.t.Errorf("expected performer not to be called with action %q, but it was", action)
            return
        }
    }
}

// AssertCallCount asserts the total number of Perform calls.
func (a *PerformAssertions) AssertCallCount(m *MockPerformer, expected int) {
    if count := m.CallCount(); count != expected {
        a.t.Errorf("expected %d Perform calls, got %d", expected, count)
    }
}