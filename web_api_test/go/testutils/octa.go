// Package octa provides utilities for octal (base-8) encoding and decoding.
// It supports conversion between integers and octal strings with optional
// "0o" prefix, validation, and pretty formatting.
package testutils

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ----------------------------------------------------------------------
// Integer <-> octal string conversions
// ----------------------------------------------------------------------

// EncodeUint64 encodes a uint64 as an octal string with an optional "0o" prefix.
// The minDigits parameter pads the result with leading zeros to the specified width.
// If minDigits is less than the actual length, it is ignored.
func EncodeUint64(v uint64, prefix bool, minDigits int) string {
	s := strconv.FormatUint(v, 8)
	if len(s) < minDigits {
		s = strings.Repeat("0", minDigits-len(s)) + s
	}
	if prefix {
		return "0o" + s
	}
	return s
}

// MustEncodeUint64 is like EncodeUint64 but panics if minDigits is negative.
func MustEncodeUint64(v uint64, prefix bool, minDigits int) string {
	if minDigits < 0 {
		panic("octa: minDigits cannot be negative")
	}
	return EncodeUint64(v, prefix, minDigits)
}

// DecodeUint64 decodes an octal string (with or without "0o" prefix) into a uint64.
// It returns an error if the string contains invalid octal digits or overflows.
func DecodeUint64(s string) (uint64, error) {
	s = strings.TrimPrefix(s, "0o")
	s = strings.TrimPrefix(s, "0O")
	return strconv.ParseUint(s, 8, 64)
}

// MustDecodeUint64 is like DecodeUint64 but panics on error.
func MustDecodeUint64(s string) uint64 {
	v, err := DecodeUint64(s)
	if err != nil {
		panic("octa: " + err.Error())
	}
	return v
}

// EncodeInt64 encodes an int64 as an octal string. Negative values are represented
// in two's complement with the specified number of bits (must be 8, 16, 32, or 64).
// The result has the minimum width for the given bit size (e.g., 6 digits for 8 bits).
// If prefix is true, a "0o" prefix is added.
func EncodeInt64(v int64, bits int, prefix bool) (string, error) {
	switch bits {
	case 8, 16, 32, 64:
		// ok
	default:
		return "", fmt.Errorf("bits must be 8, 16, 32, or 64")
	}
	mask := uint64(1<<bits - 1)
	uv := uint64(v) & mask
	// Minimum octal digits needed: ceil(bits/3)
	minDigits := (bits + 2) / 3
	return EncodeUint64(uv, prefix, minDigits), nil
}

// MustEncodeInt64 is like EncodeInt64 but panics on error.
func MustEncodeInt64(v int64, bits int, prefix bool) string {
	s, err := EncodeInt64(v, bits, prefix)
	if err != nil {
		panic("octa: " + err.Error())
	}
	return s
}

// DecodeInt64 decodes an octal string (with or without prefix) into an int64,
// interpreting the value as a signed integer with the given number of bits.
// It returns an error if the value overflows the bit width or contains invalid digits.
func DecodeInt64(s string, bits int) (int64, error) {
	uv, err := DecodeUint64(s)
	if err != nil {
		return 0, err
	}
	switch bits {
	case 8, 16, 32, 64:
		// ok
	default:
		return 0, fmt.Errorf("bits must be 8, 16, 32, or 64")
	}
	// Check overflow for unsigned range.
	maxUint := uint64(1<<bits - 1)
	if uv > maxUint {
		return 0, fmt.Errorf("value %o overflows %d bits", uv, bits)
	}
	// Sign extend if the most significant bit is set.
	if bits < 64 && uv&(1<<(bits-1)) != 0 {
		uv |= ^(maxUint) // set all higher bits
	}
	return int64(uv), nil
}

// MustDecodeInt64 is like DecodeInt64 but panics on error.
func MustDecodeInt64(s string, bits int) int64 {
	v, err := DecodeInt64(s, bits)
	if err != nil {
		panic("octa: " + err.Error())
	}
	return v
}

// ----------------------------------------------------------------------
// Validation
// ----------------------------------------------------------------------

// IsOctal reports whether the string is a valid octal number.
// It allows an optional "0o" or "0O" prefix.
func IsOctal(s string) bool {
	s = strings.TrimPrefix(s, "0o")
	s = strings.TrimPrefix(s, "0O")
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '7' {
			return false
		}
	}
	return true
}

// ----------------------------------------------------------------------
// Pretty formatting
// ----------------------------------------------------------------------

// Format takes an octal string (with or without prefix), validates it,
// and returns a grouped representation with the specified separator.
// For example, Format("123456", 2, " ") returns "12 34 56".
// If groupSize <= 0, no grouping is applied.
func Format(s string, groupSize int, separator string) (string, error) {
	// Validate and clean the input.
	clean, err := decodeClean(s)
	if err != nil {
		return "", err
	}
	if groupSize <= 0 {
		return clean, nil
	}
	var result strings.Builder
	for i := 0; i < len(clean); i += groupSize {
		if i > 0 {
			result.WriteString(separator)
		}
		end := i + groupSize
		if end > len(clean) {
			end = len(clean)
		}
		result.WriteString(clean[i:end])
	}
	return result.String(), nil
}

// MustFormat is like Format but panics on error.
func MustFormat(s string, groupSize int, separator string) string {
	res, err := Format(s, groupSize, separator)
	if err != nil {
		panic("octa: " + err.Error())
	}
	return res
}

// decodeClean removes the optional prefix and validates the octal string.
func decodeClean(s string) (string, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "0o")
	s = strings.TrimPrefix(s, "0O")
	if len(s) == 0 {
		return "", errors.New("empty string")
	}
	for _, r := range s {
		if r < '0' || r > '7' {
			return "", fmt.Errorf("invalid octal digit: %c", r)
		}
	}
	return s, nil
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     // Encode uint64 with prefix and padding
//     s := octa.EncodeUint64(42, true, 4) // "0o0052"
//
//     // Decode
//     v, _ := octa.DecodeUint64("0o52") // 42
//
//     // Signed 8‑bit two's complement
//     s2 := octa.MustEncodeInt64(-1, 8, true) // "0o377" (since -1 in 8 bits is 0xFF octal)
//     v2, _ := octa.DecodeInt64("0o377", 8)   // -1
//
//     // Validate
//     fmt.Println(octa.IsOctal("0o123")) // true
//
//     // Pretty format
//     pretty, _ := octa.Format("0o1234567", 3, "_") // "123_456_7"
// }