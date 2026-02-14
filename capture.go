// Package testutils provides helpers for capturing stdout, stderr, log output,
// and signals during tests. All capture functions are safe for concurrent use
// and restore original state automatically.
package testutils

import (
	"bytes"
	"io"
	"log"
	"os"
	"os/signal"
	"sync"
	"testing"
)

// --------------------------------------------------------------------
// Stdout / Stderr capture
// --------------------------------------------------------------------

// OutputCapture holds the captured stdout/stderr and a restore function.
type OutputCapture struct {
	stdout *os.File
	stderr *os.File
	rOut   *os.File
	wOut   *os.File
	rErr   *os.File
	wErr   *os.File
	outBuf *bytes.Buffer
	errBuf *bytes.Buffer
	mu     sync.Mutex
}

// CaptureStdout redirects os.Stdout to a pipe and returns a capture object.
// Call Restore() to restore the original stdout and retrieve captured output.
func CaptureStdout() *OutputCapture {
	return captureOutput(true, false)
}

// CaptureStderr redirects os.Stderr to a pipe and returns a capture object.
// Call Restore() to restore the original stderr and retrieve captured output.
func CaptureStderr() *OutputCapture {
	return captureOutput(false, true)
}

// CaptureBoth redirects both stdout and stderr and returns a capture object.
// Call Restore() to restore and retrieve captured output.
func CaptureBoth() *OutputCapture {
	return captureOutput(true, true)
}

func captureOutput(captureStdout, captureStderr bool) *OutputCapture {
	c := &OutputCapture{
		outBuf: &bytes.Buffer{},
		errBuf: &bytes.Buffer{},
	}

	if captureStdout {
		c.rOut, c.wOut, _ = os.Pipe()
		c.stdout = os.Stdout
		os.Stdout = c.wOut
	}

	if captureStderr {
		c.rErr, c.wErr, _ = os.Pipe()
		c.stderr = os.Stderr
		os.Stderr = c.wErr
	}

	// Start copying in background
	if captureStdout {
		go func() {
			io.Copy(c.outBuf, c.rOut)
		}()
	}
	if captureStderr {
		go func() {
			io.Copy(c.errBuf, c.rErr)
		}()
	}

	return c
}

// Restore restores original stdout/stderr and returns captured output.
// For stdout only capture, stderr output will be empty.
func (c *OutputCapture) Restore() (stdout, stderr string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.wOut != nil {
		c.wOut.Close()
		os.Stdout = c.stdout
	}
	if c.wErr != nil {
		c.wErr.Close()
		os.Stderr = c.stderr
	}
	if c.rOut != nil {
		c.rOut.Close()
	}
	if c.rErr != nil {
		c.rErr.Close()
	}

	return c.outBuf.String(), c.errBuf.String()
}

// CaptureOutput is a convenience function that captures both stdout and stderr,
// executes the given function, and returns the captured output.
// Example:
//
//	stdout, stderr := CaptureOutput(t, func() {
//		fmt.Println("hello")
//	})
func CaptureOutput(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()
	c := CaptureBoth()
	fn()
	return c.Restore()
}

// --------------------------------------------------------------------
// Log capture (log package)
// --------------------------------------------------------------------

// LogCapture captures log.Logger output.
type LogCapture struct {
	mu     sync.Mutex
	buf    *bytes.Buffer
	orig   *log.Logger
	flags  int
	prefix string
}

// CaptureLog redirects the default log.Logger to a buffer.
// Call Restore() to restore the original logger and get captured output.
func CaptureLog() *LogCapture {
	buf := &bytes.Buffer{}
	lc := &LogCapture{
		buf:    buf,
		orig:   log.Default(),
		flags:  log.Flags(),
		prefix: log.Prefix(),
	}
	log.SetOutput(buf)
	log.SetFlags(0) // no timestamps for easier comparison
	log.SetPrefix("")
	return lc
}

// Restore restores the original log.Logger and returns captured log output.
func (lc *LogCapture) Restore() string {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	log.SetOutput(lc.orig.Writer())
	log.SetFlags(lc.flags)
	log.SetPrefix(lc.prefix)
	return lc.buf.String()
}

// --------------------------------------------------------------------
// Signal capture
// --------------------------------------------------------------------

// SignalCapture captures incoming OS signals.
type SignalCapture struct {
	sigCh    chan os.Signal
	done     chan struct{}
	signals  []os.Signal
	mu       sync.Mutex
	captured []os.Signal
}

// CaptureSignals starts capturing the specified signals.
// It returns a SignalCapture that can be used to stop capture and retrieve signals.
// Example:
//
//	sc := CaptureSignals(syscall.SIGINT, syscall.SIGTERM)
//	// ... send signal ...
//	sc.Stop()
//	captured := sc.Captured()
func CaptureSignals(signals ...os.Signal) *SignalCapture {
	sc := &SignalCapture{
		sigCh:    make(chan os.Signal, 1),
		done:     make(chan struct{}),
		signals:  signals,
		captured: []os.Signal{},
	}
	signal.Notify(sc.sigCh, signals...)
	go sc.run()
	return sc
}

func (sc *SignalCapture) run() {
	for {
		select {
		case sig := <-sc.sigCh:
			sc.mu.Lock()
			sc.captured = append(sc.captured, sig)
			sc.mu.Unlock()
		case <-sc.done:
			return
		}
	}
}

// Stop stops capturing signals and restores original handling.
func (sc *SignalCapture) Stop() {
	signal.Stop(sc.sigCh)
	close(sc.done)
}

// Captured returns a copy of all signals received during capture.
func (sc *SignalCapture) Captured() []os.Signal {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	cpy := make([]os.Signal, len(sc.captured))
	copy(cpy, sc.captured)
	return cpy
}

// --------------------------------------------------------------------
// Combined capture (output + logs)
// --------------------------------------------------------------------

// FullCapture captures stdout, stderr, and default log output.
// Useful for testing complete program output.
type FullCapture struct {
	outCap *OutputCapture
	logCap *LogCapture
}

// CaptureAll starts capturing stdout, stderr, and default log.
// Call Restore() to get all captured output.
func CaptureAll() *FullCapture {
	return &FullCapture{
		outCap: CaptureBoth(),
		logCap: CaptureLog(),
	}
}

// Restore restores original output and log destinations and returns
// captured stdout, stderr, and log output.
func (fc *FullCapture) Restore() (stdout, stderr, logOutput string) {
	stdout, stderr = fc.outCap.Restore()
	logOutput = fc.logCap.Restore()
	return
}
