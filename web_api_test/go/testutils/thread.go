// Package thread provides concurrency primitives for the gateway.
package thread

import (
    "context"
    "sync"
)

// Job is a function that performs some work. It receives a context and returns an error.
type Job func(ctx context.Context) error

// Pool manages a fixed number of workers that process jobs from a queue.
type Pool struct {
    workers  int
    jobs     chan Job
    wg       sync.WaitGroup
    quit     chan struct{}
    started  bool
    mu       sync.Mutex
}

// NewPool creates a new worker pool with the given number of workers.
func NewPool(workers int) *Pool {
    return &Pool{
        workers: workers,
        jobs:    make(chan Job),
        quit:    make(chan struct{}),
    }
}

// Start launches the worker goroutines.
func (p *Pool) Start(ctx context.Context) {
    p.mu.Lock()
    defer p.mu.Unlock()
    if p.started {
        return
    }
    p.started = true
    for i := 0; i < p.workers; i++ {
        p.wg.Add(1)
        go p.worker(ctx)
    }
}

// worker runs in its own goroutine, pulling jobs from the queue.
func (p *Pool) worker(ctx context.Context) {
    defer p.wg.Done()
    for {
        select {
        case job, ok := <-p.jobs:
            if !ok {
                return // jobs channel closed
            }
            // Execute the job; errors can be logged or passed via callback.
            _ = job(ctx)
        case <-p.quit:
            return
        case <-ctx.Done():
            return
        }
    }
}

// Submit adds a job to the queue. Blocks if the queue is full (channel is unbuffered).
func (p *Pool) Submit(job Job) {
    select {
    case p.jobs <- job:
    case <-p.quit:
        // pool is stopping, ignore
    }
}

// Stop gracefully shuts down the pool. It waits for all workers to finish current jobs.
func (p *Pool) Stop() {
    p.mu.Lock()
    defer p.mu.Unlock()
    if !p.started {
        return
    }
    close(p.quit)
    p.wg.Wait()
    close(p.jobs)
    p.started = false
}