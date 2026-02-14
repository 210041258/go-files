// Package pair provides a generic pair data structure for holding two values.
// It is useful for returning two values from a function, storing key‑value
// pairs, or representing tuples.
package testutils

// Pair is a generic container for two values of possibly different types.
type Pair[A, B any] struct {
	First  A
	Second B
}

// New creates a new Pair with the given first and second values.
func New[A, B any](first A, second B) Pair[A, B] {
	return Pair[A, B]{First: first, Second: second}
}

// Swap returns a new pair with the first and second values exchanged.
func (p Pair[A, B]) Swap() Pair[B, A] {
	return Pair[B, A]{First: p.Second, Second: p.First}
}

// ToMap returns a map with the pair as a single entry. If the map is nil,
// a new map is created. If the pair's key already exists, it overwrites.
func (p Pair[A, B]) ToMap(m map[A]B) map[A]B {
	if m == nil {
		m = make(map[A]B)
	}
	m[p.First] = p.Second
	return m
}

// ----------------------------------------------------------------------
// Slice helpers
// ----------------------------------------------------------------------

// PairsFromMap converts a map to a slice of key‑value pairs.
// The order of the resulting slice is not guaranteed.
func PairsFromMap[A comparable, B any](m map[A]B) []Pair[A, B] {
	pairs := make([]Pair[A, B], 0, len(m))
	for k, v := range m {
		pairs = append(pairs, New(k, v))
	}
	return pairs
}

// MapFromPairs converts a slice of pairs to a map. If the same key appears
// multiple times, the last value overwrites earlier ones.
func MapFromPairs[A comparable, B any](pairs []Pair[A, B]) map[A]B {
	m := make(map[A]B, len(pairs))
	for _, p := range pairs {
		m[p.First] = p.Second
	}
	return m
}

// Firsts returns a slice of the first elements of all pairs.
func Firsts[A, B any](pairs []Pair[A, B]) []A {
	firsts := make([]A, len(pairs))
	for i, p := range pairs {
		firsts[i] = p.First
	}
	return firsts
}

// Seconds returns a slice of the second elements of all pairs.
func Seconds[A, B any](pairs []Pair[A, B]) []B {
	seconds := make([]B, len(pairs))
	for i, p := range pairs {
		seconds[i] = p.Second
	}
	return seconds
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     p := pair.New("key", 42)
//     fmt.Println(p.First, p.Second) // "key", 42
//
//     swapped := p.Swap()
//     fmt.Println(swapped.First, swapped.Second) // 42, "key"
//
//     m := map[string]int{"a": 1}
//     p.ToMap(m) // adds key/value to m
//
//     pairs := []pair.Pair[string, int]{
//         pair.New("x", 10),
//         pair.New("y", 20),
//     }
//     m2 := pair.MapFromPairs(pairs) // map[x:10 y:20]
// }