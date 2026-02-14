// Package testutils provides mock and simple in‑memory execution tracers
// for testing storage, memory, network, and database operations. It allows
// tests to record, inspect, and control the execution of operations.
package testutils

import (
    "context"
    "sync"
    "time"
)

// --------------------------------------------------------------------
// Executor – interface for executing operations.
// --------------------------------------------------------------------

// Executor defines a method to run an operation by name.
type Executor interface {
    // Execute runs the named operation with the given context and arguments,
    // and returns an Outcome describing the result.
    Execute(ctx context.Context, operation string, args ...interface{}) Outcome
}

// --------------------------------------------------------------------
// MockExecutor – a test double that records calls and can be programmed.
// --------------------------------------------------------------------

// ExecCall records a single Execute call.
type ExecCall struct {
    Operation string
    Args      []interface{}
    Timestamp time.Time
}

// MockExecutor implements Executor for unit tests.
type MockExecutor struct {
    mu           sync.Mutex
    executeFunc  func(ctx context.Context, operation string, args ...interface{}) Outcome
    calls        []ExecCall
    callCount    int
    returnValues map[int]Outcome // per‑call return value (1‑based)
}

// NewMockExecutor creates a new mock executor.
func NewMockExecutor() *MockExecutor {
    return &MockExecutor{
        returnValues: make(map[int]Outcome),
    }
}

// SetExecuteFunc overrides the Execute method with custom behavior.
func (m *MockExecutor) SetExecuteFunc(fn func(ctx context.Context, operation string, args ...interface{}) Outcome) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.executeFunc = fn
}

// InjectReturnValue makes the nth call to Execute return the given Outcome.
func (m *MockExecutor) InjectReturnValue(callNumber int, outcome Outcome) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.returnValues[callNumber] = outcome
}

// Execute records the call and returns the programmed outcome or calls the custom function.
func (m *MockExecutor) Execute(ctx context.Context, operation string, args ...interface{}) Outcome {
    m.mu.Lock()
    m.callCount++
    call := m.callCount
    m.calls = append(m.calls, ExecCall{
        Operation: operation,
        Args:      append([]interface{}{}, args...),
        Timestamp: time.Now(),
    })
    if ret, ok := m.returnValues[call]; ok {
        delete(m.returnValues, call)
        m.mu.Unlock()
        return ret
    }
    if m.executeFunc != nil {
        fn := m.executeFunc
        m.mu.Unlock()
        return fn(ctx, operation, args...)
    }
    m.mu.Unlock()
    // Default: return successful outcome with no data.
    return Outcome{
        Operation: operation,
        Success:   true,
        Timestamp: time.Now(),
        Resource:  "mock",
    }
}

// Calls returns a copy of the recorded calls.
func (m *MockExecutor) Calls() []ExecCall {
    m.mu.Lock()
    defer m.mu.Unlock()
    cp := make([]ExecCall, len(m.calls))
    copy(cp, m.calls)
    return cp
}

// Reset clears recorded calls and injected return values.
func (m *MockExecutor) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.calls = nil
    m.callCount = 0
    m.returnValues = make(map[int]Outcome)
    m.executeFunc = nil
}

// --------------------------------------------------------------------
// InMemoryExecutor – simulates execution with configurable latency and errors.
// --------------------------------------------------------------------

// InMemoryExecutor implements Executor with programmable behavior.
// It can simulate success/failure, add fixed latency, and record outcomes.
type InMemoryExecutor struct {
    mu       sync.Mutex
    latency  time.Duration            // fixed delay before each execution
    failOn   map[string]error         // operation name → error (always fail that operation)
    failFunc func(operation string, args ...interface{}) error // dynamic failure decision
    record   bool                      // whether to record outcomes
    outcomes []Outcome                  // recorded outcomes (if record=true)
}

// NewInMemoryExecutor creates a new executor with no delays and all operations succeeding.
func NewInMemoryExecutor() *InMemoryExecutor {
    return &InMemoryExecutor{
        failOn: make(map[string]error),
    }
}

// SetLatency adds a fixed delay before every Execute.
func (e *InMemoryExecutor) SetLatency(d time.Duration) {
    e.mu.Lock()
    defer e.mu.Unlock()
    e.latency = d
}

// FailOperation makes the named operation return the given error.
func (e *InMemoryExecutor) FailOperation(operation string, err error) {
    e.mu.Lock()
    defer e.mu.Unlock()
    e.failOn[operation] = err
}

// SetFailFunc sets a function that decides whether an operation fails.
// The function receives the operation name and arguments; if it returns a non‑nil error,
// the operation fails with that error.
func (e *InMemoryExecutor) SetFailFunc(fn func(operation string, args ...interface{}) error) {
    e.mu.Lock()
    defer e.mu.Unlock()
    e.failFunc = fn
}

// EnableRecording turns on outcome recording. Any previously recorded outcomes are cleared.
func (e *InMemoryExecutor) EnableRecording() {
    e.mu.Lock()
    defer e.mu.Unlock()
    e.record = true
    e.outcomes = nil
}

// Outcomes returns a copy of recorded outcomes (if recording was enabled).
func (e *InMemoryExecutor) Outcomes() []Outcome {
    e.mu.Lock()
    defer e.mu.Unlock()
    cp := make([]Outcome, len(e.outcomes))
    copy(cp, e.outcomes)
    return cp
}

// Execute runs the operation with simulated latency and failure.
func (e *InMemoryExecutor) Execute(ctx context.Context, operation string, args ...interface{}) Outcome {
    // Check for static failure first.
    e.mu.Lock()
    latency := e.latency
    if err, ok := e.failOn[operation]; ok {
        e.mu.Unlock()
        if latency > 0 {
            time.Sleep(latency)
        }
        outcome := Outcome{
            Operation: operation,
            Success:   false,
            Error:     err,
            Duration:  latency,
            Timestamp: time.Now(),
            Resource:  "inmem",
        }
        e.recordOutcome(outcome)
        return outcome
    }
    if e.failFunc != nil {
        err := e.failFunc(operation, args...)
        if err != nil {
            e.mu.Unlock()
            if latency > 0 {
                time.Sleep(latency)
            }
            outcome := Outcome{
                Operation: operation,
                Success:   false,
                Error:     err,
                Duration:  latency,
                Timestamp: time.Now(),
                Resource:  "inmem",
            }
            e.recordOutcome(outcome)
            return outcome
        }
    }
    e.mu.Unlock()

    start := time.Now()
    if latency > 0 {
        time.Sleep(latency)
    }
    outcome := Outcome{
        Operation: operation,
        Success:   true,
        Duration:  time.Since(start),
        Timestamp: time.Now(),
        Resource:  "inmem",
        Data:      args, // optionally store arguments as data
    }
    e.recordOutcome(outcome)
    return outcome
}

func (e *InMemoryExecutor) recordOutcome(outcome Outcome) {
    e.mu.Lock()
    defer e.mu.Unlock()
    if e.record {
        e.outcomes = append(e.outcomes, outcome)
    }
}

// --------------------------------------------------------------------
// Tracer – wraps an Executor and records each execution using an OutcomeCollector.
// --------------------------------------------------------------------

// Tracer implements Executor by delegating to another Executor and
// recording the outcome using an OutcomeCollector.
type Tracer struct {
    executor  Executor
    collector OutcomeCollector
}

// NewTracer creates a tracer that records outcomes to the given collector.
func NewTracer(executor Executor, collector OutcomeCollector) *Tracer {
    return &Tracer{
        executor:  executor,
        collector: collector,
    }
}

// Execute runs the operation and records the outcome.
func (t *Tracer) Execute(ctx context.Context, operation string, args ...interface{}) Outcome {
    outcome := t.executor.Execute(ctx, operation, args...)
    t.collector.Record(outcome)
    return outcome
}

// --------------------------------------------------------------------
// ExecutionAssertions – helper functions for testing with Executor.
// --------------------------------------------------------------------

// testingT is a minimal interface that matches *testing.T.
type testingT interface {
    Error(args ...interface{})
    Errorf(format string, args ...interface{})
}

// ExecutionAssertions provides convenience methods for verifying executor behavior.
type ExecutionAssertions struct {
    t testingT
}

// NewExecutionAssertions creates a new assertion helper.
func NewExecutionAssertions(t testingT) *ExecutionAssertions {
    return &ExecutionAssertions{t: t}
}

// AssertCalled asserts that the executor was called with the given operation.
func (a *ExecutionAssertions) AssertCalled(exec *MockExecutor, operation string) {
    for _, call := range exec.Calls() {
        if call.Operation == operation {
            return
        }
    }
    a.t.Errorf("expected executor to be called with operation %q, but it wasn't", operation)
}

// AssertNotCalled asserts that the executor was not called with the given operation.
func (a *ExecutionAssertions) AssertNotCalled(exec *MockExecutor, operation string) {
    for _, call := range exec.Calls() {
        if call.Operation == operation {
            a.t.Errorf("expected executor not to be called with operation %q, but it was", operation)
            return
        }
    }
}

// AssertCallCount asserts the total number of Execute calls.
func (a *ExecutionAssertions) AssertCallCount(exec *MockExecutor, expected int) {
    if len(exec.Calls()) != expected {
        a.t.Errorf("expected %d Execute calls, got %d", expected, len(exec.Calls()))
    }
}

// AssertOutcomeSuccess asserts that the outcome from an execution was successful.
func (a *ExecutionAssertions) AssertOutcomeSuccess(outcome Outcome) {
    if !outcome.Success {
        a.t.Errorf("expected success, got failure: %v", outcome.Error)
    }
}

// AssertOutcomeFailure asserts that the outcome was a failure with a specific error.
func (a *ExecutionAssertions) AssertOutcomeFailure(outcome Outcome, errMsg string) {
    if outcome.Success {
        a.t.Error("expected failure, got success")
        return
    }
    if outcome.Error == nil || outcome.Error.Error() != errMsg {
        a.t.Errorf("expected error %q, got %v", errMsg, outcome.Error)
    }
}