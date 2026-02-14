// id.go – cross‑platform process identifier.
// Finds PIDs by name, shows current process ID, or lists all processes.
// Usage:
//
//	go run id.go -name go.exe
//	go run id.go -pid        (show own PID)
//	go run id.go -name chrome -all
//	id -name "myapp" -v      (after go install)
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
	name := flag.String("name", "", "Process name to search for")
	showPID := flag.Bool("pid", false, "Show current process's own PID")
	all := flag.Bool("all", false, "Show all matching processes (by default only first is shown)")
	verbose := flag.Bool("v", false, "Verbose output (show additional info)")
	flag.Parse()

	// Validate arguments
	if *name == "" && !*showPID {
		fmt.Fprintln(os.Stderr, "Error: either -name or -pid must be provided")
		flag.Usage()
		os.Exit(1)
	}
	if *name != "" && *showPID {
		fmt.Fprintln(os.Stderr, "Error: specify only one of -name or -pid")
		os.Exit(1)
	}

	var exitCode int

	if *showPID {
		pid := os.Getpid()
		fmt.Printf("%d\n", pid)
		if *verbose {
			fmt.Printf("Process: %s\n", os.Args[0])
		}
		os.Exit(0)
	}

	// Find by name
	pids, err := findPIDsByName(*name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(pids) == 0 {
		fmt.Fprintf(os.Stderr, "No process named %q found.\n", *name)
		os.Exit(1)
	}

	if !*all {
		// Only first PID
		fmt.Printf("%d\n", pids[0])
		if *verbose {
			fmt.Printf("Process: %s (PID %d)\n", *name, pids[0])
			if len(pids) > 1 {
				fmt.Fprintf(os.Stderr, "Warning: %d more processes with same name (use -all to see all)\n", len(pids)-1)
			}
		}
	} else {
		// All PIDs
		for _, pid := range pids {
			if *verbose {
				fmt.Printf("%d\t%s\n", pid, *name)
			} else {
				fmt.Printf("%d\n", pid)
			}
		}
	}

	os.Exit(exitCode)
}

// findPIDsByName returns a slice of PIDs for all processes with the given name.
func findPIDsByName(name string) ([]int, error) {
	switch runtime.GOOS {
	case "windows":
		return findPIDsByNameWindows(name)
	case "linux", "darwin", "freebsd", "netbsd", "openbsd":
		return findPIDsByNameUnix(name)
	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func findPIDsByNameWindows(name string) ([]int, error) {
	cmd := exec.Command("tasklist", "/FO", "CSV", "/NH")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute tasklist: %w", err)
	}

	lines := strings.Split(string(out), "\n")
	var pids []int

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// CSV format: "image name","pid","session name","session#","mem usage"
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}
		imageName := strings.Trim(parts[0], "\"")
		pidStr := strings.Trim(parts[1], "\"")

		// Case‑insensitive match on Windows
		if strings.EqualFold(imageName, name) {
			pid, err := strconv.Atoi(pidStr)
			if err == nil {
				pids = append(pids, pid)
			}
		}
	}
	return pids, nil
}

func findPIDsByNameUnix(name string) ([]int, error) {
	// Try pgrep first (most common)
	cmd := exec.Command("pgrep", "-x", name) // -x for exact match
	out, err := cmd.Output()
	if err == nil {
		// pgrep succeeded – parse output
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		var pids []int
		for _, line := range lines {
			if line == "" {
				continue
			}
			pid, err := strconv.Atoi(line)
			if err == nil {
				pids = append(pids, pid)
			}
		}
		return pids, nil
	}

	// Fallback: ps aux + grep
	cmd = exec.Command("sh", "-c", fmt.Sprintf("ps -eo pid,comm | grep -E '\\s%s$'", name))
	out, err = cmd.Output()
	if err != nil {
		// No matches
		return []int{}, nil
	}

	lines := strings.Split(string(out), "\n")
	var pids []int
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 1 {
			pid, err := strconv.Atoi(fields[0])
			if err == nil {
				pids = append(pids, pid)
			}
		}
	}
	return pids, nil
}

func init() {
	// Custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Find process IDs by name, or show own PID.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -name notepad.exe          # show first PID\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -name chrome -all         # show all matching PIDs\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -pid -v                   # show own PID with details\n", os.Args[0])
	}
}
