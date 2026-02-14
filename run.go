// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ------------------------------------------------------------------------
// Simple command execution with output capture
// ------------------------------------------------------------------------

// RunCommand executes the given command and returns its stdout, stderr,
// and any error that occurred. The command is run with no extra environment
// (the current process environment is inherited).
func RunCommand(name string, args ...string) (stdout, stderr string, err error) {
	return RunCommandInDir("", name, args...)
}

// RunCommandInDir is like RunCommand but sets the working directory to dir.
func RunCommandInDir(dir, name string, args ...string) (stdout, stderr string, err error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return runAndCapture(cmd)
}

// RunCommandWithEnv is like RunCommand but adds the given environment variables.
// The env slice should be in the format "KEY=value".
func RunCommandWithEnv(env []string, name string, args ...string) (stdout, stderr string, err error) {
	cmd := exec.Command(name, args...)
	cmd.Env = append(cmd.Environ(), env...)
	return runAndCapture(cmd)
}

// RunCommandWithTimeout runs the command and kills it if it exceeds the timeout.
// It returns the captured output and an error that may be a timeout error.
func RunCommandWithTimeout(timeout time.Duration, name string, args ...string) (stdout, stderr string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	return runAndCapture(cmd)
}

// runAndCapture is the internal helper that runs a *exec.Cmd and captures stdout/stderr.
func runAndCapture(cmd *exec.Cmd) (stdout, stderr string, err error) {
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()
	return
}

// ------------------------------------------------------------------------
// More granular control with Command wrapper
// ------------------------------------------------------------------------

// Command wraps an exec.Cmd with additional convenience methods for testing.
type Command struct {
	*exec.Cmd
}

// NewCommand creates a new Command with the given name and arguments.
func NewCommand(name string, args ...string) *Command {
	return &Command{exec.Command(name, args...)}
}

// InDir sets the working directory for the command.
func (c *Command) InDir(dir string) *Command {
	c.Dir = dir
	return c
}

// WithEnv adds environment variables (each "KEY=value") to the command.
func (c *Command) WithEnv(env ...string) *Command {
	c.Env = append(c.Environ(), env...)
	return c
}

// WithTimeout wraps the command in a context that cancels after the given duration.
// It returns a new Command that will be killed if it runs too long.
func (c *Command) WithTimeout(timeout time.Duration) *Command {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	// Store cancel somewhere? Not trivial; we'll just use exec.CommandContext.
	// This method must return a new Command because the underlying *exec.Cmd changes.
	cmd := exec.CommandContext(ctx, c.Path, c.Args[1:]...)
	cmd.Dir = c.Dir
	cmd.Env = c.Env
	cmd.Stdin = c.Stdin
	cmd.Stdout = c.Stdout
	cmd.Stderr = c.Stderr
	return &Command{cmd}
}

// Run executes the command and returns captured output, like RunCommand.
func (c *Command) Run() (stdout, stderr string, err error) {
	return runAndCapture(c.Cmd)
}

// CombinedOutput runs the command and returns combined stdout+stderr.
func (c *Command) CombinedOutput() (string, error) {
	out, err := c.Cmd.CombinedOutput()
	return string(out), err
}

// ------------------------------------------------------------------------
// Convenience predicates for output checking
// ------------------------------------------------------------------------

// OutputContains reports whether the given output string contains substr.
// Useful for quickly checking command output in tests.
func OutputContains(output, substr string) bool {
	return strings.Contains(output, substr)
}

// OutputMatches reports whether the output string matches the pattern
// using strings.Contains; for regex, use regexp.MustCompile.
func OutputMatches(output, pattern string) bool {
	// For simplicity, we don't add regex here; just contains.
	// Users can use regexp if needed.
	return strings.Contains(output, pattern)
}