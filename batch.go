// Package batch provides utilities for batching messages.
package batch

import (
    "context"
    "sync"
    "time"

    "yourproject/plugs" // if using plugs.Message
)

// Config holds batch settings.
type Config struct {
    // MaxSize is the maximum number of messages in a batch.
    MaxSize int `json:"max_size"`
    // MaxBytes is the maximum total bytes of messages in a batch.
    MaxBytes int `json:"max_bytes"`
    // FlushInterval is the maximum duration to wait before flushing a partial batch.
    FlushInterval time.Duration `json:"flush_interval"`
    // MaxConcurrent is the maximum number of batches being processed simultaneously.
    MaxConcurrent int `json:"max_concurrent"`
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() Config {
    return Config{
        MaxSize:       100,
        MaxBytes:      1 << 20, // 1 MB
        FlushInterval: 5 * time.Second,
        MaxConcurrent: 10,
    }
}

// Batch represents a collection of messages ready for sending.
type Batch struct {
    Messages []plugs.Message
    Size     int // number of messages
    Bytes    int // total bytes (approx)
    Created  time.Time
}

// Accumulator collects messages into batches and flushes them when conditions are met.
type Accumulator struct {
    cfg       Config
    batch     *Batch
    mu        sync.Mutex
    flushC    chan *Batch
    done      chan struct{}
    timer     *time.Timer
    lastFlush time.Time
}

// NewAccumulator creates a new batch accumulator.
func NewAccumulator(cfg Config, flushC chan *Batch) *Accumulator {
    if cfg.MaxSize <= 0 {
        cfg.MaxSize = DefaultConfig().MaxSize
    }
    if cfg.FlushInterval <= 0 {
        cfg.FlushInterval = DefaultConfig().FlushInterval
    }
    a := &Accumulator{
        cfg:    cfg,
        flushC: flushC,
        done:   make(chan struct{}),
        timer:  time.NewTimer(cfg.FlushInterval),
    }
    a.resetBatch()
    go a.runTimer()
    return a
}

// Add inserts a message into the current batch, flushing if thresholds are exceeded.
func (a *Accumulator) Add(ctx context.Context, msg plugs.Message) error {
    a.mu.Lock()
    defer a.mu.Unlock()

    // Estimate message size (simplified)
    msgBytes := len(msg.Payload) + len(msg.ID) + 100 // rough estimate

    // Check if adding this message would exceed thresholds
    if (a.batch.Size+1 > a.cfg.MaxSize) || (a.batch.Bytes+msgBytes > a.cfg.MaxBytes) {
        // Flush current batch before adding
        a.flushLocked()
    }

    // Add to batch
    a.batch.Messages = append(a.batch.Messages, msg)
    a.batch.Size++
    a.batch.Bytes += msgBytes

    // If batch now full, flush immediately
    if a.batch.Size >= a.cfg.MaxSize || a.batch.Bytes >= a.cfg.MaxBytes {
        a.flushLocked()
    }

    return nil
}

// flushLocked sends the current batch to the flush channel and resets.
// Must be called with lock held.
func (a *Accumulator) flushLocked() {
    if a.batch.Size == 0 {
        return
    }
    // Send batch (non-blocking, but if channel full, we might need to handle)
    select {
    case a.flushC <- a.batch:
    default:
        // Optionally log or drop; in production, you'd want a blocking send or a retry mechanism.
    }
    a.resetBatch()
    a.lastFlush = time.Now()
}

// resetBatch creates a new empty batch.
func (a *Accumulator) resetBatch() {
    a.batch = &Batch{
        Messages: make([]plugs.Message, 0, a.cfg.MaxSize),
        Created:  time.Now(),
    }
}

// runTimer periodically flushes based on time.
func (a *Accumulator) runTimer() {
    for {
        select {
        case <-a.timer.C:
            a.mu.Lock()
            // Flush if there's anything and it's been long enough since last flush
            if a.batch.Size > 0 && time.Since(a.lastFlush) >= a.cfg.FlushInterval {
                a.flushLocked()
            }
            a.timer.Reset(a.cfg.FlushInterval)
            a.mu.Unlock()
        case <-a.done:
            a.timer.Stop()
            return
        }
    }
}

// Close stops the accumulator and flushes any remaining messages.
func (a *Accumulator) Close() {
    close(a.done)
    a.mu.Lock()
    defer a.mu.Unlock()
    if a.batch.Size > 0 {
        a.flushLocked()
    }
}

// Processor manages a pool of workers that process batches from a channel.
type Processor struct {
    cfg        Config
    flushC     <-chan *Batch
    workers    int
    processFn  func(context.Context, *Batch) error
    wg         sync.WaitGroup
    cancel     context.CancelFunc
}

// NewProcessor creates a batch processor with a pool of workers.
func NewProcessor(cfg Config, flushC <-chan *Batch, workers int, fn func(context.Context, *Batch) error) *Processor {
    if workers <= 0 {
        workers = cfg.MaxConcurrent
    }
    return &Processor{
        cfg:       cfg,
        flushC:    flushC,
        workers:   workers,
        processFn: fn,
    }
}

// Start launches the worker pool.
func (p *Processor) Start(ctx context.Context) {
    ctx, p.cancel = context.WithCancel(ctx)
    for i := 0; i < p.workers; i++ {
        p.wg.Add(1)
        go p.worker(ctx)
    }
}

// worker processes batches from the flush channel.
func (p *Processor) worker(ctx context.Context) {
    defer p.wg.Done()
    for {
        select {
        case batch, ok := <-p.flushC:
            if !ok {
                return
            }
            // Process the batch
            if err := p.processFn(ctx, batch); err != nil {
                // Handle error: maybe retry, log, send to dead letter queue
            }
        case <-ctx.Done():
            return
        }
    }
}

// Stop gracefully shuts down the processor.
func (p *Processor) Stop() {
    if p.cancel != nil {
        p.cancel()
    }
    p.wg.Wait()
}