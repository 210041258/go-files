// Package hexutil provides utilities for hexadecimal encoding and decoding.
// It extends the standard encoding/hex package with convenience functions
// for common tasks like converting between hex strings and integers,
// adding/removing prefixes, and validating hex strings.
package hexutils

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ----------------------------------------------------------------------
// Basic hex encoding/decoding
// ----------------------------------------------------------------------

// Encode returns the hexadecimal encoding of src.
func Encode(src []byte) string {
	return hex.EncodeToString(src)
}

// Decode returns the bytes represented by the hexadecimal string s.
// It supports both upper and lower case and accepts an optional "0x" prefix.
func Decode(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	return hex.DecodeString(s)
}

// MustDecode is like Decode but panics on error.
func MustDecode(s string) []byte {
	b, err := Decode(s)
	if err != nil {
		panic(err)
	}
	return b
}

// ----------------------------------------------------------------------
// Integer <-> hex string conversions
// ----------------------------------------------------------------------

// EncodeUint64 encodes a uint64 as a hex string with an optional "0x" prefix.
// The minDigits parameter pads the result with leading zeros to the specified width.
func EncodeUint64(v uint64, prefix bool, minDigits int) string {
	s := strconv.FormatUint(v, 16)
	if len(s) < minDigits {
		s = strings.Repeat("0", minDigits-len(s)) + s
	}
	if prefix {
		return "0x" + s
	}
	return s
}

// DecodeUint64 decodes a hex string (with or without "0x" prefix) into a uint64.
func DecodeUint64(s string) (uint64, error) {
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	return strconv.ParseUint(s, 16, 64)
}

// MustDecodeUint64 is like DecodeUint64 but panics on error.
func MustDecodeUint64(s string) uint64 {
	v, err := DecodeUint64(s)
	if err != nil {
		panic(err)
	}
	return v
}

// EncodeInt64 encodes an int64 as a hex string. Negative values are represented
// in two's complement with the specified number of bits (must be 8, 16, 32, or 64).
func EncodeInt64(v int64, bits int, prefix bool) (string, error) {
	switch bits {
	case 8, 16, 32, 64:
		// ok
	default:
		return "", fmt.Errorf("bits must be 8, 16, 32, or 64")
	}
	mask := uint64(1<<bits - 1)
	uv := uint64(v) & mask
	return EncodeUint64(uv, prefix, bits/4), nil
}

// MustEncodeInt64 is like EncodeInt64 but panics on error.
func MustEncodeInt64(v int64, bits int, prefix bool) string {
	s, err := EncodeInt64(v, bits, prefix)
	if err != nil {
		panic(err)
	}
	return s
}

// ----------------------------------------------------------------------
// Validation
// ----------------------------------------------------------------------

// IsHex reports whether the string is a valid hexadecimal number.
// It allows an optional "0x" prefix.
func IsHex(s string) bool {
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// ----------------------------------------------------------------------
// Pretty printing
// ----------------------------------------------------------------------

// Dump returns a hex dump of the given data, similar to hex.Dump but with
// configurable grouping and columns. For compatibility, the default is
// the same as hex.Dump: 16 bytes per line with ASCII representation.
func Dump(data []byte) string {
	return hex.Dump(data)
}

// Format pretty‑prints a hex string with optional grouping and separators.
// For example, Format("deadbeef", 2, " ") returns "de ad be ef".
// If groupSize <= 0, no grouping is applied.
func Format(s string, groupSize int, separator string) (string, error) {
	clean, err := Decode(s) // validates and removes prefix
	if err != nil {
		return "", err
	}
	encoded := Encode(clean)
	if groupSize <= 0 {
		return encoded, nil
	}
	var result strings.Builder
	for i := 0; i < len(encoded); i += groupSize {
		if i > 0 {
			result.WriteString(separator)
		}
		end := i + groupSize
		if end > len(encoded) {
			end = len(encoded)
		}
		result.WriteString(encoded[i:end])
	}
	return result.String(), nil
}

// MustFormat is like Format but panics on error.
func MustFormat(s string, groupSize int, separator string) string {
	result, err := Format(s, groupSize, separator)
	if err != nil {
		panic(err)
	}
	return result
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     // Encode bytes
//     hex := hexutil.Encode([]byte("hello")) // "68656c6c6f"
//
//     // Decode with or without prefix
//     b, _ := hexutil.Decode("0x68656c6c6f")
//
//     // Integer to hex
//     s := hexutil.EncodeUint64(255, true, 2) // "0xff"
//
//     // Validate
//     fmt.Println(hexutil.IsHex("0x123abc")) // true
//
//     // Pretty format
//     pretty, _ := hexutil.Format("deadbeef", 2, ":") // "de:ad:be:ef"
// }