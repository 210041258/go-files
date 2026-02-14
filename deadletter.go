// Package testutils provides mock and simple in-memory dead letter queues for testing.
package testutils

import (
    "context"
    "errors"
    "sync"
)

// DeadLetter is the minimal interface expected by the system for a dead letter queue.
type DeadLetter interface {
    // Send adds a message to the dead letter queue.
    Send(ctx context.Context, msg interface{}) error
}

// --------------------------------------------------------------------
// MockDeadLetter – a test double that records all messages.
// --------------------------------------------------------------------

// MockDeadLetter implements DeadLetter for unit tests.
type MockDeadLetter struct {
    mu       sync.Mutex
    messages []interface{}
}

// NewMockDeadLetter creates a new mock dead letter queue.
func NewMockDeadLetter() *MockDeadLetter {
    return &MockDeadLetter{}
}

// Send records the message. It never fails.
func (m *MockDeadLetter) Send(ctx context.Context, msg interface{}) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.messages = append(m.messages, msg)
    return nil
}

// Messages returns a copy of all messages sent so far.
func (m *MockDeadLetter) Messages() []interface{} {
    m.mu.Lock()
    defer m.mu.Unlock()
    cp := make([]interface{}, len(m.messages))
    copy(cp, m.messages)
    return cp
}

// Count returns the number of messages sent.
func (m *MockDeadLetter) Count() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return len(m.messages)
}

// Clear removes all recorded messages.
func (m *MockDeadLetter) Clear() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.messages = nil
}

// --------------------------------------------------------------------
// InMemoryDeadLetter – a simple bounded dead letter queue for integration tests.
// --------------------------------------------------------------------

// ErrDeadLetterFull is returned by InMemoryDeadLetter when the queue is full.
var ErrDeadLetterFull = errors.New("dead letter queue is full")

// InMemoryDeadLetter implements a bounded in-memory dead letter queue.
type InMemoryDeadLetter struct {
    mu       sync.Mutex
    messages []interface{}
    maxSize  int // maximum number of messages; 0 means unlimited
}

// NewInMemoryDeadLetter creates a new bounded dead letter queue.
// If maxSize <= 0, the queue is unbounded.
func NewInMemoryDeadLetter(maxSize int) *InMemoryDeadLetter {
    return &InMemoryDeadLetter{
        maxSize: maxSize,
    }
}

// Send adds a message if the queue is not full.
// Returns ErrDeadLetterFull if the queue has reached maxSize.
func (d *InMemoryDeadLetter) Send(ctx context.Context, msg interface{}) error {
    d.mu.Lock()
    defer d.mu.Unlock()
    if d.maxSize > 0 && len(d.messages) >= d.maxSize {
        return ErrDeadLetterFull
    }
    d.messages = append(d.messages, msg)
    return nil
}

// Messages returns a copy of all messages in the queue.
func (d *InMemoryDeadLetter) Messages() []interface{} {
    d.mu.Lock()
    defer d.mu.Unlock()
    cp := make([]interface{}, len(d.messages))
    copy(cp, d.messages)
    return cp
}

// Count returns the current number of messages.
func (d *InMemoryDeadLetter) Count() int {
    d.mu.Lock()
    defer d.mu.Unlock()
    return len(d.messages)
}

// Clear empties the queue.
func (d *InMemoryDeadLetter) Clear() {
    d.mu.Lock()
    defer d.mu.Unlock()
    d.messages = nil
}