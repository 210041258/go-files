// Package testutils provides deterministic and non‑deterministic random number
// generators for use in tests. It is concurrency‑safe, panic‑free on valid
// inputs, and offers both a mutex‑protected generator for shared use and a
// fast, non‑blocking variant for high‑volume single‑goroutine scenarios.
package testutils

import (
	"crypto/rand"
	"encoding/binary"
	"math/rand"
	"sync"
	"sync/atomic"
)

// ----------------------------------------------------------------------------
// Concurrency‑safe generator (mutex‑protected)
// ----------------------------------------------------------------------------

// RandomGenerator is a concurrency‑safe source of random numbers.
// It is suitable for tests and non‑cryptographic use, and can be shared
// across goroutines.
type RandomGenerator struct {
	rnd *rand.Rand
	mu  sync.Mutex
}

// NewRandomGenerator creates a generator with a non‑deterministic seed
// obtained from crypto/rand. Panics if crypto/rand fails – acceptable in tests.
func NewRandomGenerator() *RandomGenerator {
	return &RandomGenerator{
		rnd: rand.New(rand.NewSource(cryptoSeed())),
	}
}

// NewRandomGeneratorWithSeed creates a deterministic generator from a fixed seed.
// This is useful for reproducible tests.
func NewRandomGeneratorWithSeed(seed int64) *RandomGenerator {
	return &RandomGenerator{
		rnd: rand.New(rand.NewSource(seed)),
	}
}

// IntBetween returns a random integer in the inclusive range [min, max].
// If min > max, they are swapped. It safely handles min == max.
func (g *RandomGenerator) IntBetween(min, max int) int {
	if min > max {
		min, max = max, min
	}
	if min == max {
		return min
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.rnd.Intn(max-min+1) + min
}

// ----------------------------------------------------------------------------
// High‑performance, non‑concurrent generator
// ----------------------------------------------------------------------------

// FastRand is a random number generator that is NOT safe for concurrent use.
// It is designed for high‑volume loops that stay within a single goroutine.
// Use sync.Pool or per‑goroutine instances if you need both speed and concurrency.
type FastRand struct {
	rnd *rand.Rand
}

// NewFastRand creates a fast generator with a non‑deterministic seed.
// Panics if crypto/rand fails.
func NewFastRand() *FastRand {
	return &FastRand{
		rnd: rand.New(rand.NewSource(cryptoSeed())),
	}
}

// NewFastRandWithSeed creates a fast generator with a fixed seed.
func NewFastRandWithSeed(seed int64) *FastRand {
	return &FastRand{
		rnd: rand.New(rand.NewSource(seed)),
	}
}

// IntBetween returns a random integer in the inclusive range [min, max].
// It swaps min/max if necessary and handles min == max without panicking.
// This method is **not** safe for concurrent use.
func (f *FastRand) IntBetween(min, max int) int {
	if min > max {
		min, max = max, min
	}
	if min == max {
		return min
	}
	return f.rnd.Intn(max-min+1) + min
}

// ----------------------------------------------------------------------------
// Package‑level default generator – atomic, lazy, and deterministic override.
// ----------------------------------------------------------------------------

var defaultGen atomic.Pointer[RandomGenerator]

// defaultGenerator returns the lazy‑initialised default generator.
// The initial generator is non‑deterministic unless SetDefaultSeed has been
// called before the first call to IntBetween.
func defaultGenerator() *RandomGenerator {
	if gen := defaultGen.Load(); gen != nil {
		return gen
	}
	// No generator yet – create a non‑deterministic one.
	newGen := NewRandomGenerator()
	if defaultGen.CompareAndSwap(nil, newGen) {
		return newGen
	}
	// Another goroutine beat us; use the already stored generator.
	return defaultGen.Load()
}

// SetDefaultSeed replaces the default generator with a deterministic one.
// It must be called **before** any use of the package‑level IntBetween,
// typically in TestMain. This function is safe for concurrent use.
func SetDefaultSeed(seed int64) {
	defaultGen.Store(NewRandomGeneratorWithSeed(seed))
}

// IntBetween returns a random integer using the package‑level default generator.
// It is safe for concurrent use.
func IntBetween(min, max int) int {
	return defaultGenerator().IntBetween(min, max)
}

// ----------------------------------------------------------------------------
// Internal: cryptographically‑strong seed
// ----------------------------------------------------------------------------

// cryptoSeed reads 8 bytes from crypto/rand and interprets them as an int64 seed.
// Panics if reading fails – in test code this is a fatal error that should be
// noticed immediately.
func cryptoSeed() int64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return int64(binary.LittleEndian.Uint64(b[:]))
}

// ----------------------------------------------------------------------------
// Optional: sync.Pool helper for FastRand
// ----------------------------------------------------------------------------

// FastRandPool is a pool of FastRand generators that each use their own seed.
// The pool automatically seeds each new generator with a distinct seed derived
// from crypto/rand. It is safe for concurrent use and efficient for workloads
// where many short‑lived goroutines need fast randomness.
//
// Example usage:
//
//	pool := NewFastRandPool()
//	rand := pool.Get()
//	defer pool.Put(rand)
//	val := rand.IntBetween(1, 100)
type FastRandPool struct {
	pool sync.Pool
}

// NewFastRandPool creates a new pool of FastRand generators.
// Each generator created by the pool is seeded from crypto/rand.
func NewFastRandPool() *FastRandPool {
	return &FastRandPool{
		pool: sync.Pool{
			New: func() any {
				return NewFastRand()
			},
		},
	}
}

// NewFastRandPoolWithSeed creates a new pool where every generator uses the same
// fixed seed. This is useful for deterministic test suites that still want to
// use per‑goroutine generators. Note that all goroutines will see the same
// deterministic sequence – this may or may not be desirable.
func NewFastRandPoolWithSeed(seed int64) *FastRandPool {
	return &FastRandPool{
		pool: sync.Pool{
			New: func() any {
				return NewFastRandWithSeed(seed)
			},
		},
	}
}

// Get retrieves a FastRand from the pool. The generator should be returned
// with Put after use.
func (p *FastRandPool) Get() *FastRand {
	return p.pool.Get().(*FastRand)
}

// Put returns a FastRand to the pool. Do not use the generator after calling Put.
func (p *FastRandPool) Put(g *FastRand) {
	p.pool.Put(g)
}

// ----------------------------------------------------------------------------
// Compatibility note for math/rand/v2 (Go 1.22+)
// ----------------------------------------------------------------------------
// The code uses rand.New(rand.NewSource(...)). When Go 1.22 introduces
// math/rand/v2, this exact call will continue to work because v2 retains
// the same constructor signatures. No changes are required for compatibility.

// *-&-*
