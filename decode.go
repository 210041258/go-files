// Package decode provides utilities for decoding strings and bytes
// from common encodings such as base64, hex, and URL encoding.
// It includes functions with default fallbacks and Must variants for testing.
package testutils

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ----------------------------------------------------------------------
// Base64 decoding
// ----------------------------------------------------------------------

// Base64 decodes a base64-encoded string (standard encoding with padding).
// If decoding fails, it returns an empty slice and the error.
func Base64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// Base64Default decodes a base64 string; on error it returns the default value.
func Base64Default(s string, defaultValue []byte) []byte {
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b
	}
	return defaultValue
}

// MustBase64 decodes a base64 string and panics on error.
func MustBase64(s string) []byte {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(fmt.Sprintf("decode.MustBase64: %v", err))
	}
	return b
}

// Base64URL decodes a URL‑safe base64 string (without padding).
func Base64URL(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

// Base64URLDefault decodes a URL‑safe base64 string with a default fallback.
func Base64URLDefault(s string, defaultValue []byte) []byte {
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return b
	}
	return defaultValue
}

// MustBase64URL decodes a URL‑safe base64 string and panics on error.
func MustBase64URL(s string) []byte {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		panic(fmt.Sprintf("decode.MustBase64URL: %v", err))
	}
	return b
}

// ----------------------------------------------------------------------
// Hex decoding
// ----------------------------------------------------------------------

// Hex decodes a hexadecimal string.
func Hex(s string) ([]byte, error) {
	return hex.DecodeString(s)
}

// HexDefault decodes a hex string; on error returns the default value.
func HexDefault(s string, defaultValue []byte) []byte {
	if b, err := hex.DecodeString(s); err == nil {
		return b
	}
	return defaultValue
}

// MustHex decodes a hex string and panics on error.
func MustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(fmt.Sprintf("decode.MustHex: %v", err))
	}
	return b
}

// ----------------------------------------------------------------------
// URL decoding
// ----------------------------------------------------------------------

// URL decodes a URL‑encoded string (percent‑encoding).
func URL(s string) (string, error) {
	return url.QueryUnescape(s)
}

// URLDefault decodes a URL‑encoded string with a default fallback.
func URLDefault(s string, defaultValue string) string {
	if u, err := url.QueryUnescape(s); err == nil {
		return u
	}
	return defaultValue
}

// MustURL decodes a URL‑encoded string and panics on error.
func MustURL(s string) string {
	u, err := url.QueryUnescape(s)
	if err != nil {
		panic(fmt.Sprintf("decode.MustURL: %v", err))
	}
	return u
}

// ----------------------------------------------------------------------
// Binary / ASCII / UTF‑8 detection
// ----------------------------------------------------------------------

// IsASCII reports whether the byte slice contains only ASCII characters.
func IsASCII(b []byte) bool {
	for _, c := range b {
		if c > 127 {
			return false
		}
	}
	return true
}

// IsUTF8 reports whether the byte slice is valid UTF‑8.
func IsUTF8(b []byte) bool {
	return utf8.Valid(b)
}

// ----------------------------------------------------------------------
// String unescaping (like Go string literals)
// ----------------------------------------------------------------------

// Unquote unescapes a string that may contain Go‑style escape sequences
// (e.g., \n, \t, \", \\). It is a wrapper around strconv.Unquote.
// If the string is not quoted, it adds double quotes before unquoting.
func Unquote(s string) (string, error) {
	// If already quoted, use as is.
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return strconv.Unquote(s)
	}
	// Otherwise, add quotes and attempt unquote.
	return strconv.Unquote(`"` + s + `"`)
}

// UnquoteDefault unquotes with a default fallback.
func UnquoteDefault(s string, defaultValue string) string {
	if u, err := Unquote(s); err == nil {
		return u
	}
	return defaultValue
}

// MustUnquote unquotes and panics on error.
func MustUnquote(s string) string {
	u, err := Unquote(s)
	if err != nil {
		panic(fmt.Sprintf("decode.MustUnquote: %v", err))
	}
	return u
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     b := decode.MustBase64("SGVsbG8gV29ybGQ=")
//     fmt.Println(string(b)) // "Hello World"
//
//     h := decode.MustHex("48656c6c6f")
//     fmt.Println(string(h)) // "Hello"
//
//     u := decode.MustURL("Hello%20World")
//     fmt.Println(u) // "Hello World"
//
//     quoted := decode.MustUnquote(`\"Hello\"\nWorld`)
//     fmt.Println(quoted) // "Hello"\nWorld
// }