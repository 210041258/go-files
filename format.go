// Package format provides utilities for formatting values into human‑readable strings.
// It includes functions for numbers, byte sizes, durations, percentages, and more.
package testutils

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// ----------------------------------------------------------------------
// Number formatting
// ----------------------------------------------------------------------

// Comma formats an integer with thousands separators.
// For example, Comma(1234567) returns "1,234,567".
func Comma(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	return strings.Join(parts, ",")
}

// CommaUint formats a uint64 with thousands separators.
func CommaUint(n uint64) string {
	return Comma(int64(n))
}

// Percent formats a float64 as a percentage with the specified precision.
// Example: Percent(0.1234, 1) returns "12.3%".
func Percent(f float64, precision int) string {
	return strconv.FormatFloat(f*100, 'f', precision, 64) + "%"
}

// ----------------------------------------------------------------------
// Byte size formatting
// ----------------------------------------------------------------------

// Bytes converts a size in bytes to a human‑readable string (e.g., "1.5 MB").
// It uses binary (IEC) units: KiB, MiB, GiB, TiB, PiB, EiB.
// If the size is negative, it returns "0 B".
func Bytes(bytes int64) string {
	if bytes < 0 {
		return "0 B"
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	// Format with appropriate unit.
	units := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	f := float64(bytes) / float64(div)
	return fmt.Sprintf("%.1f %s", f, units[exp+1])
}

// BytesSI converts a size to a human‑readable string using decimal (SI) units:
// KB, MB, GB, etc. (1 KB = 1000 bytes).
func BytesSI(bytes int64) string {
	if bytes < 0 {
		return "0 B"
	}
	const unit = 1000
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}
	f := float64(bytes) / float64(div)
	return fmt.Sprintf("%.1f %s", f, units[exp+1])
}

// ----------------------------------------------------------------------
// Duration formatting
// ----------------------------------------------------------------------

// Duration formats a time.Duration into a human‑readable string,
// showing only the most significant non‑zero components (up to two).
// For example, 2h3m4s becomes "2h3m", 45m becomes "45m".
func Duration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	sign := ""
	if d < 0 {
		sign = "-"
		d = -d
	}
	// Extract components.
	hours := int(d / time.Hour)
	d -= time.Duration(hours) * time.Hour
	minutes := int(d / time.Minute)
	d -= time.Duration(minutes) * time.Minute
	seconds := int(d / time.Second)
	d -= time.Duration(seconds) * time.Second
	millis := int(d / time.Millisecond)

	// Build the string, taking the first two non‑zero components.
	parts := make([]string, 0, 2)
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 && len(parts) < 2 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if seconds > 0 && len(parts) < 2 && minutes == 0 { // show seconds only if no minutes shown
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}
	if millis > 0 && len(parts) == 0 && hours == 0 && minutes == 0 && seconds == 0 {
		parts = append(parts, fmt.Sprintf("%dms", millis))
	}
	if len(parts) == 0 {
		// Fallback to original string if nothing matched.
		return d.String()
	}
	return sign + strings.Join(parts, "")
}

// ----------------------------------------------------------------------
// String formatting
// ----------------------------------------------------------------------

// Ordinal returns the ordinal suffix for a number (1st, 2nd, 3rd, etc.).
func Ordinal(n int) string {
	suffix := "th"
	switch n % 10 {
	case 1:
		if n%100 != 11 {
			suffix = "st"
		}
	case 2:
		if n%100 != 12 {
			suffix = "nd"
		}
	case 3:
		if n%100 != 13 {
			suffix = "rd"
		}
	}
	return fmt.Sprintf("%d%s", n, suffix)
}

// Plural returns a word pluralized based on the count.
// For example: Plural(1, "apple", "apples") returns "apple".
func Plural(count int, singular, plural string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}

// Mask masks a string by replacing all but the last few characters with a mask character.
// For example: Mask("1234567890", 4, '*') returns "******7890".
func Mask(s string, visible int, mask rune) string {
	if visible <= 0 || len(s) <= visible {
		return s
	}
	masked := strings.Repeat(string(mask), len(s)-visible)
	return masked + s[len(s)-visible:]
}

// MaskLeft masks the left side of a string.
func MaskLeft(s string, visible int, mask rune) string {
	if visible <= 0 || len(s) <= visible {
		return s
	}
	return s[:visible] + strings.Repeat(string(mask), len(s)-visible)
}

// Truncate truncates a string to the given length (in runes) and adds an ellipsis if needed.
// The total length includes the ellipsis. For example, Truncate("foobar", 5) returns "fo...".
func Truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return strings.Repeat(".", maxLen)
	}
	return string(runes[:maxLen-3]) + "..."
}

// ----------------------------------------------------------------------
// JSON formatting
// ----------------------------------------------------------------------

// JSONPretty pretty‑prints a JSON string with indentation.
// If the input is not valid JSON, it returns an error.
func JSONPretty(jsonStr string, indent string) (string, error) {
	var v any
	err := json.Unmarshal([]byte(jsonStr), &v)
	if err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(v, "", indent)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     fmt.Println(format.Comma(1234567))          // "1,234,567"
//     fmt.Println(format.Bytes(123456789))        // "117.7 MiB"
//     fmt.Println(format.BytesSI(123456789))      // "123.5 MB"
//     fmt.Println(format.Duration(2*time.Hour + 3*time.Minute + 4*time.Second)) // "2h3m"
//     fmt.Println(format.Ordinal(42))             // "42nd"
//     fmt.Println(format.Plural(5, "apple", "apples")) // "5 apples"
//     fmt.Println(format.Mask("1234567890", 4, '*'))    // "******7890"
//     fmt.Println(format.Truncate("hello world", 8))    // "hello..."
// }