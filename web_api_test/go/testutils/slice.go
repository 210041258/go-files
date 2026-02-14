// Package slice provides generic, type‑safe utilities for slice manipulation.
// All functions work with any slice type and never modify the original slice
// unless explicitly noted (e.g., Sort, Reverse).
package testutils

import (
	"math/rand"
)

// --------------------------------------------------------------------
// Predicates and basic lookups
// --------------------------------------------------------------------

// Contains reports whether v is present in s.
func Contains[T comparable](s []T, v T) bool {
	return Index(s, v) >= 0
}

// ContainsFunc reports whether an element satisfying f exists in s.
func ContainsFunc[T any](s []T, f func(T) bool) bool {
	return IndexFunc(s, f) >= 0
}

// Index returns the index of the first occurrence of v in s, or -1 if not found.
func Index[T comparable](s []T, v T) int {
	for i, item := range s {
		if item == v {
			return i
		}
	}
	return -1
}

// IndexFunc returns the index of the first element satisfying f, or -1 if none.
func IndexFunc[T any](s []T, f func(T) bool) int {
	for i, item := range s {
		if f(item) {
			return i
		}
	}
	return -1
}

// LastIndex returns the index of the last occurrence of v in s, or -1 if not found.
func LastIndex[T comparable](s []T, v T) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == v {
			return i
		}
	}
	return -1
}

// LastIndexFunc returns the index of the last element satisfying f, or -1 if none.
func LastIndexFunc[T any](s []T, f func(T) bool) int {
	for i := len(s) - 1; i >= 0; i-- {
		if f(s[i]) {
			return i
		}
	}
	return -1
}

// Count returns the number of occurrences of v in s.
func Count[T comparable](s []T, v T) int {
	var n int
	for _, item := range s {
		if item == v {
			n++
		}
	}
	return n
}

// CountFunc returns the number of elements satisfying f.
func CountFunc[T any](s []T, f func(T) bool) int {
	var n int
	for _, item := range s {
		if f(item) {
			n++
		}
	}
	return n
}

// --------------------------------------------------------------------
// Functional transformations
// --------------------------------------------------------------------

// Map applies f to each element of s and returns a new slice of results.
func Map[T any, U any](s []T, f func(T) U) []U {
	if s == nil {
		return nil
	}
	res := make([]U, len(s))
	for i, v := range s {
		res[i] = f(v)
	}
	return res
}

// Filter returns a new slice containing only the elements that satisfy f.
func Filter[T any](s []T, f func(T) bool) []T {
	if s == nil {
		return nil
	}
	res := make([]T, 0, len(s)/2)
	for _, v := range s {
		if f(v) {
			res = append(res, v)
		}
	}
	return res
}

// FilterNot returns a new slice containing only the elements that do NOT satisfy f.
func FilterNot[T any](s []T, f func(T) bool) []T {
	return Filter(s, func(v T) bool { return !f(v) })
}

// Reduce reduces the slice to a single value using the accumulator function.
// If s is empty, returns zero.
func Reduce[T any, U any](s []T, acc func(U, T) U, initial U) U {
	result := initial
	for _, v := range s {
		result = acc(result, v)
	}
	return result
}

// ForEach executes f on each element of s.
func ForEach[T any](s []T, f func(T)) {
	for _, v := range s {
		f(v)
	}
}

// ForEachIndexed executes f on each element of s, passing index and value.
func ForEachIndexed[T any](s []T, f func(int, T)) {
	for i, v := range s {
		f(i, v)
	}
}

// Some returns true if at least one element satisfies f.
func Some[T any](s []T, f func(T) bool) bool {
	return IndexFunc(s, f) >= 0
}

// Every returns true if all elements satisfy f.
// For empty slices, it returns true (vacuously true).
func Every[T any](s []T, f func(T) bool) bool {
	for _, v := range s {
		if !f(v) {
			return false
		}
	}
	return true
}

// --------------------------------------------------------------------
// Set operations (on slices, order not preserved)
// --------------------------------------------------------------------

// Unique returns a new slice with duplicate elements removed.
// The order of the first occurrence of each element is preserved.
func Unique[T comparable](s []T) []T {
	if s == nil {
		return nil
	}
	seen := make(map[T]struct{}, len(s))
	res := make([]T, 0, len(s))
	for _, v := range s {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			res = append(res, v)
		}
	}
	return res
}

// UniqueFunc returns a new slice with duplicate elements removed based on a key.
// The key is derived by calling keyFn on each element.
func UniqueFunc[T any, K comparable](s []T, keyFn func(T) K) []T {
	if s == nil {
		return nil
	}
	seen := make(map[K]struct{}, len(s))
	res := make([]T, 0, len(s))
	for _, v := range s {
		k := keyFn(v)
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			res = append(res, v)
		}
	}
	return res
}

// Difference returns elements in a that are not in b.
// Order is preserved; duplicates in a are kept unless removed by later duplicates.
func Difference[T comparable](a, b []T) []T {
	if a == nil {
		return nil
	}
	// Build set of b
	set := make(map[T]struct{}, len(b))
	for _, v := range b {
		set[v] = struct{}{}
	}
	res := make([]T, 0, len(a))
	for _, v := range a {
		if _, ok := set[v]; !ok {
			res = append(res, v)
		}
	}
	return res
}

// Intersection returns elements present in both a and b.
// Order of first slice is preserved.
func Intersection[T comparable](a, b []T) []T {
	if a == nil || b == nil {
		return nil
	}
	set := make(map[T]struct{}, len(b))
	for _, v := range b {
		set[v] = struct{}{}
	}
	res := make([]T, 0, min(len(a), len(b)))
	for _, v := range a {
		if _, ok := set[v]; ok {
			res = append(res, v)
			delete(set, v) // avoid duplicates in result
		}
	}
	return res
}

// Union returns all distinct elements from both slices.
// Order is not guaranteed; uses Unique on concatenation.
func Union[T comparable](a, b []T) []T {
	if a == nil && b == nil {
		return nil
	}
	if a == nil {
		return Unique(b)
	}
	if b == nil {
		return Unique(a)
	}
	combined := make([]T, 0, len(a)+len(b))
	combined = append(combined, a...)
	combined = append(combined, b...)
	return Unique(combined)
}

// --------------------------------------------------------------------
// Order and shuffling
// --------------------------------------------------------------------

// Reverse reverses the elements of s in place.
func Reverse[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// Reversed returns a new slice with the elements reversed.
func Reversed[T any](s []T) []T {
	if s == nil {
		return nil
	}
	res := make([]T, len(s))
	for i, v := range s {
		res[len(s)-1-i] = v
	}
	return res
}

// Sort sorts a slice of ordered types in ascending order.
// The slice is sorted in place.
func Sort[T ~[]E, E interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr | ~float32 | ~float64 | ~string
}](s T) {
	// Use a simple insertion sort for small slices; production code should use sort.Slice.
	// For brevity, we assume the caller imports "sort" if they need high performance.
	// Here we provide a minimal generic sort that works for ordered types.
	for i := 1; i < len(s); i++ {
		j := i
		for j > 0 && s[j-1] > s[j] {
			s[j-1], s[j] = s[j], s[j-1]
			j--
		}
	}
}

// SortFunc sorts a slice using a custom less function.
// The slice is sorted in place.
func SortFunc[T any](s []T, less func(i, j int) bool) {
	// A simple generic insertion sort; in production, use sort.Slice.
	n := len(s)
	for i := 1; i < n; i++ {
		j := i
		for j > 0 && less(j, j-1) {
			s[j-1], s[j] = s[j], s[j-1]
			j--
		}
	}
}

// Shuffle randomizes the order of elements in s (in place).
func Shuffle[T any](s []T) {
	rand.Shuffle(len(s), func(i, j int) {
		s[i], s[j] = s[j], s[i]
	})
}

// Shuffled returns a new shuffled slice.
func Shuffled[T any](s []T) []T {
	if s == nil {
		return nil
	}
	res := make([]T, len(s))
	copy(res, s)
	Shuffle(res)
	return res
}

// --------------------------------------------------------------------
// Chunking and splitting
// --------------------------------------------------------------------

// Chunk splits a slice into chunks of at most size.
// The last chunk may be smaller than size.
// Returns nil if size <= 0 or s is nil.
func Chunk[T any](s []T, size int) [][]T {
	if size <= 0 || len(s) == 0 {
		return nil
	}
	n := (len(s) + size - 1) / size
	chunks := make([][]T, 0, n)
	for i := 0; i < len(s); i += size {
		end := i + size
		if end > len(s) {
			end = len(s)
		}
		chunks = append(chunks, s[i:end])
	}
	return chunks
}

// Flatten concatenates a slice of slices into a single slice.
func Flatten[T any](s [][]T) []T {
	if s == nil {
		return nil
	}
	total := 0
	for _, v := range s {
		total += len(v)
	}
	res := make([]T, 0, total)
	for _, v := range s {
		res = append(res, v...)
	}
	return res
}

// Partition splits a slice into two slices based on a predicate.
// The first slice contains elements that satisfy f, the second contains the rest.
func Partition[T any](s []T, f func(T) bool) ([]T, []T) {
	if s == nil {
		return nil, nil
	}
	left := make([]T, 0, len(s)/2)
	right := make([]T, 0, len(s)/2)
	for _, v := range s {
		if f(v) {
			left = append(left, v)
		} else {
			right = append(right, v)
		}
	}
	return left, right
}

// Drop returns a new slice with the first n elements removed.
// If n > len(s), returns an empty slice.
func Drop[T any](s []T, n int) []T {
	if n <= 0 {
		return s
	}
	if n >= len(s) {
		return []T{}
	}
	res := make([]T, len(s)-n)
	copy(res, s[n:])
	return res
}

// Take returns a new slice with the first n elements.
// If n > len(s), returns the whole slice.
func Take[T any](s []T, n int) []T {
	if n <= 0 {
		return []T{}
	}
	if n >= len(s) {
		return s
	}
	res := make([]T, n)
	copy(res, s[:n])
	return res
}

// DropWhile returns a new slice that drops elements from the beginning while f is true.
func DropWhile[T any](s []T, f func(T) bool) []T {
	i := 0
	for i < len(s) && f(s[i]) {
		i++
	}
	if i >= len(s) {
		return []T{}
	}
	res := make([]T, len(s)-i)
	copy(res, s[i:])
	return res
}

// TakeWhile returns a new slice that takes elements from the beginning while f is true.
func TakeWhile[T any](s []T, f func(T) bool) []T {
	i := 0
	for i < len(s) && f(s[i]) {
		i++
	}
	res := make([]T, i)
	copy(res, s[:i])
	return res
}

// --------------------------------------------------------------------
// Aggregations (numeric constraints)
// --------------------------------------------------------------------

// Sum returns the sum of elements in s. Works with any numeric type.
func Sum[T ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr | ~float32 | ~float64](s []T) T {
	var sum T
	for _, v := range s {
		sum += v
	}
	return sum
}

// Min returns the minimum element in s. Panics if s is empty.
func Min[T ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr | ~float32 | ~float64 | ~string](s []T) T {
	if len(s) == 0 {
		panic("slice.Min: empty slice")
	}
	min := s[0]
	for _, v := range s[1:] {
		if v < min {
			min = v
		}
	}
	return min
}

// Max returns the maximum element in s. Panics if s is empty.
func Max[T ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr | ~float32 | ~float64 | ~string](s []T) T {
	if len(s) == 0 {
		panic("slice.Max: empty slice")
	}
	max := s[0]
	for _, v := range s[1:] {
		if v > max {
			max = v
		}
	}
	return max
}

// --------------------------------------------------------------------
// Conversions
// --------------------------------------------------------------------

// ToMap creates a map from a slice using a key selector.
// If multiple elements produce the same key, the last one wins.
func ToMap[T any, K comparable](s []T, keyFn func(T) K) map[K]T {
	m := make(map[K]T, len(s))
	for _, v := range s {
		m[keyFn(v)] = v
	}
	return m
}

// GroupBy groups elements by the key returned by keyFn.
func GroupBy[T any, K comparable](s []T, keyFn func(T) K) map[K][]T {
	m := make(map[K][]T, len(s))
	for _, v := range s {
		k := keyFn(v)
		m[k] = append(m[k], v)
	}
	return m
}

// Keys returns the keys of a map as a slice.
func Keys[K comparable, V any](m map[K]V) []K {
	if m == nil {
		return nil
	}
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Values returns the values of a map as a slice.
func Values[K comparable, V any](m map[K]V) []V {
	if m == nil {
		return nil
	}
	vals := make([]V, 0, len(m))
	for _, v := range m {
		vals = append(vals, v)
	}
	return vals
}

// --------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
