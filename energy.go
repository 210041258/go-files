// Package energy provides utilities for energy unit conversions and
// basic physics calculations (kinetic, potential, etc.).
package testutils

import (
	"errors"
	"math"
)

// Unit represents an energy unit.
type Unit string

// Supported energy units.
const (
	Joule           Unit = "J"
	Kilojoule       Unit = "kJ"
	Megajoule       Unit = "MJ"
	Calorie         Unit = "cal"    // thermochemical calorie
	Kilocalorie     Unit = "kcal"   // food calorie (Cal)
	WattHour        Unit = "Wh"
	KilowattHour    Unit = "kWh"
	MegawattHour    Unit = "MWh"
	ElectronVolt    Unit = "eV"
	BritishThermalUnit Unit = "BTU"
)

// conversion factors to joules (base unit).
var toJoule = map[Unit]float64{
	Joule:           1.0,
	Kilojoule:       1e3,
	Megajoule:       1e6,
	Calorie:         4.184,          // thermochemical calorie
	Kilocalorie:     4184,           // 1000 cal
	WattHour:        3600,           // 1 Wh = 3600 J
	KilowattHour:    3.6e6,          // 1 kWh = 3.6e6 J
	MegawattHour:    3.6e9,          // 1 MWh = 3.6e9 J
	ElectronVolt:    1.602176634e-19, // exact (2019 redefinition)
	BritishThermalUnit: 1055.06,      // ISO BTU (approx)
}

// Units returns a list of all supported units.
func Units() []Unit {
	return []Unit{
		Joule, Kilojoule, Megajoule,
		Calorie, Kilocalorie,
		WattHour, KilowattHour, MegawattHour,
		ElectronVolt, BritishThermalUnit,
	}
}

// Convert converts a value from one energy unit to another.
// Returns an error if either unit is unsupported.
func Convert(value float64, from, to Unit) (float64, error) {
	factorFrom, ok := toJoule[from]
	if !ok {
		return 0, errors.New("unsupported from unit: " + string(from))
	}
	factorTo, ok := toJoule[to]
	if !ok {
		return 0, errors.New("unsupported to unit: " + string(to))
	}
	// Convert to joules, then to target.
	joules := value * factorFrom
	return joules / factorTo, nil
}

// MustConvert is like Convert but panics on error.
func MustConvert(value float64, from, to Unit) float64 {
	v, err := Convert(value, from, to)
	if err != nil {
		panic(err)
	}
	return v
}

// ----------------------------------------------------------------------
// Convenience conversion functions
// ----------------------------------------------------------------------

// JoulesToCalories converts joules to thermochemical calories.
func JoulesToCalories(j float64) float64 { return j / toJoule[Calorie] }

// CaloriesToJoules converts calories to joules.
func CaloriesToJoules(cal float64) float64 { return cal * toJoule[Calorie] }

// JoulesToKilowattHours converts joules to kilowatt‑hours.
func JoulesToKilowattHours(j float64) float64 { return j / toJoule[KilowattHour] }

// KilowattHoursToJoules converts kilowatt‑hours to joules.
func KilowattHoursToJoules(kwh float64) float64 { return kwh * toJoule[KilowattHour] }

// JoulesToElectronVolts converts joules to electronvolts.
func JoulesToElectronVolts(j float64) float64 { return j / toJoule[ElectronVolt] }

// ElectronVoltsToJoules converts electronvolts to joules.
func ElectronVoltsToJoules(ev float64) float64 { return ev * toJoule[ElectronVolt] }

// JoulesToBTU converts joules to British Thermal Units.
func JoulesToBTU(j float64) float64 { return j / toJoule[BritishThermalUnit] }

// BTUToJoules converts BTU to joules.
func BTUToJoules(btu float64) float64 { return btu * toJoule[BritishThermalUnit] }

// ----------------------------------------------------------------------
// Physics formulas
// ----------------------------------------------------------------------

// KineticEnergy returns the kinetic energy (1/2 * m * v²) in joules.
// mass is in kilograms, velocity in metres per second.
func KineticEnergy(mass, velocity float64) float64 {
	return 0.5 * mass * velocity * velocity
}

// PotentialEnergy returns the gravitational potential energy (m * g * h) in joules.
// mass is in kilograms, height in metres, g is acceleration due to gravity (default 9.81 m/s²).
// If g is zero or negative, the default gravity is used.
func PotentialEnergy(mass, height, g float64) float64 {
	if g <= 0 {
		g = 9.81
	}
	return mass * g * height
}

// RelativisticKineticEnergy returns the relativistic kinetic energy:
// (γ - 1) * m * c², where γ = 1/√(1 - v²/c²).
// mass is in kilograms, velocity in metres per second.
// It returns NaN if v >= c.
func RelativisticKineticEnergy(mass, velocity float64) float64 {
	const c = 299792458 // speed of light in m/s
	if velocity >= c {
		return math.NaN()
	}
	gamma := 1 / math.Sqrt(1-(velocity*velocity)/(c*c))
	return (gamma - 1) * mass * c * c
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     // Convert 100 kilowatt‑hours to joules
//     j, _ := energy.Convert(100, energy.KilowattHour, energy.Joule)
//     fmt.Println(j) // 3.6e8
//
//     // Kinetic energy of a 1000 kg car at 20 m/s
//     ke := energy.KineticEnergy(1000, 20)
//     fmt.Println(ke) // 200000
//
//     // Potential energy of a 10 kg mass at 5 m height
//     pe := energy.PotentialEnergy(10, 5, 0) // uses default g
//     fmt.Println(pe) // 490.5
// }