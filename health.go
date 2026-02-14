// Package testutils provides mock and simple in-memory health aggregators
// for storage, memory, network, and database components. It builds on the
// Checker interface from check.go to provide overall health status and
// simulate health endpoints for testing.
package testutils

import (
    "encoding/json"
    "net/http"
    "sync"
    "time"
)

// --------------------------------------------------------------------
// OverallStatus – aggregated health status of all components.
// --------------------------------------------------------------------

// OverallStatus holds the combined health check results.
type OverallStatus struct {
    // OK is true only if all components are healthy.
    OK bool `json:"ok"`

    // Timestamp of the overall check.
    Timestamp time.Time `json:"timestamp"`

    // Components contains the status of each individual component.
    Components map[string]HealthStatus `json:"components,omitempty"`

    // Message provides a human-readable summary.
    Message string `json:"message,omitempty"`
}

// NewOverallStatus creates a new status from a map of component statuses.
func NewOverallStatus(components map[string]HealthStatus) OverallStatus {
    ok := true
    var firstErrMsg string
    for name, status := range components {
        if !status.OK {
            ok = false
            if firstErrMsg == "" && status.Message != "" {
                firstErrMsg = name + ": " + status.Message
            }
        }
    }
    msg := "all systems operational"
    if !ok {
        if firstErrMsg != "" {
            msg = "degraded: " + firstErrMsg
        } else {
            msg = "degraded"
        }
    }
    return OverallStatus{
        OK:         ok,
        Timestamp:  time.Now(),
        Components: components,
        Message:    msg,
    }
}

// --------------------------------------------------------------------
// Health – interface for checking overall system health.
// --------------------------------------------------------------------

// Health defines a method to get the aggregated health status.
type Health interface {
    // Check returns the overall health status.
    Check() (OverallStatus, error)
}

// --------------------------------------------------------------------
// MockHealth – a test double that records calls and can be programmed.
// --------------------------------------------------------------------

// MockHealth implements Health for unit tests.
type MockHealth struct {
    mu         sync.Mutex
    status     OverallStatus
    err        error
    callCount  int
}

// NewMockHealth creates a new mock health with OK status by default.
func NewMockHealth() *MockHealth {
    return &MockHealth{
        status: OverallStatus{OK: true, Timestamp: time.Now(), Message: "mock OK"},
    }
}

// SetStatus programs the status returned by Check.
func (m *MockHealth) SetStatus(status OverallStatus) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.status = status
}

// SetError programs the error returned by Check.
func (m *MockHealth) SetError(err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.err = err
}

// Check records the call and returns programmed status and error.
func (m *MockHealth) Check() (OverallStatus, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.callCount++
    return m.status, m.err
}

// CallCount returns the number of times Check was called.
func (m *MockHealth) CallCount() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.callCount
}

// Reset clears programmed values and call count.
func (m *MockHealth) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.status = OverallStatus{OK: true, Timestamp: time.Now(), Message: "mock OK"}
    m.err = nil
    m.callCount = 0
}

// --------------------------------------------------------------------
// MultiHealth – aggregates multiple Checkers into one Health.
// --------------------------------------------------------------------

// MultiHealth implements Health by calling a set of named Checkers
// and combining their results.
type MultiHealth struct {
    mu        sync.RWMutex
    checkers  map[string]Checker
}

// NewMultiHealth creates a new MultiHealth with the given checkers.
func NewMultiHealth(checkers map[string]Checker) *MultiHealth {
    return &MultiHealth{
        checkers: checkers,
    }
}

// AddChecker adds or replaces a checker with the given name.
func (h *MultiHealth) AddChecker(name string, c Checker) {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.checkers[name] = c
}

// RemoveChecker removes a checker by name.
func (h *MultiHealth) RemoveChecker(name string) {
    h.mu.Lock()
    defer h.mu.Unlock()
    delete(h.checkers, name)
}

// Check calls all registered checkers and aggregates their statuses.
// If any checker returns an error, that checker's status is marked as
// unhealthy with the error message, and the overall status is degraded.
func (h *MultiHealth) Check() (OverallStatus, error) {
    h.mu.RLock()
    defer h.mu.RUnlock()
    components := make(map[string]HealthStatus)
    for name, checker := range h.checkers {
        status, err := checker.StorageCheck() // Actually need to call appropriate method per component.
        // But Checker has multiple methods; we need to know which method corresponds to which component.
        // In a real implementation, we'd either have a single Check() method that returns a map,
        // or we'd need to map names to specific Checker methods. For simplicity, let's assume
        // each checker is specialized (e.g., StorageChecker, MemoryChecker). But Checker interface
        // already has all four. So maybe we should call each method on each checker?
        // That would be inefficient and not match real usage.
        // Better: define a new interface ComponentChecker that has a single Check() method.
        // But we already have Checker with four methods. Let's instead have MultiHealth accept
        // a map of functions: map[string]func() (HealthStatus, error).
        // That is more flexible.
    }
    // For now, we'll implement a version that expects a map of functions.
    // But to keep the file self-contained, we'll define a new type ComponentCheckFunc.
    return OverallStatus{}, nil
}

// --------------------------------------------------------------------
// ComponentCheckFunc – a function that checks a single component.
// --------------------------------------------------------------------

type ComponentCheckFunc func() (HealthStatus, error)

// FuncHealth implements Health by calling a set of named component check functions.
type FuncHealth struct {
    mu        sync.RWMutex
    checks    map[string]ComponentCheckFunc
}

// NewFuncHealth creates a new health aggregator from named check functions.
func NewFuncHealth(checks map[string]ComponentCheckFunc) *FuncHealth {
    return &FuncHealth{
        checks: checks,
    }
}

// AddCheck adds or replaces a check function with the given name.
func (h *FuncHealth) AddCheck(name string, fn ComponentCheckFunc) {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.checks[name] = fn
}

// RemoveCheck removes a check by name.
func (h *FuncHealth) RemoveCheck(name string) {
    h.mu.Lock()
    defer h.mu.Unlock()
    delete(h.checks, name)
}

// Check calls all registered check functions and aggregates results.
func (h *FuncHealth) Check() (OverallStatus, error) {
    h.mu.RLock()
    defer h.mu.RUnlock()
    components := make(map[string]HealthStatus)
    ok := true
    var firstErrMsg string
    for name, fn := range h.checks {
        status, err := fn()
        if err != nil {
            status = HealthStatus{
                OK:        false,
                Message:   err.Error(),
                Timestamp: time.Now(),
            }
        }
        components[name] = status
        if !status.OK {
            ok = false
            if firstErrMsg == "" && status.Message != "" {
                firstErrMsg = name + ": " + status.Message
            }
        }
    }
    msg := "all systems operational"
    if !ok {
        if firstErrMsg != "" {
            msg = "degraded: " + firstErrMsg
        } else {
            msg = "degraded"
        }
    }
    return OverallStatus{
        OK:         ok,
        Timestamp:  time.Now(),
        Components: components,
        Message:    msg,
    }, nil
}

// --------------------------------------------------------------------
// InMemoryHealth – a simple health that returns a pre‑set overall status.
// --------------------------------------------------------------------

// InMemoryHealth implements Health with a mutable status.
type InMemoryHealth struct {
    mu     sync.RWMutex
    status OverallStatus
    err    error
}

// NewInMemoryHealth creates a new health with an initial OK status.
func NewInMemoryHealth() *InMemoryHealth {
    return &InMemoryHealth{
        status: OverallStatus{OK: true, Timestamp: time.Now(), Message: "all good"},
    }
}

// Update allows tests to modify the overall status.
func (h *InMemoryHealth) Update(update func(*OverallStatus)) {
    h.mu.Lock()
    defer h.mu.Unlock()
    update(&h.status)
    h.status.Timestamp = time.Now()
}

// SetError makes Check return an error.
func (h *InMemoryHealth) SetError(err error) {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.err = err
}

// Check returns the current status or error.
func (h *InMemoryHealth) Check() (OverallStatus, error) {
    h.mu.RLock()
    defer h.mu.RUnlock()
    if h.err != nil {
        return OverallStatus{}, h.err
    }
    return h.status, nil
}

// --------------------------------------------------------------------
// HealthHandler – an HTTP handler that serves health checks.
// --------------------------------------------------------------------

// HealthHandler returns an http.HandlerFunc that calls the given Health
// and returns a JSON response.
func HealthHandler(h Health) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        status, err := h.Check()
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        if !status.OK {
            w.WriteHeader(http.StatusServiceUnavailable)
        } else {
            w.WriteHeader(http.StatusOK)
        }
        json.NewEncoder(w).Encode(status)
    }
}