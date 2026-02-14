// Package testutils provides utilities for testing, including
// comparison functions for numbers, slices, maps, and other types.
package testutils

import (
	"math"
	"reflect"
)

// ----------------------------------------------------------------------
// Ordered comparisons
// ----------------------------------------------------------------------

// CompareResult represents the result of a comparison.
type CompareResult int

const (
	// Less indicates that a < b.
	Less CompareResult = -1
	// Equal indicates that a == b.
	Equal CompareResult = 0
	// Greater indicates that a > b.
	Greater CompareResult = 1
)

// Compare compares two ordered values (any type that supports <, >).
// It returns Less, Equal, or Greater.
func Compare[T ~int | ~int8 | ~int16 | ~int32 | ~int64 |
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
	~float32 | ~float64 | ~string](a, b T) CompareResult {
	switch {
	case a < b:
		return Less
	case a > b:
		return Greater
	default:
		return Equal
	}
}

// Min returns the smaller of two ordered values.
func Min[T ~int | ~int8 | ~int16 | ~int32 | ~int64 |
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
	~float32 | ~float64 | ~string](a, b T) T {
	if a < b {
		return a
	}
	return b
}

// Max returns the larger of two ordered values.
func Max[T ~int | ~int8 | ~int16 | ~int32 | ~int64 |
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
	~float32 | ~float64 | ~string](a, b T) T {
	if a > b {
		return a
	}
	return b
}

// ----------------------------------------------------------------------
// Floating point comparisons with tolerance
// ----------------------------------------------------------------------

// AlmostEqualAbsolute reports whether two float64 values are equal within
// an absolute tolerance epsilon.
func AlmostEqualAbsolute(a, b, epsilon float64) bool {
	return math.Abs(a-b) <= epsilon
}

// AlmostEqualRelative reports whether two float64 values are equal within
// a relative tolerance (percentage of the larger magnitude).
// If both numbers are zero, it returns true.
func AlmostEqualRelative(a, b, relEps float64) bool {
	if a == 0 && b == 0 {
		return true
	}
	diff := math.Abs(a - b)
	mag := math.Max(math.Abs(a), math.Abs(b))
	return diff <= relEps*mag
}

// AlmostEqual uses a combination of absolute and relative tolerance:
// it first tries absolute, then relative. This is useful for numbers
// that can be very small or very large.
func AlmostEqual(a, b, absEps, relEps float64) bool {
	if AlmostEqualAbsolute(a, b, absEps) {
		return true
	}
	return AlmostEqualRelative(a, b, relEps)
}

// ----------------------------------------------------------------------
// Slice comparisons
// ----------------------------------------------------------------------

// EqualSlice reports whether two slices are equal in length and all elements
// are equal using the built‑in == operator (requires comparable elements).
func EqualSlice[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// EqualSliceFunc reports whether two slices are equal in length and the
// provided equality function returns true for all corresponding elements.
func EqualSliceFunc[T any](a, b []T, eq func(a, b T) bool) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !eq(a[i], b[i]) {
			return false
		}
	}
	return true
}

// CompareSlice lexicographically compares two slices of ordered elements.
// It returns Less if a < b, Equal if a == b, Greater if a > b.
// The comparison is element‑by‑element; if one slice is a prefix of the other,
// the shorter slice is considered Less.
func CompareSlice[T ~int | ~int8 | ~int16 | ~int32 | ~int64 |
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
	~float32 | ~float64 | ~string](a, b []T) CompareResult {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	for i := 0; i < minLen; i++ {
		if cmp := Compare(a[i], b[i]); cmp != Equal {
			return cmp
		}
	}
	// All equal up to minLen; the shorter slice is Less.
	switch {
	case len(a) < len(b):
		return Less
	case len(a) > len(b):
		return Greater
	default:
		return Equal
	}
}

// ----------------------------------------------------------------------
// Map comparisons
// ----------------------------------------------------------------------

// EqualMap reports whether two maps have identical key‑value pairs.
// Both maps must have comparable keys and values.
func EqualMap[K, V comparable](a, b map[K]V) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok || va != vb {
			return false
		}
	}
	return true
}

// EqualMapFunc reports whether two maps are equal using a custom equality
// function for values. Keys are compared directly (must be comparable).
func EqualMapFunc[K comparable, V any](a, b map[K]V, eq func(V, V) bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok || !eq(va, vb) {
			return false
		}
	}
	return true
}

// ----------------------------------------------------------------------
// Deep equality (using reflect.DeepEqual)
// ----------------------------------------------------------------------

// DeepEqual is a wrapper around reflect.DeepEqual for convenience.
func DeepEqual(a, b any) bool {
	return reflect.DeepEqual(a, b)
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     fmt.Println(testutils.Compare(5, 3)) // Greater
//     fmt.Println(testutils.Max(2.5, 1.8)) // 2.5
//
//     a := []float64{0.1, 0.2}
//     b := []float64{0.1000001, 0.2}
//     fmt.Println(testutils.AlmostEqualSlice(a, b, 1e-6, 0)) // true
//
//     m1 := map[string]int{"a":1, "b":2}
//     m2 := map[string]int{"a":1, "b":2}
//     fmt.Println(testutils.EqualMap(m1, m2)) // true
// }