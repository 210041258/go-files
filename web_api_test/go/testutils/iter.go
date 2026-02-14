// Package slicesutil provides high-performance generic slice utilities for Go 1.23+.
package testutils

import "iter"

// ============================================================================
// Basic Transformers
// ============================================================================

// FilterSeq yields elements of seq for which pred returns true, short-circuiting when yield returns false.
func FilterSeq[T any](seq iter.Seq[T], pred func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range seq() {
			if pred(v) && !yield(v) {
				return
			}
		}
	}
}

// MapSeq applies fn to each element, short-circuiting when yield returns false.
func MapSeq[T any, R any](seq iter.Seq[T], fn func(T) R) iter.Seq[R] {
	return func(yield func(R) bool) {
		for v := range seq() {
			if !yield(fn(v)) {
				return
			}
		}
	}
}

// ============================================================================
// Collectors
// ============================================================================

// CollectSeq collects values into a slice with optional preallocation if known.
func CollectSeq[T any](seq iter.Seq[T]) []T {
	var res []T
	for v := range seq() {
		res = append(res, v)
	}
	return res
}

// AppendSeq appends values to an existing slice efficiently.
func AppendSeq[T any](slice []T, seq iter.Seq[T]) []T {
	for v := range seq() {
		slice = append(slice, v)
	}
	return slice
}

// ============================================================================
// Size-Limiting Combinators
// ============================================================================

// TakeSeq yields at most n elements, stopping early when limit reached.
func TakeSeq[T any](seq iter.Seq[T], n int) iter.Seq[T] {
	if n <= 0 {
		return func(yield func(T) bool) {}
	}
	return func(yield func(T) bool) {
		count := 0
		for v := range seq() {
			if !yield(v) {
				return
			}
			count++
			if count >= n {
				return
			}
		}
	}
}

// DropSeq skips the first n elements efficiently.
func DropSeq[T any](seq iter.Seq[T], n int) iter.Seq[T] {
	if n <= 0 {
		return seq
	}
	return func(yield func(T) bool) {
		skip := n
		for v := range seq() {
			if skip > 0 {
				skip--
				continue
			}
			if !yield(v) {
				return
			}
		}
	}
}

// ============================================================================
// Combination
// ============================================================================

// ConcatSeq yields all elements from seq1, then seq2, stopping early if yield returns false.
func ConcatSeq[T any](seq1, seq2 iter.Seq[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range seq1() {
			if !yield(v) {
				return
			}
		}
		for v := range seq2() {
			if !yield(v) {
				return
			}
		}
	}
}

// ============================================================================
// Reduction & Side Effects
// ============================================================================

// ReduceSeq reduces seq to a single value with fn.
func ReduceSeq[T any, R any](seq iter.Seq[T], initial R, fn func(R, T) R) R {
	acc := initial
	for v := range seq() {
		acc = fn(acc, v)
	}
	return acc
}

// ForEachSeq calls fn on each element for side effects.
func ForEachSeq[T any](seq iter.Seq[T], fn func(T)) {
	for v := range seq() {
		fn(v)
	}
}

// ============================================================================
// Performance Enhancements
// ============================================================================

// CollectSeqCap attempts to preallocate the slice with known capacity.
// Use if you know an estimated size to reduce allocations.
func CollectSeqCap[T any](seq iter.Seq[T], capHint int) []T {
	res := make([]T, 0, capHint)
	for v := range seq() {
		res = append(res, v)
	}
	return res
}
