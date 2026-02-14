// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// ------------------------------------------------------------------------
// Mock – base struct for building mocks
// ------------------------------------------------------------------------

// Mock is the core struct that can be embedded in your own mock types.
// It tracks expected calls and actual calls, and provides assertion helpers.
type Mock struct {
	mu          sync.Mutex
	expected    []*ExpectedCall
	actual      []*ActualCall
	strictOrder bool
}

// ExpectedCall represents a single expected method call.
type ExpectedCall struct {
	method      string
	args        []interface{}
	returns     []interface{}
	times       int
	callCount   int
	optional    bool
}

// ActualCall represents a single actual method call that was recorded.
type ActualCall struct {
	Method string
	Args   []interface{}
}

// ------------------------------------------------------------------------
// Setting expectations
// ------------------------------------------------------------------------

// On starts defining an expectation for a method call.
// It returns an ExpectedCall that can be further configured.
func (m *Mock) On(method string, args ...interface{}) *ExpectedCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	ec := &ExpectedCall{
		method: method,
		args:   args,
		times:  1, // default to exactly one call
	}
	m.expected = append(m.expected, ec)
	return ec
}

// Return sets the return values for the expected call.
func (ec *ExpectedCall) Return(returns ...interface{}) *ExpectedCall {
	ec.returns = returns
	return ec
}

// Times sets the exact number of times this call is expected.
// If times <= 0, the call is considered optional (any number, including zero).
func (ec *ExpectedCall) Times(n int) *ExpectedCall {
	if n < 0 {
		n = 0
	}
	ec.times = n
	ec.optional = (n == 0)
	return ec
}

// Optional marks the call as optional – it may or may not happen.
func (ec *ExpectedCall) Optional() *ExpectedCall {
	ec.times = 0
	ec.optional = true
	return ec
}

// Once is a shortcut for Times(1).
func (ec *ExpectedCall) Once() *ExpectedCall {
	return ec.Times(1)
}

// Twice is a shortcut for Times(2).
func (ec *ExpectedCall) Twice() *ExpectedCall {
	return ec.Times(2)
}

// ------------------------------------------------------------------------
// Recording actual calls
// ------------------------------------------------------------------------

// Called records that a method was invoked with the given arguments.
// It returns the pre‑configured return values, or nil if no expectation matches.
// If multiple expectations match, the first one with remaining calls is used.
// It panics if a call is unexpected (strict mode) or if no matching expectation is found.
func (m *Mock) Called(method string, args ...interface{}) []interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the actual call.
	m.actual = append(m.actual, &ActualCall{
		Method: method,
		Args:   args,
	})

	// Find a matching expected call that still has remaining calls.
	var match *ExpectedCall
	for _, ec := range m.expected {
		if ec.method == method && m.argsMatch(ec.args, args) {
			if ec.times == 0 || ec.callCount < ec.times {
				match = ec
				break
			}
		}
	}

	if match == nil {
		// No matching expectation.
		panic(fmt.Sprintf("testutils.Mock: unexpected call to %s with args %v", method, args))
	}

	match.callCount++

	// If this call exceeded the expected count, panic unless it's optional.
	if match.times > 0 && match.callCount > match.times {
		panic(fmt.Sprintf("testutils.Mock: too many calls to %s (expected %d, got %d)",
			method, match.times, match.callCount))
	}

	return match.returns
}

// argsMatch checks whether the actual arguments match the expected arguments.
// It uses reflect.DeepEqual, except that if the expected argument is of type
// func(interface{}) bool, it calls that function to validate the actual argument.
// This allows for custom matchers.
func (m *Mock) argsMatch(expected, actual []interface{}) bool {
	if len(expected) != len(actual) {
		return false
	}
	for i, exp := range expected {
		act := actual[i]
		// If the expected argument is a function that can validate, call it.
		if fn, ok := exp.(func(interface{}) bool); ok {
			if !fn(act) {
				return false
			}
		} else if !reflect.DeepEqual(exp, act) {
			return false
		}
	}
	return true
}

// ------------------------------------------------------------------------
// Assertions
// ------------------------------------------------------------------------

// AssertExpectations verifies that all expected calls (with times>0) were made
// the required number of times. It returns an error if any expectation is not met.
func (m *Mock) AssertExpectations() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []string
	for _, ec := range m.expected {
		if ec.times > 0 && ec.callCount != ec.times {
			errs = append(errs, fmt.Sprintf("expected %d call(s) to %s with args %v, but got %d",
				ec.times, ec.method, ec.args, ec.callCount))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("mock expectations failed:\n%s", strings.Join(errs, "\n"))
	}
	return nil
}

// AssertCalled verifies that a specific method was called at least once.
func (m *Mock) AssertCalled(t TestingT, method string, args ...interface{}) bool {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, ac := range m.actual {
		if ac.Method == method && m.argsMatch(args, ac.Args) {
			return true
		}
	}
	t.Errorf("expected call to %s with args %v, but it was not called", method, args)
	return false
}

// AssertNotCalled verifies that a specific method was not called with the given args.
func (m *Mock) AssertNotCalled(t TestingT, method string, args ...interface{}) bool {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, ac := range m.actual {
		if ac.Method == method && m.argsMatch(args, ac.Args) {
			t.Errorf("unexpected call to %s with args %v", method, args)
			return false
		}
	}
	return true
}

// ------------------------------------------------------------------------
// Matchers (common argument matchers)
// ------------------------------------------------------------------------

// Anything is a matcher that matches any value of any type.
func Anything() func(interface{}) bool {
	return func(interface{}) bool { return true }
}

// AnyInt matches any int (of any size).
func AnyInt() func(interface{}) bool {
	return func(v interface{}) bool {
		switch v.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return true
		default:
			return false
		}
	}
}

// AnyString matches any string.
func AnyString() func(interface{}) bool {
	return func(v interface{}) bool {
		_, ok := v.(string)
		return ok
	}
}

// AnyBool matches any bool.
func AnyBool() func(interface{}) bool {
	return func(v interface{}) bool {
		_, ok := v.(bool)
		return ok
	}
}

// NotNil matches any non‑nil value.
func NotNil() func(interface{}) bool {
	return func(v interface{}) bool {
		return v != nil
	}
}

// ------------------------------------------------------------------------
// Helper interface for testing.TB compatibility
// ------------------------------------------------------------------------

// TestingT is a minimal interface that matches *testing.T and *testing.B.
type TestingT interface {
	Helper()
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
}