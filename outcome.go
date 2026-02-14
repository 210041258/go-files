// Package testutils provides mock and simple in-memory outcome collectors
// for testing storage, memory, network, and database operations. It allows
// tests to record, inspect, and verify the results of operations.
package testutils

import (
    "errors"
    "sync"
    "time"
)

// --------------------------------------------------------------------
// Outcome – represents the result of a single operation.
// --------------------------------------------------------------------

// Outcome holds the details of an operation's execution.
type Outcome struct {
    // Operation is the name or identifier of the operation (e.g., "write", "read", "query").
    Operation string

    // Success indicates whether the operation completed without error.
    Success bool

    // Error contains the error if the operation failed; nil if successful.
    Error error

    // Duration is the time taken by the operation.
    Duration time.Duration

    // Timestamp marks when the operation finished.
    Timestamp time.Time

    // Data optionally holds any output or metadata (e.g., bytes written, rows affected).
    Data interface{}

    // Resource identifies the target resource (e.g., "disk1", "memory", "network", "db").
    Resource string
}

// String returns a simple summary of the outcome.
func (o Outcome) String() string {
    if o.Success {
        return "SUCCESS: " + o.Operation + " on " + o.Resource
    }
    return "FAILURE: " + o.Operation + " on " + o.Resource + " – " + o.Error.Error()
}

// --------------------------------------------------------------------
// OutcomeCollector – interface for recording outcomes.
// --------------------------------------------------------------------

// OutcomeCollector defines methods for recording and retrieving outcomes.
type OutcomeCollector interface {
    // Record saves an outcome.
    Record(outcome Outcome)

    // Outcomes returns a copy of all recorded outcomes.
    Outcomes() []Outcome

    // Clear removes all recorded outcomes.
    Clear()

    // Last returns the most recent outcome, or nil if none.
    Last() *Outcome
}

// --------------------------------------------------------------------
// MockOutcomeCollector – a test double that records calls and can be programmed.
// --------------------------------------------------------------------

// MockOutcomeCollector implements OutcomeCollector for unit tests.
type MockOutcomeCollector struct {
    mu          sync.Mutex
    outcomes    []Outcome
    recordFunc  func(Outcome) // optional custom behavior for Record
    recordCalls int
    clearCalls  int
}

// NewMockOutcomeCollector creates a new mock collector.
func NewMockOutcomeCollector() *MockOutcomeCollector {
    return &MockOutcomeCollector{}
}

// SetRecordFunc overrides the Record method with custom behavior.
func (m *MockOutcomeCollector) SetRecordFunc(fn func(Outcome)) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.recordFunc = fn
}

// Record records the outcome or calls the custom function.
func (m *MockOutcomeCollector) Record(outcome Outcome) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.recordCalls++
    if m.recordFunc != nil {
        m.recordFunc(outcome)
        return
    }
    m.outcomes = append(m.outcomes, outcome)
}

// Outcomes returns a copy of all recorded outcomes.
func (m *MockOutcomeCollector) Outcomes() []Outcome {
    m.mu.Lock()
    defer m.mu.Unlock()
    cp := make([]Outcome, len(m.outcomes))
    copy(cp, m.outcomes)
    return cp
}

// Clear removes all recorded outcomes.
func (m *MockOutcomeCollector) Clear() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.clearCalls++
    m.outcomes = nil
}

// Last returns the most recent outcome, or nil.
func (m *MockOutcomeCollector) Last() *Outcome {
    m.mu.Lock()
    defer m.mu.Unlock()
    if len(m.outcomes) == 0 {
        return nil
    }
    last := m.outcomes[len(m.outcomes)-1]
    return &last
}

// RecordCalls returns the number of times Record was called.
func (m *MockOutcomeCollector) RecordCalls() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.recordCalls
}

// ClearCalls returns the number of times Clear was called.
func (m *MockOutcomeCollector) ClearCalls() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.clearCalls
}

// Reset clears recorded outcomes and resets call counters.
func (m *MockOutcomeCollector) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.outcomes = nil
    m.recordCalls = 0
    m.clearCalls = 0
    m.recordFunc = nil
}

// --------------------------------------------------------------------
// InMemoryOutcomeCollector – a simple collector that stores outcomes in a slice.
// --------------------------------------------------------------------

// InMemoryOutcomeCollector implements OutcomeCollector with an in-memory slice.
type InMemoryOutcomeCollector struct {
    mu       sync.RWMutex
    outcomes []Outcome
}

// NewInMemoryOutcomeCollector creates a new empty collector.
func NewInMemoryOutcomeCollector() *InMemoryOutcomeCollector {
    return &InMemoryOutcomeCollector{}
}

// Record saves an outcome.
func (c *InMemoryOutcomeCollector) Record(outcome Outcome) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.outcomes = append(c.outcomes, outcome)
}

// Outcomes returns a copy of all recorded outcomes.
func (c *InMemoryOutcomeCollector) Outcomes() []Outcome {
    c.mu.RLock()
    defer c.mu.RUnlock()
    cp := make([]Outcome, len(c.outcomes))
    copy(cp, c.outcomes)
    return cp
}

// Clear removes all recorded outcomes.
func (c *InMemoryOutcomeCollector) Clear() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.outcomes = nil
}

// Last returns the most recent outcome, or nil.
func (c *InMemoryOutcomeCollector) Last() *Outcome {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if len(c.outcomes) == 0 {
        return nil
    }
    last := c.outcomes[len(c.outcomes)-1]
    return &last
}

// Len returns the number of outcomes recorded.
func (c *InMemoryOutcomeCollector) Len() int {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return len(c.outcomes)
}

// Filter returns outcomes matching a predicate.
func (c *InMemoryOutcomeCollector) Filter(pred func(Outcome) bool) []Outcome {
    c.mu.RLock()
    defer c.mu.RUnlock()
    var result []Outcome
    for _, o := range c.outcomes {
        if pred(o) {
            result = append(result, o)
        }
    }
    return result
}

// --------------------------------------------------------------------
// OutcomeConditioner – wraps an OutcomeCollector to inject delays and errors.
// --------------------------------------------------------------------

// OutcomeConditioner adds configurable delays and can simulate failures
// when recording outcomes. Useful for testing how components react to
// slow or failing outcome logging.
type OutcomeConditioner struct {
    mu             sync.Mutex
    collector      OutcomeCollector
    recordDelay    time.Duration
    recordErrors   map[int]error // call number -> error
    recordCalls    int
}

// NewOutcomeConditioner creates a conditioner around an existing OutcomeCollector.
func NewOutcomeConditioner(col OutcomeCollector) *OutcomeConditioner {
    return &OutcomeConditioner{
        collector:    col,
        recordErrors: make(map[int]error),
    }
}

// SetRecordDelay adds a fixed delay before every Record.
func (c *OutcomeConditioner) SetRecordDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.recordDelay = d
}

// InjectRecordError makes the nth call to Record return the given error.
func (c *OutcomeConditioner) InjectRecordError(callNumber int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.recordErrors[callNumber] = err
}

// Record implements OutcomeCollector with delays and error injection.
func (c *OutcomeConditioner) Record(outcome Outcome) error {
    c.mu.Lock()
    c.recordCalls++
    call := c.recordCalls
    delay := c.recordDelay
    err, ok := c.recordErrors[call]
    if ok {
        delete(c.recordErrors, call)
    }
    c.mu.Unlock()

    if ok {
        return err
    }
    if delay > 0 {
        time.Sleep(delay)
    }
    c.collector.Record(outcome)
    return nil
}

// Outcomes delegates to the underlying collector.
func (c *OutcomeConditioner) Outcomes() []Outcome {
    return c.collector.Outcomes()
}

// Clear delegates to the underlying collector.
func (c *OutcomeConditioner) Clear() {
    c.collector.Clear()
}

// Last delegates to the underlying collector.
func (c *OutcomeConditioner) Last() *Outcome {
    return c.collector.Last()
}

// --------------------------------------------------------------------
// OutcomeAssertions – helper functions for testing with OutcomeCollector.
// --------------------------------------------------------------------

// OutcomeAssertions provides convenience methods for verifying outcomes in tests.
type OutcomeAssertions struct {
    t       testing.TB // we cannot import "testing" here because it would create a cycle,
    // but in real usage you'd pass the *testing.T. Since this is testutils, it's acceptable
    // to import "testing". We'll do so, but note that in a real file you'd have `import "testing"`.
}

// NewOutcomeAssertions creates a new assertion helper.
func NewOutcomeAssertions(t testing.TB) *OutcomeAssertions {
    return &OutcomeAssertions{t: t}
}

// AssertSuccess asserts that the last outcome was successful.
func (a *OutcomeAssertions) AssertSuccess(col OutcomeCollector) {
    last := col.Last()
    if last == nil {
        a.t.Error("no outcome recorded")
        return
    }
    if !last.Success {
        a.t.Errorf("expected success, got failure: %v", last.Error)
    }
}

// AssertFailure asserts that the last outcome was a failure with a specific error message.
func (a *OutcomeAssertions) AssertFailure(col OutcomeCollector, errMsg string) {
    last := col.Last()
    if last == nil {
        a.t.Error("no outcome recorded")
        return
    }
    if last.Success {
        a.t.Error("expected failure, got success")
        return
    }
    if last.Error == nil || last.Error.Error() != errMsg {
        a.t.Errorf("expected error %q, got %v", errMsg, last.Error)
    }
}

// AssertCount asserts the total number of outcomes.
func (a *OutcomeAssertions) AssertCount(col OutcomeCollector, expected int) {
    outcomes := col.Outcomes()
    if len(outcomes) != expected {
        a.t.Errorf("expected %d outcomes, got %d", expected, len(outcomes))
    }
}

// --------------------------------------------------------------------
// To avoid an import cycle in this generated file, we comment out the testing import.
// In a real implementation, you would include `import "testing"`.
// For now, we'll leave the type without the concrete t.
// --------------------------------------------------------------------
type testingT interface {
    Error(args ...interface{})
    Errorf(format string, args ...interface{})
}