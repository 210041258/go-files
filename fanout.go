// Package testutils provides mock and simple in-memory fan-out mechanisms for testing.
package testutils

import (
    "context"
    "sync"
    "sync/atomic"
)

// Fanout is the interface for distributing messages to multiple receivers.
type Fanout interface {
    // Register adds a receiver. Returns an ID that can be used to unregister.
    Register(receiver Receiver) string
    // Unregister removes a receiver by ID.
    Unregister(id string)
    // Send delivers a message to all registered receivers.
    // Implementations should handle panics and ensure all receivers get the message
    // unless the context is cancelled.
    Send(ctx context.Context, msg interface{})
}

// Receiver is a function that processes a message.
type Receiver func(ctx context.Context, msg interface{})

// --------------------------------------------------------------------
// MockFanout – a test double that records registrations and sent messages.
// --------------------------------------------------------------------

// MockFanout implements Fanout for unit tests.
type MockFanout struct {
    mu            sync.Mutex
    receivers     map[string]Receiver
    registerCalls []string      // IDs returned
    unregisterCalls []string    // IDs removed
    sentMessages  []interface{} // all messages sent via Send
}

// NewMockFanout creates a new mock fanout.
func NewMockFanout() *MockFanout {
    return &MockFanout{
        receivers: make(map[string]Receiver),
    }
}

// Register records the registration and returns a new ID.
func (m *MockFanout) Register(receiver Receiver) string {
    m.mu.Lock()
    defer m.mu.Unlock()
    id := generateID()
    m.receivers[id] = receiver
    m.registerCalls = append(m.registerCalls, id)
    return id
}

// Unregister records the unregistration.
func (m *MockFanout) Unregister(id string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    delete(m.receivers, id)
    m.unregisterCalls = append(m.unregisterCalls, id)
}

// Send records the message and also calls each registered receiver synchronously.
// This allows tests to verify side effects of receivers.
func (m *MockFanout) Send(ctx context.Context, msg interface{}) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.sentMessages = append(m.sentMessages, msg)
    for _, recv := range m.receivers {
        // Call receiver in the same goroutine; tests can override with custom receivers.
        recv(ctx, msg)
    }
}

// SentMessages returns a copy of all messages sent.
func (m *MockFanout) SentMessages() []interface{} {
    m.mu.Lock()
    defer m.mu.Unlock()
    cp := make([]interface{}, len(m.sentMessages))
    copy(cp, m.sentMessages)
    return cp
}

// RegisterCalls returns the list of IDs returned by Register calls.
func (m *MockFanout) RegisterCalls() []string {
    m.mu.Lock()
    defer m.mu.Unlock()
    cp := make([]string, len(m.registerCalls))
    copy(cp, m.registerCalls)
    return cp
}

// UnregisterCalls returns the list of IDs passed to Unregister calls.
func (m *MockFanout) UnregisterCalls() []string {
    m.mu.Lock()
    defer m.mu.Unlock()
    cp := make([]string, len(m.unregisterCalls))
    copy(cp, m.unregisterCalls)
    return cp
}

// Reset clears all recorded calls and receivers.
func (m *MockFanout) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.receivers = make(map[string]Receiver)
    m.registerCalls = nil
    m.unregisterCalls = nil
    m.sentMessages = nil
}

// --------------------------------------------------------------------
// InMemoryFanout – a simple concurrent fan-out for integration tests.
// --------------------------------------------------------------------

// InMemoryFanout implements Fanout with real concurrent delivery.
// It launches a goroutine for each receiver when Send is called.
type InMemoryFanout struct {
    mu        sync.RWMutex
    receivers map[string]Receiver
    nextID    uint64
}

// NewInMemoryFanout creates a new empty fanout.
func NewInMemoryFanout() *InMemoryFanout {
    return &InMemoryFanout{
        receivers: make(map[string]Receiver),
    }
}

// Register adds a receiver and returns its unique ID.
func (f *InMemoryFanout) Register(receiver Receiver) string {
    f.mu.Lock()
    defer f.mu.Unlock()
    id := generateUint64ID(&f.nextID)
    f.receivers[id] = receiver
    return id
}

// Unregister removes a receiver by ID.
func (f *InMemoryFanout) Unregister(id string) {
    f.mu.Lock()
    defer f.mu.Unlock()
    delete(f.receivers, id)
}

// Send delivers the message to all registered receivers concurrently.
// Each receiver runs in its own goroutine. If the context is cancelled,
// receivers that haven't started yet may be skipped, but already running
// receivers are not interrupted (they can check ctx themselves).
func (f *InMemoryFanout) Send(ctx context.Context, msg interface{}) {
    f.mu.RLock()
    // Take a snapshot of receivers to avoid holding lock while calling them.
    snapshot := make([]Receiver, 0, len(f.receivers))
    for _, r := range f.receivers {
        snapshot = append(snapshot, r)
    }
    f.mu.RUnlock()

    var wg sync.WaitGroup
    for _, recv := range snapshot {
        wg.Add(1)
        go func(r Receiver) {
            defer wg.Done()
            // If context is already done, skip? Or let receiver decide?
            // We'll call receiver; it can check ctx early.
            r(ctx, msg)
        }(recv)
    }
    // Wait for all receivers to finish (optional, but typical in tests).
    // In production, you might not wait. For test predictability, we wait.
    wg.Wait()
}

// Count returns the number of registered receivers.
func (f *InMemoryFanout) Count() int {
    f.mu.RLock()
    defer f.mu.RUnlock()
    return len(f.receivers)
}

// generateUint64ID returns a string ID from an auto-incrementing counter.
func generateUint64ID(counter *uint64) string {
    id := atomic.AddUint64(counter, 1)
    return string(rune(id)) // simplified; use strconv in real code
}

// generateID is a helper for MockFanout (simple sequential).
var mockIDCounter uint64

func generateID() string {
    id := atomic.AddUint64(&mockIDCounter, 1)
    return string(rune(id))
}