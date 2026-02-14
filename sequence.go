// Package sequence provides generic utilities for working with slices as sequences.
// It offers functional operations like Map, Filter, Reduce, as well as helpers
// for generating numeric ranges, chunking, zipping, and more.
package testutils

import (
	"sort"

	"github.com/yourproject/pair" // optional, for Zip; if not available, we can define a local pair.
)

// If pair package is not available, we can embed a simple Pair type.
type Pair[T, U any] struct {
	First  T
	Second U
}

// ----------------------------------------------------------------------
// Generation
// ----------------------------------------------------------------------

// Range generates a slice of integers from start (inclusive) to end (exclusive),
// stepping by 1. If start >= end, returns an empty slice.
func Range(start, end int) []int {
	if start >= end {
		return []int{}
	}
	n := end - start
	r := make([]int, n)
	for i := 0; i < n; i++ {
		r[i] = start + i
	}
	return r
}

// RangeStep generates a slice of integers from start (inclusive) to end (exclusive),
// stepping by step (must be > 0). If step <= 0, it panics.
func RangeStep(start, end, step int) []int {
	if step <= 0 {
		panic("sequence: step must be positive")
	}
	if start >= end {
		return []int{}
	}
	// Approximate length.
	n := (end - start + step - 1) / step
	r := make([]int, 0, n)
	for i := start; i < end; i += step {
		r = append(r, i)
	}
	return r
}

// ----------------------------------------------------------------------
// Transformations
// ----------------------------------------------------------------------

// Map applies a function to each element of a slice and returns a new slice.
func Map[T, U any](seq []T, f func(T) U) []U {
	result := make([]U, len(seq))
	for i, v := range seq {
		result[i] = f(v)
	}
	return result
}

// Filter returns a new slice containing only the elements for which pred returns true.
func Filter[T any](seq []T, pred func(T) bool) []T {
	result := make([]T, 0, len(seq))
	for _, v := range seq {
		if pred(v) {
			result = append(result, v)
		}
	}
	return result
}

// Reduce combines the elements of a slice using a binary function, starting from init.
// Example: sum := Reduce(seq, 0, func(acc, v int) int { return acc + v })
func Reduce[T, U any](seq []T, init U, f func(U, T) U) U {
	acc := init
	for _, v := range seq {
		acc = f(acc, v)
	}
	return acc
}

// ----------------------------------------------------------------------
// Sub‑slicing
// ----------------------------------------------------------------------

// Take returns the first n elements of the slice. If n > len(seq), returns the whole slice.
func Take[T any](seq []T, n int) []T {
	if n <= 0 {
		return []T{}
	}
	if n >= len(seq) {
		return seq
	}
	return seq[:n]
}

// Drop returns the slice with the first n elements removed. If n >= len(seq), returns empty.
func Drop[T any](seq []T, n int) []T {
	if n <= 0 {
		return seq
	}
	if n >= len(seq) {
		return []T{}
	}
	return seq[n:]
}

// TakeWhile returns the longest prefix of the slice where pred returns true.
func TakeWhile[T any](seq []T, pred func(T) bool) []T {
	for i, v := range seq {
		if !pred(v) {
			return seq[:i]
		}
	}
	return seq
}

// DropWhile returns the suffix of the slice after removing the longest prefix where pred returns true.
func DropWhile[T any](seq []T, pred func(T) bool) []T {
	for i, v := range seq {
		if !pred(v) {
			return seq[i:]
		}
	}
	return []T{}
}

// ----------------------------------------------------------------------
// Combination
// ----------------------------------------------------------------------

// Zip combines two slices into a slice of pairs. The length of the result is the
// minimum of the two input lengths.
func Zip[T, U any](seqT []T, seqU []U) []Pair[T, U] {
	n := len(seqT)
	if len(seqU) < n {
		n = len(seqU)
	}
	result := make([]Pair[T, U], n)
	for i := 0; i < n; i++ {
		result[i] = Pair[T, U]{First: seqT[i], Second: seqU[i]}
	}
	return result
}

// Unzip splits a slice of pairs into two separate slices.
func Unzip[T, U any](pairs []Pair[T, U]) ([]T, []U) {
	first := make([]T, len(pairs))
	second := make([]U, len(pairs))
	for i, p := range pairs {
		first[i] = p.First
		second[i] = p.Second
	}
	return first, second
}

// Flatten concatenates multiple slices into one.
func Flatten[T any](seqs [][]T) []T {
	total := 0
	for _, s := range seqs {
		total += len(s)
	}
	result := make([]T, 0, total)
	for _, s := range seqs {
		result = append(result, s...)
	}
	return result
}

// ----------------------------------------------------------------------
// Chunking
// ----------------------------------------------------------------------

// Chunk splits a slice into slices of at most size elements.
// The last chunk may be smaller than size.
func Chunk[T any](seq []T, size int) [][]T {
	if size <= 0 {
		panic("sequence: chunk size must be positive")
	}
	if len(seq) == 0 {
		return [][]T{}
	}
	chunks := make([][]T, 0, (len(seq)+size-1)/size)
	for i := 0; i < len(seq); i += size {
		end := i + size
		if end > len(seq) {
			end = len(seq)
		}
		chunks = append(chunks, seq[i:end])
	}
	return chunks
}

// ----------------------------------------------------------------------
// Order
// ----------------------------------------------------------------------

// Reverse returns a new slice with elements in reverse order.
func Reverse[T any](seq []T) []T {
	rev := make([]T, len(seq))
	for i, v := range seq {
		rev[len(seq)-1-i] = v
	}
	return rev
}

// Sort sorts a slice in place using the provided comparison function.
// The less function should return true if a < b.
func Sort[T any](seq []T, less func(a, b T) bool) {
	sort.Slice(seq, func(i, j int) bool {
		return less(seq[i], seq[j])
	})
}

// ----------------------------------------------------------------------
// Membership
// ----------------------------------------------------------------------

// Contains reports whether an element equal to the target is present in the slice.
func Contains[T comparable](seq []T, target T) bool {
	for _, v := range seq {
		if v == target {
			return true
		}
	}
	return false
}

// Index returns the first index of the target, or -1 if not found.
func Index[T comparable](seq []T, target T) int {
	for i, v := range seq {
		if v == target {
			return i
		}
	}
	return -1
}

// ----------------------------------------------------------------------
// Set operations (assuming elements are comparable)
// ----------------------------------------------------------------------

// Dedup removes duplicate elements from a slice, preserving order of first occurrence.
func Dedup[T comparable](seq []T) []T {
	seen := make(map[T]struct{}, len(seq))
	result := make([]T, 0, len(seq))
	for _, v := range seq {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

// Union returns the set union of two slices (order not guaranteed).
func Union[T comparable](a, b []T) []T {
	seen := make(map[T]struct{}, len(a)+len(b))
	for _, v := range a {
		seen[v] = struct{}{}
	}
	for _, v := range b {
		seen[v] = struct{}{}
	}
	result := make([]T, 0, len(seen))
	for v := range seen {
		result = append(result, v)
	}
	return result
}

// Intersect returns the set intersection of two slices (order not guaranteed).
func Intersect[T comparable](a, b []T) []T {
	setA := make(map[T]struct{}, len(a))
	for _, v := range a {
		setA[v] = struct{}{}
	}
	result := make([]T, 0, len(a))
	for _, v := range b {
		if _, ok := setA[v]; ok {
			result = append(result, v)
			delete(setA, v) // ensure uniqueness in result
		}
	}
	return result
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     seq := sequence.Range(1, 6) // [1,2,3,4,5]
//     squares := sequence.Map(seq, func(x int) int { return x*x })
//     evens := sequence.Filter(seq, func(x int) bool { return x%2 == 0 })
//     sum := sequence.Reduce(seq, 0, func(acc, x int) int { return acc + x })
//     pairs := sequence.Zip(seq, squares) // [(1,1), (2,4), (3,9), (4,16), (5,25)]
// }