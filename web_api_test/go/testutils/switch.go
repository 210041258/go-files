// Package testutils provides mock and simple in‑memory switches for testing.
// A switch is a component that can be in one of several states (e.g., "on", "off")
// and allows toggling or setting the state. Useful for testing feature flags,
// mode changes, and conditional behavior.
package testutils

import (
    "errors"
    "sync"
    "time"
)

// --------------------------------------------------------------------
// Switch – interface for a stateful switch.
// --------------------------------------------------------------------

// Switch defines methods for getting and setting the state of a switch.
type Switch interface {
    // State returns the current state.
    State() (string, error)

    // Set changes the state. Returns an error if the state is invalid or
    // the switch cannot be changed.
    Set(state string) error

    // Toggle switches to the opposite state (if the state space has two values).
    // Returns an error if the switch is not binary or cannot be toggled.
    Toggle() error
}

// --------------------------------------------------------------------
// MockSwitch – a test double that records calls and can be programmed.
// --------------------------------------------------------------------

// MockSwitch implements Switch for unit tests.
type MockSwitch struct {
    mu         sync.Mutex
    state      string
    stateErr   error
    setFunc    func(string) error
    toggleFunc func() error
    stateCalls int
    setCalls   []string // recorded states from Set
    toggleCalls int
    setErrors  map[int]error // per‑call error for Set (1‑based)
    toggleErrors map[int]error // per‑call error for Toggle (1‑based)
}

// NewMockSwitch creates a new mock switch with an initial state.
func NewMockSwitch(initialState string) *MockSwitch {
    return &MockSwitch{
        state:        initialState,
        setErrors:    make(map[int]error),
        toggleErrors: make(map[int]error),
    }
}

// SetStateFunc overrides the State method with custom behavior.
func (m *MockSwitch) SetStateFunc(fn func() (string, error)) {
    m.mu.Lock()
    defer m.mu.Unlock()
    // Not directly storing a func for State; we'll handle via programmed errors/values.
    // Instead, we'll just allow setting state and error separately.
}

// SetSetFunc overrides the Set method with custom behavior.
func (m *MockSwitch) SetSetFunc(fn func(string) error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.setFunc = fn
}

// SetToggleFunc overrides the Toggle method with custom behavior.
func (m *MockSwitch) SetToggleFunc(fn func() error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.toggleFunc = fn
}

// SetStateValue sets the value returned by State (and clears any State error).
func (m *MockSwitch) SetStateValue(state string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.state = state
    m.stateErr = nil
}

// SetStateError makes State return an error.
func (m *MockSwitch) SetStateError(err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.stateErr = err
}

// InjectSetError makes the nth call to Set return the given error.
func (m *MockSwitch) InjectSetError(callNumber int, err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.setErrors[callNumber] = err
}

// InjectToggleError makes the nth call to Toggle return the given error.
func (m *MockSwitch) InjectToggleError(callNumber int, err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.toggleErrors[callNumber] = err
}

// State returns the current state or error.
func (m *MockSwitch) State() (string, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.stateCalls++
    return m.state, m.stateErr
}

// Set records the call and returns programmed error or calls custom function.
func (m *MockSwitch) Set(state string) error {
    m.mu.Lock()
    m.setCalls = append(m.setCalls, state)
    call := len(m.setCalls)
    if err, ok := m.setErrors[call]; ok {
        delete(m.setErrors, call)
        m.mu.Unlock()
        return err
    }
    if m.setFunc != nil {
        fn := m.setFunc
        m.mu.Unlock()
        return fn(state)
    }
    // Default: update state
    m.state = state
    m.mu.Unlock()
    return nil
}

// Toggle records the call and returns programmed error or calls custom function.
func (m *MockSwitch) Toggle() error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.toggleCalls++
    if err, ok := m.toggleErrors[m.toggleCalls]; ok {
        delete(m.toggleErrors, m.toggleCalls)
        return err
    }
    if m.toggleFunc != nil {
        return m.toggleFunc()
    }
    // Default: simple toggle between "on" and "off"
    if m.state == "on" {
        m.state = "off"
    } else if m.state == "off" {
        m.state = "on"
    } else {
        return errors.New("mock switch: cannot toggle non‑binary state")
    }
    return nil
}

// CallCounts returns the number of calls to each method.
func (m *MockSwitch) CallCounts() (state, set, toggle int) {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.stateCalls, len(m.setCalls), m.toggleCalls
}

// SetCalls returns a copy of the states passed to Set.
func (m *MockSwitch) SetCalls() []string {
    m.mu.Lock()
    defer m.mu.Unlock()
    cp := make([]string, len(m.setCalls))
    copy(cp, m.setCalls)
    return cp
}

// Reset clears recorded calls and injected errors, optionally resetting state.
func (m *MockSwitch) Reset(state ...string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.stateCalls = 0
    m.setCalls = nil
    m.toggleCalls = 0
    m.setErrors = make(map[int]error)
    m.toggleErrors = make(map[int]error)
    m.setFunc = nil
    m.toggleFunc = nil
    if len(state) > 0 {
        m.state = state[0]
    }
    m.stateErr = nil
}

// --------------------------------------------------------------------
// InMemorySwitch – a simple stateful switch with optional validation.
// --------------------------------------------------------------------

// InMemorySwitch implements Switch with an in‑memory state and optional allowed states.
type InMemorySwitch struct {
    mu       sync.RWMutex
    state    string
    allowed  map[string]bool // if nil, any state is allowed
    err      error           // if set, all methods return this error
}

// NewInMemorySwitch creates a switch with an initial state and no restrictions.
func NewInMemorySwitch(initialState string) *InMemorySwitch {
    return &InMemorySwitch{
        state: initialState,
    }
}

// NewRestrictedSwitch creates a switch that only allows specific states.
func NewRestrictedSwitch(initialState string, allowedStates []string) (*InMemorySwitch, error) {
    allowed := make(map[string]bool)
    for _, s := range allowedStates {
        allowed[s] = true
    }
    if !allowed[initialState] {
        return nil, errors.New("initial state not allowed")
    }
    return &InMemorySwitch{
        state:   initialState,
        allowed: allowed,
    }, nil
}

// SetError makes all methods return the given error (for simulating failures).
func (s *InMemorySwitch) SetError(err error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.err = err
}

// State returns the current state.
func (s *InMemorySwitch) State() (string, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    if s.err != nil {
        return "", s.err
    }
    return s.state, nil
}

// Set changes the state if allowed.
func (s *InMemorySwitch) Set(newState string) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.err != nil {
        return s.err
    }
    if s.allowed != nil && !s.allowed[newState] {
        return errors.New("in‑memory switch: state not allowed")
    }
    s.state = newState
    return nil
}

// Toggle switches between "on" and "off" if those are the only two allowed states.
// If the switch has more than two allowed states, Toggle returns an error.
func (s *InMemorySwitch) Toggle() error {
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.err != nil {
        return s.err
    }
    if s.allowed != nil && len(s.allowed) != 2 {
        return errors.New("in‑memory switch: cannot toggle, state space is not binary")
    }
    // Determine opposite state
    if s.state == "on" {
        s.state = "off"
    } else if s.state == "off" {
        s.state = "on"
    } else {
        // If not binary, attempt to guess? Better to error.
        return errors.New("in‑memory switch: cannot toggle, current state not binary")
    }
    return nil
}

// --------------------------------------------------------------------
// SwitchConditioner – wraps a Switch to add delays and per‑call errors.
// --------------------------------------------------------------------

// SwitchConditioner adds configurable delays and error injection to any Switch.
type SwitchConditioner struct {
    mu           sync.Mutex
    sw           Switch
    stateDelay   time.Duration
    setDelay     time.Duration
    toggleDelay  time.Duration
    stateErrors  map[int]error
    setErrors    map[int]error
    toggleErrors map[int]error
    stateCalls   int
    setCalls     int
    toggleCalls  int
}

// NewSwitchConditioner creates a conditioner around an existing Switch.
func NewSwitchConditioner(sw Switch) *SwitchConditioner {
    return &SwitchConditioner{
        sw:           sw,
        stateErrors:  make(map[int]error),
        setErrors:    make(map[int]error),
        toggleErrors: make(map[int]error),
    }
}

// SetStateDelay adds a fixed delay before State.
func (c *SwitchConditioner) SetStateDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.stateDelay = d
}

// SetSetDelay adds a fixed delay before Set.
func (c *SwitchConditioner) SetSetDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.setDelay = d
}

// SetToggleDelay adds a fixed delay before Toggle.
func (c *SwitchConditioner) SetToggleDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.toggleDelay = d
}

// InjectStateError makes the nth call to State return the given error.
func (c *SwitchConditioner) InjectStateError(callNumber int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.stateErrors[callNumber] = err
}

// InjectSetError makes the nth call to Set return the given error.
func (c *SwitchConditioner) InjectSetError(callNumber int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.setErrors[callNumber] = err
}

// InjectToggleError makes the nth call to Toggle return the given error.
func (c *SwitchConditioner) InjectToggleError(callNumber int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.toggleErrors[callNumber] = err
}

// State implements Switch with delay and error injection.
func (c *SwitchConditioner) State() (string, error) {
    c.mu.Lock()
    c.stateCalls++
    call := c.stateCalls
    delay := c.stateDelay
    err, ok := c.stateErrors[call]
    if ok {
        delete(c.stateErrors, call)
        c.mu.Unlock()
        return "", err
    }
    c.mu.Unlock()
    if delay > 0 {
        time.Sleep(delay)
    }
    return c.sw.State()
}

// Set implements Switch with delay and error injection.
func (c *SwitchConditioner) Set(state string) error {
    c.mu.Lock()
    c.setCalls++
    call := c.setCalls
    delay := c.setDelay
    err, ok := c.setErrors[call]
    if ok {
        delete(c.setErrors, call)
        c.mu.Unlock()
        return err
    }
    c.mu.Unlock()
    if delay > 0 {
        time.Sleep(delay)
    }
    return c.sw.Set(state)
}

// Toggle implements Switch with delay and error injection.
func (c *SwitchConditioner) Toggle() error {
    c.mu.Lock()
    c.toggleCalls++
    call := c.toggleCalls
    delay := c.toggleDelay
    err, ok := c.toggleErrors[call]
    if ok {
        delete(c.toggleErrors, call)
        c.mu.Unlock()
        return err
    }
    c.mu.Unlock()
    if delay > 0 {
        time.Sleep(delay)
    }
    return c.sw.Toggle()
}

// --------------------------------------------------------------------
// SwitchAssertions – helper functions for testing with Switch.
// --------------------------------------------------------------------

type testingT interface {
    Error(args ...interface{})
    Errorf(format string, args ...interface{})
}

// SwitchAssertions provides convenience methods for verifying switch behavior.
type SwitchAssertions struct {
    t testingT
}

// NewSwitchAssertions creates a new assertion helper.
func NewSwitchAssertions(t testingT) *SwitchAssertions {
    return &SwitchAssertions{t: t}
}

// AssertState asserts that the switch's current state matches expected.
func (a *SwitchAssertions) AssertState(sw Switch, expected string) {
    state, err := sw.State()
    if err != nil {
        a.t.Errorf("unexpected error getting state: %v", err)
        return
    }
    if state != expected {
        a.t.Errorf("expected state %q, got %q", expected, state)
    }
}

// AssertStateError asserts that State returns a specific error.
func (a *SwitchAssertions) AssertStateError(sw Switch, expectedErr string) {
    _, err := sw.State()
    if err == nil {
        a.t.Error("expected error, got none")
        return
    }
    if err.Error() != expectedErr {
        a.t.Errorf("expected error %q, got %q", expectedErr, err.Error())
    }
}

// AssertSetCalled asserts that Set was called with the given state.
func (a *SwitchAssertions) AssertSetCalled(m *MockSwitch, state string) {
    for _, s := range m.SetCalls() {
        if s == state {
            return
        }
    }
    a.t.Errorf("expected Set(%q) to be called, but it wasn't", state)
}

// AssertSetNotCalled asserts that Set was not called with the given state.
func (a *SwitchAssertions) AssertSetNotCalled(m *MockSwitch, state string) {
    for _, s := range m.SetCalls() {
        if s == state {
            a.t.Errorf("expected Set(%q) not to be called, but it was", state)
            return
        }
    }
}

// AssertToggleCalled asserts that Toggle was called.
func (a *SwitchAssertions) AssertToggleCalled(m *MockSwitch) {
    if m.toggleCalls == 0 {
        a.t.Error("expected Toggle to be called, but it wasn't")
    }
}

// AssertToggleCount asserts the exact number of Toggle calls.
func (a *SwitchAssertions) AssertToggleCount(m *MockSwitch, expected int) {
    if m.toggleCalls != expected {
        a.t.Errorf("expected %d Toggle calls, got %d", expected, m.toggleCalls)
    }
}