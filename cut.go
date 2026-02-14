// Package cut provides utilities for cutting slices and strings.
// It includes functions for extracting substrings, splitting at positions,
// and cutting around delimiters.
package testutils

// ----------------------------------------------------------------------
// String cutting (by rune indices)
// ----------------------------------------------------------------------

// String returns a substring of s from start to end (inclusive start, exclusive end)
// measured in runes. If start < 0, it is treated as 0. If end > len(runes) or end < 0,
// it is clamped to the string length. If start >= end, an empty string is returned.
func String(s string, start, end int) string {
	runes := []rune(s)
	total := len(runes)
	if start < 0 {
		start = 0
	}
	if end < 0 || end > total {
		end = total
	}
	if start >= end {
		return ""
	}
	return string(runes[start:end])
}

// Before returns the part of the string before the first occurrence of sep,
// excluding sep. If sep is not found, it returns the whole string and false.
// It is a wrapper around strings.Cut for convenience.
func Before(s, sep string) (string, bool) {
	before, _, found := strings.Cut(s, sep)
	return before, found
}

// After returns the part of the string after the first occurrence of sep,
// excluding sep. If sep is not found, it returns an empty string and false.
func After(s, sep string) (string, bool) {
	_, after, found := strings.Cut(s, sep)
	return after, found
}

// ----------------------------------------------------------------------
// Slice cutting (generic)
// ----------------------------------------------------------------------

// SplitAt splits a slice into two parts at index i.
// The first part contains elements [0, i), the second part contains [i, len(s)).
// If i < 0, i is set to 0. If i > len(s), i is set to len(s).
// The original slice is not modified.
func SplitAt[T any](s []T, i int) ([]T, []T) {
	if i < 0 {
		i = 0
	}
	if i > len(s) {
		i = len(s)
	}
	first := make([]T, i)
	copy(first, s[:i])
	second := make([]T, len(s)-i)
	copy(second, s[i:])
	return first, second
}

// Before returns the elements of the slice before index i.
// Equivalent to s[:i] but returns a copy.
func Before[T any](s []T, i int) []T {
	if i <= 0 {
		return []T{}
	}
	if i >= len(s) {
		i = len(s)
	}
	result := make([]T, i)
	copy(result, s[:i])
	return result
}

// After returns the elements of the slice from index i onward.
// Equivalent to s[i:] but returns a copy.
func After[T any](s []T, i int) []T {
	if i < 0 {
		i = 0
	}
	if i >= len(s) {
		return []T{}
	}
	result := make([]T, len(s)-i)
	copy(result, s[i:])
	return result
}

// Remove removes the element at index i and returns the resulting slice.
// The original slice is not modified.
func Remove[T any](s []T, i int) []T {
	if i < 0 || i >= len(s) {
		return Copy(s) // return a copy unchanged
	}
	result := make([]T, len(s)-1)
	copy(result, s[:i])
	copy(result[i:], s[i+1:])
	return result
}

// Insert inserts the elements v at index i and returns the resulting slice.
// The original slice is not modified.
func Insert[T any](s []T, i int, v ...T) []T {
	if i < 0 {
		i = 0
	}
	if i > len(s) {
		i = len(s)
	}
	result := make([]T, len(s)+len(v))
	copy(result, s[:i])
	copy(result[i:], v)
	copy(result[i+len(v):], s[i:])
	return result
}

// ----------------------------------------------------------------------
// Helper
// ----------------------------------------------------------------------

// Copy returns a copy of the slice.
func Copy[T any](s []T) []T {
	if s == nil {
		return nil
	}
	c := make([]T, len(s))
	copy(c, s)
	return c
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     // String cutting
//     fmt.Println(cut.String("Hello 世界", 0, 5)) // "Hello"
//     before, ok := cut.Before("a=b", "=")       // "a", true
//
//     // Slice cutting
//     s := []int{1,2,3,4,5}
//     first, second := cut.SplitAt(s, 2)        // [1,2], [3,4,5]
//     after := cut.After(s, 3)                  // [4,5]
//     removed := cut.Remove(s, 2)                // [1,2,4,5]
// }