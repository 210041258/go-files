// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"reflect"
)

// ------------------------------------------------------------------------
// Slice utilities
// ------------------------------------------------------------------------

// Map applies f to each element of slice and returns a new slice.
func Map[T, U any](slice []T, f func(T) U) []U {
	result := make([]U, len(slice))
	for i, v := range slice {
		result[i] = f(v)
	}
	return result
}

// Filter returns a new slice containing only the elements for which keep returns true.
func Filter[T any](slice []T, keep func(T) bool) []T {
	var result []T
	for _, v := range slice {
		if keep(v) {
			result = append(result, v)
		}
	}
	return result
}

// Reduce combines the elements of slice using f, starting with initial.
func Reduce[T, U any](slice []T, initial U, f func(U, T) U) U {
	result := initial
	for _, v := range slice {
		result = f(result, v)
	}
	return result
}

// Contains reports whether v is present in slice using ==.
func Contains[T comparable](slice []T, v T) bool {
	for _, item := range slice {
		if item == v {
			return true
		}
	}
	return false
}

// ContainsFunc reports whether any element of slice satisfies the predicate.
func ContainsFunc[T any](slice []T, f func(T) bool) bool {
	for _, v := range slice {
		if f(v) {
			return true
		}
	}
	return false
}

// IndexOf returns the first index of v in slice, or -1 if not present.
func IndexOf[T comparable](slice []T, v T) int {
	for i, item := range slice {
		if item == v {
			return i
		}
	}
	return -1
}

// Find returns the first element for which f returns true, and true.
// If no element satisfies f, it returns the zero value of T and false.
func Find[T any](slice []T, f func(T) bool) (T, bool) {
	for _, v := range slice {
		if f(v) {
			return v, true
		}
	}
	var zero T
	return zero, false
}

// Unique returns a new slice containing the first occurrence of each distinct element.
func Unique[T comparable](slice []T) []T {
	seen := make(map[T]struct{})
	var result []T
	for _, v := range slice {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

// UniqueFunc returns a new slice containing the first element for each distinct key
// returned by key. Elements are considered equal if their keys are equal.
func UniqueFunc[T any, K comparable](slice []T, key func(T) K) []T {
	seen := make(map[K]struct{})
	var result []T
	for _, v := range slice {
		k := key(v)
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

// Chunk splits a slice into chunks of at most size. The last chunk may be smaller.
// It panics if size <= 0.
func Chunk[T any](slice []T, size int) [][]T {
	if size <= 0 {
		panic("testutils: Chunk size must be positive")
	}
	var chunks [][]T
	for i := 0; i < len(slice); i += size {
		end := i + size
		if end > len(slice) {
			end = len(slice)
		}
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}

// Reverse returns a new slice with the elements in reverse order.
func Reverse[T any](slice []T) []T {
	result := make([]T, len(slice))
	for i, v := range slice {
		result[len(slice)-1-i] = v
	}
	return result
}

// CopySlice returns a shallow copy of the slice.
func CopySlice[T any](slice []T) []T {
	result := make([]T, len(slice))
	copy(result, slice)
	return result
}

// EqualSlices reports whether two slices are equal using reflect.DeepEqual.
// For a more efficient comparison of comparable types, use reflect.DeepEqual directly,
// but this is convenient in tests.
func EqualSlices[T any](a, b []T) bool {
	return reflect.DeepEqual(a, b)
}

// MapSlice converts a slice of type T to a slice of type U using f.
// (Alias for Map for readability in conversions.)
func MapSlice[T, U any](slice []T, f func(T) U) []U {
	return Map(slice, f)
}

// ------------------------------------------------------------------------
// Pointer utilities
// ------------------------------------------------------------------------

// PtrTo returns a pointer to the given value.
func PtrTo[T any](v T) *T {
	return &v
}

// ValOrZero returns the value pointed to by p, or the zero value of T if p is nil.
func ValOrZero[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

// ------------------------------------------------------------------------
// Channel utilities
// ------------------------------------------------------------------------

// MergeChannels merges multiple channels of the same type into a single channel.
// The returned channel is closed when all input channels are closed.
func MergeChannels[T any](chans ...<-chan T) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		if len(chans) == 0 {
			return
		}
		cases := make([]reflect.SelectCase, len(chans))
		for i, ch := range chans {
			cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
		}
		for len(cases) > 0 {
			chosen, val, ok := reflect.Select(cases)
			if !ok {
				// Channel closed – remove it from cases
				cases = append(cases[:chosen], cases[chosen+1:]...)
				continue
			}
			out <- val.Interface().(T)
		}
	}()
	return out
}

// Take reads up to n values from ch and returns them as a slice.
// It blocks until n values are received or ch is closed.
func Take[T any](ch <-chan T, n int) []T {
	result := make([]T, 0, n)
	for i := 0; i < n; i++ {
		v, ok := <-ch
		if !ok {
			break
		}
		result = append(result, v)
	}
	return result
}

// First returns the first value received from ch, or the zero value and false
// if ch is closed without sending.
func First[T any](ch <-chan T) (T, bool) {
	v, ok := <-ch
	return v, ok
}