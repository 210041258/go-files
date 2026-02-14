// Package clear provides utilities for clearing data structures,
// buffers, and terminal screens. It offers generic helpers for
// common clear operations.
package testutils

import (
	"os"
)

// Map removes all entries from a map. The map must be non‑nil.
// After calling Map, the map will be empty.
func Map[K comparable, V any](m map[K]V) {
	for k := range m {
		delete(m, k)
	}
}

// Slice truncates a slice to zero length, leaving it empty.
// The underlying array may still be referenced, but the slice
// length becomes 0.
func Slice[T any](s *[]T) {
	*s = (*s)[:0]
}

// Terminal clears the terminal screen using ANSI escape codes.
// It works on most Unix terminals and Windows 10+ (if ANSI support is enabled).
// The cursor is moved to the home position (top‑left).
func Terminal() {
	os.Stdout.WriteString("\033[2J\033[H")
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     m := map[string]int{"a": 1, "b": 2}
//     clear.Map(m)
//     fmt.Println(len(m)) // 0
//
//     s := []int{1,2,3}
//     clear.Slice(&s)
//     fmt.Println(len(s)) // 0
//
//     clear.Terminal()
// }