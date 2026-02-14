// Package hashutils provides an enterprise-grade, high-performance hashing library
// supporting context-aware streaming, generics, and pluggable algorithms.
package testutils

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"fmt"
	"hash"
	"hash/fnv"
	"io"
	"os"
	"sync"
)

// HashSum is a generic constraint for supported hash output types.
type HashSum interface {
	[]byte | uint64
}

// Algorithm defines the behavior of a specific hash algorithm.
type Algorithm interface {
	// Name returns the standard name of the algorithm (e.g., "sha256").
	Name() string
	// New returns a new hash.Hash instance.
	New() hash.Hash
	// Is64Bit returns true if this algorithm produces a 64-bit integer sum.
	Is64Bit() bool
}

// algo wraps standard library hash constructors to satisfy the Algorithm interface.
type algo struct {
	name    string
	newHash func() hash.Hash
	is64Bit bool
}

func (a *algo) Name() string   { return a.name }
func (a *algo) New() hash.Hash { return a.newHash() }
func (a *algo) Is64Bit() bool  { return a.is64Bit }

// HashEngine manages a registry of algorithms and handles computations.
// It is designed for dependency injection and isolated state.
type HashEngine struct {
	registry map[string]Algorithm
	mu       sync.RWMutex
	bufPool  *sync.Pool
}

// NewEngine creates a new HashEngine with the specified algorithms.
func NewEngine(algs []Algorithm) *HashEngine {
	e := &HashEngine{
		registry: make(map[string]Algorithm, len(algs)),
		// Buffer pool optimized for 32KB chunks (L2 cache friendly)
		bufPool: &sync.Pool{
			New: func() interface{} {
				return make([]byte, 32*1024)
			},
		},
	}
	for _, alg := range algs {
		e.registry[alg.Name()] = alg
	}
	return e
}

// DefaultEngine returns an engine pre-configured with common secure and insecure algorithms.
func DefaultEngine() *HashEngine {
	return NewEngine([]Algorithm{
		&algo{"md5", md5.New, false},
		&algo{"sha1", sha1.New, false},
		&algo{"sha256", sha256.New, false},
		&algo{"sha512", sha512.New, false},
		&algo{"fnv64a", func() hash.Hash { return fnv.New64a() }, true},
		&algo{"fnv", func() hash.Hash { return fnv.New64() }, true},
	})
}

// SecureEngine returns an engine containing only cryptographically secure algorithms.
func SecureEngine() *HashEngine {
	return NewEngine([]Algorithm{
		&algo{"sha256", sha256.New, false},
		&algo{"sha512", sha512.New, false},
		// Add SHA3, BLAKE2, etc. here as needed
	})
}

// Register adds a new algorithm to the engine dynamically.
func (e *HashEngine) Register(alg Algorithm) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.registry[alg.Name()] = alg
}

// Compute hashes the input data using the named algorithm.
// It utilizes Go Generics to return the sum as either []byte or uint64.
func Compute[T HashSum](e *HashEngine, algoName string, data []byte) (T, error) {
	var zero T

	e.mu.RLock()
	alg, ok := e.registry[algoName]
	e.mu.RUnlock()

	if !ok {
		return zero, fmt.Errorf("unknown algorithm: %q", algoName)
	}

	h := alg.New()
	h.Write(data)

	// Type switch to handle the generic constraint logic
	switch any(zero).(type) {
	case []byte:
		return any(h.Sum(nil)).(T), nil
	case uint64:
		if h64, ok := h.(hash.Hash64); ok {
			return any(h64.Sum64()).(T), nil
		}
		return zero, fmt.Errorf("algorithm %q does not support 64-bit sums", algoName)
	default:
		return zero, errors.New("unsupported return type requested")
	}
}

// Stream hashes data from an io.Reader, respecting the provided context.
// This is ideal for large files or network streams.
func Stream(ctx context.Context, e *HashEngine, algoName string, r io.Reader) ([]byte, error) {
	e.mu.RLock()
	alg, ok := e.registry[algoName]
	e.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown algorithm: %q", algoName)
	}

	h := alg.New()
	buf := e.bufPool.Get().([]byte)
	defer e.bufPool.Put(buf)

	// Wrap the reader to check context on every read
	cr := &contextReader{ctx: ctx, r: r}

	if _, err := io.CopyBuffer(h, cr, buf); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

// File is a convenience wrapper around Stream for local file paths.
func File(ctx context.Context, e *HashEngine, algoName, path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Stream(ctx, e, algoName, f)
}

// contextReader wraps an io.Reader to respect context cancellation.
type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (cr *contextReader) Read(p []byte) (n int, err error) {
	if err := cr.ctx.Err(); err != nil {
		return 0, err
	}
	return cr.r.Read(p)
}
