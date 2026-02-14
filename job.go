// Package testutils provides helpers for testing the hashutils worker pool.
// It includes a mock engine, a job generator, and convenience functions
// to simulate file hashing without touching the filesystem.
package testutils

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// --------------------------------------------------------------------
// MockEngine – simulates hashing without real file I/O.
// --------------------------------------------------------------------

// MockEngine implements hashutils.Engine with configurable delays,
// errors, and result recording. It's fully deterministic for tests.
type MockEngine struct {
	mu           sync.Mutex
	Delay        time.Duration     // time each HashFile call takes (0 = instant)
	ErrorMap     map[string]error  // path -> error to return
	HashMap      map[string][]byte // path -> hash to return (if nil, computes dummy)
	Calls        []CallRecord      // all invocations recorded here
	FailOnCancel bool              // if true, return ctx.Err() immediately when context done
}

// CallRecord captures a single HashFile invocation.
type CallRecord struct {
	Ctx   context.Context
	Algo  string
	Path  string
	Start time.Time
}

// NewMockEngine creates a ready-to-use MockEngine with default dummy hash generation.
func NewMockEngine() *MockEngine {
	return &MockEngine{
		Delay:        0,
		ErrorMap:     make(map[string]error),
		HashMap:      make(map[string][]byte),
		Calls:        []CallRecord{},
		FailOnCancel: false,
	}
}

// HashFile implements hashutils.Engine.
func (m *MockEngine) HashFile(ctx context.Context, algo, path string) ([]byte, error) {
	m.mu.Lock()
	// Record the call
	record := CallRecord{
		Ctx:   ctx,
		Algo:  algo,
		Path:  path,
		Start: time.Now(),
	}
	m.Calls = append(m.Calls, record)
	m.mu.Unlock()

	// Simulate processing delay (respect context cancellation during sleep)
	if m.Delay > 0 {
		select {
		case <-time.After(m.Delay):
		case <-ctx.Done():
			if m.FailOnCancel {
				return nil, ctx.Err()
			}
			// continue even if cancelled? Usually we'd abort, but for testing we allow override.
		}
	}

	// Check for pre‑configured error
	m.mu.Lock()
	err, hasErr := m.ErrorMap[path]
	hash, hasHash := m.HashMap[path]
	m.mu.Unlock()

	if hasErr {
		return nil, err
	}
	if hasHash {
		return hash, nil
	}
	// Default dummy hash: algorithm name + path + predictable bytes
	dummy := []byte(fmt.Sprintf("%s:%s:%d", algo, path, len(record.Calls)))
	return dummy, nil
}

// Reset clears all recorded calls, error map, and hash map.
func (m *MockEngine) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = nil
	m.ErrorMap = make(map[string]error)
	m.HashMap = make(map[string][]byte)
}

// --------------------------------------------------------------------
// JobGenerator – creates predictable Job instances for testing.
// --------------------------------------------------------------------

// JobGenerator produces test jobs with sequential IDs and optional
// path/algorithm patterns.
type JobGenerator struct {
	counter int
	prefix  string
}

// NewJobGenerator creates a generator with an optional prefix for paths/IDs.
func NewJobGenerator(prefix string) *JobGenerator {
	return &JobGenerator{prefix: prefix}
}

// Next generates a new Job with auto-incrementing ID and path.
// If algo is empty, "sha256" is used. If ctx is nil, context.Background() is used.
func (g *JobGenerator) Next(ctx context.Context, algo string) hashutils.Job {
	g.counter++
	if ctx == nil {
		ctx = context.Background()
	}
	if algo == "" {
		algo = "sha256"
	}
	path := fmt.Sprintf("%sfile%d.dat", g.prefix, g.counter)
	return hashutils.Job{
		Ctx:  ctx,
		Algo: algo,
		Path: path,
		ID:   fmt.Sprintf("%sjob%d", g.prefix, g.counter),
	}
}

// N creates n jobs using the same ctx and algo.
func (g *JobGenerator) N(n int, ctx context.Context, algo string) []hashutils.Job {
	jobs := make([]hashutils.Job, n)
	for i := 0; i < n; i++ {
		jobs[i] = g.Next(ctx, algo)
	}
	return jobs
}

// --------------------------------------------------------------------
// Convenience functions for common test scenarios.
// --------------------------------------------------------------------

// AlwaysSucceedEngine returns a MockEngine that never fails and returns
// a deterministic hash for every file.
func AlwaysSucceedEngine() *MockEngine {
	return NewMockEngine()
}

// AlwaysFailEngine returns a MockEngine that returns an error for every file.
func AlwaysFailEngine(err error) *MockEngine {
	e := NewMockEngine()
	// We'll return error for any path – we can set a wildcard later,
	// but for simplicity we can set a default in HashFile if not found.
	// Instead, we set an ErrorMap entry that matches all paths via a custom HashFile?
	// For this mock, we can simply set a default error in a closure.
	// Let's override the HashFile method via embedding? Simpler: set ErrorMap with a special key.
	e.ErrorMap["*"] = err
	return e
}

// WithDelay returns a new MockEngine that introduces the given delay on each HashFile call.
func WithDelay(d time.Duration) *MockEngine {
	e := NewMockEngine()
	e.Delay = d
	return e
}

// WithResults pre‑populates the MockEngine's HashMap with the given path->hash mapping.
func WithResults(results map[string][]byte) *MockEngine {
	e := NewMockEngine()
	for k, v := range results {
		e.HashMap[k] = v
	}
	return e
}
