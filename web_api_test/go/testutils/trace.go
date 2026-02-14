// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"bytes"
	"fmt"
	"runtime"
	"strings"
)

// ------------------------------------------------------------------------
// Stack capture
// ------------------------------------------------------------------------

// CaptureStack returns the current stack trace as a string, skipping the
// call to CaptureStack itself and any testutils internal frames.
// skip can be used to skip additional frames (0 means skip only CaptureStack).
func CaptureStack(skip int) string {
	// Skip this function, plus the caller's caller as requested.
	const baseSkip = 2 // CaptureStack + runtime.Callers
	return captureStack(baseSkip + skip)
}

// CaptureStackAll returns the full stack trace (including all goroutines)
// as a string. This is useful for debugging deadlocks or hangs.
func CaptureStackAll() string {
	buf := make([]byte, 1<<20) // 1 MB
	n := runtime.Stack(buf, true)
	return string(buf[:n])
}

// captureStack is the internal implementation.
func captureStack(skip int) string {
	pc := make([]uintptr, 32)
	n := runtime.Callers(skip, pc)
	if n == 0 {
		return ""
	}
	pc = pc[:n]
	frames := runtime.CallersFrames(pc)

	var buf bytes.Buffer
	for {
		frame, more := frames.Next()
		if !strings.Contains(frame.File, "runtime/") {
			fmt.Fprintf(&buf, "%s:%d %s\n", frame.File, frame.Line, frame.Function)
		}
		if !more {
			break
		}
	}
	return buf.String()
}

// FormatStack formats a raw stack trace (as returned by runtime.Stack) into a
// more readable form by stripping the Go root path and runtime frames.
func FormatStack(stack []byte) string {
	lines := strings.Split(string(stack), "\n")
	var out []string
	for _, line := range lines {
		// Skip runtime.goexit and similar.
		if strings.Contains(line, "runtime.") {
			continue
		}
		// Optionally shorten file paths.
		if idx := strings.Index(line, "/go/src/"); idx != -1 {
			line = line[idx+len("/go/src/"):]
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// ------------------------------------------------------------------------
// StackError – error with stack trace
// ------------------------------------------------------------------------

// StackError is an error that carries a stack trace captured at its creation.
type StackError struct {
	Err   error
	Stack string
}

// Error implements the error interface.
func (e *StackError) Error() string {
	return e.Err.Error()
}

// Unwrap returns the underlying error (for errors.Is/As).
func (e *StackError) Unwrap() error {
	return e.Err
}

// WithStack annotates an error with a stack trace (skipping the call to WithStack).
func WithStack(err error) error {
	if err == nil {
		return nil
	}
	return &StackError{
		Err:   err,
		Stack: CaptureStack(1),
	}
}

// ------------------------------------------------------------------------
// Trace – lightweight creation trace
// ------------------------------------------------------------------------

// Trace holds the file and line where a value was created. It can be embedded
// in structs to aid debugging by showing where an object originated.
type Trace struct {
	File string
	Line int
	Func string
}

// NewTrace captures the creation point, skipping the given number of frames.
// skip=0 captures the caller of NewTrace.
func NewTrace(skip int) Trace {
	pc, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return Trace{File: "unknown", Line: 0}
	}
	fn := runtime.FuncForPC(pc)
	funcName := ""
	if fn != nil {
		funcName = fn.Name()
	}
	return Trace{
		File: file,
		Line: line,
		Func: funcName,
	}
}

// String returns a short representation of the trace, e.g. "file.go:42".
func (t Trace) String() string {
	return fmt.Sprintf("%s:%d", t.File, t.Line)
}

// Full returns a longer representation including function name.
func (t Trace) Full() string {
	return fmt.Sprintf("%s:%d %s", t.File, t.Line, t.Func)
}