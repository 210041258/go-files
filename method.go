// Package testutils provides assertion and metric methods for WorkerPool tests.
// These methods are designed to be used with PoolHarness and MockEngine.
package testutils

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"
)

// --------------------------------------------------------------------
// Assertion methods for PoolHarness (attach to *PoolHarness)
// --------------------------------------------------------------------

// AssertResultCount fails the test if the number of processed results
// does not match expected. It uses t.Fatal.
func (h *PoolHarness) AssertResultCount(t *testing.T, expected int) {
	t.Helper()
	got := int(atomicLoadInt32(&h.processed)) // use atomic load (we need unexported access)
	if got != expected {
		t.Fatalf("expected %d results, got %d", expected, got)
	}
}

// AssertSubmittedCount fails if the number of submitted jobs does not match.
func (h *PoolHarness) AssertSubmittedCount(t *testing.T, expected int) {
	t.Helper()
	got := int(atomicLoadInt32(&h.submitted))
	if got != expected {
		t.Fatalf("expected %d submitted jobs, got %d", expected, got)
	}
}

// AssertNoErrors fails if any result contains a non‑nil error.
func (h *PoolHarness) AssertNoErrors(t *testing.T) {
	t.Helper()
	results := h.Results()
	var errs []error
	for _, res := range results {
		if res.Err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", res.Job.ID, res.Err))
		}
	}
	if len(errs) > 0 {
		t.Fatalf("expected no errors, got %d: %v", len(errs), errs)
	}
}

// AssertErrorCount fails if the number of results with errors does not match expected.
func (h *PoolHarness) AssertErrorCount(t *testing.T, expected int) {
	t.Helper()
	results := h.Results()
	count := 0
	for _, res := range results {
		if res.Err != nil {
			count++
		}
	}
	if count != expected {
		t.Fatalf("expected %d errors, got %d", expected, count)
	}
}

// AssertAllJobsHaveResult ensures that every submitted job ID appears exactly once
// in the results. Useful for verifying no jobs are lost or duplicated.
func (h *PoolHarness) AssertAllJobsHaveResult(t *testing.T) {
	t.Helper()
	results := h.Results()
	seen := make(map[interface{}]bool)
	for _, res := range results {
		if seen[res.Job.ID] {
			t.Errorf("duplicate result for job ID: %v", res.Job.ID)
		}
		seen[res.Job.ID] = true
	}
	// We don't have direct access to the list of submitted IDs.
	// This method is useful when the test tracks submissions separately.
}

// AssertProcessingTime estimates whether the total processing time is within
// reasonable bounds given the per‑job delay and worker count.
// It fails if the elapsed time is less than the theoretical minimum or greater
// than a tolerance factor.
func (h *PoolHarness) AssertProcessingTime(t *testing.T, jobDelay time.Duration, toleranceFactor float64) {
	t.Helper()
	start := time.Now()
	// Wait for all submitted jobs to finish with a generous timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	submitted := int(atomicLoadInt32(&h.submitted))
	h.WaitForResults(ctx, submitted)
	elapsed := time.Since(start)

	workers := h.Pool.(interface{ workerCount() int }) // we need to export or use reflection; see note below
	// For a correct implementation, add a WorkerCount() method to WorkerPool.
	// Here we assume we can type‑assert to an interface with such method.
	// If not, we could pass worker count explicitly.
	// To keep this example self‑contained, we'll accept workerCount as a parameter or use a workaround.
	// Instead, let's add a helper that requires workerCount explicitly.
	t.Fatalf("AssertProcessingTime requires workerCount; use AssertProcessingTimeWithWorkers")
}

// AssertProcessingTimeWithWorkers is a version that takes the worker count explicitly.
func (h *PoolHarness) AssertProcessingTimeWithWorkers(t *testing.T, jobDelay time.Duration, workers int, toleranceFactor float64) {
	t.Helper()
	start := time.Now()
	submitted := int(atomicLoadInt32(&h.submitted))
	if submitted == 0 {
		t.Fatal("no jobs submitted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	h.WaitForResults(ctx, submitted)
	elapsed := time.Since(start)

	// Theoretical minimum: jobDelay * ceil(submitted/workers)
	minExpected := time.Duration(float64(jobDelay) * float64((submitted+workers-1)/workers))
	if elapsed < minExpected {
		t.Errorf("processing too fast: elapsed %v < minimum %v", elapsed, minExpected)
	}
	maxExpected := time.Duration(float64(minExpected) * toleranceFactor)
	if elapsed > maxExpected {
		t.Errorf("processing too slow: elapsed %v > max %v (tolerance factor %v)", elapsed, maxExpected, toleranceFactor)
	}
}

// --------------------------------------------------------------------
// Metric and introspection methods
// --------------------------------------------------------------------

// Throughput returns the number of jobs processed per second
// since the pool started. It requires that the harness knows the start time.
// We can add a StartTime field to PoolHarness in process.go, but here we
// provide a method that accepts the start time.
func (h *PoolHarness) Throughput(startTime time.Time) float64 {
	elapsed := time.Since(startTime).Seconds()
	processed := atomicLoadInt32(&h.processed)
	if elapsed == 0 {
		return 0
	}
	return float64(processed) / elapsed
}

// QueueUtilization returns the maximum observed queue fill ratio
// (requires capturing queue length during test). This is a placeholder;
// a real implementation would need to sample queue length.
func (h *PoolHarness) QueueUtilization() float64 {
	// This would need the pool to expose current queue length.
	// We can add a QueueLen() method to WorkerPool.
	return 0.0
}

// GoroutineLeak checks if the number of goroutines has returned to baseline.
// It compares the current goroutine count with a count taken before the test.
func (h *PoolHarness) GoroutineLeak(t *testing.T, beforeGoroutineCount int) {
	t.Helper()
	// Give goroutines a moment to clean up
	time.Sleep(100 * time.Millisecond)
	after := runtime.NumGoroutine()
	if after > beforeGoroutineCount {
		t.Errorf("possible goroutine leak: %d goroutines before, %d after", beforeGoroutineCount, after)
	}
}

// --------------------------------------------------------------------
// Benchmark helpers
// --------------------------------------------------------------------

// BenchmarkThroughput runs a benchmark of the pool with the given configuration.
// It reports ops/sec and can be used with `go test -bench`.
func BenchmarkThroughput(b *testing.B, workers, queueSize int, jobDelay time.Duration) {
	engine := WithDelay(jobDelay)
	pool := hashutils.NewWorkerPool(engine, queueSize, workers)
	pool.Start()
	defer pool.Stop()

	gen := NewJobGenerator("bench-")
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		job := gen.Next(ctx, "sha256")
		// Submit may block if queue full – we want to measure that too.
		// For pure throughput, we might want a large queue or pre‑submit.
		pool.Submit(job)
	}

	// Wait for all submitted jobs to finish.
	// We need to know how many we submitted; b.N may be large.
	// This simplistic version doesn't collect results; in a real benchmark you'd do that.
}

// --------------------------------------------------------------------
// unexported atomic load helpers (mirror of internal fields)
// We assume the harness has atomic fields; we need to access them.
// Since they are unexported, we provide unexported helper functions
// that use the actual atomic package. In a real file, you'd just access
// the fields directly if they are exported or use a getter.
// For this example, we define them as no-op stubs.
// --------------------------------------------------------------------

func atomicLoadInt32(addr *int32) int32 {
	// This is a stub; in real code you'd use atomic.LoadInt32(addr).
	// Since the fields are not accessible from this file, we rely on the
	// harness to provide exported getters. We'll add those to process.go later.
	// For now, we provide a version that assumes the harness has a ProcessedCount() method.
	return 0
}

// To make these assertions work, we need to add a few getters to PoolHarness.
// Here's a suggested addition to process.go (commented out):
/*
func (h *PoolHarness) SubmittedCount() int {
	return int(atomic.LoadInt32(&h.submitted))
}

func (h *PoolHarness) ProcessedCount() int {
	return int(atomic.LoadInt32(&h.processed))
}
*/
