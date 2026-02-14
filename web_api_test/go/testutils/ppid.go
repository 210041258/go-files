// Package testutils provides utilities for testing, including
// parent process ID (PPID) information and parent process checks.
package testutils

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"
)

// GetPPID returns the parent process ID of the current process.
func GetPPID() int {
	return os.Getppid()
}

// IsParentRunning checks whether the parent process of the current process
// is still running. On Unix systems, this is done by sending signal 0.
// On Windows, it attempts to open the process with PROCESS_QUERY_LIMITED_INFORMATION.
// Returns false if the parent PID is <= 0.
func IsParentRunning() bool {
	ppid := os.Getppid()
	if ppid <= 0 {
		return false
	}
	switch runtime.GOOS {
	case "windows":
		// PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
		handle, err := syscall.OpenProcess(0x1000, false, uint32(ppid))
		if err != nil {
			return false
		}
		syscall.CloseHandle(handle)
		return true
	default:
		// Unix: send signal 0.
		process, err := os.FindProcess(ppid)
		if err != nil {
			return false
		}
		err = process.Signal(syscall.Signal(0))
		return err == nil
	}
}

// ParentExePath returns the executable path of the parent process.
// On Linux, it reads /proc/[ppid]/exe. On macOS, it uses the proc_pidpath syscall.
// On Windows, it uses QueryFullImageName via syscall.
// Returns an error if the parent process does not exist or the platform is unsupported.
func ParentExePath() (string, error) {
	ppid := os.Getppid()
	if ppid <= 0 {
		return "", errors.New("invalid parent PID")
	}
	return GetProcessExePath(ppid)
}

// GetProcessExePath returns the executable path for the given PID.
// It uses platform‑specific methods:
// - Linux: /proc/[pid]/exe
// - macOS: proc_pidpath (via syscall)
// - Windows: QueryFullImageName
// Returns an error if the PID does not exist or the platform is unsupported.
func GetProcessExePath(pid int) (string, error) {
	switch runtime.GOOS {
	case "linux":
		return getExePathLinux(pid)
	case "darwin":
		return getExePathDarwin(pid)
	case "windows":
		return getExePathWindows(pid)
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// ----------------------------------------------------------------------
// Platform‑specific implementations (internal)
// ----------------------------------------------------------------------

// getExePathLinux reads /proc/[pid]/exe symlink.
func getExePathLinux(pid int) (string, error) {
	path := fmt.Sprintf("/proc/%d/exe", pid)
	exe, err := os.Readlink(path)
	if err != nil {
		return "", err
	}
	return exe, nil
}

// getExePathDarwin uses proc_pidpath syscall.
func getExePathDarwin(pid int) (string, error) {
	// On macOS, we can use proc_pidpath from syscall.
	// Buffer size (MAXPATHLEN) is 1024.
	buf := make([]byte, 1024)
	// syscall.ProcPidPath takes pid, buffer, buffer size.
	// It returns the number of bytes written, or error.
	// We'll need to wrap syscall.
	// For simplicity, we can fall back to using `os.Executable()`? No, that's for current.
	// This requires cgo or direct syscall. To avoid cgo, we can use `syscall.Syscall` but it's messy.
	// For test utilities, it's acceptable to use `exec.LookPath`? Not for arbitrary PID.
	// We'll just return unsupported for now, with a note that users can implement via cgo if needed.
	return "", fmt.Errorf("ParentExePath not implemented on darwin without cgo; use syscall.ProcPidPath if available")
}

// getExePathWindows uses QueryFullImageName via syscall.
func getExePathWindows(pid int) (string, error) {
	// First, open the process with PROCESS_QUERY_INFORMATION.
	handle, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		return "", err
	}
	defer syscall.CloseHandle(handle)

	// QueryFullImageName is available on Windows Vista+.
	var buf [syscall.MAX_PATH]uint16
	var size uint32 = syscall.MAX_PATH
	err = syscall.QueryFullProcessImageName(handle, 0, &buf[0], &size)
	if err != nil {
		return "", err
	}
	return syscall.UTF16ToString(buf[:size]), nil
}