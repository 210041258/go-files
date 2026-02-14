// Package power provides utilities for exponentiation and related
// power functions, including fast integer exponentiation, power‑of‑two
// checks, and root approximations.
package testutils

import (
	"math"
)

// ----------------------------------------------------------------------
// Integer exponentiation (fast exponentiation)
// ----------------------------------------------------------------------

// Pow returns x raised to the power n, where n is a non‑negative integer.
// It uses the binary exponentiation algorithm (fast power).
// If n < 0, it returns 0 (unsupported for negative exponents; use PowFloat).
func Pow(x int64, n int) int64 {
	if n < 0 {
		return 0
	}
	result := int64(1)
	for n > 0 {
		if n&1 == 1 {
			result *= x
		}
		x *= x
		n >>= 1
	}
	return result
}

// PowInt returns x raised to the power n as an int, using fast exponentiation.
// It panics if n < 0 (use PowFloat for negative exponents).
func PowInt(x int, n int) int {
	if n < 0 {
		panic("power.PowInt: negative exponent not supported; use PowFloat")
	}
	result := 1
	for n > 0 {
		if n&1 == 1 {
			result *= x
		}
		x *= x
		n >>= 1
	}
	return result
}

// PowInt64 is an alias for Pow.
func PowInt64(x int64, n int) int64 { return Pow(x, n) }

// ----------------------------------------------------------------------
// Floating‑point powers
// ----------------------------------------------------------------------

// PowFloat returns x raised to the power y, using math.Pow.
func PowFloat(x, y float64) float64 {
	return math.Pow(x, y)
}

// Pow10 returns 10 raised to the power n.
// It uses math.Pow10 for small exponents and falls back to math.Pow for larger ones.
func Pow10(n int) float64 {
	if n >= -308 && n <= 308 {
		return math.Pow10(n)
	}
	return math.Pow(10, float64(n))
}

// ----------------------------------------------------------------------
// Power of two
// ----------------------------------------------------------------------

// IsPowerOfTwo reports whether n is a power of two (greater than 0).
func IsPowerOfTwo(n int) bool {
	return n > 0 && (n&(n-1)) == 0
}

// IsPowerOfTwo64 reports whether n (uint64) is a power of two (greater than 0).
func IsPowerOfTwo64(n uint64) bool {
	return n > 0 && (n&(n-1)) == 0
}

// NextPowerOfTwo returns the smallest power of two that is >= n.
// If n <= 0, it returns 1.
func NextPowerOfTwo(n int) int {
	if n <= 0 {
		return 1
	}
	// For 32‑bit, but we use 64‑bit to avoid overflow.
	x := uint64(n - 1)
	x |= x >> 1
	x |= x >> 2
	x |= x >> 4
	x |= x >> 8
	x |= x >> 16
	x |= x >> 32
	return int(x + 1)
}

// NextPowerOfTwo64 returns the smallest power of two >= n as a uint64.
func NextPowerOfTwo64(n uint64) uint64 {
	if n == 0 {
		return 1
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n |= n >> 32
	return n + 1
}

// ----------------------------------------------------------------------
// Roots
// ----------------------------------------------------------------------

// Sqrt returns the square root of x, using math.Sqrt.
func Sqrt(x float64) float64 {
	return math.Sqrt(x)
}

// Cbrt returns the cube root of x, using math.Cbrt.
func Cbrt(x float64) float64 {
	return math.Cbrt(x)
}

// Root returns the n‑th root of x, where n is a positive integer.
// It returns NaN for negative x and even n.
func Root(x float64, n int) float64 {
	if n <= 0 {
		return math.NaN()
	}
	if n == 1 {
		return x
	}
	if n == 2 {
		return math.Sqrt(x)
	}
	if n == 3 {
		return math.Cbrt(x)
	}
	return math.Pow(x, 1.0/float64(n))
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     fmt.Println(power.Pow(2, 10))      // 1024
//     fmt.Println(power.PowInt(3, 4))    // 81
//     fmt.Println(power.PowFloat(2.5, 2)) // 6.25
//     fmt.Println(power.IsPowerOfTwo(16)) // true
//     fmt.Println(power.NextPowerOfTwo(13)) // 16
//     fmt.Println(power.Root(27, 3))      // 3
// }