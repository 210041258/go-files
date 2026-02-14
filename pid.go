// Package testutils provides utilities for testing, including
// process ID file management and process existence checks.
package testutils

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// WritePidFile writes the current process ID to the specified file.
// It creates the file with 0644 permissions. If the file already exists,
// it is overwritten. Returns an error if writing fails.
func WritePidFile(path string) error {
	pid := os.Getpid()
	data := []byte(strconv.Itoa(pid) + "\n")
	return ioutil.WriteFile(path, data, 0644)
}

// ReadPidFile reads a PID from a file. It returns the PID and any error.
// The file should contain only the PID (optionally with trailing whitespace).
func ReadPidFile(path string) (int, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, err
	}
	content := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(content)
	if err != nil {
		return 0, fmt.Errorf("invalid PID in %s: %w", path, err)
	}
	return pid, nil
}

// RemovePidFile removes the PID file if it exists. It ignores "not exist" errors.
func RemovePidFile(path string) error {
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// IsProcessRunning checks whether a process with the given PID exists.
// On Unix systems, this is done by sending signal 0. On Windows, it attempts
// to open the process with PROCESS_QUERY_LIMITED_INFORMATION; if the handle
// cannot be opened, the process is considered not running.
// Returns false for PIDs <= 0.
func IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	switch runtime.GOOS {
	case "windows":
		// PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
		handle, err := syscall.OpenProcess(0x1000, false, uint32(pid))
		if err != nil {
			return false
		}
		syscall.CloseHandle(handle)
		return true
	default:
		// Unix: send signal 0.
		process, err := os.FindProcess(pid)
		if err != nil {
			return false
		}
		err = process.Signal(syscall.Signal(0))
		return err == nil
	}
}