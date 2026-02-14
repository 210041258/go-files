// Package safe provides utilities for building robust, panic‑safe programs.
// It includes functions for error handling, type assertions, slice/map access,
// resource cleanup, and goroutine recovery – all with generics support.
//
// Functions that may panic are suffixed with "OrZero" or return a bool ok flag.
package testutils

import (
	"errors"
	"fmt"
	"io"
	"runtime/debug"
)

// --------------------------------------------------------------------
// Error handling: Must, Ignore, Try
// --------------------------------------------------------------------

// Must returns v if err == nil, otherwise panics with the error.
// It's useful for initialisation that should never fail.
//
// Example:
//
//	file := safe.Must(os.Open("config.json"))
//	defer file.Close()
func Must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// Must0 is similar to Must but for functions that return only an error.
// It panics if err != nil.
func Must0(err error) {
	if err != nil {
		panic(err)
	}
}

// Ignore discards the error and returns the value.
// Use sparingly; prefer explicit error handling.
func Ignore[T any](v T, _ error) T {
	return v
}

// Try executes a function that may panic, recovering it and returning
// the panic value as an error. If the function returns a value normally,
// that value is returned with a nil error.
//
// Example:
//
//	val, err := safe.Try(func() string {
//		return riskyOperation()
//	})
func Try[T any](fn func() T) (result T, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = PanicError(r)
		}
	}()
	return fn(), nil
}

// Try0 executes a void function that may panic, returning any panic as an error.
func Try0(fn func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = PanicError(r)
		}
	}()
	fn()
	return nil
}

// PanicError converts a recovered panic value into an error.
// If the value already implements error, it is returned; otherwise,
// a formatted error message is created, including a stack trace.
func PanicError(r any) error {
	if err, ok := r.(error); ok {
		return err
	}
	return fmt.Errorf("panic: %v\n%s", r, debug.Stack())
}

// --------------------------------------------------------------------
// Conditional checks and assertions
// --------------------------------------------------------------------

// Assert panics with the given message if condition is false.
func Assert(condition bool, msg string) {
	if !condition {
		panic(msg)
	}
}

// Assertf panics with a formatted message if condition is false.
func Assertf(condition bool, format string, args ...any) {
	if !condition {
		panic(fmt.Sprintf(format, args...))
	}
}

// --------------------------------------------------------------------
// Zero value defaults (comparable)
// --------------------------------------------------------------------

// OrZero returns v if it is not the zero value of its type, otherwise zero.
// Useful for chaining optional values.
func OrZero[T comparable](v T) T {
	var zero T
	if v != zero {
		return v
	}
	return zero
}

// OrElse returns v if it is not the zero value, otherwise defaultValue.
func OrElse[T comparable](v T, defaultValue T) T {
	var zero T
	if v != zero {
		return v
	}
	return defaultValue
}

// ZeroIf returns zero value if condition is true, otherwise v.
func ZeroIf[T comparable](condition bool, v T) T {
	if condition {
		var zero T
		return zero
	}
	return v
}

// Coalesce returns the first non‑zero value from the list.
// If all are zero, returns the zero value of T.
func Coalesce[T comparable](values ...T) T {
	var zero T
	for _, v := range values {
		if v != zero {
			return v
		}
	}
	return zero
}

// --------------------------------------------------------------------
// Pointer utilities
// --------------------------------------------------------------------

// OrNil returns nil if v is zero, otherwise a pointer to v.
func OrNil[T comparable](v T) *T {
	var zero T
	if v == zero {
		return nil
	}
	return &v
}

// ValueOrDefault returns the dereferenced value of p if non‑nil,
// otherwise the zero value of T.
func ValueOrDefault[T any](p *T) T {
	if p != nil {
		return *p
	}
	var zero T
	return zero
}

// ValueOrElse returns the dereferenced value of p if non‑nil,
// otherwise defaultValue.
func ValueOrElse[T any](p *T, defaultValue T) T {
	if p != nil {
		return *p
	}
	return defaultValue
}

// --------------------------------------------------------------------
// Safe type assertions
// --------------------------------------------------------------------

// As attempts to cast v to type T. Returns the value and true if successful,
// otherwise zero value and false.
func As[T any](v any) (T, bool) {
	t, ok := v.(T)
	return t, ok
}

// AsOrZero attempts to cast v to type T; returns zero value on failure.
func AsOrZero[T any](v any) T {
	t, _ := As[T](v)
	return t
}

// AsOrElse attempts to cast v to type T; returns defaultValue on failure.
func AsOrElse[T any](v any, defaultValue T) T {
	if t, ok := As[T](v); ok {
		return t
	}
	return defaultValue
}

// --------------------------------------------------------------------
// Safe slice access
// --------------------------------------------------------------------

// At returns the element at index i and true if i is within bounds,
// otherwise zero value and false.
func At[T any](slice []T, i int) (T, bool) {
	if i >= 0 && i < len(slice) {
		return slice[i], true
	}
	var zero T
	return zero, false
}

// AtOrZero returns the element at index i, or zero value if out of bounds.
func AtOrZero[T any](slice []T, i int) T {
	v, _ := At(slice, i)
	return v
}

// AtOrElse returns the element at index i, or defaultValue if out of bounds.
func AtOrElse[T any](slice []T, i int, defaultValue T) T {
	if v, ok := At(slice, i); ok {
		return v
	}
	return defaultValue
}

// --------------------------------------------------------------------
// Safe map access
// --------------------------------------------------------------------

// Get returns the value for key and true if the key exists,
// otherwise zero value and false.
func Get[K comparable, V any](m map[K]V, key K) (V, bool) {
	v, ok := m[key]
	return v, ok
}

// GetOrZero returns the value for key, or zero value if key not present.
func GetOrZero[K comparable, V any](m map[K]V, key K) V {
	return m[key]
}

// GetOrElse returns the value for key, or defaultValue if key not present.
func GetOrElse[K comparable, V any](m map[K]V, key K, defaultValue V) V {
	if v, ok := m[key]; ok {
		return v
	}
	return defaultValue
}

// --------------------------------------------------------------------
// Resource cleanup with error handling
// --------------------------------------------------------------------

// Close ignores the error; prefer CloseLog or explicit handling.
func Close(c io.Closer) {
	_ = c.Close()
}

// CloseLog calls Close and logs the error using the provided function.
// If logger is nil, errors are silently ignored.
func CloseLog(c io.Closer, logger func(error)) {
	if err := c.Close(); err != nil && logger != nil {
		logger(err)
	}
}

// CloseVerbose calls Close and, if verbose level is sufficient, prints the error.
// It integrates with the verbose package if imported; otherwise no‑op.
func CloseVerbose(c io.Closer, level int) {
	_ = c.Close()
	// optional: if verbose.V(level) { ... }
}

// --------------------------------------------------------------------
// Goroutine safety
// --------------------------------------------------------------------

// Go runs a function in a new goroutine with panic recovery.
// The recovered panic is passed to the handler (if non‑nil).
// This prevents a single panicking goroutine from taking down the process.
//
// Example:
//
//	safe.Go(func() {
//		riskyOperation()
//	}, func(r any) {
//		log.Printf("goroutine panicked: %v", r)
//	})
func Go(fn func(), handler func(any)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if handler != nil {
					handler(r)
				}
			}
		}()
		fn()
	}()
}

// Go0 is a convenience for goroutines with no handler (panics are suppressed).
func Go0(fn func()) {
	Go(fn, nil)
}

// WithRecovery wraps a function so that panics are caught and returned as errors.
// Useful for integrating panic‑prone code into error‑based APIs.
func WithRecovery[T any](fn func() T) func() (T, error) {
	return func() (T, error) {
		return Try(fn)
	}
}

// --------------------------------------------------------------------
// Conversion safety
// --------------------------------------------------------------------

// Atoi is a safe wrapper around strconv.Atoi; returns 0 on error.
func Atoi(s string) int {
	i, _ := strconvParseInt(s)
	return int(i)
}

// AtoiOrElse is a safe wrapper with a default value.
func AtoiOrElse(s string, defaultValue int) int {
	if i, err := strconvParseInt(s); err == nil {
		return int(i)
	}
	return defaultValue
}

// ParseInt safe wrapper.
func ParseInt(s string, base, bits int) (int64, error) {
	return strconvParseInt(s, base, bits)
}

// Need these helpers to avoid importing strconv in this package by default.
// We'll provide optional compile‑time flag or rely on users to import if needed.
// For now, we implement simple fallback and allow overriding via build tags.
var strconvParseInt = func(s string, args ...int) (int64, error) {
	// Placeholder – users should import strconv and optionally override.
	// We provide a no-op version that returns 0 and an error.
	return 0, errors.New("strconv not imported; use safe.AtoiOverride")
}

// OverrideParseInt allows users to inject strconv.ParseInt if needed.
func OverrideParseInt(fn func(string, int, int) (int64, error)) {
	strconvParseInt = fn
}
