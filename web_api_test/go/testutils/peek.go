// Package peek provides utilities for peeking at data without consuming it.
// It includes a peekable reader for io.Reader and slice peek functions.
package peek

import (
	"bytes"
	"errors"
	"io"
)

// ----------------------------------------------------------------------
// PeekableReader wraps an io.Reader and allows peeking at upcoming bytes
// without advancing the read position.
// ----------------------------------------------------------------------

// PeekableReader is an io.Reader that supports Peek.
type PeekableReader struct {
	r    io.Reader
	buf  []byte
}

// NewPeekableReader creates a new PeekableReader wrapping r.
func NewPeekableReader(r io.Reader) *PeekableReader {
	return &PeekableReader{r: r}
}

// Read reads data into p, consuming from the internal buffer first,
// then from the underlying reader.
func (pr *PeekableReader) Read(p []byte) (n int, err error) {
	if len(pr.buf) == 0 {
		return pr.r.Read(p)
	}
	// Copy from buffer.
	n = copy(p, pr.buf)
	pr.buf = pr.buf[n:]
	if len(pr.buf) == 0 && n < len(p) {
		// Buffer exhausted, read more from underlying reader.
		var more int
		more, err = pr.r.Read(p[n:])
		n += more
	}
	return n, err
}

// Peek returns the next n bytes without advancing the reader.
// It returns an error if fewer than n bytes are available (io.ErrUnexpectedEOF).
func (pr *PeekableReader) Peek(n int) ([]byte, error) {
	if n <= len(pr.buf) {
		// Already have enough in buffer.
		return pr.buf[:n], nil
	}
	// Need to read more.
	want := n - len(pr.buf)
	buf := make([]byte, want)
	read, err := io.ReadAtLeast(pr.r, buf, want)
	pr.buf = append(pr.buf, buf[:read]...)
	if err != nil {
		return nil, err
	}
	return pr.buf[:n], nil
}

// Discard skips the next n bytes, returning the number actually discarded.
func (pr *PeekableReader) Discard(n int) (int, error) {
	if n <= len(pr.buf) {
		pr.buf = pr.buf[n:]
		return n, nil
	}
	discarded := len(pr.buf)
	pr.buf = nil
	need := n - discarded
	// Use io.CopyN to discard from underlying reader.
	_, err := io.CopyN(io.Discard, pr.r, int64(need))
	return n, err
}

// ----------------------------------------------------------------------
// Slice peeking
// ----------------------------------------------------------------------

// First returns the first n elements of slice s without modifying the slice.
// If n > len(s), it returns the whole slice.
func First[T any](s []T, n int) []T {
	if n >= len(s) {
		return s
	}
	return s[:n]
}

// Last returns the last n elements of slice s without modifying the slice.
// If n > len(s), it returns the whole slice.
func Last[T any](s []T, n int) []T {
	if n >= len(s) {
		return s
	}
	return s[len(s)-n:]
}

// ----------------------------------------------------------------------
// String peek helpers (convenience)
// ----------------------------------------------------------------------

// FirstString returns the first n characters of a string.
func FirstString(s string, n int) string {
	runes := []rune(s)
	if n >= len(runes) {
		return s
	}
	return string(runes[:n])
}

// LastString returns the last n characters of a string.
func LastString(s string, n int) string {
	runes := []rune(s)
	if n >= len(runes) {
		return s
	}
	return string(runes[len(runes)-n:])
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     // Reader peek
//     r := strings.NewReader("hello world")
//     pr := peek.NewPeekableReader(r)
//     b, _ := pr.Peek(5)
//     fmt.Println(string(b)) // "hello"
//     // Read afterwards
//     buf := make([]byte, 5)
//     pr.Read(buf)
//     fmt.Println(string(buf)) // "hello" again (consumed)
//
//     // Slice peek
//     s := []int{1,2,3,4,5}
//     fmt.Println(peek.First(s, 2)) // [1 2]
//     fmt.Println(peek.Last(s, 2))  // [4 5]
//
//     // String peek
//     fmt.Println(peek.FirstString("你好世界", 2)) // "你好"
// }