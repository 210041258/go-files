// Package testutils provides mock and simple in‑memory component abstractions
// for testing storage, memory, network, and database resources. It allows
// tests to define and verify component behavior in a unified way.
package testutils

import (
    "errors"
    "sync"
    "time"
)

// --------------------------------------------------------------------
// Component – unified interface for any testable resource.
// --------------------------------------------------------------------

// Component defines the common methods that all test resources should implement.
type Component interface {
    // Name returns the component's identifier.
    Name() string

    // Start initializes the component.
    Start() error

    // Stop shuts down the component.
    Stop() error

    // Status returns the current operational status (e.g., "running", "stopped", "degraded").
    Status() (string, error)

    // Health performs a health check and returns a boolean indicating if the component is healthy.
    Health() (bool, error)

    // Stats returns component‑specific statistics as a map.
    Stats() (map[string]interface{}, error)
}

// --------------------------------------------------------------------
// MockComponent – a test double that records calls and can be programmed.
// --------------------------------------------------------------------

// ComponentCall records a single method invocation.
type ComponentCall struct {
    Method    string
    Args      []interface{}
    Timestamp time.Time
}

// MockComponent implements Component for unit tests.
type MockComponent struct {
    mu           sync.Mutex
    name         string
    startFunc    func() error
    stopFunc     func() error
    statusFunc   func() (string, error)
    healthFunc   func() (bool, error)
    statsFunc    func() (map[string]interface{}, error)
    calls        []ComponentCall
    startCalls   int
    stopCalls    int
    statusCalls  int
    healthCalls  int
    statsCalls   int
    startErrors  map[int]error
    stopErrors   map[int]error
    statusErrors map[int]error
    statusValues map[int]string
    healthErrors map[int]error
    healthValues map[int]bool
    statsErrors  map[int]error
    statsValues  map[int]map[string]interface{}
}

// NewMockComponent creates a new mock component with the given name.
func NewMockComponent(name string) *MockComponent {
    return &MockComponent{
        name:         name,
        startErrors:  make(map[int]error),
        stopErrors:   make(map[int]error),
        statusErrors: make(map[int]error),
        statusValues: make(map[int]string),
        healthErrors: make(map[int]error),
        healthValues: make(map[int]bool),
        statsErrors:  make(map[int]error),
        statsValues:  make(map[int]map[string]interface{}),
    }
}

// SetStartFunc overrides the Start method.
func (m *MockComponent) SetStartFunc(fn func() error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.startFunc = fn
}

// SetStopFunc overrides the Stop method.
func (m *MockComponent) SetStopFunc(fn func() error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.stopFunc = fn
}

// SetStatusFunc overrides the Status method.
func (m *MockComponent) SetStatusFunc(fn func() (string, error)) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.statusFunc = fn
}

// SetHealthFunc overrides the Health method.
func (m *MockComponent) SetHealthFunc(fn func() (bool, error)) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.healthFunc = fn
}

// SetStatsFunc overrides the Stats method.
func (m *MockComponent) SetStatsFunc(fn func() (map[string]interface{}, error)) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.statsFunc = fn
}

// InjectStartError makes the nth call to Start return the given error.
func (m *MockComponent) InjectStartError(callNumber int, err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.startErrors[callNumber] = err
}

// InjectStopError makes the nth call to Stop return the given error.
func (m *MockComponent) InjectStopError(callNumber int, err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.stopErrors[callNumber] = err
}

// InjectStatusError makes the nth call to Status return the given error.
func (m *MockComponent) InjectStatusError(callNumber int, err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.statusErrors[callNumber] = err
}

// InjectStatusValue makes the nth call to Status return the given string and nil error.
func (m *MockComponent) InjectStatusValue(callNumber int, status string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.statusValues[callNumber] = status
}

// InjectHealthError makes the nth call to Health return the given error.
func (m *MockComponent) InjectHealthError(callNumber int, err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.healthErrors[callNumber] = err
}

// InjectHealthValue makes the nth call to Health return the given bool and nil error.
func (m *MockComponent) InjectHealthValue(callNumber int, healthy bool) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.healthValues[callNumber] = healthy
}

// InjectStatsError makes the nth call to Stats return the given error.
func (m *MockComponent) InjectStatsError(callNumber int, err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.statsErrors[callNumber] = err
}

// InjectStatsValue makes the nth call to Stats return the given map and nil error.
func (m *MockComponent) InjectStatsValue(callNumber int, stats map[string]interface{}) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.statsValues[callNumber] = stats
}

// Name returns the component's name.
func (m *MockComponent) Name() string {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.name
}

// Start records the call and returns programmed error or calls custom function.
func (m *MockComponent) Start() error {
    m.mu.Lock()
    m.startCalls++
    call := m.startCalls
    m.calls = append(m.calls, ComponentCall{Method: "Start", Timestamp: time.Now()})
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
func (m *MockComponent) Stop() error {
    m.mu.Lock()
    m.stopCalls++
    call := m.stopCalls
    m.calls = append(m.calls, ComponentCall{Method: "Stop", Timestamp: time.Now()})
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
func (m *MockComponent) Status() (string, error) {
    m.mu.Lock()
    m.statusCalls++
    call := m.statusCalls
    m.calls = append(m.calls, ComponentCall{Method: "Status", Timestamp: time.Now()})
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

// Health records the call and returns programmed health/error or calls custom function.
func (m *MockComponent) Health() (bool, error) {
    m.mu.Lock()
    m.healthCalls++
    call := m.healthCalls
    m.calls = append(m.calls, ComponentCall{Method: "Health", Timestamp: time.Now()})
    if err, ok := m.healthErrors[call]; ok {
        delete(m.healthErrors, call)
        m.mu.Unlock()
        return false, err
    }
    if healthy, ok := m.healthValues[call]; ok {
        delete(m.healthValues, call)
        m.mu.Unlock()
        return healthy, nil
    }
    if m.healthFunc != nil {
        fn := m.healthFunc
        m.mu.Unlock()
        return fn()
    }
    m.mu.Unlock()
    return true, nil
}

// Stats records the call and returns programmed stats/error or calls custom function.
func (m *MockComponent) Stats() (map[string]interface{}, error) {
    m.mu.Lock()
    m.statsCalls++
    call := m.statsCalls
    m.calls = append(m.calls, ComponentCall{Method: "Stats", Timestamp: time.Now()})
    if err, ok := m.statsErrors[call]; ok {
        delete(m.statsErrors, call)
        m.mu.Unlock()
        return nil, err
    }
    if stats, ok := m.statsValues[call]; ok {
        delete(m.statsValues, call)
        m.mu.Unlock()
        return stats, nil
    }
    if m.statsFunc != nil {
        fn := m.statsFunc
        m.mu.Unlock()
        return fn()
    }
    m.mu.Unlock()
    return map[string]interface{}{}, nil
}

// Calls returns a copy of all recorded calls.
func (m *MockComponent) Calls() []ComponentCall {
    m.mu.Lock()
    defer m.mu.Unlock()
    cp := make([]ComponentCall, len(m.calls))
    copy(cp, m.calls)
    return cp
}

// CallCounts returns the number of calls to each method.
func (m *MockComponent) CallCounts() (start, stop, status, health, stats int) {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.startCalls, m.stopCalls, m.statusCalls, m.healthCalls, m.statsCalls
}

// Reset clears recorded calls and injected values.
func (m *MockComponent) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.calls = nil
    m.startCalls = 0
    m.stopCalls = 0
    m.statusCalls = 0
    m.healthCalls = 0
    m.statsCalls = 0
    m.startErrors = make(map[int]error)
    m.stopErrors = make(map[int]error)
    m.statusErrors = make(map[int]error)
    m.statusValues = make(map[int]string)
    m.healthErrors = make(map[int]error)
    m.healthValues = make(map[int]bool)
    m.statsErrors = make(map[int]error)
    m.statsValues = make(map[int]map[string]interface{})
    m.startFunc = nil
    m.stopFunc = nil
    m.statusFunc = nil
    m.healthFunc = nil
    m.statsFunc = nil
}

// --------------------------------------------------------------------
// InMemoryComponent – a simple stateful component.
// --------------------------------------------------------------------

// InMemoryComponent implements Component with an in‑memory state.
type InMemoryComponent struct {
    mu         sync.RWMutex
    name       string
    state      string // "stopped", "running", "degraded", "error"
    healthOK   bool
    stats      map[string]interface{}
    startErr   error
    stopErr    error
    statusErr  error
    healthErr  error
    statsErr   error
}

// NewInMemoryComponent creates a new component in "stopped" state with default healthy.
func NewInMemoryComponent(name string) *InMemoryComponent {
    return &InMemoryComponent{
        name:     name,
        state:    "stopped",
        healthOK: true,
        stats:    make(map[string]interface{}),
    }
}

// SetState sets the component's state.
func (c *InMemoryComponent) SetState(state string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.state = state
}

// SetHealth sets the health status.
func (c *InMemoryComponent) SetHealth(ok bool) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.healthOK = ok
}

// SetStat sets a single statistic.
func (c *InMemoryComponent) SetStat(key string, value interface{}) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.stats[key] = value
}

// SetStartError makes Start return an error (and not change state).
func (c *InMemoryComponent) SetStartError(err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.startErr = err
}

// SetStopError makes Stop return an error.
func (c *InMemoryComponent) SetStopError(err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.stopErr = err
}

// SetStatusError makes Status return an error.
func (c *InMemoryComponent) SetStatusError(err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.statusErr = err
}

// SetHealthError makes Health return an error.
func (c *InMemoryComponent) SetHealthError(err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.healthErr = err
}

// SetStatsError makes Stats return an error.
func (c *InMemoryComponent) SetStatsError(err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.statsErr = err
}

// Name returns the component's name.
func (c *InMemoryComponent) Name() string {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return c.name
}

// Start transitions to "running" unless error is set.
func (c *InMemoryComponent) Start() error {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.startErr != nil {
        return c.startErr
    }
    c.state = "running"
    return nil
}

// Stop transitions to "stopped" unless error is set.
func (c *InMemoryComponent) Stop() error {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.stopErr != nil {
        return c.stopErr
    }
    c.state = "stopped"
    return nil
}

// Status returns current state unless error is set.
func (c *InMemoryComponent) Status() (string, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if c.statusErr != nil {
        return "", c.statusErr
    }
    return c.state, nil
}

// Health returns health status unless error is set.
func (c *InMemoryComponent) Health() (bool, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if c.healthErr != nil {
        return false, c.healthErr
    }
    return c.healthOK, nil
}

// Stats returns a copy of the stats map unless error is set.
func (c *InMemoryComponent) Stats() (map[string]interface{}, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if c.statsErr != nil {
        return nil, c.statsErr
    }
    cp := make(map[string]interface{})
    for k, v := range c.stats {
        cp[k] = v
    }
    return cp, nil
}

// --------------------------------------------------------------------
// ComponentConditioner – wraps a Component to add delays and per‑call errors.
// --------------------------------------------------------------------

// ComponentConditioner adds configurable delays and error injection to any Component.
type ComponentConditioner struct {
    mu           sync.Mutex
    component    Component
    startDelay   time.Duration
    stopDelay    time.Duration
    statusDelay  time.Duration
    healthDelay  time.Duration
    statsDelay   time.Duration
    startErrors  map[int]error
    stopErrors   map[int]error
    statusErrors map[int]error
    healthErrors map[int]error
    statsErrors  map[int]error
    startCalls   int
    stopCalls    int
    statusCalls  int
    healthCalls  int
    statsCalls   int
}

// NewComponentConditioner creates a conditioner around an existing Component.
func NewComponentConditioner(comp Component) *ComponentConditioner {
    return &ComponentConditioner{
        component:    comp,
        startErrors:  make(map[int]error),
        stopErrors:   make(map[int]error),
        statusErrors: make(map[int]error),
        healthErrors: make(map[int]error),
        statsErrors:  make(map[int]error),
    }
}

// SetStartDelay adds a fixed delay before Start.
func (c *ComponentConditioner) SetStartDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.startDelay = d
}

// SetStopDelay adds a fixed delay before Stop.
func (c *ComponentConditioner) SetStopDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.stopDelay = d
}

// SetStatusDelay adds a fixed delay before Status.
func (c *ComponentConditioner) SetStatusDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.statusDelay = d
}

// SetHealthDelay adds a fixed delay before Health.
func (c *ComponentConditioner) SetHealthDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.healthDelay = d
}

// SetStatsDelay adds a fixed delay before Stats.
func (c *ComponentConditioner) SetStatsDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.statsDelay = d
}

// InjectStartError makes the nth call to Start return the given error.
func (c *ComponentConditioner) InjectStartError(callNumber int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.startErrors[callNumber] = err
}

// InjectStopError makes the nth call to Stop return the given error.
func (c *ComponentConditioner) InjectStopError(callNumber int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.stopErrors[callNumber] = err
}

// InjectStatusError makes the nth call to Status return the given error.
func (c *ComponentConditioner) InjectStatusError(callNumber int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.statusErrors[callNumber] = err
}

// InjectHealthError makes the nth call to Health return the given error.
func (c *ComponentConditioner) InjectHealthError(callNumber int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.healthErrors[callNumber] = err
}

// InjectStatsError makes the nth call to Stats return the given error.
func (c *ComponentConditioner) InjectStatsError(callNumber int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.statsErrors[callNumber] = err
}

// Name returns the component's name.
func (c *ComponentConditioner) Name() string {
    return c.component.Name()
}

// Start adds delay then delegates.
func (c *ComponentConditioner) Start() error {
    c.mu.Lock()
    c.startCalls++
    call := c.startCalls
    delay := c.startDelay
    err, ok := c.startErrors[call]
    if ok {
        delete(c.startErrors, call)
        c.mu.Unlock()
        return err
    }
    c.mu.Unlock()
    if delay > 0 {
        time.Sleep(delay)
    }
    return c.component.Start()
}

// Stop adds delay then delegates.
func (c *ComponentConditioner) Stop() error {
    c.mu.Lock()
    c.stopCalls++
    call := c.stopCalls
    delay := c.stopDelay
    err, ok := c.stopErrors[call]
    if ok {
        delete(c.stopErrors, call)
        c.mu.Unlock()
        return err
    }
    c.mu.Unlock()
    if delay > 0 {
        time.Sleep(delay)
    }
    return c.component.Stop()
}

// Status adds delay then delegates.
func (c *ComponentConditioner) Status() (string, error) {
    c.mu.Lock()
    c.statusCalls++
    call := c.statusCalls
    delay := c.statusDelay
    err, ok := c.statusErrors[call]
    if ok {
        delete(c.statusErrors, call)
        c.mu.Unlock()
        return "", err
    }
    c.mu.Unlock()
    if delay > 0 {
        time.Sleep(delay)
    }
    return c.component.Status()
}

// Health adds delay then delegates.
func (c *ComponentConditioner) Health() (bool, error) {
    c.mu.Lock()
    c.healthCalls++
    call := c.healthCalls
    delay := c.healthDelay
    err, ok := c.healthErrors[call]
    if ok {
        delete(c.healthErrors, call)
        c.mu.Unlock()
        return false, err
    }
    c.mu.Unlock()
    if delay > 0 {
        time.Sleep(delay)
    }
    return c.component.Health()
}

// Stats adds delay then delegates.
func (c *ComponentConditioner) Stats() (map[string]interface{}, error) {
    c.mu.Lock()
    c.statsCalls++
    call := c.statsCalls
    delay := c.statsDelay
    err, ok := c.statsErrors[call]
    if ok {
        delete(c.statsErrors, call)
        c.mu.Unlock()
        return nil, err
    }
    c.mu.Unlock()
    if delay > 0 {
        time.Sleep(delay)
    }
    return c.component.Stats()
}

// --------------------------------------------------------------------
// ComponentAssertions – helper functions for testing with Component.
// --------------------------------------------------------------------

type testingT interface {
    Error(args ...interface{})
    Errorf(format string, args ...interface{})
}

// ComponentAssertions provides convenience methods for verifying component calls.
type ComponentAssertions struct {
    t testingT
}

// NewComponentAssertions creates a new assertion helper.
func NewComponentAssertions(t testingT) *ComponentAssertions {
    return &ComponentAssertions{t: t}
}

// AssertStartCalled asserts that Start was called at least once.
func (a *ComponentAssertions) AssertStartCalled(m *MockComponent) {
    if m.startCalls == 0 {
        a.t.Error("expected Start to be called, but it wasn't")
    }
}

// AssertStartNotCalled asserts that Start was not called.
func (a *ComponentAssertions) AssertStartNotCalled(m *MockComponent) {
    if m.startCalls > 0 {
        a.t.Errorf("expected Start not to be called, but it was called %d times", m.startCalls)
    }
}

// AssertStopCalled asserts that Stop was called at least once.
func (a *ComponentAssertions) AssertStopCalled(m *MockComponent) {
    if m.stopCalls == 0 {
        a.t.Error("expected Stop to be called, but it wasn't")
    }
}

// AssertStatusCalled asserts that Status was called at least once.
func (a *ComponentAssertions) AssertStatusCalled(m *MockComponent) {
    if m.statusCalls == 0 {
        a.t.Error("expected Status to be called, but it wasn't")
    }
}

// AssertHealthCalled asserts that Health was called at least once.
func (a *ComponentAssertions) AssertHealthCalled(m *MockComponent) {
    if m.healthCalls == 0 {
        a.t.Error("expected Health to be called, but it wasn't")
    }
}

// AssertStatsCalled asserts that Stats was called at least once.
func (a *ComponentAssertions) AssertStatsCalled(m *MockComponent) {
    if m.statsCalls == 0 {
        a.t.Error("expected Stats to be called, but it wasn't")
    }
}