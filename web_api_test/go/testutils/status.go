// Package testutils provides mock and simple in‑memory status providers
// for testing storage, memory, network, and database components. It allows
// tests to simulate various status conditions and verify that the system
// reacts appropriately.
package testutils

import (
    "sync"
    "time"
)

// --------------------------------------------------------------------
// Status – represents the current condition of a component.
// --------------------------------------------------------------------

// Status holds the operational state and metadata for a component.
type Status struct {
    // Component is the name of the component reporting this status.
    Component string `json:"component"`

    // State is a short string describing the current state
    // (e.g., "running", "stopped", "degraded", "error").
    State string `json:"state"`

    // Message provides a human‑readable explanation, especially useful
    // when the state is not "running".
    Message string `json:"message,omitempty"`

    // Timestamp marks when the status was captured.
    Timestamp time.Time `json:"timestamp"`

    // Details can hold arbitrary structured data about the component
    // (e.g., version, uptime, custom metrics).
    Details map[string]interface{} `json:"details,omitempty"`
}

// IsRunning returns true if the status indicates a healthy, running component.
func (s Status) IsRunning() bool {
    return s.State == "running"
}

// IsDegraded returns true if the component is running but with issues.
func (s Status) IsDegraded() bool {
    return s.State == "degraded"
}

// IsStopped returns true if the component is intentionally stopped.
func (s Status) IsStopped() bool {
    return s.State == "stopped"
}

// IsError returns true if the component is in an error state.
func (s Status) IsError() bool {
    return s.State == "error"
}

// --------------------------------------------------------------------
// StatusProvider – interface for obtaining component status.
// --------------------------------------------------------------------

// StatusProvider defines a method to retrieve the current status.
type StatusProvider interface {
    // Status returns the current status of the component.
    Status() (Status, error)
}

// --------------------------------------------------------------------
// MockStatusProvider – a test double that records calls and can be programmed.
// --------------------------------------------------------------------

// MockStatusProvider implements StatusProvider for unit tests.
type MockStatusProvider struct {
    mu        sync.Mutex
    status    Status
    err       error
    callCount int
    callFunc  func() (Status, error) // optional custom behavior
}

// NewMockStatusProvider creates a new mock provider with a default running status.
func NewMockStatusProvider() *MockStatusProvider {
    return &MockStatusProvider{
        status: Status{
            State:     "running",
            Timestamp: time.Now(),
        },
    }
}

// SetStatus programs the status returned by Status.
func (m *MockStatusProvider) SetStatus(status Status) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.status = status
}

// SetError programs the error returned by Status.
func (m *MockStatusProvider) SetError(err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.err = err
}

// SetCallFunc overrides the Status method with custom behavior.
func (m *MockStatusProvider) SetCallFunc(fn func() (Status, error)) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.callFunc = fn
}

// Status records the call and returns programmed status/error or calls custom function.
func (m *MockStatusProvider) Status() (Status, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.callCount++
    if m.callFunc != nil {
        return m.callFunc()
    }
    return m.status, m.err
}

// CallCount returns the number of times Status was called.
func (m *MockStatusProvider) CallCount() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.callCount
}

// Reset clears programmed values and call count.
func (m *MockStatusProvider) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.status = Status{
        State:     "running",
        Timestamp: time.Now(),
    }
    m.err = nil
    m.callCount = 0
    m.callFunc = nil
}

// --------------------------------------------------------------------
// InMemoryStatusProvider – a simple mutable status provider.
// --------------------------------------------------------------------

// InMemoryStatusProvider implements StatusProvider with a mutable status.
type InMemoryStatusProvider struct {
    mu     sync.RWMutex
    status Status
    err    error
}

// NewInMemoryStatusProvider creates a new provider with an initial status.
// The initial status is "running" with the current timestamp.
func NewInMemoryStatusProvider() *InMemoryStatusProvider {
    return &InMemoryStatusProvider{
        status: Status{
            State:     "running",
            Timestamp: time.Now(),
        },
    }
}

// SetStatus updates the status returned by Status.
func (p *InMemoryStatusProvider) SetStatus(status Status) {
    p.mu.Lock()
    defer p.mu.Unlock()
    p.status = status
    p.status.Timestamp = time.Now()
}

// Update allows tests to modify the status via a callback.
func (p *InMemoryStatusProvider) Update(update func(*Status)) {
    p.mu.Lock()
    defer p.mu.Unlock()
    update(&p.status)
    p.status.Timestamp = time.Now()
}

// SetError makes Status return an error.
func (p *InMemoryStatusProvider) SetError(err error) {
    p.mu.Lock()
    defer p.mu.Unlock()
    p.err = err
}

// Status returns the current status or error.
func (p *InMemoryStatusProvider) Status() (Status, error) {
    p.mu.RLock()
    defer p.mu.RUnlock()
    if p.err != nil {
        return Status{}, p.err
    }
    return p.status, nil
}

// --------------------------------------------------------------------
// MultiStatusProvider – aggregates multiple named status providers.
// --------------------------------------------------------------------

// MultiStatusProvider collects status from several providers and returns
// a map of component names to their status. The overall status can be
// computed from the individual ones.
type MultiStatusProvider struct {
    mu        sync.RWMutex
    providers map[string]StatusProvider
}

// NewMultiStatusProvider creates a new empty multi‑provider.
func NewMultiStatusProvider() *MultiStatusProvider {
    return &MultiStatusProvider{
        providers: make(map[string]StatusProvider),
    }
}

// AddProvider adds or replaces a provider with the given name.
func (m *MultiStatusProvider) AddProvider(name string, provider StatusProvider) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.providers[name] = provider
}

// RemoveProvider removes a provider by name.
func (m *MultiStatusProvider) RemoveProvider(name string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    delete(m.providers, name)
}

// Status calls each provider and returns a map of component statuses.
// If any provider returns an error, that component's status is omitted
// from the map and the error is returned as part of the overall status.
// The returned Status is a composite representing the overall health.
func (m *MultiStatusProvider) Status() (Status, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    components := make(map[string]Status)
    overallOK := true
    var firstErrMsg string

    for name, provider := range m.providers {
        status, err := provider.Status()
        if err != nil {
            // If a provider fails, we treat it as error state.
            status = Status{
                Component: name,
                State:     "error",
                Message:   err.Error(),
                Timestamp: time.Now(),
            }
            overallOK = false
            if firstErrMsg == "" {
                firstErrMsg = name + ": " + err.Error()
            }
        }
        components[name] = status
        if status.State != "running" {
            overallOK = false
            if firstErrMsg == "" && status.Message != "" {
                firstErrMsg = name + ": " + status.Message
            }
        }
    }

    overallState := "running"
    if !overallOK {
        overallState = "degraded"
        if firstErrMsg != "" {
            overallState = "degraded: " + firstErrMsg
        }
    }

    overall := Status{
        State:     overallState,
        Timestamp: time.Now(),
        Details: map[string]interface{}{
            "components": components,
        },
    }
    return overall, nil
}

// --------------------------------------------------------------------
// StatusConditioner – wraps a StatusProvider to add delays and per‑call errors.
// --------------------------------------------------------------------

// StatusConditioner adds configurable delays and error injection to a StatusProvider.
type StatusConditioner struct {
    mu        sync.Mutex
    provider  StatusProvider
    delay     time.Duration
    errors    map[int]error
    callCount int
}

// NewStatusConditioner creates a conditioner around an existing StatusProvider.
func NewStatusConditioner(provider StatusProvider) *StatusConditioner {
    return &StatusConditioner{
        provider: provider,
        errors:   make(map[int]error),
    }
}

// SetDelay adds a fixed delay before every Status call.
func (c *StatusConditioner) SetDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.delay = d
}

// InjectError makes the nth call to Status return the given error.
func (c *StatusConditioner) InjectError(callNumber int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.errors[callNumber] = err
}

// Status implements StatusProvider with delay and error injection.
func (c *StatusConditioner) Status() (Status, error) {
    c.mu.Lock()
    c.callCount++
    call := c.callCount
    delay := c.delay
    err, ok := c.errors[call]
    if ok {
        delete(c.errors, call)
        c.mu.Unlock()
        return Status{}, err
    }
    c.mu.Unlock()

    if delay > 0 {
        time.Sleep(delay)
    }
    return c.provider.Status()
}

// --------------------------------------------------------------------
// StatusAssertions – helper functions for testing with StatusProvider.
// --------------------------------------------------------------------

type testingT interface {
    Error(args ...interface{})
    Errorf(format string, args ...interface{})
}

// StatusAssertions provides convenience methods for verifying statuses.
type StatusAssertions struct {
    t testingT
}

// NewStatusAssertions creates a new assertion helper.
func NewStatusAssertions(t testingT) *StatusAssertions {
    return &StatusAssertions{t: t}
}

// AssertStatusRunning asserts that the provider returns a running status with no error.
func (a *StatusAssertions) AssertStatusRunning(provider StatusProvider) {
    status, err := provider.Status()
    if err != nil {
        a.t.Errorf("expected no error, got %v", err)
        return
    }
    if !status.IsRunning() {
        a.t.Errorf("expected status running, got %q: %s", status.State, status.Message)
    }
}

// AssertStatusError asserts that the provider returns a specific error.
func (a *StatusAssertions) AssertStatusError(provider StatusProvider, expectedErrMsg string) {
    status, err := provider.Status()
    if err == nil {
        a.t.Error("expected error, got none")
        return
    }
    if err.Error() != expectedErrMsg {
        a.t.Errorf("expected error %q, got %q", expectedErrMsg, err.Error())
    }
}

// AssertStatusState asserts that the provider returns a specific state.
func (a *StatusAssertions) AssertStatusState(provider StatusProvider, expectedState string) {
    status, err := provider.Status()
    if err != nil {
        a.t.Errorf("expected no error, got %v", err)
        return
    }
    if status.State != expectedState {
        a.t.Errorf("expected state %q, got %q", expectedState, status.State)
    }
}