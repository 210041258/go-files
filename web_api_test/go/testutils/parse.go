// Package parse provides utilities for parsing strings into common Go types.
// It includes functions with default values and Must variants for simplicity.
package testutils

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ----------------------------------------------------------------------
// Basic types with defaults
// ----------------------------------------------------------------------

// Int parses s as an int. If parsing fails, it returns the default value.
func Int(s string, defaultValue int) int {
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return defaultValue
}

// MustInt parses s as an int. It panics on error.
func MustInt(s string) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		panic(fmt.Errorf("parse.Int: %w", err))
	}
	return v
}

// Int64 parses s as an int64. If parsing fails, it returns the default value.
func Int64(s string, defaultValue int64) int64 {
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return v
	}
	return defaultValue
}

// MustInt64 parses s as an int64. It panics on error.
func MustInt64(s string) int64 {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		panic(fmt.Errorf("parse.Int64: %w", err))
	}
	return v
}

// Uint parses s as a uint. If parsing fails, it returns the default value.
func Uint(s string, defaultValue uint) uint {
	if v, err := strconv.ParseUint(s, 10, 64); err == nil {
		return uint(v)
	}
	return defaultValue
}

// MustUint parses s as a uint. It panics on error.
func MustUint(s string) uint {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		panic(fmt.Errorf("parse.Uint: %w", err))
	}
	return uint(v)
}

// Float64 parses s as a float64. If parsing fails, it returns the default value.
func Float64(s string, defaultValue float64) float64 {
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v
	}
	return defaultValue
}

// MustFloat64 parses s as a float64. It panics on error.
func MustFloat64(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		panic(fmt.Errorf("parse.Float64: %w", err))
	}
	return v
}

// Bool parses s as a bool (accepts 1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False).
// If parsing fails, it returns the default value.
func Bool(s string, defaultValue bool) bool {
	if v, err := strconv.ParseBool(s); err == nil {
		return v
	}
	return defaultValue
}

// MustBool parses s as a bool. It panics on error.
func MustBool(s string) bool {
	v, err := strconv.ParseBool(s)
	if err != nil {
		panic(fmt.Errorf("parse.Bool: %w", err))
	}
	return v
}

// Duration parses s as a time.Duration (using time.ParseDuration).
// If parsing fails, it returns the default value.
func Duration(s string, defaultValue time.Duration) time.Duration {
	if v, err := time.ParseDuration(s); err == nil {
		return v
	}
	return defaultValue
}

// MustDuration parses s as a time.Duration. It panics on error.
func MustDuration(s string) time.Duration {
	v, err := time.ParseDuration(s)
	if err != nil {
		panic(fmt.Errorf("parse.Duration: %w", err))
	}
	return v
}

// ----------------------------------------------------------------------
// Byte size parsing (e.g., "10MB", "1.5GiB")
// ----------------------------------------------------------------------

var byteUnits = []struct {
	suffix  string
	multiplier uint64
}{
	{"KB", 1000}, {"MB", 1000 * 1000}, {"GB", 1000 * 1000 * 1000},
	{"TB", 1000 * 1000 * 1000 * 1000}, {"PB", 1000 * 1000 * 1000 * 1000 * 1000},
	{"KiB", 1024}, {"MiB", 1024 * 1024}, {"GiB", 1024 * 1024 * 1024},
	{"TiB", 1024 * 1024 * 1024 * 1024}, {"PiB", 1024 * 1024 * 1024 * 1024 * 1024},
	{"K", 1000}, {"M", 1000 * 1000}, {"G", 1000 * 1000 * 1000},
	{"T", 1000 * 1000 * 1000 * 1000}, {"P", 1000 * 1000 * 1000 * 1000 * 1000},
}

// ByteSize parses a string representing a byte size (e.g., "10MB", "1.5GiB").
// If parsing fails, it returns the default value.
func ByteSize(s string, defaultValue uint64) uint64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return defaultValue
	}
	for _, unit := range byteUnits {
		if strings.HasSuffix(s, unit.suffix) {
			numPart := strings.TrimSuffix(s, unit.suffix)
			numPart = strings.TrimSpace(numPart)
			if v, err := strconv.ParseFloat(numPart, 64); err == nil {
				return uint64(v * float64(unit.multiplier))
			}
			break
		}
	}
	// No unit suffix, try plain number
	if v, err := strconv.ParseUint(s, 10, 64); err == nil {
		return v
	}
	return defaultValue
}

// MustByteSize parses a byte size string. It panics on error.
func MustByteSize(s string) uint64 {
	s = strings.TrimSpace(s)
	if s == "" {
		panic("parse.ByteSize: empty string")
	}
	for _, unit := range byteUnits {
		if strings.HasSuffix(s, unit.suffix) {
			numPart := strings.TrimSuffix(s, unit.suffix)
			numPart = strings.TrimSpace(numPart)
			if v, err := strconv.ParseFloat(numPart, 64); err == nil {
				return uint64(v * float64(unit.multiplier))
			}
			panic(fmt.Errorf("parse.ByteSize: invalid number part %q", numPart))
		}
	}
	// No unit suffix, try plain number
	if v, err := strconv.ParseUint(s, 10, 64); err == nil {
		return v
	}
	panic(fmt.Errorf("parse.ByteSize: invalid format %q", s))
}

// ----------------------------------------------------------------------
// Key-value parsing
// ----------------------------------------------------------------------

// KeyValue splits a string by the first occurrence of sep into key and value.
// If sep is not found, it returns the whole string as key and empty value.
func KeyValue(s, sep string) (key, value string) {
	idx := strings.Index(s, sep)
	if idx == -1 {
		return strings.TrimSpace(s), ""
	}
	return strings.TrimSpace(s[:idx]), strings.TrimSpace(s[idx+len(sep):])
}

// KeyValueMap parses a slice of strings in "key=value" format into a map.
// Lines without the separator are ignored.
func KeyValueMap(lines []string, sep string) map[string]string {
	m := make(map[string]string)
	for _, line := range lines {
		if key, val := KeyValue(line, sep); key != "" {
			m[key] = val
		}
	}
	return m
}

// ----------------------------------------------------------------------
// List parsing
// ----------------------------------------------------------------------

// CSV splits a comma-separated string into a slice of trimmed strings.
// Empty elements are omitted.
func CSV(s string) []string {
	return List(s, ",")
}

// List splits a string by sep into a slice of trimmed strings.
// Empty elements are omitted.
func List(s, sep string) []string {
	parts := strings.Split(s, sep)
	res := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			res = append(res, trimmed)
		}
	}
	return res
}

// ----------------------------------------------------------------------
// Environment variable style parsing (simple)
// ----------------------------------------------------------------------

// EnvDefault returns the value of an environment variable or a default if not set.
// This is a convenience wrapper around os.Getenv.
func EnvDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     i := parse.Int("42", 0) // 42
//     b := parse.Bool("yes", false) // false (because "yes" is not a valid bool)
//     dur := parse.Duration("10m", 5*time.Second) // 10 minutes
//     size := parse.ByteSize("1.5GB", 0) // 1500000000
//     key, val := parse.KeyValue("a=b", "=") // "a", "b"
//     list := parse.CSV("a, b, c") // ["a", "b", "c"]
// }