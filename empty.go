// Package testutils provides utilities for testing web APIs.
package testutils

// Empty is a zero‑size type useful as a placeholder or for signaling.
// It can be used, for example, as a channel element type when only the
// event matters, not the content.
type Empty struct{}

// Zero returns the zero value of type T.
// This is helpful in generic tests when you need to obtain the zero value
// of a type parameter.
func Zero[T any]() T {
	var zero T
	return zero
}