// Package copy provides utilities for copying slices, maps, and files.
// It includes shallow copy functions for slices and maps, and a file copy function.
package testutils

import (
	"fmt"
	"io"
	"os"
)

// ----------------------------------------------------------------------
// Slice copying
// ----------------------------------------------------------------------

// Slice returns a new slice with the same elements as the input.
// It performs a shallow copy; elements are not deeply copied.
func Slice[T any](s []T) []T {
	if s == nil {
		return nil
	}
	c := make([]T, len(s))
	copy(c, s)
	return c
}

// SliceWithCapacity returns a new slice with the same elements and the given capacity.
// If capacity < len(s), the slice is truncated to capacity.
func SliceWithCapacity[T any](s []T, cap int) []T {
	if cap < 0 {
		cap = len(s)
	}
	if cap < len(s) {
		// Truncate
		c := make([]T, cap)
		copy(c, s[:cap])
		return c
	}
	c := make([]T, len(s), cap)
	copy(c, s)
	return c
}

// ----------------------------------------------------------------------
// Map copying
// ----------------------------------------------------------------------

// Map returns a new map with the same key‑value pairs as the input.
// It performs a shallow copy; values are not deeply copied.
func Map[K comparable, V any](m map[K]V) map[K]V {
	if m == nil {
		return nil
	}
	c := make(map[K]V, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

// MapWithCapacity returns a new map with the same pairs and the given hint capacity.
func MapWithCapacity[K comparable, V any](m map[K]V, cap int) map[K]V {
	if cap < 0 {
		cap = len(m)
	}
	c := make(map[K]V, cap)
	for k, v := range m {
		c[k] = v
	}
	return c
}

// ----------------------------------------------------------------------
// File copying
// ----------------------------------------------------------------------

// File copies the contents of src to dst.
// If dst already exists, it is overwritten.
// The file permissions of src are not preserved.
func File(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("copy.File: open src: %w", err)
	}
	defer s.Close()

	d, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("copy.File: create dst: %w", err)
	}
	defer d.Close()

	_, err = io.Copy(d, s)
	if err != nil {
		return fmt.Errorf("copy.File: copy: %w", err)
	}
	return nil
}

// FilePreserve copies a file and attempts to preserve permissions.
func FilePreserve(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("copy.FilePreserve: open src: %w", err)
	}
	defer s.Close()

	info, err := s.Stat()
	if err != nil {
		return fmt.Errorf("copy.FilePreserve: stat src: %w", err)
	}

	d, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("copy.FilePreserve: create dst: %w", err)
	}
	defer d.Close()

	_, err = io.Copy(d, s)
	if err != nil {
		return fmt.Errorf("copy.FilePreserve: copy: %w", err)
	}

	// Restore permissions
	if err := os.Chmod(dst, info.Mode()); err != nil {
		return fmt.Errorf("copy.FilePreserve: chmod: %w", err)
	}

	// Optionally restore timestamps? Not included for simplicity.
	return nil
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     s := []int{1,2,3}
//     s2 := copy.Slice(s) // [1,2,3]
//
//     m := map[string]int{"a":1}
//     m2 := copy.Map(m)   // map[a:1]
//
//     err := copy.File("src.txt", "dst.txt")
// }