// Package level provides a simple logging level type with common levels
// and helper functions for comparison and parsing.
package testutils

import (
	"fmt"
	"strings"
)

// Level represents a logging level.
type Level int

// Standard logging levels.
const (
	Debug Level = -4
	Info  Level = 0
	Warn  Level = 4
	Error Level = 8
	Fatal Level = 12
)

// String returns the string representation of the level.
func (l Level) String() string {
	switch l {
	case Debug:
		return "DEBUG"
	case Info:
		return "INFO"
	case Warn:
		return "WARN"
	case Error:
		return "ERROR"
	case Fatal:
		return "FATAL"
	default:
		return fmt.Sprintf("LEVEL(%d)", l)
	}
}

// ParseLevel parses a string into a Level.
// Recognized strings: debug, info, warn, error, fatal (case‑insensitive).
// Returns an error if the string is not recognized.
func ParseLevel(s string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return Debug, nil
	case "info":
		return Info, nil
	case "warn", "warning":
		return Warn, nil
	case "error":
		return Error, nil
	case "fatal":
		return Fatal, nil
	default:
		return Info, fmt.Errorf("unknown level: %q", s)
	}
}

// Enabled reports whether the current level is enabled for the given target level.
// For example, if l = Info, then l.Enabled(Debug) returns false, but l.Enabled(Error) returns true.
func (l Level) Enabled(target Level) bool {
	return l <= target
}

// Min returns the minimum (lowest) level among two.
func Min(a, b Level) Level {
	if a < b {
		return a
	}
	return b
}

// Max returns the maximum (highest) level among two.
func Max(a, b Level) Level {
	if a > b {
		return a
	}
	return b
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     lvl := level.Info
//     fmt.Println(lvl) // "INFO"
//     fmt.Println(lvl.Enabled(level.Debug)) // false
//     fmt.Println(lvl.Enabled(level.Error)) // true
// }