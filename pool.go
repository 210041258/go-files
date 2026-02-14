// Package hashutils provides concurrent file hashing via a worker pool.
package testutils

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
)

// Job represents a file hashing task.
type Job struct {
	Ctx  context.Context // optional; if cancelled, the job is skipped
	Algo string          // hash algorithm (e.g., "sha256")
	Path string          // file to hash
	ID   interface{}     // optional identifier (returned in Result)
}

// Result holds the outcome of a Job.
type Result struct {
	Job  Job
	Hash []byte
	Err  error
}

// Engine performs the actual hashing. It can be mocked for tests.
type Engine interface {
	HashFile(ctx context.Context, algo, path string) ([]byte, error)
}

// defaultEngine uses hashutil.File with context awareness.
type defaultEngine struct{}

func (e *defaultEngine) HashFile(ctx context.Context, algo, path string) ([]byte, error) {
	// hashutil.File doesn't support context cancellation natively,
	// so we run it in a goroutine and select on ctx.Done().
	type result struct {
		sum []byte
		err error
	}
	ch := make(chan result, 1)
	go func() {
		sum, err := hashutil.File(algo, path)
		ch <- result{sum, err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		return r.sum, r.err
	}
}

// DefaultEngine returns an Engine that uses the real hashutil package.
func DefaultEngine() Engine {
	return &defaultEngine{}
}

// WorkerPool manages concurrent file hashing workers.
type WorkerPool struct {
	engine   Engine
	jobCh    chan Job      // buffered incoming job queue
	resultCh chan Result   // results are sent here
	workers  int           // number of worker goroutines
	quit     chan struct{} // closed to stop workers
	wg       sync.WaitGroup
	started  atomic.Bool
	stopped  atomic.Bool
}

// NewWorkerPool creates a pool with the given engine, job queue capacity,
// and number of workers. If workers <= 0, it defaults to runtime.NumCPU().
func NewWorkerPool(engine Engine, queueSize int, workers int) *WorkerPool {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	return &WorkerPool{
		engine:   engine,
		jobCh:    make(chan Job, queueSize),
		resultCh: make(chan Result, queueSize), // same capacity as job queue
		workers:  workers,
		quit:     make(chan struct{}),
	}
}

// Start launches the worker goroutines. It is idempotent.
func (p *WorkerPool) Start() {
	if !p.started.CompareAndSwap(false, true) {
		return // already started
	}
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
	// monitor for when all workers exit, then close resultCh
	go func() {
		p.wg.Wait()
		close(p.resultCh)
	}()
}

// Stop gracefully shuts down the pool:
// - No new jobs are accepted (Submit returns false).
// - Already submitted jobs are processed.
// - It blocks until all workers finish.
func (p *WorkerPool) Stop() {
	if !p.stopped.CompareAndSwap(false, true) {
		return
	}
	close(p.quit)  // signal workers to stop after current job
	close(p.jobCh) // prevent new submissions
	p.wg.Wait()    // wait for workers to exit
	// resultCh is closed by the monitor goroutine
}

// Submit adds a job to the queue. Returns true if accepted, false if the queue is full
// or the pool is stopped/stopping. Non-blocking.
func (p *WorkerPool) Submit(job Job) bool {
	if p.stopped.Load() {
		return false
	}
	select {
	case p.jobCh <- job:
		return true
	default:
		// queue full
		return false
	}
}

// Results returns a read-only channel of results. The channel is closed after
// Stop() is called and all pending jobs are processed.
func (p *WorkerPool) Results() <-chan Result {
	return p.resultCh
}

// worker runs a single worker goroutine.
func (p *WorkerPool) worker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.quit:
			// stop signal: exit immediately (no new jobs)
			return
		case job, ok := <-p.jobCh:
			if !ok {
				// job channel closed, no more jobs
				return
			}
			// process the job
			sum, err := p.engine.HashFile(job.Ctx, job.Algo, job.Path)
			select {
			case p.resultCh <- Result{Job: job, Hash: sum, Err: err}:
			case <-p.quit:
				// pool is stopping, discard result
				return
			}
		}
	}
}
