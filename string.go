// Package strutil provides string manipulation utilities not found in
// the standard strings package. It includes functions for truncation,
// padding, reversal, and common checks.
package testutils

import (
	"strings"
	"unicode"
)

// ----------------------------------------------------------------------
// Basic checks
// ----------------------------------------------------------------------

// IsBlank reports whether a string contains only whitespace.
func IsBlank(s string) bool {
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// IsNotBlank reports whether a string contains at least one non‑space character.
func IsNotBlank(s string) bool {
	return !IsBlank(s)
}

// IsEmpty reports whether the string is empty.
func IsEmpty(s string) bool {
	return len(s) == 0
}

// IsNotEmpty reports whether the string is not empty.
func IsNotEmpty(s string) bool {
	return len(s) > 0
}

// IsNumeric reports whether the string consists only of numeric characters.
func IsNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// IsAlpha reports whether the string consists only of letters (Unicode).
func IsAlpha(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// IsAlphanumeric reports whether the string consists only of letters and digits.
func IsAlphanumeric(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// ----------------------------------------------------------------------
// Truncation and ellipsis
// ----------------------------------------------------------------------

// Truncate truncates a string to the given length (measured in runes).
// If the string is longer, it is cut to length and no suffix is added.
func Truncate(s string, length int) string {
	runes := []rune(s)
	if len(runes) <= length {
		return s
	}
	return string(runes[:length])
}

// Ellipsis truncates a string and appends "..." if it was truncated.
// The total length includes the ellipsis. For example, Ellipsis("foobar", 5) -> "fo...".
func Ellipsis(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if maxLen <= 3 {
		return strings.Repeat(".", maxLen)
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	ellipsis := "..."
	// Leave room for ellipsis.
	return string(runes[:maxLen-3]) + ellipsis
}

// ----------------------------------------------------------------------
// Padding
// ----------------------------------------------------------------------

// PadLeft pads the string on the left to reach the target length using the given pad character.
func PadLeft(s string, length int, pad rune) string {
	runes := []rune(s)
	if len(runes) >= length {
		return s
	}
	return strings.Repeat(string(pad), length-len(runes)) + s
}

// PadRight pads the string on the right.
func PadRight(s string, length int, pad rune) string {
	runes := []rune(s)
	if len(runes) >= length {
		return s
	}
	return s + strings.Repeat(string(pad), length-len(runes))
}

// Pad pads the string equally on both sides, favouring the right if padding is odd.
func Pad(s string, length int, pad rune) string {
	runes := []rune(s)
	if len(runes) >= length {
		return s
	}
	total := length - len(runes)
	left := total / 2
	right := total - left
	return strings.Repeat(string(pad), left) + s + strings.Repeat(string(pad), right)
}

// ----------------------------------------------------------------------
// Reversal
// ----------------------------------------------------------------------

// Reverse returns the string with its Unicode runes reversed.
func Reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// ----------------------------------------------------------------------
// Splitting and joining
// ----------------------------------------------------------------------

// SplitLines splits a string into lines, trimming any trailing newline.
func SplitLines(s string) []string {
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}

// JoinLines joins lines with a newline.
func JoinLines(lines []string) string {
	return strings.Join(lines, "\n")
}

// SplitAndTrim splits a string by a separator and trims whitespace from each element.
// Empty elements are retained.
func SplitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}

// SplitAndTrimEmpty splits a string by a separator, trims whitespace,
// and removes empty elements.
func SplitAndTrimEmpty(s, sep string) []string {
	parts := SplitAndTrim(s, sep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// ----------------------------------------------------------------------
// Substring
// ----------------------------------------------------------------------

// Substring returns a substring of the given string by rune index.
// start is inclusive, end is exclusive. If end is negative, it counts from the end.
// If start is out of range, it returns an empty string.
func Substring(s string, start, end int) string {
	runes := []rune(s)
	total := len(runes)
	if start < 0 {
		start = total + start
	}
	if end < 0 {
		end = total + end
	}
	if start < 0 {
		start = 0
	}
	if end > total {
		end = total
	}
	if start >= end || start >= total || end <= 0 {
		return ""
	}
	return string(runes[start:end])
}

// ----------------------------------------------------------------------
// Replacements
// ----------------------------------------------------------------------

// ReplaceAllInMap replaces all occurrences of keys in the map with their values.
// It processes the string in a single pass, not overlapping.
func ReplaceAllInMap(s string, repl map[string]string) string {
	if len(repl) == 0 {
		return s
	}
	// Build a strings.Replacer from the map.
	pairs := make([]string, 0, len(repl)*2)
	for k, v := range repl {
		pairs = append(pairs, k, v)
	}
	return strings.NewReplacer(pairs...).Replace(s)
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     fmt.Println(strutil.IsBlank("  \t\n"))            // true
//     fmt.Println(strutil.Ellipsis("hello world", 8))   // "hello..."
//     fmt.Println(strutil.Reverse("hello"))             // "olleh"
//     fmt.Println(strutil.PadLeft("42", 5, '0'))        // "00042"
//     fmt.Println(strutil.Substring("hello world", 0, 5)) // "hello"
// }