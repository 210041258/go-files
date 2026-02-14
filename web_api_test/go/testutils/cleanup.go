// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

// Cleanup is a simple registry of cleanup functions to be run later.
type Cleanup struct {
	funcs []func()
}

// Add registers a cleanup function.
func (c *Cleanup) Add(f func()) {
	c.funcs = append(c.funcs, f)
}

// Run executes all registered cleanup functions in reverse order
// (last added, first executed).
func (c *Cleanup) Run() {
	for i := len(c.funcs) - 1; i >= 0; i-- {
		c.funcs[i]()
	}
}

// Defer registers the cleanup with testing.TB's Cleanup method,
// ensuring it runs when the test completes.
func (c *Cleanup) Defer(t testing.TB) {
	t.Helper()
	t.Cleanup(c.Run)
}

// TempDir creates a temporary directory and returns its path along with a
// cleanup function that removes it. The cleanup is automatically registered
// with the provided testing.TB if t is not nil.
func TempDir(t testing.TB, pattern string) (string, func()) {
	t.Helper()
	dir, err := ioutil.TempDir("", pattern)
	if err != nil {
		t.Fatalf("TempDir: %v", err)
	}
	cleanup := func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("failed to remove temp dir %s: %v", dir, err)
		}
	}
	if t != nil {
		t.Cleanup(cleanup)
	}
	return dir, cleanup
}

// TempFile creates a temporary file with optional content and returns its path
// along with a cleanup function that removes it. The cleanup is automatically
// registered with the provided testing.TB if t is not nil.
func TempFile(t testing.TB, dir, pattern string, content []byte) (string, func()) {
	t.Helper()
	f, err := ioutil.TempFile(dir, pattern)
	if err != nil {
		t.Fatalf("TempFile: %v", err)
	}
	if len(content) > 0 {
		if _, err := f.Write(content); err != nil {
			f.Close()
			t.Fatalf("writing temp file: %v", err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatalf("closing temp file: %v", err)
	}
	path := f.Name()
	cleanup := func() {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			t.Errorf("failed to remove temp file %s: %v", path, err)
		}
	}
	if t != nil {
		t.Cleanup(cleanup)
	}
	return path, cleanup
}

// Chdir changes the current working directory to the given path and returns a
// cleanup function that restores the original working directory. If t is not nil,
// the cleanup is automatically registered.
func Chdir(t testing.TB, dir string) func() {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Chdir: failed to get current directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: failed to change directory to %s: %v", dir, err)
	}
	cleanup := func() {
		if err := os.Chdir(orig); err != nil {
			t.Errorf("failed to restore working directory to %s: %v", orig, err)
		}
	}
	if t != nil {
		t.Cleanup(cleanup)
	}
	return cleanup
}