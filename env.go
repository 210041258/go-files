// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// LoadEnvFile reads a .env file and sets environment variables.
// Lines in the format KEY=VALUE are processed; comments (starting with #)
// and empty lines are ignored. Values are not expanded.
// Returns the first error encountered, if any.
func LoadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue // ignore malformed lines
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		// Remove surrounding quotes if present (simple handling)
		if len(value) > 1 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// GetEnv retrieves the value of an environment variable, or returns the
// fallback if the variable is not set or empty.
func GetEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// MustGetEnv retrieves the value of an environment variable and panics if it
// is not set or empty. Useful for required configuration in tests.
func MustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic("required environment variable " + key + " is not set")
	}
	return v
}

// SetEnv temporarily sets an environment variable for the duration of a test.
// It returns a cleanup function that restores the original value.
// Usage:
//   cleanup := SetEnv(t, "KEY", "value")
//   defer cleanup()
func SetEnv(t testing.TB, key, value string) func() {
	t.Helper()
	orig, exists := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("failed to set env %s: %v", key, err)
	}
	return func() {
		if exists {
			os.Setenv(key, orig)
		} else {
			os.Unsetenv(key)
		}
	}
}

// IsCI checks common environment variables to detect if the test is running
// in a continuous integration environment.
func IsCI() bool {
	// Common CI providers set these environment variables.
	ciVars := []string{
		"CI",                 // GitHub Actions, Travis, CircleCI, GitLab CI, etc.
		"CONTINUOUS_INTEGRATION", // many
		"BUILD_NUMBER",       // Jenkins
		"TEAMCITY_VERSION",   // TeamCity
		"TF_BUILD",           // Azure Pipelines
	}
	for _, v := range ciVars {
		if os.Getenv(v) != "" {
			return true
		}
	}
	return false
}

// IsTest reports whether the program is running in a test context.
// It checks if the executable name ends with ".test" (typical for go test).
func IsTest() bool {
	return strings.HasSuffix(os.Args[0], ".test")
}