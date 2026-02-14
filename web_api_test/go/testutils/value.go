// Package value provides generic optional values (Option) and result types (Result)
// for safer, more expressive handling of absent values and fallible operations.
//
// Option eliminates nil pointer bugs and makes the possibility of missing values
// explicit in the type signature. Result captures both success and error outcomes
// without requiring multiple return values.
//
// Example:
//
//	func FindUser(id int) value.Option[User] {
//		if user, ok := db.Find(id); ok {
//			return value.Some(user)
//		}
//		return value.None[User]()
//	}
//
//	func ParseAge(s string) value.Result[int, error] {
//		age, err := strconv.Atoi(s)
//		if err != nil {
//			return value.Err[int](err)
//		}
//		return value.Ok(age)
//	}
package testutils

import (
	"errors"
	"fmt"
)

// --------------------------------------------------------------------
// Option – an optional value (Maybe monad)
// --------------------------------------------------------------------

// Option represents a value that may or may not be present.
// It is a generic type that eliminates nil pointers.
type Option[T any] struct {
	value   T
	present bool
}

// Some constructs an Option containing a value.
func Some[T any](value T) Option[T] {
	return Option[T]{
		value:   value,
		present: true,
	}
}

// None constructs an Option with no value.
func None[T any]() Option[T] {
	return Option[T]{
		present: false,
	}
}

// IsSome returns true if the Option contains a value.
func (o Option[T]) IsSome() bool {
	return o.present
}

// IsNone returns true if the Option contains no value.
func (o Option[T]) IsNone() bool {
	return !o.present
}

// Get returns the value and a boolean indicating presence.
// Idiomatic Go: val, ok := opt.Get()
func (o Option[T]) Get() (T, bool) {
	return o.value, o.present
}

// MustGet returns the value or panics if none is present.
// Use only when you are certain the value exists.
func (o Option[T]) MustGet() T {
	if !o.present {
		panic("value: MustGet called on None")
	}
	return o.value
}

// OrElse returns the value if present, otherwise returns defaultValue.
func (o Option[T]) OrElse(defaultValue T) T {
	if o.present {
		return o.value
	}
	return defaultValue
}

// OrElseGet returns the value if present, otherwise calls f and returns its result.
func (o Option[T]) OrElseGet(f func() T) T {
	if o.present {
		return o.value
	}
	return f()
}

// OrZero returns the value if present, otherwise the zero value of T.
func (o Option[T]) OrZero() T {
	if o.present {
		return o.value
	}
	var zero T
	return zero
}

// Map applies a function to the contained value, returning an Option of the result.
// If the original Option is None, it remains None.
func Map[T, U any](o Option[T], f func(T) U) Option[U] {
	if o.present {
		return Some(f(o.value))
	}
	return None[U]()
}

// FlatMap applies a function returning an Option to the contained value.
// If the original Option is None, it remains None.
func FlatMap[T, U any](o Option[T], f func(T) Option[U]) Option[U] {
	if o.present {
		return f(o.value)
	}
	return None[U]()
}

// Filter returns the Option if it contains a value that satisfies the predicate,
// otherwise returns None.
func Filter[T any](o Option[T], pred func(T) bool) Option[T] {
	if o.present && pred(o.value) {
		return o
	}
	return None[T]()
}

// String implements fmt.Stringer.
func (o Option[T]) String() string {
	if o.present {
		return fmt.Sprintf("Some(%v)", o.value)
	}
	return "None"
}

// --------------------------------------------------------------------
// Result – a value that is either Ok or Err (Either monad)
// --------------------------------------------------------------------

// Result represents either a success (Ok) containing a value of type T,
// or a failure (Err) containing an error of type E.
type Result[T, E any] struct {
	value T
	err   E
	ok    bool
}

// Ok constructs a successful Result.
func Ok[T, E any](value T) Result[T, E] {
	return Result[T, E]{
		value: value,
		ok:    true,
	}
}

// Err constructs a failed Result.
func Err[T, E any](err E) Result[T, E] {
	return Result[T, E]{
		err: err,
		ok:  false,
	}
}

// IsOk returns true if the Result is Ok.
func (r Result[T, E]) IsOk() bool {
	return r.ok
}

// IsErr returns true if the Result is Err.
func (r Result[T, E]) IsErr() bool {
	return !r.ok
}

// OkGet returns the Ok value and a boolean indicating success.
func (r Result[T, E]) OkGet() (T, bool) {
	return r.value, r.ok
}

// ErrGet returns the Err value and a boolean indicating failure.
func (r Result[T, E]) ErrGet() (E, bool) {
	return r.err, !r.ok
}

// MustOk returns the Ok value or panics if the Result is Err.
func (r Result[T, E]) MustOk() T {
	if !r.ok {
		panic(fmt.Sprintf("value: MustOk called on Err: %v", r.err))
	}
	return r.value
}

// UnwrapOr returns the Ok value if present, otherwise returns defaultValue.
func (r Result[T, E]) UnwrapOr(defaultValue T) T {
	if r.ok {
		return r.value
	}
	return defaultValue
}

// UnwrapOrElse returns the Ok value if present, otherwise calls f and returns its result.
func (r Result[T, E]) UnwrapOrElse(f func(E) T) T {
	if r.ok {
		return r.value
	}
	return f(r.err)
}

// MapOk applies a function to the Ok value, producing a new Result.
// If the original is Err, it remains unchanged.
func MapOk[T, U, E any](r Result[T, E], f func(T) U) Result[U, E] {
	if r.ok {
		return Ok[U, E](f(r.value))
	}
	return Err[U, E](r.err)
}

// MapErr applies a function to the Err value, producing a new Result.
// If the original is Ok, it remains unchanged.
func MapErr[T, E, F any](r Result[T, E], f func(E) F) Result[T, F] {
	if r.ok {
		return Ok[T, F](r.value)
	}
	return Err[T, F](f(r.err))
}

// AndThen chains a Result‑producing function after an Ok.
func AndThen[T, U, E any](r Result[T, E], f func(T) Result[U, E]) Result[U, E] {
	if r.ok {
		return f(r.value)
	}
	return Err[U, E](r.err)
}

// OrElse returns the original Result if it is Ok, otherwise returns the result
// of f applied to the Err value.
func OrElse[T, E any](r Result[T, E], f func(E) Result[T, E]) Result[T, E] {
	if r.ok {
		return r
	}
	return f(r.err)
}

// String implements fmt.Stringer.
func (r Result[T, E]) String() string {
	if r.ok {
		return fmt.Sprintf("Ok(%v)", r.value)
	}
	return fmt.Sprintf("Err(%v)", r.err)
}

// --------------------------------------------------------------------
// Utilities for common cases
// --------------------------------------------------------------------

// OkIf returns Ok(value) if condition is true, otherwise Err(err).
// Useful for concise validation.
func OkIf[T, E any](condition bool, value T, err E) Result[T, E] {
	if condition {
		return Ok[T, E](value)
	}
	return Err[T, E](err)
}

// ErrIf is the inverse of OkIf.
func ErrIf[T, E any](condition bool, value T, err E) Result[T, E] {
	if condition {
		return Err[T, E](err)
	}
	return Ok[T, E](value)
}

// Coalesce returns the first non‑zero value from the given options.
// If all values are zero, returns the zero value of T.
func Coalesce[T comparable](values ...T) T {
	var zero T
	for _, v := range values {
		if v != zero {
			return v
		}
	}
	return zero
}

// DefaultIfZero returns v if it is non‑zero, otherwise returns def.
func DefaultIfZero[T comparable](v, def T) T {
	var zero T
	if v != zero {
		return v
	}
	return def
}

// --------------------------------------------------------------------
// Conversion from Option to Result and vice versa
// --------------------------------------------------------------------

// ToResult converts an Option to a Result, using err as the error value if None.
func (o Option[T]) ToResult[E any](err E) Result[T, E] {
	if o.present {
		return Ok[T, E](o.value)
	}
	return Err[T, E](err)
}

// ToOption converts a Result to an Option, discarding the error.
func (r Result[T, E]) ToOption() Option[T] {
	if r.ok {
		return Some(r.value)
	}
	return None[T]()
}