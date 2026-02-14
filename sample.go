// Package testutils provides mock and simple in-memory sampling utilities for testing.
package testutils

import (
    "sync"
    "time"
)

// Sampler is the interface for collecting data samples.
type Sampler interface {
    // Add records a sample value with the current time.
    Add(value interface{})
    // AddAt records a sample with a specific timestamp.
    AddAt(timestamp time.Time, value interface{})
    // Samples returns all samples collected so far.
    Samples() []Sample
    // Clear removes all samples.
    Clear()
}

// Sample represents a single data point.
type Sample struct {
    Timestamp time.Time
    Value     interface{}
}

// --------------------------------------------------------------------
// MockSampler – a test double that records all adds and can be programmed.
// --------------------------------------------------------------------

// MockSampler implements Sampler for unit tests.
type MockSampler struct {
    mu       sync.Mutex
    samples  []Sample
    addFunc  func(interface{})          // optional custom behavior for Add
    addAtFunc func(time.Time, interface{}) // optional custom behavior for AddAt
}

// NewMockSampler creates a new mock sampler.
func NewMockSampler() *MockSampler {
    return &MockSampler{}
}

// SetAddFunc overrides the Add method with custom behavior.
func (m *MockSampler) SetAddFunc(fn func(interface{})) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.addFunc = fn
}

// SetAddAtFunc overrides the AddAt method with custom behavior.
func (m *MockSampler) SetAddAtFunc(fn func(time.Time, interface{})) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.addAtFunc = fn
}

// Add records the call and delegates to custom function if set.
func (m *MockSampler) Add(value interface{}) {
    m.mu.Lock()
    defer m.mu.Unlock()
    if m.addFunc != nil {
        m.addFunc(value)
        return
    }
    m.samples = append(m.samples, Sample{Timestamp: time.Now(), Value: value})
}

// AddAt records the call and delegates to custom function if set.
func (m *MockSampler) AddAt(timestamp time.Time, value interface{}) {
    m.mu.Lock()
    defer m.mu.Unlock()
    if m.addAtFunc != nil {
        m.addAtFunc(timestamp, value)
        return
    }
    m.samples = append(m.samples, Sample{Timestamp: timestamp, Value: value})
}

// Samples returns a copy of all recorded samples.
func (m *MockSampler) Samples() []Sample {
    m.mu.Lock()
    defer m.mu.Unlock()
    cp := make([]Sample, len(m.samples))
    copy(cp, m.samples)
    return cp
}

// Clear removes all recorded samples.
func (m *MockSampler) Clear() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.samples = nil
}

// Reset clears samples and custom functions.
func (m *MockSampler) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.samples = nil
    m.addFunc = nil
    m.addAtFunc = nil
}

// --------------------------------------------------------------------
// InMemorySampler – a simple in-memory sampler for integration tests.
// --------------------------------------------------------------------

// InMemorySampler implements Sampler with an in-memory slice.
type InMemorySampler struct {
    mu      sync.RWMutex
    samples []Sample
}

// NewInMemorySampler creates a new empty sampler.
func NewInMemorySampler() *InMemorySampler {
    return &InMemorySampler{}
}

// Add records a sample with the current time.
func (s *InMemorySampler) Add(value interface{}) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.samples = append(s.samples, Sample{Timestamp: time.Now(), Value: value})
}

// AddAt records a sample with the given timestamp.
func (s *InMemorySampler) AddAt(timestamp time.Time, value interface{}) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.samples = append(s.samples, Sample{Timestamp: timestamp, Value: value})
}

// Samples returns a copy of all samples.
func (s *InMemorySampler) Samples() []Sample {
    s.mu.RLock()
    defer s.mu.RUnlock()
    cp := make([]Sample, len(s.samples))
    copy(cp, s.samples)
    return cp
}

// Clear removes all samples.
func (s *InMemorySampler) Clear() {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.samples = nil
}

// Len returns the number of samples.
func (s *InMemorySampler) Len() int {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return len(s.samples)
}