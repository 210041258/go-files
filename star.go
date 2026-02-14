// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"fmt"
	"math/rand"
	"strconv"
)

// Star represents a rating from 1 to 5 (inclusive).
// The zero value is invalid and should not be used.
type Star int

// Predefined star ratings for convenience.
const (
	OneStar   Star = 1
	TwoStar   Star = 2
	ThreeStar Star = 3
	FourStar  Star = 4
	FiveStar  Star = 5
)

// Valid reports whether the star value is between 1 and 5.
func (s Star) Valid() bool {
	return s >= 1 && s <= 5
}

// Float64 returns the star rating as a floating‑point number (1.0–5.0).
func (s Star) Float64() float64 {
	return float64(s)
}

// String returns the star rating as a string, e.g. "3".
func (s Star) String() string {
	return strconv.Itoa(int(s))
}

// StarString returns a string like "★★★☆☆" for a rating of 3.
func (s Star) StarString() string {
	if !s.Valid() {
		return "invalid"
	}
	filled := "★★★★★"[:s]
	empty := "☆☆☆☆☆"[s:]
	return filled + empty
}

// ParseStar parses a string as a Star.
// It accepts "1", "2", "3", "4", "5" (leading/trailing spaces are trimmed).
func ParseStar(s string) (Star, error) {
	var st Star
	_, err := fmt.Sscan(s, &st)
	if err != nil {
		return 0, fmt.Errorf("invalid star format: %w", err)
	}
	if !st.Valid() {
		return 0, fmt.Errorf("star value %d out of range (1-5)", st)
	}
	return st, nil
}

// RandomStar returns a pseudo‑random star rating between 1 and 5.
// It uses the global random source.
func RandomStar() Star {
	return Star(rand.Intn(5) + 1) // 1–5
}

// ------------------------------------------------------------------------
// StarProvider – controllable star provider for tests
// ------------------------------------------------------------------------

// StarProvider defines an interface for obtaining a star rating.
type StarProvider interface {
	// Get returns the current star rating.
	Get() Star
}

// RealStarProvider returns a fixed star rating (used as a fallback).
type RealStarProvider struct {
	value Star
}

// NewRealStarProvider creates a provider that always returns the given star.
func NewRealStarProvider(value Star) *RealStarProvider {
	return &RealStarProvider{value: value}
}

// Get returns the fixed star rating.
func (p *RealStarProvider) Get() Star { return p.value }

// MockStarProvider is a controllable star provider for tests.
type MockStarProvider struct {
	current Star
}

// NewMockStarProvider creates a provider with an initial star rating.
func NewMockStarProvider(initial Star) *MockStarProvider {
	return &MockStarProvider{current: initial}
}

// Get returns the current mocked star rating.
func (m *MockStarProvider) Get() Star {
	return m.current
}

// Set changes the current star rating.
func (m *MockStarProvider) Set(s Star) {
	m.current = s
}