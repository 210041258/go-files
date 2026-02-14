// Package info provides system and runtime information utilities.
// It includes functions to retrieve hostname, operating system, architecture,
// Go version, process uptime, and environment variables.
package testutils

import (
	"os"
	"runtime"
	"time"
)

var startTime = time.Now()

// Hostname returns the system's hostname.
// If an error occurs, it returns "unknown".
func Hostname() string {
	name, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return name
}

// OS returns the operating system target (e.g., "linux", "windows").
func OS() string {
	return runtime.GOOS
}

// Arch returns the architecture target (e.g., "amd64", "arm64").
func Arch() string {
	return runtime.GOARCH
}

// GoVersion returns the Go version used to build the program.
func GoVersion() string {
	return runtime.Version()
}

// NumCPU returns the number of logical CPUs usable by the current process.
func NumCPU() int {
	return runtime.NumCPU()
}

// NumGoroutine returns the number of existing goroutines.
func NumGoroutine() int {
	return runtime.NumGoroutine()
}

// Uptime returns the duration since the process started.
func Uptime() time.Duration {
	return time.Since(startTime)
}

// PID returns the process ID of the current process.
func PID() int {
	return os.Getpid()
}

// PPID returns the parent process ID.
func PPID() int {
	return os.Getppid()
}

// Env retrieves the value of the environment variable named by the key.
// It returns the value and a boolean indicating whether the variable is present.
func Env(key string) (string, bool) {
	return os.LookupEnv(key)
}

// EnvOrDefault returns the value of the environment variable, or a default if not set.
func EnvOrDefault(key, defaultValue string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return defaultValue
}

// EnvMap returns all environment variables as a map.
func EnvMap() map[string]string {
	env := os.Environ()
	m := make(map[string]string, len(env))
	for _, e := range env {
		for i := 0; i < len(e); i++ {
			if e[i] == '=' {
				m[e[:i]] = e[i+1:]
				break
			}
		}
	}
	return m
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     fmt.Println("Hostname:", info.Hostname())
//     fmt.Println("OS:", info.OS())
//     fmt.Println("Arch:", info.Arch())
//     fmt.Println("Go version:", info.GoVersion())
//     fmt.Println("Uptime:", info.Uptime())
// }