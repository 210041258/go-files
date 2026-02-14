// Package length provides utilities for converting and formatting
// length measurements between metric and imperial units.
// Supported units: meter (m), kilometer (km), centimeter (cm),
// millimeter (mm), mile (mi), yard (yd), foot (ft), inch (in).
package testutils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Unit represents a length unit.
type Unit string

// Constants for supported length units.
const (
	Meter       Unit = "m"
	Kilometer   Unit = "km"
	Centimeter  Unit = "cm"
	Millimeter  Unit = "mm"
	Mile        Unit = "mi"
	Yard        Unit = "yd"
	Foot        Unit = "ft"
	Inch        Unit = "in"
)

// conversion factors to meters (base unit).
var toMeter = map[Unit]float64{
	Meter:      1.0,
	Kilometer:  1000.0,
	Centimeter: 0.01,
	Millimeter: 0.001,
	Mile:       1609.344,
	Yard:       0.9144,
	Foot:       0.3048,
	Inch:       0.0254,
}

// Units returns a list of all supported units.
func Units() []Unit {
	return []Unit{Meter, Kilometer, Centimeter, Millimeter, Mile, Yard, Foot, Inch}
}

// Convert converts a value from one unit to another.
// Returns an error if either unit is unsupported.
func Convert(value float64, from, to Unit) (float64, error) {
	factorFrom, ok := toMeter[from]
	if !ok {
		return 0, fmt.Errorf("unsupported unit: %s", from)
	}
	factorTo, ok := toMeter[to]
	if !ok {
		return 0, fmt.Errorf("unsupported unit: %s", to)
	}
	// Convert to meters, then to target.
	meters := value * factorFrom
	return meters / factorTo, nil
}

// MustConvert is like Convert but panics on error.
func MustConvert(value float64, from, to Unit) float64 {
	v, err := Convert(value, from, to)
	if err != nil {
		panic(err)
	}
	return v
}

// Parse parses a string like "10.5km" or "3ft" into a value and unit.
// The unit suffix is case‑sensitive; use lowercase as defined.
// Returns an error if the format is invalid or the unit is unknown.
func Parse(s string) (value float64, unit Unit, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, "", fmt.Errorf("empty string")
	}
	// Use regex to split number and unit.
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
	// Verify unit is supported.
	if _, ok := toMeter[unit]; !ok {
		return 0, "", fmt.Errorf("unsupported unit: %s", unit)
	}
	return value, unit, nil
}

// MustParse is like Parse but panics on error.
func MustParse(s string) (value float64, unit Unit) {
	v, u, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return v, u
}

// Format formats a value with its unit, using the specified precision.
// If precision < 0, the smallest number of digits necessary is used.
func Format(value float64, unit Unit, precision int) string {
	return strconv.FormatFloat(value, 'f', precision, 64) + string(unit)
}

// FormatWithConversion converts the value to the desired unit and formats it.
// Equivalent to Convert followed by Format.
func FormatWithConversion(value float64, from, to Unit, precision int) (string, error) {
	converted, err := Convert(value, from, to)
	if err != nil {
		return "", err
	}
	return Format(converted, to, precision), nil
}

// ----------------------------------------------------------------------
// Metric to Imperial conversions (convenience functions)
// ----------------------------------------------------------------------

// MetersToFeet converts meters to feet.
func MetersToFeet(m float64) float64 { return m / toMeter[Foot] }

// FeetToMeters converts feet to meters.
func FeetToMeters(ft float64) float64 { return ft * toMeter[Foot] }

// KilometersToMiles converts kilometers to miles.
func KilometersToMiles(km float64) float64 { return km / (toMeter[Mile] / 1000) }

// MilesToKilometers converts miles to kilometers.
func MilesToKilometers(mi float64) float64 { return mi * toMeter[Mile] / 1000 }

// MetersToYards converts meters to yards.
func MetersToYards(m float64) float64 { return m / toMeter[Yard] }

// YardsToMeters converts yards to meters.
func YardsToMeters(yd float64) float64 { return yd * toMeter[Yard] }

// ----------------------------------------------------------------------
// Distance calculations
// ----------------------------------------------------------------------

// Distance calculates the Euclidean distance between two points (x1,y1) and (x2,y2).
// The result is in the same unit as the inputs.
func Distance(x1, y1, x2, y2 float64) float64 {
	dx := x2 - x1
	dy := y2 - y1
	return math.Sqrt(dx*dx + dy*dy)
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     // Convert 5 miles to kilometers
//     km, _ := length.Convert(5, length.Mile, length.Kilometer)
//     fmt.Println(km) // 8.04672
//
//     // Parse "10.5 ft"
//     val, unit, _ := length.Parse("10.5 ft")
//     fmt.Println(val, unit) // 10.5 ft
//
//     // Format 3 meters with unit
//     s := length.Format(3, length.Meter, 2) // "3.00m"
// }