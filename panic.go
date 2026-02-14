// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
)

// ------------------------------------------------------------------------
// Panic catching
// ------------------------------------------------------------------------

// PanicInfo holds information about a caught panic.
type PanicInfo struct {
	// Value is the argument passed to panic.
	Value interface{}
	// Stack is the stack trace at the point of panic.
	Stack []byte
}

// CatchPanic executes the given function and recovers from any panic.
// If the function panics, it returns a non‑nil PanicInfo and true.
// If the function does not panic, it returns nil and false.
func CatchPanic(f func()) (info *PanicInfo, panicked bool) {
	var (
		value   interface{}
		stack   []byte
		recover bool
	)
	defer func() {
		if r := recover(); r != nil {
			value = r
			stack = debugStack()
			recover = true
		}
	}()
	f()
	if recover {
		return &PanicInfo{Value: value, Stack: stack}, true
	}
	return nil, false
}

// debugStack returns the current stack trace, skipping the runtime package.
func debugStack() []byte {
	buf := make([]byte, 1024)
	for {
		n := runtime.Stack(buf, false)
		if n < len(buf) {
			return buf[:n]
		}
		buf = make([]byte, len(buf)*2)
	}
}

// ------------------------------------------------------------------------
// Assertions about panics
// ------------------------------------------------------------------------

// PanicWithValue asserts that f panics and the panic value equals expected.
// It returns true if the assertion passes, otherwise it logs an error and returns false.
func PanicWithValue(t TestingT, expected interface{}, f func(), msgAndArgs ...interface{}) bool {
	t.Helper()
	info, panicked := CatchPanic(f)
	if !panicked {
		fail(t, "Expected panic with value %#v, but code did not panic", expected, msgAndArgs...)
		return false
	}
	if !reflect.DeepEqual(info.Value, expected) {
		fail(t, "Panic value mismatch:\nexpected: %#v\nactual  : %#v\n%s",
			expected, info.Value, info.Stack, msgAndArgs...)
		return false
	}
	return true
}

// PanicWithType asserts that f panics and the panic value is of the same type as typ.
// typ should be a value of the expected type (e.g., (*MyError)(nil)).
// It returns true if the assertion passes.
func PanicWithType(t TestingT, typ interface{}, f func(), msgAndArgs ...interface{}) bool {
	t.Helper()
	info, panicked := CatchPanic(f)
	if !panicked {
		fail(t, "Expected panic of type %T, but code did not panic", typ, msgAndArgs...)
		return false
	}
	// Get type of typ (which may be a nil pointer to a type)
	typType := reflect.TypeOf(typ)
	// If typ is a nil pointer, we want the type it points to.
	if typType != nil && typType.Kind() == reflect.Ptr {
		// For checking type, we can use Elem() if the value is also a pointer.
		// But simpler: just compare the types using reflect.TypeOf on the actual value.
		// We want the actual value's type to be assignable to the type of typ (if typ is a pointer).
		// Actually, we want the type to match exactly? Usually we want to check that the panic value
		// implements or is of the same type as typ. Let's use IsType.
	}
	// Use reflect.TypeOf on the actual panic value.
	valType := reflect.TypeOf(info.Value)
	if typType == nil {
		// typ is nil, we just want the panic value to be nil? That's handled by PanicWithValue.
		if info.Value != nil {
			fail(t, "Expected panic nil, got %T (%#v)", info.Value, info.Value, msgAndArgs...)
			return false
		}
		return true
	}
	// If typType is a pointer, we might allow the value to be a pointer of that type.
	// For simplicity, we'll use reflect.TypeOf on typ and compare directly.
	if valType != typType {
		fail(t, "Panic type mismatch:\nexpected: %v\nactual  : %v\n%s",
			typType, valType, info.Stack, msgAndArgs...)
		return false
	}
	return true
}

// PanicWithMessage asserts that f panics and the panic value (as a string or error)
// contains the substring substr.
func PanicWithMessage(t TestingT, substr string, f func(), msgAndArgs ...interface{}) bool {
	t.Helper()
	info, panicked := CatchPanic(f)
	if !panicked {
		fail(t, "Expected panic with message containing %q, but code did not panic", substr, msgAndArgs...)
		return false
	}
	var msg string
	switch v := info.Value.(type) {
	case string:
		msg = v
	case error:
		msg = v.Error()
	default:
		msg = fmt.Sprint(v)
	}
	if !strings.Contains(msg, substr) {
		fail(t, "Panic message does not contain %q:\nactual: %q\n%s",
			substr, msg, info.Stack, msgAndArgs...)
		return false
	}
	return true
}

// PanicStackTraceContains asserts that f panics and the stack trace contains the given substring.
func PanicStackTraceContains(t TestingT, substr string, f func(), msgAndArgs ...interface{}) bool {
	t.Helper()
	info, panicked := CatchPanic(f)
	if !panicked {
		fail(t, "Expected panic with stack trace containing %q, but code did not panic", substr, msgAndArgs...)
		return false
	}
	if !strings.Contains(string(info.Stack), substr) {
		fail(t, "Panic stack trace does not contain %q:\n%s", substr, info.Stack, msgAndArgs...)
		return false
	}
	return true
}

// NoPanic asserts that f does NOT panic.
// It is similar to NotPanics in assert.go but provides the panic info if it does panic.
func NoPanic(t TestingT, f func(), msgAndArgs ...interface{}) bool {
	t.Helper()
	info, panicked := CatchPanic(f)
	if panicked {
		fail(t, "Unexpected panic: %#v\n%s", info.Value, info.Stack, msgAndArgs...)
		return false
	}
	return true
}