// Package slicesutil provides generic, high‑performance utilities for slices.
// It wraps and extends the standard slices package with additional algorithms
// and functional patterns (Map, Reduce, Filter, etc.).
package testutils

import (
	"cmp"
	"math/rand/v2"
	"slices"
)

// ============================================================================
//  Basic Statistics
// ============================================================================

// Min returns the smallest element in the slice and true.
// If the slice is empty, it returns the zero value and false.
func Min[T cmp.Ordered](data []T) (T, bool) {
	if len(data) == 0 {
		var zero T
		return zero, false
	}
	min := data[0]
	for _, v := range data[1:] {
		if v < min {
			min = v
		}
	}
	return min, true
}

// Max returns the largest element in the slice and true.
// If the slice is empty, it returns the zero value and false.
func Max[T cmp.Ordered](data []T) (T, bool) {
	if len(data) == 0 {
		var zero T
		return zero, false
	}
	max := data[0]
	for _, v := range data[1:] {
		if v > max {
			max = v
		}
	}
	return max, true
}

// ============================================================================
//  Sorting
// ============================================================================

// SortStrategy selects a sorting algorithm.
// Since Go 1.21, slices.Sort is always recommended; the other options
// are kept for backward compatibility or educational purposes.
type SortStrategy int

const (
	// SortAuto uses the best available sort (currently slices.Sort).
	SortAuto SortStrategy = iota
	// SortFast is an alias for slices.Sort.
	SortFast
)

// Sort sorts the slice according to the chosen strategy.
// For any non‑zero strategy, it currently delegates to slices.Sort.
func Sort[T cmp.Ordered](data []T, _ SortStrategy) {
	slices.Sort(data)
}

// SortStd is an alias for slices.Sort; it sorts the slice in ascending order.
func SortStd[T cmp.Ordered](data []T) {
	slices.Sort(data)
}

// ============================================================================
//  Searching
// ============================================================================

// BinarySearch searches for target in a sorted slice and returns the position
// where target would appear, and a boolean indicating if it is found.
// The slice must be sorted in ascending order.
func BinarySearch[T cmp.Ordered](data []T, target T) (int, bool) {
	return slices.BinarySearch(data, target)
}

// BinarySearchFunc works like BinarySearch but uses a custom comparison function.
// The slice must be sorted in increasing order according to the same cmp function.
func BinarySearchFunc[T any](data []T, target T, cmp func(T, T) int) (int, bool) {
	return slices.BinarySearchFunc(data, target, cmp)
}

// Contains reports whether target is present in the slice.
func Contains[T comparable](data []T, target T) bool {
	return slices.Contains(data, target)
}

// Index returns the index of the first occurrence of target, or -1 if not present.
func Index[T comparable](data []T, target T) int {
	return slices.Index(data, target)
}

// ============================================================================
//  Transformations (Functional)
// ============================================================================

// Filter returns a new slice containing only the elements for which pred is true.
func Filter[T any](data []T, pred func(T) bool) []T {
	res := make([]T, 0, len(data))
	for _, v := range data {
		if pred(v) {
			res = append(res, v)
		}
	}
	return res
}

// Map applies fn to each element and returns a new slice of the results.
func Map[T any, R any](data []T, fn func(T) R) []R {
	res := make([]R, len(data))
	for i, v := range data {
		res[i] = fn(v)
	}
	return res
}

// Reduce combines the elements of the slice using the given function,
// starting from the initial accumulator value.
func Reduce[T any, R any](data []T, initial R, fn func(R, T) R) R {
	acc := initial
	for _, v := range data {
		acc = fn(acc, v)
	}
	return acc
}

// ============================================================================
//  Slice Combination & Manipulation
// ============================================================================

// Merge concatenates two slices into a new slice.
func Merge[T any](a, b []T) []T {
	res := make([]T, len(a)+len(b))
	copy(res, a)
	copy(res[len(a):], b)
	return res
}

// MergeSorted merges two sorted slices into a single sorted slice.
// The input slices must be sorted in ascending order.
func MergeSorted[T cmp.Ordered](a, b []T) []T {
	res := make([]T, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i] <= b[j] {
			res = append(res, a[i])
			i++
		} else {
			res = append(res, b[j])
			j++
		}
	}
	res = append(res, a[i:]...)
	res = append(res, b[j:]...)
	return res
}

// Reverse reverses the elements of the slice in place.
func Reverse[T any](data []T) {
	slices.Reverse(data)
}

// Shuffle randomizes the order of the slice using the default random source.
func Shuffle[T any](data []T) {
	rand.Shuffle(len(data), func(i, j int) {
		data[i], data[j] = data[j], data[i]
	})
}

// Unique returns a new slice containing the first occurrence of each distinct element.
// The order of appearance is preserved.
func Unique[T comparable](data []T) []T {
	if len(data) == 0 {
		return nil
	}
	seen := make(map[T]struct{}, len(data))
	res := make([]T, 0, len(data))
	for _, v := range data {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			res = append(res, v)
		}
	}
	return res
}

// Chunk splits the slice into chunks of the given size (except possibly the last chunk).
// If size <= 0, it returns nil.
func Chunk[T any](data []T, size int) [][]T {
	if size <= 0 {
		return nil
	}
	return slices.Chunk(data, size) // Go 1.23+
}
