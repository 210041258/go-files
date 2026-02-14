// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"fmt"
	"time"
)

// Time represents a time of day (hour, minute, second, nanosecond) without a date.
// It is a lightweight value type for time‑of‑day operations.
type Time struct {
	hour, min, sec, nsec int
}

// NewTime creates a Time from the given components. It does not validate
// that the values are within normal ranges (e.g., hour 0‑23, minute 0‑59).
func NewTime(hour, min, sec, nsec int) Time {
	return Time{hour: hour, min: min, sec: sec, nsec: nsec}
}

// FromTime extracts the time of day from a time.Time (using the local time zone).
func FromTime(t time.Time) Time {
	h, m, s := t.Clock()
	return Time{hour: h, min: m, sec: s, nsec: t.Nanosecond()}
}

// Now returns the current time of day in the local time zone.
// For tests, consider using a TimeProvider to control the time.
func Now() Time {
	return FromTime(time.Now())
}

// Hour returns the hour of the time (0‑23).
func (t Time) Hour() int { return t.hour }

// Minute returns the minute of the time (0‑59).
func (t Time) Minute() int { return t.min }

// Second returns the second of the time (0‑59).
func (t Time) Second() int { return t.sec }

// Nanosecond returns the nanosecond offset (0‑999999999).
func (t Time) Nanosecond() int { return t.nsec }

// ToTime returns a time.Time on the zero date (0000‑01‑01) with this time of day in UTC.
func (t Time) ToTime() time.Time {
	return time.Date(0, 1, 1, t.hour, t.min, t.sec, t.nsec, time.UTC)
}

// String returns the time in format "15:04:05.999999999".
func (t Time) String() string {
	// Format nanoseconds only if non‑zero.
	if t.nsec == 0 {
		return fmt.Sprintf("%02d:%02d:%02d", t.hour, t.min, t.sec)
	}
	// Use a format with 9 digits and trim trailing zeros.
	s := fmt.Sprintf("%02d:%02d:%02d.%09d", t.hour, t.min, t.sec, t.nsec)
	// Trim trailing zeros after the decimal point.
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] != '0' {
			if s[i] == '.' {
				return s[:i]
			}
			return s[:i+1]
		}
	}
	return s
}

// Format returns a textual representation of the time formatted according to layout.
// The layout must use the reference time "15:04:05.999999999" (like time.Time.Format).
func (t Time) Format(layout string) string {
	return t.ToTime().Format(layout)
}

// ParseTime parses a time string using the given layout and returns a Time.
// The layout must use the reference time "15:04:05.999999999".
func ParseTime(layout, value string) (Time, error) {
	t, err := time.Parse(layout, value)
	if err != nil {
		return Time{}, err
	}
	return FromTime(t), nil
}

// Add returns the time t + d, wrapping around the 24‑hour clock.
// The result is normalized to the range [0,24h).
func (t Time) Add(d time.Duration) Time {
	// Convert t to a duration since midnight.
	midnight := time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC)
	tt := t.ToTime()
	offset := tt.Sub(midnight)
	// Add the duration and take modulo 24h.
	total := (offset + d) % (24 * time.Hour)
	if total < 0 {
		total += 24 * time.Hour
	}
	return FromTime(midnight.Add(total))
}

// Sub returns the duration t - other, measured in the 24‑hour clock.
// The result is in the range (-12h, 12h]? Actually, it's the difference
// with wrap‑around considered. We'll compute the smallest absolute difference.
func (t Time) Sub(other Time) time.Duration {
	// Convert both to durations since midnight.
	midnight := time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC)
	dt := t.ToTime().Sub(midnight)
	do := other.ToTime().Sub(midnight)
	diff := dt - do
	// Adjust to the range (-12h, 12h] if desired.
	// For simplicity, return the raw difference (may be negative).
	// Callers can use time.Duration mod if they need wrap‑around.
	return diff
}

// Before reports whether t is strictly before other on the 24‑hour clock,
// ignoring date and wrap‑around (i.e., 23:00 is before 01:00? No).
// This is a simple numeric comparison, not circular.
func (t Time) Before(other Time) bool {
	if t.hour != other.hour {
		return t.hour < other.hour
	}
	if t.min != other.min {
		return t.min < other.min
	}
	if t.sec != other.sec {
		return t.sec < other.sec
	}
	return t.nsec < other.nsec
}

// After reports whether t is strictly after other.
func (t Time) After(other Time) bool {
	return other.Before(t)
}

// Equal reports whether t equals other.
func (t Time) Equal(other Time) bool {
	return t.hour == other.hour &&
		t.min == other.min &&
		t.sec == other.sec &&
		t.nsec == other.nsec
}

// IsZero reports whether the time is the zero value (all fields zero).
func (t Time) IsZero() bool {
	return t.hour == 0 && t.min == 0 && t.sec == 0 && t.nsec == 0
}

// ------------------------------------------------------------------------
// TimeProvider – abstraction for obtaining the current time of day in tests
// ------------------------------------------------------------------------

// TimeProvider defines an interface for getting the current time of day.
type TimeProvider interface {
	// Now returns the current time of day.
	Now() Time
}

// RealTimeProvider returns the actual current time of day (from time.Now()).
type RealTimeProvider struct{}

// Now returns the current time of day.
func (RealTimeProvider) Now() Time { return Now() }

// MockTimeProvider is a controllable time provider for tests.
type MockTimeProvider struct {
	current Time
}

// NewMockTimeProvider creates a provider with a fixed starting time.
func NewMockTimeProvider(start Time) *MockTimeProvider {
	return &MockTimeProvider{current: start}
}

// Now returns the current mocked time of day.
func (m *MockTimeProvider) Now() Time {
	return m.current
}

// Set changes the current time.
func (m *MockTimeProvider) Set(t Time) {
	m.current = t
}

// Advance adds a duration to the current time, wrapping around the 24‑hour clock.
func (m *MockTimeProvider) Advance(d time.Duration) {
	m.current = m.current.Add(d)
}

// Freeze returns a copy of the provider that will always return the same time,
// regardless of Advance calls on the original. Useful for isolating tests.
func (m *MockTimeProvider) Freeze() *MockTimeProvider {
	return NewMockTimeProvider(m.current)
}