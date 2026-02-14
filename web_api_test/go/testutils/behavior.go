// Package testutils provides mock and simple in‑memory behavior simulators
// for testing storage, memory, network, and database components. It allows
// tests to define expected behaviors and verify interactions.
package testutils

import (
    "sync"
    "time"
)

// --------------------------------------------------------------------
// Behavior – interface for a component's lifecycle and actions.
// --------------------------------------------------------------------

// Behavior defines the methods that a testable component should implement.
// This is a generic interface that can be adapted to any resource.
type Behavior interface {
    // Start initializes the component.
    Start() error
    // Stop shuts down the component.
    Stop() error
    // Status returns the current state of the component.
    Status() (string, error)
    // Perform executes a named action with optional arguments.
    Perform(action string, args ...interface{}) (interface{}, error)
}

// --------------------------------------------------------------------
// MockBehavior – a test double that records calls and can be programmed.
// --------------------------------------------------------------------

// BehaviorCall records a single call to a behavior method.
type BehaviorCall struct {
    Method    string
    Action    string // for Perform
    Args      []interface{}
    Timestamp time.Time
}

// MockBehavior implements Behavior for unit tests.
type MockBehavior struct {
    mu            sync.Mutex
    startFunc     func() error
    stopFunc      func() error
    statusFunc    func() (string, error)
    performFunc   func(action string, args ...interface{}) (interface{}, error)
    calls         []BehaviorCall
    startCalls    int
    stopCalls     int
    statusCalls   int
    performCalls  int
    startErrors   map[int]error
    stopErrors    map[int]error
    statusErrors  map[int]error
    statusValues  map[int]string
    performResults map[int]struct {
        val interface{}
        err error
    }
}

// NewMockBehavior creates a new mock behavior.
func NewMockBehavior() *MockBehavior {
    return &MockBehavior{
        startErrors:    make(map[int]error),
        stopErrors:     make(map[int]error),
        statusErrors:   make(map[int]error),
        statusValues:   make(map[int]string),
        performResults: make(map[int]struct {
            val interface{}
            err error
        }),
    }
}

// SetStartFunc overrides the Start method.
func (m *MockBehavior) SetStartFunc(fn func() error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.startFunc = fn
}

// SetStopFunc overrides the Stop method.
func (m *MockBehavior) SetStopFunc(fn func() error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.stopFunc = fn
}

// SetStatusFunc overrides the Status method.
func (m *MockBehavior) SetStatusFunc(fn func() (string, error)) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.statusFunc = fn
}

// SetPerformFunc overrides the Perform method.
func (m *MockBehavior) SetPerformFunc(fn func(action string, args ...interface{}) (interface{}, error)) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.performFunc = fn
}

// InjectStartError makes the nth call to Start return the given error.
func (m *MockBehavior) InjectStartError(callNumber int, err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.startErrors[callNumber] = err
}

// InjectStopError makes the nth call to Stop return the given error.
func (m *MockBehavior) InjectStopError(callNumber int, err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.stopErrors[callNumber] = err
}

// InjectStatusError makes the nth call to Status return the given error.
func (m *MockBehavior) InjectStatusError(callNumber int, err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.statusErrors[callNumber] = err
}

// InjectStatusValue makes the nth call to Status return the given string and nil error.
func (m *MockBehavior) InjectStatusValue(callNumber int, status string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.statusValues[callNumber] = status
}

// InjectPerformResult makes the nth call to Perform return the given value and error.
func (m *MockBehavior) InjectPerformResult(callNumber int, val interface{}, err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.performResults[callNumber] = struct {
        val interface{}
        err error
    }{val, err}
}

// Start records the call and returns programmed error or calls custom function.
func (m *MockBehavior) Start() error {
    m.mu.Lock()
    m.startCalls++
    call := m.startCalls
    m.calls = append(m.calls, BehaviorCall{Method: "Start", Timestamp: time.Now()})
    if err, ok := m.startErrors[call]; ok {
        delete(m.startErrors, call)
        m.mu.Unlock()
        return err
    }
    if m.startFunc != nil {
        fn := m.startFunc
        m.mu.Unlock()
        return fn()
    }
    m.mu.Unlock()
    return nil
}

// Stop records the call and returns programmed error or calls custom function.
func (m *MockBehavior) Stop() error {
    m.mu.Lock()
    m.stopCalls++
    call := m.stopCalls
    m.calls = append(m.calls, BehaviorCall{Method: "Stop", Timestamp: time.Now()})
    if err, ok := m.stopErrors[call]; ok {
        delete(m.stopErrors, call)
        m.mu.Unlock()
        return err
    }
    if m.stopFunc != nil {
        fn := m.stopFunc
        m.mu.Unlock()
        return fn()
    }
    m.mu.Unlock()
    return nil
}

// Status records the call and returns programmed status/error or calls custom function.
func (m *MockBehavior) Status() (string, error) {
    m.mu.Lock()
    m.statusCalls++
    call := m.statusCalls
    m.calls = append(m.calls, BehaviorCall{Method: "Status", Timestamp: time.Now()})
    if err, ok := m.statusErrors[call]; ok {
        delete(m.statusErrors, call)
        m.mu.Unlock()
        return "", err
    }
    if status, ok := m.statusValues[call]; ok {
        delete(m.statusValues, call)
        m.mu.Unlock()
        return status, nil
    }
    if m.statusFunc != nil {
        fn := m.statusFunc
        m.mu.Unlock()
        return fn()
    }
    m.mu.Unlock()
    return "unknown", nil
}

// Perform records the call and returns programmed result or calls custom function.
func (m *MockBehavior) Perform(action string, args ...interface{}) (interface{}, error) {
    m.mu.Lock()
    m.performCalls++
    call := m.performCalls
    m.calls = append(m.calls, BehaviorCall{
        Method:    "Perform",
        Action:    action,
        Args:      append([]interface{}{}, args...),
        Timestamp: time.Now(),
    })
    if res, ok := m.performResults[call]; ok {
        delete(m.performResults, call)
        m.mu.Unlock()
        return res.val, res.err
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
func (m *MockBehavior) Calls() []BehaviorCall {
    m.mu.Lock()
    defer m.mu.Unlock()
    cp := make([]BehaviorCall, len(m.calls))
    copy(cp, m.calls)
    return cp
}

// CallCounts returns the number of calls to each method.
func (m *MockBehavior) CallCounts() (start, stop, status, perform int) {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.startCalls, m.stopCalls, m.statusCalls, m.performCalls
}

// Reset clears recorded calls and injected errors.
func (m *MockBehavior) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.calls = nil
    m.startCalls = 0
    m.stopCalls = 0
    m.statusCalls = 0
    m.performCalls = 0
    m.startErrors = make(map[int]error)
    m.stopErrors = make(map[int]error)
    m.statusErrors = make(map[int]error)
    m.statusValues = make(map[int]string)
    m.performResults = make(map[int]struct {
        val interface{}
        err error
    })
    m.startFunc = nil
    m.stopFunc = nil
    m.statusFunc = nil
    m.performFunc = nil
}

// --------------------------------------------------------------------
// InMemoryBehavior – a simple stateful behavior simulator.
// --------------------------------------------------------------------

// InMemoryBehavior implements Behavior with a simple state machine.
type InMemoryBehavior struct {
    mu         sync.Mutex
    state      string
    startErr   error
    stopErr    error
    statusErr  error
    performMap map[string]func(args ...interface{}) (interface{}, error)
}

// NewInMemoryBehavior creates a new behavior starting in "stopped" state.
func NewInMemoryBehavior() *InMemoryBehavior {
    return &InMemoryBehavior{
        state:      "stopped",
        performMap: make(map[string]func(args ...interface{}) (interface{}, error)),
    }
}

// SetStartError makes Start return the given error (and not change state).
func (b *InMemoryBehavior) SetStartError(err error) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.startErr = err
}

// SetStopError makes Stop return the given error.
func (b *InMemoryBehavior) SetStopError(err error) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.stopErr = err
}

// SetStatusError makes Status return the given error.
func (b *InMemoryBehavior) SetStatusError(err error) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.statusErr = err
}

// RegisterAction registers a handler for a Perform action.
func (b *InMemoryBehavior) RegisterAction(name string, handler func(args ...interface{}) (interface{}, error)) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.performMap[name] = handler
}

// Start transitions to "running" unless error is set.
func (b *InMemoryBehavior) Start() error {
    b.mu.Lock()
    defer b.mu.Unlock()
    if b.startErr != nil {
        return b.startErr
    }
    b.state = "running"
    return nil
}

// Stop transitions to "stopped" unless error is set.
func (b *InMemoryBehavior) Stop() error {
    b.mu.Lock()
    defer b.mu.Unlock()
    if b.stopErr != nil {
        return b.stopErr
    }
    b.state = "stopped"
    return nil
}

// Status returns current state unless error is set.
func (b *InMemoryBehavior) Status() (string, error) {
    b.mu.Lock()
    defer b.mu.Unlock()
    if b.statusErr != nil {
        return "", b.statusErr
    }
    return b.state, nil
}

// Perform executes a registered action.
func (b *InMemoryBehavior) Perform(action string, args ...interface{}) (interface{}, error) {
    b.mu.Lock()
    handler, ok := b.performMap[action]
    b.mu.Unlock()
    if !ok {
        return nil, nil // no handler, no error
    }
    return handler(args...)
}

// --------------------------------------------------------------------
// BehaviorConditioner – wraps a Behavior to add delays and per‑call errors.
// --------------------------------------------------------------------

// BehaviorConditioner adds configurable delays and error injection to any Behavior.
type BehaviorConditioner struct {
    mu             sync.Mutex
    behavior       Behavior
    startDelay     time.Duration
    stopDelay      time.Duration
    statusDelay    time.Duration
    performDelay   time.Duration
    startErrors    map[int]error
    stopErrors     map[int]error
    statusErrors   map[int]error
    performErrors  map[int]error
    startCalls     int
    stopCalls      int
    statusCalls    int
    performCalls   int
}

// NewBehaviorConditioner creates a conditioner around an existing Behavior.
func NewBehaviorConditioner(behavior Behavior) *BehaviorConditioner {
    return &BehaviorConditioner{
        behavior:      behavior,
        startErrors:   make(map[int]error),
        stopErrors:    make(map[int]error),
        statusErrors:  make(map[int]error),
        performErrors: make(map[int]error),
    }
}

// SetStartDelay adds a fixed delay before Start.
func (c *BehaviorConditioner) SetStartDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.startDelay = d
}

// SetStopDelay adds a fixed delay before Stop.
func (c *BehaviorConditioner) SetStopDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.stopDelay = d
}

// SetStatusDelay adds a fixed delay before Status.
func (c *BehaviorConditioner) SetStatusDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.statusDelay = d
}

// SetPerformDelay adds a fixed delay before Perform.
func (c *BehaviorConditioner) SetPerformDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.performDelay = d
}

// InjectStartError makes the nth call to Start return the given error.
func (c *BehaviorConditioner) InjectStartError(callNumber int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.startErrors[callNumber] = err
}

// InjectStopError makes the nth call to Stop return the given error.
func (c *BehaviorConditioner) InjectStopError(callNumber int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.stopErrors[callNumber] = err
}

// InjectStatusError makes the nth call to Status return the given error.
func (c *BehaviorConditioner) InjectStatusError(callNumber int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.statusErrors[callNumber] = err
}

// InjectPerformError makes the nth call to Perform return the given error.
func (c *BehaviorConditioner) InjectPerformError(callNumber int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.performErrors[callNumber] = err
}

// Start implements Behavior with delay and error injection.
func (c *BehaviorConditioner) Start() error {
    c.mu.Lock()
    c.startCalls++
    call := c.startCalls
    delay := c.startDelay
    err, ok := c.startErrors[call]
    if ok {
        delete(c.startErrors, call)
    }
    c.mu.Unlock()
    if ok {
        return err
    }
    if delay > 0 {
        time.Sleep(delay)
    }
    return c.behavior.Start()
}

// Stop implements Behavior with delay and error injection.
func (c *BehaviorConditioner) Stop() error {
    c.mu.Lock()
    c.stopCalls++
    call := c.stopCalls
    delay := c.stopDelay
    err, ok := c.stopErrors[call]
    if ok {
        delete(c.stopErrors, call)
    }
    c.mu.Unlock()
    if ok {
        return err
    }
    if delay > 0 {
        time.Sleep(delay)
    }
    return c.behavior.Stop()
}

// Status implements Behavior with delay and error injection.
func (c *BehaviorConditioner) Status() (string, error) {
    c.mu.Lock()
    c.statusCalls++
    call := c.statusCalls
    delay := c.statusDelay
    err, ok := c.statusErrors[call]
    if ok {
        delete(c.statusErrors, call)
    }
    c.mu.Unlock()
    if ok {
        return "", err
    }
    if delay > 0 {
        time.Sleep(delay)
    }
    return c.behavior.Status()
}

// Perform implements Behavior with delay and error injection.
func (c *BehaviorConditioner) Perform(action string, args ...interface{}) (interface{}, error) {
    c.mu.Lock()
    c.performCalls++
    call := c.performCalls
    delay := c.performDelay
    err, ok := c.performErrors[call]
    if ok {
        delete(c.performErrors, call)
    }
    c.mu.Unlock()
    if ok {
        return nil, err
    }
    if delay > 0 {
        time.Sleep(delay)
    }
    return c.behavior.Perform(action, args...)
}

// --------------------------------------------------------------------
// BehaviorAssertions – helper functions for testing with Behavior.
// --------------------------------------------------------------------

type testingT interface {
    Error(args ...interface{})
    Errorf(format string, args ...interface{})
}

// BehaviorAssertions provides convenience methods for verifying behavior calls.
type BehaviorAssertions struct {
    t testingT
}

// NewBehaviorAssertions creates a new assertion helper.
func NewBehaviorAssertions(t testingT) *BehaviorAssertions {
    return &BehaviorAssertions{t: t}
}

// AssertStartCalled asserts that Start was called at least once.
func (a *BehaviorAssertions) AssertStartCalled(m *MockBehavior) {
    if m.startCalls == 0 {
        a.t.Error("expected Start to be called, but it wasn't")
    }
}

// AssertStartNotCalled asserts that Start was not called.
func (a *BehaviorAssertions) AssertStartNotCalled(m *MockBehavior) {
    if m.startCalls > 0 {
        a.t.Errorf("expected Start not to be called, but it was called %d times", m.startCalls)
    }
}

// AssertStopCalled asserts that Stop was called at least once.
func (a *BehaviorAssertions) AssertStopCalled(m *MockBehavior) {
    if m.stopCalls == 0 {
        a.t.Error("expected Stop to be called, but it wasn't")
    }
}

// AssertPerformCalled asserts that Perform was called with the given action.
func (a *BehaviorAssertions) AssertPerformCalled(m *MockBehavior, action string) {
    for _, call := range m.calls {
        if call.Method == "Perform" && call.Action == action {
            return
        }
    }
    a.t.Errorf("expected Perform with action %q to be called, but it wasn't", action)
}