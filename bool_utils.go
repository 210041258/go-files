// Package boolutils provides utilities for working with boolean values,
// including parsing, conversion, and logical operations on collections.
package testutils

import (
	"strconv"
	"strings"
)

// ----------------------------------------------------------------------
// Parsing with defaults
// ----------------------------------------------------------------------

// Parse parses a string as a boolean using strconv.ParseBool.
// It returns an error if the string cannot be parsed.
func Parse(s string) (bool, error) {
	return strconv.ParseBool(s)
}

// ParseDefault parses a string as a boolean. If parsing fails,
// it returns the provided default value.
func ParseDefault(s string, defaultValue bool) bool {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return defaultValue
	}
	return v
}

// MustParse parses a string as a boolean. It panics on error.
func MustParse(s string) bool {
	v, err := strconv.ParseBool(s)
	if err != nil {
		panic("boolutils.MustParse: " + err.Error())
	}
	return v
}

// IsTrue returns true if the string represents a truthy value.
// It accepts common aliases: true, false, 1, 0, yes, no, on, off, t, f.
// Comparison is case‑insensitive.
func IsTrue(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "true", "1", "yes", "on", "t":
		return true
	default:
		return false
	}
}

// IsFalse returns true if the string represents a falsy value.
// It is the logical complement of IsTrue for the same set of aliases.
func IsFalse(s string) bool {
	return !IsTrue(s)
}

// ----------------------------------------------------------------------
// Conversion to other types
// ----------------------------------------------------------------------

// ToInt returns 1 for true, 0 for false.
func ToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ToString returns "true" or "false".
func ToString(b bool) string {
	return strconv.FormatBool(b)
}

// ToYesNo returns "yes" for true, "no" for false.
func ToYesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// ToOnOff returns "on" for true, "off" for false.
func ToOnOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// FromInt converts an int to bool. Zero is false, non‑zero is true.
func FromInt(i int) bool {
	return i != 0
}

// FromInt64 converts an int64 to bool. Zero is false, non‑zero is true.
func FromInt64(i int64) bool {
	return i != 0
}

// ----------------------------------------------------------------------
// Ternary (generic)
// ----------------------------------------------------------------------

// If is a generic ternary operator. If cond is true, it returns a, otherwise b.
func If[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}

// ----------------------------------------------------------------------
// Collection operations
// ----------------------------------------------------------------------

// All returns true if all values in the slice are true.
func All(vals []bool) bool {
	for _, v := range vals {
		if !v {
			return false
		}
	}
	return true
}

// Any returns true if at least one value in the slice is true.
func Any(vals []bool) bool {
	for _, v := range vals {
		if v {
			return true
		}
	}
	return false
}

// None returns true if no values in the slice are true.
func None(vals []bool) bool {
	for _, v := range vals {
		if v {
			return false
		}
	}
	return true
}

// Count returns the number of true values in the slice.
func Count(vals []bool) int {
	c := 0
	for _, v := range vals {
		if v {
			c++
		}
	}
	return c
}

// And returns the logical AND of all values (true if all true).
// Returns true for an empty slice.
func And(vals []bool) bool {
	for _, v := range vals {
		if !v {
			return false
		}
	}
	return true
}

// Or returns the logical OR of all values (true if any true).
// Returns false for an empty slice.
func Or(vals []bool) bool {
	for _, v := range vals {
		if v {
			return true
		}
	}
	return false
}

// Xor returns the exclusive OR of two booleans (true if they differ).
func Xor(a, b bool) bool {
	return a != b
}

// ----------------------------------------------------------------------
// Conditional builder (fluent style)
// ----------------------------------------------------------------------

// Cond is a helper for building conditional expressions.
type Cond struct {
	cond bool
}

// If creates a Cond for the given condition.
func IfCond(cond bool) *Cond {
	return &Cond{cond: cond}
}

// Then returns the provided value if the condition is true,
// otherwise it returns a placeholder that expects Else.
func (c *Cond) Then[T any](val T) *ThenHolder[T] {
	return &ThenHolder[T]{
		cond:   c.cond,
		thenVal: val,
	}
}

// ThenHolder holds the state after Then.
type ThenHolder[T any] struct {
	cond    bool
	thenVal T
}

// Else returns the then value if the condition was true, otherwise the else value.
func (h *ThenHolder[T]) Else(elseVal T) T {
	if h.cond {
		return h.thenVal
	}
	return elseVal
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     fmt.Println(boolutils.ParseDefault("yes", false)) // true
//     fmt.Println(boolutils.IsTrue("ON"))               // true
//     fmt.Println(boolutils.ToInt(true))                // 1
//     fmt.Println(boolutils.If(true, "a", "b"))         // "a"
//     fmt.Println(boolutils.Any([]bool{false, true}))   // true
//     fmt.Println(boolutils.IfCond(false).Then(42).Else(0)) // 0
// }