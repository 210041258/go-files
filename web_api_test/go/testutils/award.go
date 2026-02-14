// Package award provides utilities for generating award strings,
// badges, and simple achievement tracking.
package testutils

import (
	"fmt"
	"strings"
)

// Badge generates a simple badge string with optional emoji.
// Example: Badge("Gold", "🥇") returns "🥇 Gold".
func Badge(name, emoji string) string {
	if emoji != "" {
		return fmt.Sprintf("%s %s", emoji, name)
	}
	return name
}

// Medal returns a medal string based on rank (1,2,3).
// For rank 1, returns "🥇 Gold"; rank 2: "🥈 Silver"; rank 3: "🥉 Bronze";
// otherwise returns the rank as a number.
func Medal(rank int) string {
	switch rank {
	case 1:
		return "🥇 Gold"
	case 2:
		return "🥈 Silver"
	case 3:
		return "🥉 Bronze"
	default:
		return fmt.Sprintf("#%d", rank)
	}
}

// Achievement represents a simple achievement.
type Achievement struct {
	Name        string
	Description string
	Points      int
}

// String returns a formatted achievement string.
func (a Achievement) String() string {
	return fmt.Sprintf("%s (%d pts) - %s", a.Name, a.Points, a.Description)
}

// Trophy returns a trophy emoji string.
func Trophy() string {
	return "🏆"
}

// Star returns a star emoji string repeated n times.
func Star(n int) string {
	return strings.Repeat("⭐", n)
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     fmt.Println(award.Badge("Champion", "🏆"))
//     fmt.Println(award.Medal(1))
//     fmt.Println(award.Trophy())
//     fmt.Println(award.Star(3))
// }