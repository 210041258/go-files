// Package len provides simple length conversion utilities.
// It focuses on the most common units: meters, feet, and inches.
// For more comprehensive unit support, see the length package.
package testutils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Unit represents a length unit.
type Unit string

// Supported units.
const (
	Meter Unit = "m"
	Foot  Unit = "ft"
	Inch  Unit = "in"
)

// toMeter converts a unit to meters.
func toMeter(u Unit) float64 {
	switch u {
	case Meter:
		return 1.0
	case Foot:
		return 0.3048
	case Inch:
		return 0.0254
	default:
		return 0
	}
}

// Convert converts a value from one unit to another.
// Supported units: "m" (meter), "ft" (foot), "in" (inch).
func Convert(value float64, from, to Unit) (float64, error) {
	meters := value * toMeter(from)
	if meters == 0 && value != 0 {
		return 0, fmt.Errorf("unsupported from unit: %s", from)
	}
	toFactor := toMeter(to)
	if toFactor == 0 {
		return 0, fmt.Errorf("unsupported to unit: %s", to)
	}
	return meters / toFactor, nil
}

// MustConvert is like Convert but panics on error.
func MustConvert(value float64, from, to Unit) float64 {
	v, err := Convert(value, from, to)
	if err != nil {
		panic(err)
	}
	return v
}

// Parse parses a string like "10m" or "5.5ft" into a value and unit.
// The unit suffix must be one of "m", "ft", "in".
func Parse(s string) (value float64, unit Unit, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, "", fmt.Errorf("empty string")
	}
	re := regexp.MustCompile(`^([-+]?[0-9]*\.?[0-9]+)\s*([a-z]+)$`)
	matches := re.FindStringSubmatch(s)
	if len(matches) != 3 {
		return 0, "", fmt.Errorf("invalid format: %s", s)
	}
	valStr, unitStr := matches[1], matches[2]
	value, err = strconv.ParseFloat(valStr, 64)
	if err != nil {
		return 0, "", fmt.Errorf("invalid number: %s", valStr)
	}
	unit = Unit(unitStr)
	switch unit {
	case Meter, Foot, Inch:
		// valid
	default:
		return 0, "", fmt.Errorf("unsupported unit: %s", unit)
	}
	return value, unit, nil
}

// Format formats a value with its unit.
// The precision specifies the number of decimal places.
func Format(value float64, unit Unit, precision int) string {
	return strconv.FormatFloat(value, 'f', precision, 64) + string(unit)
}

// MetersToFeet converts meters to feet.
func MetersToFeet(m float64) float64 { return m / toMeter(Foot) }

// FeetToMeters converts feet to meters.
func FeetToMeters(ft float64) float64 { return ft * toMeter(Foot) }

// FeetToInches converts feet to inches.
func FeetToInches(ft float64) float64 { return ft * 12 }

// InchesToFeet converts inches to feet.
func InchesToFeet(in float64) float64 { return in / 12 }

// MetersToInches converts meters to inches.
func MetersToInches(m float64) float64 { return m / toMeter(Inch) }

// InchesToMeters converts inches to meters.
func InchesToMeters(in float64) float64 { return in * toMeter(Inch) }