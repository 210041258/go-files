// Package append provides utilities for appending to slices and files.
// It offers generic functions for safe and conditional appending.
package testutils

import (
	"os"
)

// ----------------------------------------------------------------------
// Slice appending (generic)
// ----------------------------------------------------------------------

// Copy returns a new slice with the elements appended, without modifying the original.
// This is useful when you want to avoid aliasing.
func Copy[T any](slice []T, elems ...T) []T {
	newSlice := make([]T, len(slice), len(slice)+len(elems))
	copy(newSlice, slice)
	return append(newSlice, elems...)
}

// If appends elems to the slice only if condition is true.
// It returns the original slice if condition is false, otherwise a new slice if needed.
func If[T any](slice []T, condition bool, elems ...T) []T {
	if !condition {
		return slice
	}
	return append(slice, elems...)
}

// Uniq appends an element only if it is not already present in the slice.
// It returns the (possibly new) slice and a boolean indicating whether the element was added.
func Uniq[T comparable](slice []T, elem T) ([]T, bool) {
	for _, v := range slice {
		if v == elem {
			return slice, false
		}
	}
	return append(slice, elem), true
}

// UniqSlice appends all elements from elems that are not already present in slice.
// The order of elems is preserved. It returns the new slice.
func UniqSlice[T comparable](slice []T, elems []T) []T {
	seen := make(map[T]struct{}, len(slice))
	for _, v := range slice {
		seen[v] = struct{}{}
	}
	for _, v := range elems {
		if _, ok := seen[v]; !ok {
			slice = append(slice, v)
			seen[v] = struct{}{}
		}
	}
	return slice
}

// ----------------------------------------------------------------------
// File appending
// ----------------------------------------------------------------------

// ToFile appends data to the specified file. If the file does not exist,
// it is created with 0644 permissions.
func ToFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

// Line appends a single line to a file, adding a newline if not present.
func Line(path string, line string) error {
	if len(line) == 0 || line[len(line)-1] != '\n' {
		line += "\n"
	}
	return ToFile(path, []byte(line))
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     s := []int{1,2,3}
//     s2 := append.Copy(s, 4,5)      // [1,2,3,4,5]
//     s3 := append.If(s, true, 6)    // [1,2,3,6]
//     s4, added := append.Uniq(s, 2) // false, s4 unchanged
//     append.ToFile("test.txt", []byte("hello"))
// }