// Package testutils provides utilities for testing, including
// sorting helpers for slices and checking sorted order.
package testutils

import (
	"sort"
)

// ----------------------------------------------------------------------
// Slice sorting (generic wrappers)
// ----------------------------------------------------------------------

// Sort sorts the slice in place using the provided less function.
// It is a generic wrapper around sort.Slice.
func Sort[T any](slice []T, less func(a, b T) bool) {
	sort.Slice(slice, func(i, j int) bool {
		return less(slice[i], slice[j])
	})
}

// SortStable sorts the slice in place using the provided less function,
// keeping the original order of equal elements.
// It is a generic wrapper around sort.SliceStable.
func SortStable[T any](slice []T, less func(a, b T) bool) {
	sort.SliceStable(slice, func(i, j int) bool {
		return less(slice[i], slice[j])
	})
}

// Sorted returns a new slice that is a copy of the input sorted according to less.
// The original slice is not modified.
func Sorted[T any](slice []T, less func(a, b T) bool) []T {
	cpy := make([]T, len(slice))
	copy(cpy, slice)
	Sort(cpy, less)
	return cpy
}

// ----------------------------------------------------------------------
// Sortedness checks
// ----------------------------------------------------------------------

// IsSorted reports whether the slice is sorted in non‑decreasing order
// according to the provided less function.
func IsSorted[T any](slice []T, less func(a, b T) bool) bool {
	for i := 1; i < len(slice); i++ {
		if less(slice[i], slice[i-1]) {
			return false
		}
	}
	return true
}

// IsStrictlySorted reports whether the slice is sorted in strictly increasing order.
func IsStrictlySorted[T any](slice []T, less func(a, b T) bool) bool {
	for i := 1; i < len(slice); i++ {
		if !less(slice[i-1], slice[i]) {
			return false
		}
	}
	return true
}

// ----------------------------------------------------------------------
// Convenience helpers for common types
// ----------------------------------------------------------------------

// SortInts sorts a slice of ints in place.
func SortInts(slice []int) {
	sort.Ints(slice)
}

// IntsAreSorted reports whether a slice of ints is sorted in increasing order.
func IntsAreSorted(slice []int) bool {
	return sort.IntsAreSorted(slice)
}

// SortStrings sorts a slice of strings in place.
func SortStrings(slice []string) {
	sort.Strings(slice)
}

// StringsAreSorted reports whether a slice of strings is sorted.
func StringsAreSorted(slice []string) bool {
	return sort.StringsAreSorted(slice)
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     nums := []int{3,1,4,1,5}
//     testutils.Sort(nums, func(a,b int) bool { return a < b })
//     fmt.Println(nums) // [1,1,3,4,5]
//
//     words := []string{"banana", "apple", "cherry"}
//     sorted := testutils.Sorted(words, func(a,b string) bool { return a < b })
//     fmt.Println(sorted) // [apple banana cherry]
//
//     fmt.Println(testutils.IsSorted(nums, func(a,b int) bool { return a < b })) // true
// }