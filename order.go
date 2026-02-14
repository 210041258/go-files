// Package order provides ordering and sequencing utilities for the data gateway.
package testutils

import (
    "container/heap"
    "sync"
    "time"
)

// Sequencer assigns sequence numbers to messages.
type Sequencer struct {
    mu        sync.Mutex
    counter   uint64
    prefix    string // optional prefix for IDs
}

// NewSequencer creates a sequencer starting from a given offset.
func NewSequencer(offset uint64, prefix string) *Sequencer {
    return &Sequencer{counter: offset, prefix: prefix}
}

// Next returns the next sequence number (as uint64) and a string ID.
func (s *Sequencer) Next() (uint64, string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.counter++
    return s.counter, s.prefix + itoa(s.counter)
}

// Sequenceable is an interface for items that have a sequence number.
type Sequenceable interface {
    Sequence() uint64
}

// Message is a simple implementation of Sequenceable.
type Message struct {
    Seq   uint64
    Data  interface{}
    Time  time.Time
}

func (m Message) Sequence() uint64 { return m.Seq }

// ReorderBuffer holds out-of-order messages and releases them in order.
type ReorderBuffer struct {
    expected uint64
    pending  map[uint64]Sequenceable
    heap     *seqHeap
    mu       sync.Mutex
    out      chan Sequenceable
}

// NewReorderBuffer creates a buffer that will emit messages in order.
func NewReorderBuffer(startSeq uint64, bufferSize int) *ReorderBuffer {
    rb := &ReorderBuffer{
        expected: startSeq,
        pending:  make(map[uint64]Sequenceable),
        heap:     &seqHeap{},
        out:      make(chan Sequenceable, bufferSize),
    }
    heap.Init(rb.heap)
    return rb
}

// Insert adds a message to the buffer. It may be released immediately
// if its sequence is the next expected, or held otherwise.
func (rb *ReorderBuffer) Insert(msg Sequenceable) {
    rb.mu.Lock()
    defer rb.mu.Unlock()
    seq := msg.Sequence()
    if seq == rb.expected {
        // Fast path: this is the next expected message
        rb.out <- msg
        rb.expected++
        // Release any subsequent messages that are now contiguous
        rb.releaseContiguous()
    } else if seq > rb.expected {
        // Out-of-order; store in pending and heap
        rb.pending[seq] = msg
        heap.Push(rb.heap, seq)
    } else {
        // seq < rb.expected: duplicate or late message - drop or handle
    }
}

// releaseContiguous flushes from heap all messages that are now in order.
func (rb *ReorderBuffer) releaseContiguous() {
    for rb.heap.Len() > 0 {
        smallest := (*rb.heap)[0]
        if smallest == rb.expected {
            // Found the next expected
            heap.Pop(rb.heap)
            msg := rb.pending[smallest]
            delete(rb.pending, smallest)
            rb.out <- msg
            rb.expected++
        } else {
            break
        }
    }
}

// Output returns the channel from which ordered messages can be read.
func (rb *ReorderBuffer) Output() <-chan Sequenceable {
    return rb.out
}

// Close stops the buffer.
func (rb *ReorderBuffer) Close() {
    close(rb.out)
}

// seqHeap is a min-heap of uint64 sequence numbers.
type seqHeap []uint64

func (h seqHeap) Len() int           { return len(h) }
func (h seqHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h seqHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *seqHeap) Push(x interface{}) { *h = append(*h, x.(uint64)) }
func (h *seqHeap) Pop() interface{} {
    old := *h
    n := len(old)
    x := old[n-1]
    *h = old[:n-1]
    return x
}

// Helper to convert uint64 to string.
func itoa(n uint64) string {
    return string(rune(n)) // placeholder; use strconv.FormatUint in real code
}