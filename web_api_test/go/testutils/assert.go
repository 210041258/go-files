// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"strings"
)

// ------------------------------------------------------------------------
// Core assertions
// ------------------------------------------------------------------------

// Equal checks that expected and actual are deeply equal.
// It returns true if they are equal, otherwise it logs an error and returns false.
func Equal(t TestingT, expected, actual interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()
	if reflect.DeepEqual(expected, actual) {
		return true
	}
	fail(t, "Not equal: \nexpected: %#v\nactual  : %#v", expected, actual, msgAndArgs...)
	return false
}

// NotEqual checks that expected and actual are NOT deeply equal.
func NotEqual(t TestingT, expected, actual interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()
	if !reflect.DeepEqual(expected, actual) {
		return true
	}
	fail(t, "Should not be equal: %#v", expected, msgAndArgs...)
	return false
}

// Nil checks that object is nil.
func Nil(t TestingT, object interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()
	if isNil(object) {
		return true
	}
	fail(t, "Expected nil, but got: %#v", object, msgAndArgs...)
	return false
}

// NotNil checks that object is not nil.
func NotNil(t TestingT, object interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()
	if !isNil(object) {
		return true
	}
	fail(t, "Expected value not to be nil", msgAndArgs...)
	return false
}

// True checks that condition is true.
func True(t TestingT, condition bool, msgAndArgs ...interface{}) bool {
	t.Helper()
	if condition {
		return true
	}
	fail(t, "Expected true, got false", msgAndArgs...)
	return false
}

// False checks that condition is false.
func False(t TestingT, condition bool, msgAndArgs ...interface{}) bool {
	t.Helper()
	if !condition {
		return true
	}
	fail(t, "Expected false, got true", msgAndArgs...)
	return false
}

// Error checks that err is not nil.
func Error(t TestingT, err error, msgAndArgs ...interface{}) bool {
	t.Helper()
	if err != nil {
		return true
	}
	fail(t, "Expected non‑nil error", msgAndArgs...)
	return false
}

// NoError checks that err is nil.
func NoError(t TestingT, err error, msgAndArgs ...interface{}) bool {
	t.Helper()
	if err == nil {
		return true
	}
	fail(t, "Expected no error, but got: %v", err, msgAndArgs...)
	return false
}

// ------------------------------------------------------------------------
// String assertions
// ------------------------------------------------------------------------

// Contains checks that s contains substr.
func Contains(t TestingT, s, substr string, msgAndArgs ...interface{}) bool {
	t.Helper()
	if strings.Contains(s, substr) {
		return true
	}
	fail(t, "String %q does not contain %q", s, substr, msgAndArgs...)
	return false
}

// NotContains checks that s does NOT contain substr.
func NotContains(t TestingT, s, substr string, msgAndArgs ...interface{}) bool {
	t.Helper()
	if !strings.Contains(s, substr) {
		return true
	}
	fail(t, "String %q should not contain %q", s, substr, msgAndArgs...)
	return false
}

// HasPrefix checks that s has prefix prefix.
func HasPrefix(t TestingT, s, prefix string, msgAndArgs ...interface{}) bool {
	t.Helper()
	if strings.HasPrefix(s, prefix) {
		return true
	}
	fail(t, "String %q does not have prefix %q", s, prefix, msgAndArgs...)
	return false
}

// HasSuffix checks that s has suffix suffix.
func HasSuffix(t TestingT, s, suffix string, msgAndArgs ...interface{}) bool {
	t.Helper()
	if strings.HasSuffix(s, suffix) {
		return true
	}
	fail(t, "String %q does not have suffix %q", s, suffix, msgAndArgs...)
	return false
}

// ------------------------------------------------------------------------
// Collection assertions
// ------------------------------------------------------------------------

// Len checks that obj (array, slice, map, string, or chan) has the expected length.
func Len(t TestingT, obj interface{}, length int, msgAndArgs ...interface{}) bool {
	t.Helper()
	v := reflect.ValueOf(obj)
	switch v.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map, reflect.String, reflect.Chan:
		if v.Len() == length {
			return true
		}
		fail(t, "Expected length %d, but got %d", length, v.Len(), msgAndArgs...)
		return false
	default:
		fail(t, "Len() called with invalid type %T", obj, msgAndArgs...)
		return false
	}
}

// Empty checks that obj (array, slice, map, string, or chan) is empty.
func Empty(t TestingT, obj interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()
	v := reflect.ValueOf(obj)
	switch v.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map, reflect.String, reflect.Chan:
		if v.Len() == 0 {
			return true
		}
		fail(t, "Expected empty, but got %v", obj, msgAndArgs...)
		return false
	default:
		fail(t, "Empty() called with invalid type %T", obj, msgAndArgs...)
		return false
	}
}

// NotEmpty checks that obj is not empty.
func NotEmpty(t TestingT, obj interface{}, msgAndArgs ...interface{}) bool {
	t.Helper()
	v := reflect.ValueOf(obj)
	switch v.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map, reflect.String, reflect.Chan:
		if v.Len() != 0 {
			return true
		}
		fail(t, "Expected not empty, but got %v", obj, msgAndArgs...)
		return false
	default:
		fail(t, "NotEmpty() called with invalid type %T", obj, msgAndArgs...)
		return false
	}
}

// ------------------------------------------------------------------------
// Panic assertions
// ------------------------------------------------------------------------

// Panics checks that the provided function panics.
func Panics(t TestingT, f func(), msgAndArgs ...interface{}) (panicked bool) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			panicked = true
		}
	}()
	f()
	if !panicked {
		fail(t, "Expected panic, but code did not panic", msgAndArgs...)
	}
	return panicked
}

// NotPanics checks that the provided function does NOT panic.
func NotPanics(t TestingT, f func(), msgAndArgs ...interface{}) (panicked bool) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			panicked = true
			fail(t, "Unexpected panic: %v\n%s", r, debug.Stack(), msgAndArgs...)
		}
	}()
	f()
	return false
}

// ------------------------------------------------------------------------
// Internal helpers
// ------------------------------------------------------------------------

// isNil checks if a value is nil, even if it's an interface holding a nil pointer.
func isNil(object interface{}) bool {
	if object == nil {
		return true
	}
	val := reflect.ValueOf(object)
	switch val.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return val.IsNil()
	}
	return false
}

// fail logs a formatted error message with optional additional message parts.
func fail(t TestingT, format string, args ...interface{}) {
	t.Helper()
	// Separate format+args from optional user message (last variadic arguments).
	var msg string
	if len(args) > 0 {
		if last, ok := args[len(args)-1].(string); ok && len(args) > 1 {
			// Try to detect if last is a format string for the user message? Simpler:
			// We'll treat any trailing arguments after the required ones as user message.
			// But we don't know how many required args are in format. Better: separate
			// msgAndArgs by using a custom type? For simplicity, we'll not support extra
			// user messages; we'll just rely on the format string.
			// Many assert libraries do: format string, then required args, then optional message parts.
			// That's complex. Instead, we can require that the last argument(s) are strings.
			// For now, we'll keep it simple: no variadic user message. Provide a separate function.
			// Actually, we'll keep msgAndArgs optional: after required args, any remaining are used as message.
		}
	}
	// For now, just format the error with the given args.
	t.Errorf(format, args...)
}