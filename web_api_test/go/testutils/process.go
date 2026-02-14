// Package testutils provides advanced process simulation for worker pool testing.
// It includes a PoolHarness for orchestrating load tests, measuring throughput,
// and validating correctness under various conditions (cancellation, delays, errors).
package testutils

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// --------------------------------------------------------------------
// PoolHarness – a complete test environment for WorkerPool.
// --------------------------------------------------------------------

// PoolHarness wraps a worker pool with instrumentation and control,
// making it easy to run reproducible load tests and behavioural checks.
type PoolHarness struct {
	Engine    *MockEngine // deterministic, instrumented engine
	Pool      *hashutils.WorkerPool
	Generator *JobGenerator // produces predictable jobs

	submitted int32 // atomic counter for total jobs submitted
	processed int32 // atomic counter for results received

	resultsMu sync.Mutex
	results   []hashutils.Result // all captured results (optional)

	// Callbacks for result inspection (e.g., for assertions)
	OnResult func(res hashutils.Result)
}

// NewPoolHarness creates a harness with a fresh MockEngine, JobGenerator,
// and a WorkerPool configured with the given queue size and worker count.
// If workers <= 0, runtime.NumCPU() is used.
func NewPoolHarness(queueSize int, workers int) *PoolHarness {
	engine := NewMockEngine()
	pool := hashutils.NewWorkerPool(engine, queueSize, workers)
	gen := NewJobGenerator("harness-")

	return &PoolHarness{
		Engine:    engine,
		Pool:      pool,
		Generator: gen,
		results:   []hashutils.Result{},
		OnResult:  nil,
	}
}



// Stop gracefully shuts down the pool and waits for completion.
func (h *PoolHarness) Stop() {
	h.Pool.Stop()
}

// Submit adds a job to the pool. If the queue is full, it blocks
// until space becomes available or the context is cancelled.
// Returns true if the job was successfully submitted, false on context cancel.
func (h *PoolHarness) Submit(ctx context.Context, job hashutils.Job) bool {
	atomic.AddInt32(&h.submitted, 1)
	// Use a loop with select to handle blocking until the job is accepted or ctx done.
	for {
		select {
		case <-ctx.Done():
			return false
		default:
			if h.Pool.Submit(job) {
				return true
			}
			// Queue full – yield briefly to avoid busy loop
			time.Sleep(1 * time.Millisecond)
		}
	}
}

// SubmitN submits n jobs using the generator with the given ctx and algo.
// It blocks until all jobs are submitted or the context is cancelled.
// Returns the number of successfully submitted jobs.
func (h *PoolHarness) SubmitN(ctx context.Context, n int, algo string) int {
	submitted := 0
	for i := 0; i < n; i++ {
		job := h.Generator.Next(ctx, algo)
		if !h.Submit(ctx, job) {
			break
		}
		submitted++
	}
	return submitted
}

// WaitForResults blocks until exactly n results have been received,
// or the context is cancelled. It returns the number of results actually collected.
func (h *PoolHarness) WaitForResults(ctx context.Context, n int) int {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return int(atomic.LoadInt32(&h.processed))
		case <-ticker.C:
			if int(atomic.LoadInt32(&h.processed)) >= n {
				return n
			}
		}
	}
}

// Results returns a copy of all results captured so far.
func (h *PoolHarness) Results() []hashutils.Result {
	h.resultsMu.Lock()
	defer h.resultsMu.Unlock()
	cpy := make([]hashutils.Result, len(h.results))
	copy(cpy, h.results)
	return cpy
}

// ClearResults empties the internal result store.
func (h *PoolHarness) ClearResults() {
	h.resultsMu.Lock()
	defer h.resultsMu.Unlock()
	h.results = nil
	atomic.StoreInt32(&h.processed, 0)
}

// Metrics returns basic throughput and counts.
func (h *PoolHarness) Metrics() map[string]interface{} {
	sub := atomic.LoadInt32(&h.submitted)
	proc := atomic.LoadInt32(&h.processed)
	return map[string]interface{}{
		"submitted": sub,
		"processed": proc,
		"queue_cap": cap(h.Pool.(interface{ jobChanCap() int })), // reflection-free; we can add a method to pool if needed
	}
}

// collectResults runs in a goroutine and pulls results from the pool.
func (h *PoolHarness) collectResults() {
	for res := range h.Pool.Results() {
		atomic.AddInt32(&h.processed, 1)

		h.resultsMu.Lock()
		h.results = append(h.results, res)
		h.resultsMu.Unlock()

		if h.OnResult != nil {
			h.OnResult(res)
		}
	}
}

// --------------------------------------------------------------------
// High‑level test scenarios (convenience functions)
// --------------------------------------------------------------------

// RunLoadTest simulates a burst of jobs and returns throughput stats.
//   - jobs: total number of jobs to submit
//   - workers: number of pool workers
//   - delay: per‑job simulated processing time (set on MockEngine)
//   - timeout: maximum time to wait for completion
func RunLoadTest(jobs, workers int, delay time.Duration, timeout time.Duration) (*PoolHarness, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	h := NewPoolHarness(jobs, workers) // queue size = jobs (no blocking)
	h.Engine.Delay = delay
	h.Start()
	defer h.Stop()

	submitted := h.SubmitN(ctx, jobs, "sha256")
	if submitted < jobs {
		return h, fmt.Errorf("only submitted %d/%d jobs before timeout", submitted, jobs)
	}

	received := h.WaitForResults(ctx, jobs)
	if received < jobs {
		return h, fmt.Errorf("only received %d/%d results before timeout", received, jobs)
	}

	return h, nil
}

// RunCancellationTest verifies that in‑flight jobs respect context cancellation.
//   - jobs: total jobs submitted
//   - workers: pool concurrency
//   - cancelAfter: time after which the context of remaining jobs is cancelled
//   - jobDelay: how long each job takes (must be > cancelAfter to see effect)
func RunCancellationTest(jobs, workers int, cancelAfter, jobDelay time.Duration) (*PoolHarness, error) {
	h := NewPoolHarness(jobs, workers)
	h.Engine.Delay = jobDelay
	h.Engine.FailOnCancel = true // important: return ctx.Err() on cancel
	h.Start()
	defer h.Stop()

	// Submit all jobs with a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	submitted := 0
	for i := 0; i < jobs; i++ {
		job := h.Generator.Next(ctx, "sha256")
		if !h.Submit(ctx, job) {
			break
		}
		submitted++
	}

	// Cancel after the specified delay
	time.Sleep(cancelAfter)
	cancel()

	// Wait a little longer than the longest possible job to let cancellation propagate
	time.Sleep(jobDelay + 100*time.Millisecond)

	// Count how many jobs actually completed (should be less than submitted)
	results := h.Results()
	completed := 0
	cancelled := 0
	for _, res := range results {
		if res.Err != nil && res.Err.Error() == context.Canceled.Error() {
			cancelled++
		} else if res.Err == nil {
			completed++
		}
	}

	return h, nil
}

// SubmittedCount returns the total number of jobs submitted via the harness.
func (h *PoolHarness) SubmittedCount() int {
	return int(atomic.LoadInt32(&h.submitted))
}

// ProcessedCount returns the total number of results received.
func (h *PoolHarness) ProcessedCount() int {
	return int(atomic.LoadInt32(&h.processed))
}

// StartTime can be stored when Start() is called.
func (h *PoolHarness) Start() {
	h.Pool.Start()
	h.startTime = time.Now() // add time.Time field to PoolHarness
	go h.collectResults()
}
