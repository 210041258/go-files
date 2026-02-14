// Package char provides utilities for character and string manipulation.
// It extends the standard strings and unicode packages with common operations
// like slugification, case conversion, random string generation, and validation.
package testutils

import (
	"math/rand"
	"regexp"
	"strings"
	"time"
	"unicode"
)

var (
	// DefaultRandSource is the random source used by Random functions.
	// It is seeded with the current time at init.
	DefaultRandSource = rand.NewSource(time.Now().UnixNano())
	defaultRand       = rand.New(DefaultRandSource)

	// Common character sets for random string generation.
	Digits         = "0123456789"
	HexDigits      = "0123456789abcdef"
	UpperLetters   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	LowerLetters   = "abcdefghijklmnopqrstuvwxyz"
	Letters        = UpperLetters + LowerLetters
	Alphanumeric   = Digits + Letters
	Punctuation    = "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"
	ASCII          = Alphanumeric + Punctuation
)

// ----------------------------------------------------------------------
// Random string generation
// ----------------------------------------------------------------------

// Random returns a random string of length n using characters from the given set.
// If charset is empty, Alphanumeric is used.
func Random(n int, charset string) string {
	if n <= 0 {
		return ""
	}
	if charset == "" {
		charset = Alphanumeric
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[defaultRand.Intn(len(charset))]
	}
	return string(b)
}

// RandomDigits returns a random numeric string of length n.
func RandomDigits(n int) string {
	return Random(n, Digits)
}

// RandomHex returns a random hexadecimal string of length n.
func RandomHex(n int) string {
	return Random(n, HexDigits)
}

// RandomLetters returns a random alphabetic string of length n.
func RandomLetters(n int) string {
	return Random(n, Letters)
}

// ----------------------------------------------------------------------
// Slugification
// ----------------------------------------------------------------------

var (
	// Regexp for characters that are allowed in a slug.
	slugAllowed = regexp.MustCompile(`[^a-z0-9-]+`)
	// Multiple dashes are collapsed to one.
	multiDash = regexp.MustCompile(`-+`)
)

// Slug converts a string into a URL‑friendly slug.
// It converts to lowercase, replaces spaces and punctuation with dashes,
// and removes any characters that are not alphanumeric or dash.
func Slug(s string) string {
	// Convert to lowercase and replace spaces and underscores with dashes.
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '_' {
			return '-'
		}
		return r
	}, s)

	// Remove all non‑allowed characters.
	s = slugAllowed.ReplaceAllString(s, "")
	// Collapse multiple dashes.
	s = multiDash.ReplaceAllString(s, "-")
	// Trim leading and trailing dashes.
	s = strings.Trim(s, "-")
	if s == "" {
		// Fallback if everything was stripped.
		return "n-a"
	}
	return s
}

// ----------------------------------------------------------------------
// Case conversion
// ----------------------------------------------------------------------

// ToSnakeCase converts a camelCase or PascalCase string to snake_case.
// Example: "UserID" -> "user_id", "HTTPRequest" -> "http_request".
func ToSnakeCase(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s) + 4) // approximate
	for i, r := range s {
		if unicode.IsUpper(r) {
			// Add underscore before the uppercase letter if it's not the first character
			// and the previous character is not uppercase (to handle "HTTP" -> "http").
			if i > 0 && !unicode.IsUpper(rune(s[i-1])) {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ToCamelCase converts a snake_case string to camelCase.
// Example: "user_id" -> "userId", "http_request" -> "httpRequest".
func ToCamelCase(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	upperNext := false
	for i, r := range s {
		if r == '_' {
			upperNext = true
			continue
		}
		if upperNext || i == 0 {
			b.WriteRune(unicode.ToUpper(r))
			upperNext = false
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ToPascalCase converts a snake_case string to PascalCase (CamelCase with first letter uppercase).
// Example: "user_id" -> "UserId", "http_request" -> "HttpRequest".
func ToPascalCase(s string) string {
	if s == "" {
		return ""
	}
	camel := ToCamelCase(s)
	return strings.ToUpper(camel[:1]) + camel[1:]
}

// ----------------------------------------------------------------------
// Truncation
// ----------------------------------------------------------------------

// Truncate shortens a string to n runes, adding a suffix if truncation occurred.
// If n <= 3, the suffix is not added.
func Truncate(s string, n int, suffix string) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	if n <= 3 {
		return string(runes[:n])
	}
	return string(runes[:n-len(suffix)]) + suffix
}

// TruncateBytes truncates a byte slice to n bytes, ensuring valid UTF‑8.
// If the slice is cut in the middle of a multi‑byte character, that character
// is omitted entirely.
func TruncateBytes(b []byte, n int) []byte {
	if n <= 0 || len(b) <= n {
		return b
	}
	// Fast path: all bytes are ASCII.
	for i := 0; i < n; i++ {
		if b[i] >= 128 {
			goto slow
		}
	}
	return b[:n]
slow:
	// Scan backwards to find the start of the last complete character.
	// We start at n-1 and move left until we find a byte that is not a continuation byte.
	end := n - 1
	for end >= 0 && b[end]&0xC0 == 0x80 { // continuation byte
		end--
	}
	if end < 0 {
		return []byte{}
	}
	return b[:end+1]
}

// ----------------------------------------------------------------------
// Character class checks
// ----------------------------------------------------------------------

// IsASCII reports whether the string consists only of ASCII characters.
func IsASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > unicode.MaxASCII {
			return false
		}
	}
	return true
}

// IsAlphanumeric reports whether the string contains only letters and digits.
func IsAlphanumeric(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// IsPrintable reports whether all runes in the string are printable
// (including space).
func IsPrintable(s string) bool {
	for _, r := range s {
		if !unicode.IsPrint(r) && r != '\t' && r != '\n' {
			return false
		}
	}
	return true
}

// ContainsOnly reports whether the string contains only characters from the given set.
func ContainsOnly(s, charset string) bool {
	for _, r := range s {
		if !strings.ContainsRune(charset, r) {
			return false
		}
	}
	return true
}

// ----------------------------------------------------------------------
// Padding
// ----------------------------------------------------------------------

// PadLeft pads the string on the left side with the given pad character
// to reach the target length.
func PadLeft(s string, length int, pad rune) string {
	runes := []rune(s)
	if len(runes) >= length {
		return s
	}
	pads := strings.Repeat(string(pad), length-len(runes))
	return pads + s
}

// PadRight pads the string on the right side.
func PadRight(s string, length int, pad rune) string {
	runes := []rune(s)
	if len(runes) >= length {
		return s
	}
	pads := strings.Repeat(string(pad), length-len(runes))
	return s + pads
}

// Pad pads the string equally on both sides, favouring the right side
// if the total padding amount is odd.
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
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     fmt.Println(char.Slug("Hello, World!"))      // "hello-world"
//     fmt.Println(char.RandomLetters(10))          // "aB3kLpQrSt"
//     fmt.Println(char.ToSnakeCase("UserID"))     // "user_id"
//     fmt.Println(char.ToCamelCase("user_id"))    // "userId"
//     fmt.Println(char.Truncate("foobar", 4, "...")) // "f..."
// }