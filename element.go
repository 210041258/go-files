// Package element provides a generic element container.
// It is useful for building data structures where elements need to be
// stored and manipulated generically.
package testutils

// Element is a generic wrapper for a value.
type Element[T any] struct {
	value T
}

// New creates a new element with the given value.
func New[T any](v T) *Element[T] {
	return &Element[T]{value: v}
}

// Value returns the stored value.
func (e *Element[T]) Value() T {
	return e.value
}

// Set updates the stored value.
func (e *Element[T]) Set(v T) {
	e.value = v
}