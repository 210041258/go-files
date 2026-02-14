// Package trophy provides utilities for creating and managing
// trophy representations, including level‑based names and emojis.
package testutils

import (
	"fmt"
)

// Trophy represents a trophy with a level and a name.
type Trophy struct {
	Level int
	Name  string
}

// New creates a new trophy for the given level.
// The name is automatically chosen based on the level:
// 1: Bronze, 2: Silver, 3: Gold, 4: Platinum, >=5: Diamond.
func New(level int) Trophy {
	return Trophy{
		Level: level,
		Name:  defaultName(level),
	}
}

// defaultName returns the default name for a trophy level.
func defaultName(level int) string {
	switch level {
	case 1:
		return "Bronze"
	case 2:
		return "Silver"
	case 3:
		return "Gold"
	case 4:
		return "Platinum"
	default:
		return "Diamond"
	}
}

// String returns a formatted trophy string with an appropriate emoji.
func (t Trophy) String() string {
	emoji := trophyEmoji(t.Level)
	return fmt.Sprintf("%s %s", emoji, t.Name)
}

// trophyEmoji returns the emoji for the given level.
func trophyEmoji(level int) string {
	switch level {
	case 1:
		return "🥉"
	case 2:
		return "🥈"
	case 3:
		return "🥇"
	case 4:
		return "🏆"
	default:
		return "💎"
	}
}

// Emoji returns the generic trophy emoji.
func Emoji() string {
	return "🏆"
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     t := trophy.New(3)
//     fmt.Println(t) // "🥇 Gold"
//
//     fmt.Println(trophy.Emoji()) // "🏆"
// }