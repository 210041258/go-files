// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"
)

// ------------------------------------------------------------------------
// Signal sending (Unix) / process termination (Windows)
// ------------------------------------------------------------------------

// TerminateProcess sends a termination signal to the process.
// On Unix it sends SIGTERM; on Windows it attempts a graceful shutdown
// using taskkill (if available) or falls back to forceful termination.
func TerminateProcess(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID: %d", pid)
	}
	switch runtime.GOOS {
	case "windows":
		return terminateProcessWindows(pid, false)
	default:
		return terminateProcessUnix(pid, syscall.SIGTERM)
	}
}

// KillProcess forcefully kills the process.
// On Unix it sends SIGKILL; on Windows it uses taskkill /F or TerminateProcess.
func KillProcess(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID: %d", pid)
	}
	switch runtime.GOOS {
	case "windows":
		return terminateProcessWindows(pid, true)
	default:
		return terminateProcessUnix(pid, syscall.SIGKILL)
	}
}

// terminateProcessUnix sends a signal to a process on Unix-like systems.
func terminateProcessUnix(pid int, sig syscall.Signal) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process: %w", err)
	}
	return proc.Signal(sig)
}

// terminateProcessWindows terminates a process on Windows.
// If force is false, it tries taskkill without /F first; if force is true,
// it uses taskkill /F (or direct TerminateProcess if taskkill fails).
func terminateProcessWindows(pid int, force bool) error {
	// Prefer using taskkill as it handles process trees better.
	var args []string
	if force {
		args = []string{"/F", "/PID", fmt.Sprint(pid)}
	} else {
		args = []string{"/PID", fmt.Sprint(pid)}
	}
	cmd := execCommand("taskkill", args...)
	err := cmd.Run()
	if err == nil {
		return nil
	}
	// If taskkill fails, fall back to TerminateProcess via syscall.
	// PROCESS_TERMINATE = 0x0001
	handle, err := syscall.OpenProcess(0x0001, false, uint32(pid))
	if err != nil {
		return fmt.Errorf("open process: %w", err)
	}
	defer syscall.CloseHandle(handle)
	err = syscall.TerminateProcess(handle, 1)
	if err != nil {
		return fmt.Errorf("terminate process: %w", err)
	}
	return nil
}

// execCommand is a thin wrapper around os/exec.Command; declared here to avoid
// unused import warnings if we don't actually need it (but we do on Windows).
// We'll import os/exec conditionally? Instead we'll just import it and use it.
// To keep the file clean, we import os/exec only when needed via build tags,
// but for simplicity we'll import it and rely on the Go tool.
import "os/exec"

// ------------------------------------------------------------------------
// Waiting for process exit
// ------------------------------------------------------------------------

// WaitProcessExit waits for the process to exit. It polls every 10 ms.
// Returns true if the process exited within the timeout, false otherwise.
func WaitProcessExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if !IsProcessRunning(pid) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		<-ticker.C
	}
}

// ------------------------------------------------------------------------
// Combined stop operations
// ------------------------------------------------------------------------

// StopProcessWithTimeout tries to terminate the process and waits for it to exit.
// If the process does not exit within the timeout, it is killed.
func StopProcessWithTimeout(pid int, timeout time.Duration) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID: %d", pid)
	}
	// If it's already dead, nothing to do.
	if !IsProcessRunning(pid) {
		return nil
	}
	// Try graceful termination.
	if err := TerminateProcess(pid); err != nil {
		return fmt.Errorf("terminate process: %w", err)
	}
	// Wait for graceful exit.
	if WaitProcessExit(pid, timeout) {
		return nil
	}
	// Force kill.
	if err := KillProcess(pid); err != nil {
		return fmt.Errorf("kill process: %w", err)
	}
	// Wait a little for the kill to take effect.
	if WaitProcessExit(pid, 500*time.Millisecond) {
		return nil
	}
	return fmt.Errorf("process %d did not exit after kill", pid)
}

// StopProcessByPidFile reads a PID from a file, stops the process with the
// given timeout, and removes the PID file if successful.
func StopProcessByPidFile(pidFile string, timeout time.Duration) error {
	pid, err := ReadPidFile(pidFile)
	if err != nil {
		return fmt.Errorf("read PID file: %w", err)
	}
	if !IsProcessRunning(pid) {
		// Process already gone; just remove the file.
		return RemovePidFile(pidFile)
	}
	if err := StopProcessWithTimeout(pid, timeout); err != nil {
		return err
	}
	return RemovePidFile(pidFile)
}