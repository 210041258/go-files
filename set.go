// Package testutils provides utilities for testing web APIs.
package testutils

// Set is a generic set data structure for comparable types.
// It is not safe for concurrent use unless explicitly noted.
type Set[T comparable] map[T]struct{}

// NewSet creates a new Set containing the given elements.
func NewSet[T comparable](elems ...T) Set[T] {
	s := make(Set[T], len(elems))
	for _, e := range elems {
		s[e] = struct{}{}
	}
	return s
}

// Add inserts an element into the set.
func (s Set[T]) Add(elem T) {
	s[elem] = struct{}{}
}

// Remove deletes an element from the set.
func (s Set[T]) Remove(elem T) {
	delete(s, elem)
}

// Contains reports whether the set contains the element.
func (s Set[T]) Contains(elem T) bool {
	_, ok := s[elem]
	return ok
}

// Len returns the number of elements in the set.
func (s Set[T]) Len() int {
	return len(s)
}

// IsEmpty reports whether the set has no elements.
func (s Set[T]) IsEmpty() bool {
	return len(s) == 0
}

// Clear removes all elements from the set.
func (s Set[T]) Clear() {
	for k := range s {
		delete(s, k)
	}
}

// Elements returns a slice containing all elements of the set.
// The order is non‑deterministic.
func (s Set[T]) Elements() []T {
	elems := make([]T, 0, len(s))
	for e := range s {
		elems = append(elems, e)
	}
	return elems
}

// Clone returns a new independent copy of the set.
func (s Set[T]) Clone() Set[T] {
	c := make(Set[T], len(s))
	for e := range s {
		c[e] = struct{}{}
	}
	return c
}

// ------------------------------------------------------------------------
// Set operations (return new sets)
// ------------------------------------------------------------------------

// Union returns a new set containing all elements from s and other.
func (s Set[T]) Union(other Set[T]) Set[T] {
	result := s.Clone()
	for e := range other {
		result[e] = struct{}{}
	}
	return result
}

// Intersection returns a new set containing elements present in both s and other.
func (s Set[T]) Intersection(other Set[T]) Set[T] {
	// Iterate over the smaller set for efficiency.
	small, large := s, other
	if len(large) < len(small) {
		small, large = large, small
	}
	result := make(Set[T])
	for e := range small {
		if _, ok := large[e]; ok {
			result[e] = struct{}{}
		}
	}
	return result
}

// Difference returns a new set containing elements in s that are not in other.
func (s Set[T]) Difference(other Set[T]) Set[T] {
	result := make(Set[T])
	for e := range s {
		if _, ok := other[e]; !ok {
			result[e] = struct{}{}
		}
	}
	return result
}

// SymmetricDifference returns a new set containing elements in either s or other,
// but not in both.
func (s Set[T]) SymmetricDifference(other Set[T]) Set[T] {
	// union - intersection
	return s.Union(other).Difference(s.Intersection(other))
}

// ------------------------------------------------------------------------
// Predicates
// ------------------------------------------------------------------------

// IsSubsetOf reports whether every element of s is also in other.
func (s Set[T]) IsSubsetOf(other Set[T]) bool {
	for e := range s {
		if _, ok := other[e]; !ok {
			return false
		}
	}
	return true
}

// IsSupersetOf reports whether every element of other is also in s.
func (s Set[T]) IsSupersetOf(other Set[T]) bool {
	return other.IsSubsetOf(s)
}

// Equals reports whether s and other contain exactly the same elements.
func (s Set[T]) Equals(other Set[T]) bool {
	if len(s) != len(other) {
		return false
	}
	for e := range s {
		if _, ok := other[e]; !ok {
			return false
		}
	}
	return true
}

// ------------------------------------------------------------------------
// Convenience type aliases
// ------------------------------------------------------------------------

// StringSet is a set of strings.
type StringSet = Set[string]

// IntSet is a set of ints.
type IntSet = Set[int]

// FloatSet is a set of float64s.
type FloatSet = Set[float64]