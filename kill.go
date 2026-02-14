// kill.go – cross‑platform process termination utility.
// Usage:
//
//	go run kill.go -pid 1234
//	go run kill.go -name notepad.exe -force=false
//	kill -name "myapp"          (after go install)
package testutils

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

func main() {
	// Command line flags
	pid := flag.Int("pid", 0, "Process ID to terminate")
	name := flag.String("name", "", "Process name to terminate (e.g., 'go.exe', 'myapp')")
	force := flag.Bool("force", true, "Force kill (SIGKILL on Unix, /F on Windows); if false, attempt graceful termination")
	flag.Parse()

	// Validate arguments
	if *pid == 0 && *name == "" {
		fmt.Fprintln(os.Stderr, "Error: either -pid or -name must be provided")
		flag.Usage()
		os.Exit(1)
	}
	if *pid != 0 && *name != "" {
		fmt.Fprintln(os.Stderr, "Error: specify only one of -pid or -name")
		os.Exit(1)
	}

	var exitCode int

	if *pid != 0 {
		if err := killByPID(*pid, *force); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exitCode = 1
		}
	} else {
		if err := killByName(*name, *force); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

// killByPID terminates a process using its PID.
func killByPID(pid int, force bool) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("process with PID %d not found: %w", pid, err)
	}

	if force {
		// Forceful kill – cross‑platform via Process.Kill()
		return proc.Kill()
	}

	// Graceful termination – platform dependent
	if runtime.GOOS == "windows" {
		// Windows: taskkill without /F sends WM_CLOSE (graceful)
		cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	// Unix: send SIGTERM
	return proc.Signal(os.Interrupt) // or syscall.SIGTERM
}

// killByName terminates all processes matching the given name.
func killByName(name string, force bool) error {
	switch runtime.GOOS {
	case "windows":
		return killByNameWindows(name, force)
	case "linux", "darwin", "freebsd", "netbsd", "openbsd":
		return killByNameUnix(name, force)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func killByNameWindows(name string, force bool) error {
	args := []string{"/IM", name}
	if force {
		args = append([]string{"/F"}, args...)
	}
	cmd := exec.Command("taskkill", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		// taskkill returns non‑zero if no process was killed
		if strings.Contains(err.Error(), "exit status 128") || // taskkill specific?
			strings.Contains(err.Error(), "not found") {
			fmt.Printf("No process named %q found.\n", name)
			return nil
		}
		return err
	}
	return nil
}

func killByNameUnix(name string, force bool) error {
	var cmd *exec.Cmd
	if force {
		cmd = exec.Command("pkill", "-9", name)
	} else {
		cmd = exec.Command("pkill", name)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		// pkill returns exit code 1 if no processes matched
		if strings.Contains(err.Error(), "exit status 1") {
			fmt.Printf("No process named %q found.\n", name)
			return nil
		}
		return err
	}
	return nil
}

func init() {
	// Custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Terminate processes by PID or name.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -pid 1234\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -name notepad.exe -force=false\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -name 'myapp' -force\n", os.Args[0])
	}
}
