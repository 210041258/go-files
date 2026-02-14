// Package unique provides utilities for generating unique identifiers,
// deduplicating slices, tracking uniqueness in concurrent environments,
// and streaming uniqueness with probabilistic data structures.
//
// ID generation includes:
//   - UUID v4 (RFC 4122)
//   - NanoID (secure, URL‑safe)
//   - Timestamp‑based (lexicographically sortable)
//   - Custom random strings
//   - Human‑readable names (via pathname)
//
// Deduplication helpers extend the slice package with duplicate detection,
// order‑preserving uniqueness, and concurrent‑safe sets.
package testutils

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// --------------------------------------------------------------------
// ID generation
// --------------------------------------------------------------------

// UUID represents a 128‑bit UUID (RFC 4122).
type UUID [16]byte

// String returns the standard hexadecimal representation:
// xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.
func (u UUID) String() string {
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		u[0:4], u[4:6], u[6:8], u[8:10], u[10:16])
}

// Bytes returns the raw 16‑byte slice.
func (u UUID) Bytes() []byte { return u[:] }

// IsZero reports whether the UUID is the zero value.
func (u UUID) IsZero() bool { return u == UUID{} }

// UUIDv4 generates a new random UUID version 4.
// It is safe for concurrent use.
func UUIDv4() (UUID, error) {
	var u UUID
	if _, err := rand.Read(u[:]); err != nil {
		return u, fmt.Errorf("unique: generate UUIDv4: %w", err)
	}
	// Set version (4) and variant (RFC 4122)
	u[6] = (u[6] & 0x0f) | 0x40 // version 4
	u[8] = (u[8] & 0x3f) | 0x80 // variant 1
	return u, nil
}

// MustUUIDv4 panics if UUID generation fails.
func MustUUIDv4() UUID {
	u, err := UUIDv4()
	safe.Must0(err)
	return u
}

// NanoID is a secure, URL‑friendly unique string identifier.
// Default length is 21 characters (~128 bits of entropy).
type NanoID string

// NewNanoID generates a new NanoID with the given length.
// If length <= 0, default 21 is used.
// Uses crypto/rand for secure randomness.
func NewNanoID(length int) (NanoID, error) {
	if length <= 0 {
		length = 21
	}
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ_abcdefghijklmnopqrstuvwxyz-"
	const alphabetLen = 64 // 2^6
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("unique: generate NanoID: %w", err)
	}
	for i, b := range bytes {
		bytes[i] = alphabet[b%alphabetLen]
	}
	return NanoID(bytes), nil
}

// MustNanoID panics if NanoID generation fails.
func MustNanoID(length int) NanoID {
	id, err := NewNanoID(length)
	safe.Must0(err)
	return id
}

// String returns the NanoID as a string.
func (id NanoID) String() string { return string(id) }

// TimeID returns a lexicographically sortable unique ID based on current time.
// Format: {timestamp-ns}-{random-suffix}
// Suitable for distributed systems (not guaranteed unique under extreme concurrency).
func TimeID() string {
	now := time.Now().UnixNano()
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(now))
	// Append 4 random bytes
	randBytes := make([]byte, 4)
	rand.Read(randBytes) // best effort; error ignored
	return hex.EncodeToString(buf[:]) + hex.EncodeToString(randBytes)
}

// RandomString generates a random hex string of length n.
// If n is odd, it is rounded up to the next even number.
func RandomString(n int) (string, error) {
	if n <= 0 {
		return "", nil
	}
	bytes := make([]byte, (n+1)/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("unique: random string: %w", err)
	}
	return hex.EncodeToString(bytes)[:n], nil
}

// MustRandomString panics if random string generation fails.
func MustRandomString(n int) string {
	s, err := RandomString(n)
	safe.Must0(err)
	return s
}

// --------------------------------------------------------------------
// Set data structures (generic, concurrent‑safe)
// --------------------------------------------------------------------

// Set is a generic, concurrent‑safe set of comparable values.
type Set[T comparable] struct {
	mu    sync.RWMutex
	items map[T]struct{}
}

// NewSet creates a new empty Set.
func NewSet[T comparable]() *Set[T] {
	return &Set[T]{
		items: make(map[T]struct{}),
	}
}

// NewSetFrom creates a Set containing the elements from the slice.
func NewSetFrom[T comparable](elems []T) *Set[T] {
	s := NewSet[T]()
	for _, e := range elems {
		s.Add(e)
	}
	return s
}

// Add inserts a value into the set. Returns true if the value was newly added.
func (s *Set[T]) Add(v T) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.items[v]
	s.items[v] = struct{}{}
	return !ok
}

// Remove deletes a value from the set. Returns true if it existed.
func (s *Set[T]) Remove(v T) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.items[v]
	delete(s.items, v)
	return ok
}

// Contains reports whether v is in the set.
func (s *Set[T]) Contains(v T) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.items[v]
	return ok
}

// Len returns the number of elements in the set.
func (s *Set[T]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

// Slice returns all elements in the set as a slice (order not guaranteed).
func (s *Set[T]) Slice() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]T, 0, len(s.items))
	for v := range s.items {
		res = append(res, v)
	}
	return res
}

// ForEach iterates over the set and calls fn for each element.
func (s *Set[T]) ForEach(fn func(T)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for v := range s.items {
		fn(v)
	}
}

// --------------------------------------------------------------------
// Deduplication helpers (extending slice package)
// --------------------------------------------------------------------

// Duplicates returns a slice of values that appear more than once in s.
// Order of first occurrence is preserved.
func Duplicates[T comparable](s []T) []T {
	seen := make(map[T]bool)
	var dup []T
	for _, v := range s {
		if _, ok := seen[v]; ok {
			// Already seen at least once
			if seen[v] {
				// First duplicate: add to result and mark as added
				dup = append(dup, v)
				seen[v] = false
			}
		} else {
			seen[v] = true
		}
	}
	return dup
}

// UniquePreserveOrder returns a new slice with duplicate elements removed,
// preserving the order of the first occurrence of each element.
// It is a convenience wrapper around slice.Unique.
func UniquePreserveOrder[T comparable](s []T) []T {
	return slice.Unique(s)
}

// IsUnique reports whether all elements in s are distinct.
func IsUnique[T comparable](s []T) bool {
	seen := make(map[T]struct{}, len(s))
	for _, v := range s {
		if _, ok := seen[v]; ok {
			return false
		}
		seen[v] = struct{}{}
	}
	return true
}

// Frequency returns a map of element counts.
func Frequency[T comparable](s []T) map[T]int {
	freq := make(map[T]int, len(s))
	for _, v := range s {
		freq[v]++
	}
	return freq
}

// MostCommon returns the element with the highest frequency and its count.
// If the slice is empty, returns zero value and 0.
func MostCommon[T comparable](s []T) (value T, count int) {
	if len(s) == 0 {
		return value, 0
	}
	freq := Frequency(s)
	maxCount := -1
	var maxVal T
	for v, c := range freq {
		if c > maxCount {
			maxCount = c
			maxVal = v
		}
	}
	return maxVal, maxCount
}

// --------------------------------------------------------------------
// Concurrent uniqueness tracker
// --------------------------------------------------------------------

// Tracker provides a thread‑safe way to claim unique values.
// Useful for generating unique IDs across goroutines without pre‑allocation.
type Tracker struct {
	mu    sync.Mutex
	used  map[any]struct{}
	count int64 // atomic generator fallback
}

// NewTracker creates a new Tracker.
func NewTracker() *Tracker {
	return &Tracker{
		used: make(map[any]struct{}),
	}
}

// TryAdd attempts to add a value to the tracker. Returns true if the value
// was not already present and was successfully added.
func (t *Tracker) TryAdd(v any) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.used[v]; ok {
		return false
	}
	t.used[v] = struct{}{}
	return true
}

// ReserveInt reserves a unique integer starting from 1.
// It is safe for concurrent use.
func (t *Tracker) ReserveInt() int {
	for {
		next := int(atomic.AddInt64(&t.count, 1))
		if t.TryAdd(next) {
			return next
		}
	}
}

// ReserveString generates a unique random hex string of length n.
// It retries until a unique string is generated.
func (t *Tracker) ReserveString(n int) (string, error) {
	for {
		s, err := RandomString(n)
		if err != nil {
			return "", err
		}
		if t.TryAdd(s) {
			return s, nil
		}
	}
}

// --------------------------------------------------------------------
// Integration with value.Option
// --------------------------------------------------------------------

// ToOption converts a value and a uniqueness predicate into an Option.
// If the value is already used according to the predicate, returns None.
// Otherwise, returns Some(value) and marks it as used.
//
// This is useful for streaming uniqueness with external state.
func ToOption[T comparable](v T, used func(T) bool, mark func(T)) value.Option[T] {
	if used(v) {
		return value.None[T]()
	}
	mark(v)
	return value.Some(v)
}

// --------------------------------------------------------------------
// Probabilistic uniqueness (Bloom filter) – requires build tag `bloom`
// See bloom.go for implementation.
// --------------------------------------------------------------------
