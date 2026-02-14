// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"time"
)

// Period represents a half‑open time interval [Start, End).
// It includes the start instant but excludes the end.
type Period struct {
	Start time.Time
	End   time.Time
}

// NewPeriod creates a new period. It panics if start is after end.
func NewPeriod(start, end time.Time) Period {
	if start.After(end) {
		panic("testutils: period start must be before or equal to end")
	}
	return Period{Start: start, End: end}
}

// Duration returns the length of the period.
func (p Period) Duration() time.Duration {
	return p.End.Sub(p.Start)
}

// Contains reports whether t lies within the period (start ≤ t < end).
func (p Period) Contains(t time.Time) bool {
	return (t.Equal(p.Start) || t.After(p.Start)) && t.Before(p.End)
}

// Overlaps reports whether two periods share any common instant.
func (p Period) Overlaps(other Period) bool {
	return p.Start.Before(other.End) && other.Start.Before(p.End)
}

// Merge returns the smallest period that covers both periods.
// If they are disjoint, the result will cover the gap as well.
func (p Period) Merge(other Period) Period {
	start := p.Start
	if other.Start.Before(start) {
		start = other.Start
	}
	end := p.End
	if other.End.After(end) {
		end = other.End
	}
	return Period{Start: start, End: end}
}

// Intersect returns the overlapping part of two periods.
// If they do not overlap, a zero period (IsZero() true) is returned.
func (p Period) Intersect(other Period) Period {
	if !p.Overlaps(other) {
		return Period{}
	}
	start := p.Start
	if other.Start.After(start) {
		start = other.Start
	}
	end := p.End
	if other.End.Before(end) {
		end = other.End
	}
	return Period{Start: start, End: end}
}

// IsZero reports whether the period is the zero value (both times zero).
func (p Period) IsZero() bool {
	return p.Start.IsZero() && p.End.IsZero()
}

// Split divides the period at time t, returning two periods: [Start, t) and [t, End).
// If t is outside the period, the original period and a zero period are returned.
func (p Period) Split(t time.Time) (Period, Period) {
	if t.Before(p.Start) || t.After(p.End) {
		return p, Period{}
	}
	return Period{Start: p.Start, End: t}, Period{Start: t, End: p.End}
}

// Series generates a slice of consecutive periods of the given step duration,
// starting from Start and ending before End. The last period may be shorter
// if step does not evenly divide the total duration.
func (p Period) Series(step time.Duration) []Period {
	if step <= 0 {
		panic("testutils: step must be positive")
	}
	var periods []Period
	for t := p.Start; t.Before(p.End); t = t.Add(step) {
		end := t.Add(step)
		if end.After(p.End) {
			end = p.End
		}
		periods = append(periods, Period{Start: t, End: end})
	}
	return periods
}