// Package floatutil provides utilities for floating‑point operations.
// It includes functions for comparison with tolerance, rounding,
// parsing, and basic statistics.
package floatutil

import (
	"math"
	"strconv"
)

// ----------------------------------------------------------------------
// Comparison with tolerance
// ----------------------------------------------------------------------

// AlmostEqual compares two floats with an absolute tolerance.
// It returns true if the difference is <= epsilon.
func AlmostEqual(a, b, epsilon float64) bool {
	return math.Abs(a-b) <= epsilon
}

// AlmostEqualRelative compares two floats with a relative tolerance.
// The tolerance is scaled by the magnitude of the numbers.
// Returns true if |a-b| <= relEps * max(|a|,|b|).
func AlmostEqualRelative(a, b, relEps float64) bool {
	diff := math.Abs(a - b)
	mag := math.Max(math.Abs(a), math.Abs(b))
	if mag == 0 {
		return diff <= relEps
	}
	return diff <= relEps*mag
}

// ----------------------------------------------------------------------
// Rounding
// ----------------------------------------------------------------------

// Round rounds a float to the specified number of decimal places.
// Precision can be negative to round to powers of ten.
func Round(x float64, precision int) float64 {
	pow := math.Pow10(precision)
	return math.Round(x*pow) / pow
}

// RoundToInt rounds a float to the nearest integer.
func RoundToInt(x float64) int {
	return int(math.Round(x))
}

// FloorToInt returns the greatest integer value less than or equal to x.
func FloorToInt(x float64) int {
	return int(math.Floor(x))
}

// CeilToInt returns the least integer value greater than or equal to x.
func CeilToInt(x float64) int {
	return int(math.Ceil(x))
}

// Truncate discards decimal places beyond the specified precision
// (i.e., rounds toward zero).
func Truncate(x float64, precision int) float64 {
	pow := math.Pow10(precision)
	return math.Trunc(x*pow) / pow
}

// ----------------------------------------------------------------------
// Parsing and formatting
// ----------------------------------------------------------------------

// ParseFloat parses a string as a float64. If parsing fails,
// it returns the provided default value.
func ParseFloat(s string, defaultValue float64) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return defaultValue
	}
	return v
}

// MustParseFloat parses a string as a float64. It panics on error.
func MustParseFloat(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		panic("floatutil.MustParseFloat: " + err.Error())
	}
	return v
}

// ToString formats a float with the given precision.
// It uses 'f' format (no exponent).
func ToString(x float64, precision int) string {
	return strconv.FormatFloat(x, 'f', precision, 64)
}

// ToStringScientific formats a float in scientific notation.
func ToStringScientific(x float64, precision int) string {
	return strconv.FormatFloat(x, 'e', precision, 64)
}

// ----------------------------------------------------------------------
// Slice operations
// ----------------------------------------------------------------------

// Min returns the smallest value in the slice.
// If the slice is empty, it returns 0 and false.
func Min(vals []float64) (float64, bool) {
	if len(vals) == 0 {
		return 0, false
	}
	min := vals[0]
	for _, v := range vals[1:] {
		if v < min {
			min = v
		}
	}
	return min, true
}

// Max returns the largest value in the slice.
func Max(vals []float64) (float64, bool) {
	if len(vals) == 0 {
		return 0, false
	}
	max := vals[0]
	for _, v := range vals[1:] {
		if v > max {
			max = v
		}
	}
	return max, true
}

// Sum returns the sum of all values in the slice.
func Sum(vals []float64) float64 {
	s := 0.0
	for _, v := range vals {
		s += v
	}
	return s
}

// Mean returns the arithmetic mean of the slice.
// If the slice is empty, it returns 0 and false.
func Mean(vals []float64) (float64, bool) {
	if len(vals) == 0 {
		return 0, false
	}
	return Sum(vals) / float64(len(vals)), true
}

// Variance returns the sample variance (unbiased) of the slice.
// If len(vals) < 2, it returns 0 and false.
func Variance(vals []float64) (float64, bool) {
	if len(vals) < 2 {
		return 0, false
	}
	mean, _ := Mean(vals)
	var sum float64
	for _, v := range vals {
		diff := v - mean
		sum += diff * diff
	}
	return sum / float64(len(vals)-1), true
}

// StdDev returns the sample standard deviation.
func StdDev(vals []float64) (float64, bool) {
	v, ok := Variance(vals)
	if !ok {
		return 0, false
	}
	return math.Sqrt(v), true
}

// ----------------------------------------------------------------------
// Special values
// ----------------------------------------------------------------------

// IsNaN reports whether f is an IEEE 754 “not‑a‑number” value.
func IsNaN(f float64) bool {
	return math.IsNaN(f)
}

// IsInf reports whether f is an infinity, according to sign.
// sign > 0: +Inf, sign < 0: -Inf, sign == 0: either.
func IsInf(f float64, sign int) bool {
	return math.IsInf(f, sign)
}

// ----------------------------------------------------------------------
// Angle conversion
// ----------------------------------------------------------------------

// DegToRad converts degrees to radians.
func DegToRad(deg float64) float64 {
	return deg * math.Pi / 180
}

// RadToDeg converts radians to degrees.
func RadToDeg(rad float64) float64 {
	return rad * 180 / math.Pi
}

// NormalizeAngle returns an angle in degrees normalized to [0, 360).
func NormalizeAngle(deg float64) float64 {
	deg = math.Mod(deg, 360)
	if deg < 0 {
		deg += 360
	}
	return deg
}

// ----------------------------------------------------------------------
// Float32 variants
// ----------------------------------------------------------------------

// F32 provides the same functions but for float32.
type F32 struct{}

// AlmostEqual compares two float32 with absolute tolerance.
func (F32) AlmostEqual(a, b, epsilon float32) bool {
	return math.Abs(float64(a-b)) <= float64(epsilon)
}

// Round rounds a float32 to specified precision.
func (F32) Round(x float32, precision int) float32 {
	pow := math.Pow10(precision)
	return float32(math.Round(float64(x)*pow) / pow)
}

// ParseFloat parses a string as float32 with default.
func (F32) ParseFloat(s string, defaultValue float32) float32 {
	v, err := strconv.ParseFloat(s, 32)
	if err != nil {
		return defaultValue
	}
	return float32(v)
}

// Sum returns the sum of float32 slice.
func (F32) Sum(vals []float32) float32 {
	var s float32
	for _, v := range vals {
		s += v
	}
	return s
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     fmt.Println(floatutil.AlmostEqual(0.1+0.2, 0.3, 1e-9)) // true
//     fmt.Println(floatutil.Round(3.14159, 2))               // 3.14
//     fmt.Println(floatutil.ToScientific(123.456, 2))        // "1.23e+02"
//     mean, _ := floatutil.Mean([]float64{1,2,3,4})
//     fmt.Println(mean) // 2.5
// }