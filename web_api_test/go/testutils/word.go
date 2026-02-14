// Package testutils provides advanced text, string, and word processing utilities.
// It includes high-performance normalization, secure string generation, and
// case conversion utilities.
package testutils

import (
	"bytes"
	"crypto/rand"
	"errors"
	"math/big"
	"regexp"
	"unicode"
	"unicode/utf8"
)

// --------------------------------------------------------------------
// Constants for Character Sets
// --------------------------------------------------------------------

const (
	// AlphaChars contains lowercase and uppercase ASCII letters.
	AlphaChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	// NumericChars contains ASCII digits.
	NumericChars = "0123456789"

	// AlphaNumeric contains Alphanumerics.
	AlphaNumeric = AlphaChars + NumericChars

	// SafeChars contains characters safe for filenames and URLs (alphanumeric + hyphen + underscore).
	SafeChars = AlphaNumeric + "-_"

	// LowerHex contains lowercase hexadecimal characters.
	LowerHex = "0123456789abcdef"
)

var (
	// Common regex patterns (compiled once for performance in validation functions).
	// Note: Slugify and Case conversion below use manual loops for higher performance.
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
)

// --------------------------------------------------------------------
// Random String Generation
// --------------------------------------------------------------------

// RandomString generates a cryptographically secure random string of length n
// using the provided character set.
// Use this for passwords, API keys, and tokens.
func RandomString(charset string, n int) (string, error) {
	if n <= 0 {
		return "", errors.New("word: length must be positive")
	}
	if len(charset) == 0 {
		return "", errors.New("word: charset cannot be empty")
	}

	b := make([]byte, n)
	max := big.NewInt(int64(len(charset)))

	for i := range b {
		// crypto/rand.Int is slow but secure. Suitable for secrets.
		idx, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b[i] = charset[idx.Int64()]
	}

	return string(b), nil
}

// RandomAlphaNumeric generates a secure random alphanumeric string.
func RandomAlphaNumeric(n int) (string, error) {
	return RandomString(AlphaNumeric, n)
}

// --------------------------------------------------------------------
// Normalization & Transformation
// --------------------------------------------------------------------

// Slugify converts a string to a URL-friendly slug.
// It replaces spaces with hyphens, removes special characters, and lowercases the result.
// High-performance manual implementation (no regex).
func Slugify(s string) string {
	var buf bytes.Buffer
	var prevDash bool

	for i := 0; i < len(s); i++ {
		r := rune(s[i])
		// Handle multi-byte runes properly
		if r >= utf8.RuneSelf {
			r, _ = utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError {
				continue
			}
			i += utf8.RuneLen(r) - 1
		}

		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			buf.WriteRune(unicode.ToLower(r))
			prevDash = false
		case unicode.IsSpace(r) || r == '-' || r == '_' || r == '/':
			if !prevDash {
				buf.WriteRune('-')
				prevDash = true
			}
		}
	}

	// Trim trailing dash
	result := buf.String()
	if len(result) > 0 && result[len(result)-1] == '-' {
		return result[:len(result)-1]
	}
	return result
}

// ToCamelCase converts a snake_case or kebab-case string to CamelCase.
// Example: "hello_world" -> "HelloWorld"
func ToCamelCase(s string) string {
	var buf bytes.Buffer
	capNext := true

	for i := 0; i < len(s); i++ {
		r := rune(s[i])
		if r >= utf8.RuneSelf {
			r, _ = utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError {
				continue
			}
			i += utf8.RuneLen(r) - 1
		}

		if r == '_' || r == '-' || r == ' ' {
			capNext = true
		} else if capNext {
			buf.WriteRune(unicode.ToUpper(r))
			capNext = false
		} else {
			buf.WriteRune(unicode.ToLower(r))
		}
	}
	return buf.String()
}

// ToSnakeCase converts a CamelCase string to snake_case.
// Example: "HelloWorld" -> "hello_world"
func ToSnakeCase(s string) string {
	var buf bytes.Buffer

	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				// Add underscore before capital if previous char was not capital/non-alnum
				prev := rune(s[i-1])
				if unicode.IsLower(prev) || unicode.IsDigit(prev) {
					buf.WriteRune('_')
				}
			}
			buf.WriteRune(unicode.ToLower(r))
		} else {
			// Replace spaces/hyphens with underscores
			if r == ' ' || r == '-' {
				buf.WriteRune('_')
			} else {
				buf.WriteRune(r)
			}
		}
	}
	return buf.String()
}

// Elide truncates a string to a maximum length and appends "..." if truncated.
// It respects UTF-8 character boundaries.
func Elide(s string, max int) string {
	if max < 3 {
		return "..." // Cannot fit content
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}

	runes := []rune(s)
	return string(runes[:max-3]) + "..."
}

// Truncate cuts a string off at a specific length without adding an ellipsis.
func Truncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max])
}

// --------------------------------------------------------------------
// Validation & Analysis
// --------------------------------------------------------------------

// IsEmail performs a basic validation check for an email address format.
func IsEmail(email string) bool {
	return emailRegex.MatchString(email)
}

// WordCount estimates the number of words in a string.
// It splits by whitespace.
func WordCount(s string) int {
	// Fast path for empty strings
	if len(s) == 0 {
		return 0
	}

	count := 0
	inWord := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if inWord {
				count++
				inWord = false
			}
		} else {
			inWord = true
		}
	}
	if inWord {
		count++
	}
	return count
}

// --------------------------------------------------------------------
// Manipulation
// --------------------------------------------------------------------

// Shuffle randomly shuffles the runes in a string using crypto/rand.
// This is computationally expensive for very long strings.
func Shuffle(s string) (string, error) {
	runes := []rune(s)
	n := len(runes)

	// Fisher-Yates shuffle
	for i := n - 1; i > 0; i-- {
		// Generate secure random number between 0 and i
		jBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", err
		}
		j := int(jBig.Int64())

		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes), nil
}

// Mask obscures a string by replacing all but the first 'keep' characters
// with the mask character (default '*').
func Mask(s string, keep int, maskChar rune) string {
	runes := []rune(s)
	if len(runes) <= keep {
		return s
	}
	for i := keep; i < len(runes); i++ {
		runes[i] = maskChar
	}
	return string(runes)
}

// Reverse reverses a string while respecting UTF-8 runes.
func Reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
