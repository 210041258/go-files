// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"fmt"
	"time"
)

// Date represents a calendar date (year, month, day) without time or timezone.
// It is a lightweight value type for date‑oriented operations.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// NewDate creates a Date from the given components. It does not validate.
func NewDate(year int, month time.Month, day int) Date {
	return Date{Year: year, Month: month, Day: day}
}

// Today returns the current date in the local time zone.
// For tests, consider using a DateProvider to control the date.
func Today() Date {
	return FromTime(time.Now())
}

// FromTime extracts the date from a time.Time (using the local time zone).
func FromTime(t time.Time) Date {
	y, m, d := t.Date()
	return Date{Year: y, Month: m, Day: d}
}

// ToTime returns a time.Time at 00:00 UTC on this date.
func (d Date) ToTime() time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

// String returns the date in YYYY-MM-DD format.
func (d Date) String() string {
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}

// ParseDate parses a string in YYYY-MM-DD format and returns a Date.
// It returns an error if the format is invalid or the date is invalid.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return Date{}, err
	}
	return FromTime(t), nil
}

// Compare returns -1 if d < other, 0 if d == other, 1 if d > other.
func (d Date) Compare(other Date) int {
	if d.Year != other.Year {
		if d.Year < other.Year {
			return -1
		}
		return 1
	}
	if d.Month != other.Month {
		if d.Month < other.Month {
			return -1
		}
		return 1
	}
	if d.Day != other.Day {
		if d.Day < other.Day {
			return -1
		}
		return 1
	}
	return 0
}

// Before reports whether d is before other.
func (d Date) Before(other Date) bool {
	return d.Compare(other) < 0
}

// After reports whether d is after other.
func (d Date) After(other Date) bool {
	return d.Compare(other) > 0
}

// Equal reports whether d equals other.
func (d Date) Equal(other Date) bool {
	return d.Compare(other) == 0
}

// Between reports whether d is in the inclusive range [start, end].
func (d Date) Between(start, end Date) bool {
	return !d.Before(start) && !d.After(end)
}

// AddDays returns the date n days after d (n may be negative).
func (d Date) AddDays(n int) Date {
	return FromTime(d.ToTime().AddDate(0, 0, n))
}

// AddMonths returns the date n months after d (n may be negative).
// It handles month overflows by normalising the date.
func (d Date) AddMonths(n int) Date {
	return FromTime(d.ToTime().AddDate(0, n, 0))
}

// AddYears returns the date n years after d (n may be negative).
func (d Date) AddYears(n int) Date {
	return FromTime(d.ToTime().AddDate(n, 0, 0))
}

// DaysSince returns the number of days between d and other (d - other).
func (d Date) DaysSince(other Date) int {
	// Convert to Unix timestamps at noon to avoid daylight saving issues.
	const secondsPerDay = 24 * 60 * 60
	unixD := time.Date(d.Year, d.Month, d.Day, 12, 0, 0, 0, time.UTC).Unix()
	unixOther := time.Date(other.Year, other.Month, other.Day, 12, 0, 0, 0, time.UTC).Unix()
	return int((unixD - unixOther) / secondsPerDay)
}

// Weekday returns the day of the week for this date.
func (d Date) Weekday() time.Weekday {
	return d.ToTime().Weekday()
}

// IsZero reports whether the date is the zero value (year 0, month 1, day 1?).
// We define zero as all fields zero, but time.Month zero is January.
// For simplicity, we treat (0, time.Month(0), 0) as zero.
func (d Date) IsZero() bool {
	return d.Year == 0 && d.Month == 0 && d.Day == 0
}

// ------------------------------------------------------------------------
// DateProvider – abstraction for obtaining the current date in tests
// ------------------------------------------------------------------------

// DateProvider defines an interface for getting the current date.
type DateProvider interface {
	// Now returns the current date.
	Now() Date
}

// RealDateProvider returns the actual current date (from time.Now()).
type RealDateProvider struct{}

// Now returns today's date.
func (RealDateProvider) Now() Date { return Today() }

// MockDateProvider is a controllable date provider for tests.
type MockDateProvider struct {
	current Date
}

// NewMockDateProvider creates a provider with a fixed starting date.
func NewMockDateProvider(start Date) *MockDateProvider {
	return &MockDateProvider{current: start}
}

// Now returns the current mocked date.
func (m *MockDateProvider) Now() Date {
	return m.current
}

// Set changes the current date.
func (m *MockDateProvider) Set(d Date) {
	m.current = d
}

// Advance adds days to the current date.
func (m *MockDateProvider) Advance(days int) {
	m.current = m.current.AddDays(days)
}

// Freeze returns a copy of the provider that will always return the same date,
// regardless of Advance calls on the original. Useful for isolating tests.
func (m *MockDateProvider) Freeze() *MockDateProvider {
	return NewMockDateProvider(m.current)
}