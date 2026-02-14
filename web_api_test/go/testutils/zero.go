// Package zero provides utilities for detecting and handling zero values.
// It offers two levels of functionality:
//   - Basic: works with comparable types, no reflect import (fast)
//   - Extended: works with any type via reflect (use build tag `reflect`)
//
// All functions have consistent naming: Is, Or, Coalesce, etc.
package testutils

// --------------------------------------------------------------------
// Basic zero detection (comparable constraint, no reflect)
// --------------------------------------------------------------------

// Is reports whether v is the zero value of its type.
// Only works with comparable types.
func Is[T comparable](v T) bool {
	var zero T
	return v == zero
}

// IsNot reports whether v is not the zero value of its type.
// Only works with comparable types.
func IsNot[T comparable](v T) bool {
	return !Is(v)
}

// Or returns v if it is not zero, otherwise returns def.
// Only works with comparable types.
func Or[T comparable](v T, def T) T {
	if IsNot(v) {
		return v
	}
	return def
}

// OrZero returns v if it is not zero, otherwise returns the zero value of T.
// Only works with comparable types.
func OrZero[T comparable](v T) T {
	return Or(v, zero[T]())
}

// Coalesce returns the first non‑zero value in the list.
// If all values are zero, returns the zero value of T.
// Only works with comparable types.
func Coalesce[T comparable](values ...T) T {
	for _, v := range values {
		if IsNot(v) {
			return v
		}
	}
	return zero[T]()
}

// zero returns the zero value of T.
func zero[T any]() T {
	var z T
	return z
}

// --------------------------------------------------------------------
// Pointer zero detection (nil is zero)
// --------------------------------------------------------------------

// IsNilPtr reports whether p is nil or points to the zero value of T.
// Only works with comparable T.
func IsNilPtr[T comparable](p *T) bool {
	return p == nil || Is(*p)
}

// PtrOr returns p if it points to a non‑zero value, otherwise returns defPtr.
// If defPtr is nil and p is nil or zero, returns nil.
// Only works with comparable T.
func PtrOr[T comparable](p *T, defPtr *T) *T {
	if p != nil && IsNot(*p) {
		return p
	}
	return defPtr
}

// PtrOrZero returns p if it points to a non‑zero value, otherwise returns nil.
// Only works with comparable T.
func PtrOrZero[T comparable](p *T) *T {
	return PtrOr(p, nil)
}
