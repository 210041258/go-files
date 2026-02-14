// Package testutils provides utilities for simulating resource constraints
// (network, storage, memory) in tests.
package testutils

import (
    "errors"
    "io"
    "sync"
    "time"
)

// --------------------------------------------------------------------
// NetworkConditioner – simulates network latency, bandwidth, and errors.
// --------------------------------------------------------------------

// NetworkConditioner implements a network-like interface with configurable
// delays, rate limits, and error injection.
type NetworkConditioner struct {
    mu          sync.Mutex
    latency     time.Duration
    bandwidth   int           // bytes per second (0 = unlimited)
    lossRate    float64       // 0.0–1.0 probability of packet loss
    writeErrors map[int]error // per-call error simulation (for testing)
    callCount   int
}

// NewNetworkConditioner creates a new conditioner with default unlimited, zero latency.
func NewNetworkConditioner() *NetworkConditioner {
    return &NetworkConditioner{
        writeErrors: make(map[int]error),
    }
}

// SetLatency adds a fixed delay to every write.
func (n *NetworkConditioner) SetLatency(d time.Duration) {
    n.mu.Lock()
    defer n.mu.Unlock()
    n.latency = d
}

// SetBandwidth limits throughput to bytes per second.
func (n *NetworkConditioner) SetBandwidth(bytesPerSec int) {
    n.mu.Lock()
    defer n.mu.Unlock()
    n.bandwidth = bytesPerSec
}

// SetLossRate sets the probability (0.0–1.0) of dropping a write (simulating packet loss).
func (n *NetworkConditioner) SetLossRate(rate float64) {
    n.mu.Lock()
    defer n.mu.Unlock()
    n.lossRate = rate
}

// InjectWriteError makes the nth call to Write (1‑based) return the given error.
func (n *NetworkConditioner) InjectWriteError(callNumber int, err error) {
    n.mu.Lock()
    defer n.mu.Unlock()
    n.writeErrors[callNumber] = err
}

// Write implements io.Writer with simulated conditions.
func (n *NetworkConditioner) Write(p []byte) (int, error) {
    n.mu.Lock()
    n.callCount++
    call := n.callCount
    if err, ok := n.writeErrors[call]; ok {
        delete(n.writeErrors, call)
        n.mu.Unlock()
        return 0, err
    }
    latency := n.latency
    bandwidth := n.bandwidth
    lossRate := n.lossRate
    n.mu.Unlock()

    // Simulate packet loss
    if lossRate > 0 {
        if randFloat() < lossRate {
            return len(p), nil // pretend it was sent but lost (no error)
        }
    }

    // Simulate bandwidth limit (sleep proportional to size)
    if bandwidth > 0 {
        sleepTime := time.Duration(len(p)) * time.Second / time.Duration(bandwidth)
        time.Sleep(sleepTime)
    }

    // Simulate latency
    if latency > 0 {
        time.Sleep(latency)
    }

    return len(p), nil
}

// --------------------------------------------------------------------
// StorageConditioner – simulates disk full, slow I/O, and errors.
// --------------------------------------------------------------------

// StorageConditioner implements io.ReadWriteCloser with simulated storage constraints.
type StorageConditioner struct {
    mu          sync.Mutex
    data        []byte
    capacity    int           // max bytes (0 = unlimited)
    writeSpeed  int           // bytes per second (0 = unlimited)
    readSpeed   int           // bytes per second (0 = unlimited)
    writeErrors map[int]error
    readErrors  map[int]error
    writeCalls  int
    readCalls   int
}

// NewStorageConditioner creates a new conditioner with unlimited capacity and speed.
func NewStorageConditioner() *StorageConditioner {
    return &StorageConditioner{
        writeErrors: make(map[int]error),
        readErrors:  make(map[int]error),
    }
}

// SetCapacity limits the total storage to max bytes.
func (s *StorageConditioner) SetCapacity(max int) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.capacity = max
}

// SetWriteSpeed limits write throughput in bytes per second.
func (s *StorageConditioner) SetWriteSpeed(bytesPerSec int) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.writeSpeed = bytesPerSec
}

// SetReadSpeed limits read throughput in bytes per second.
func (s *StorageConditioner) SetReadSpeed(bytesPerSec int) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.readSpeed = bytesPerSec
}

// InjectWriteError makes the nth call to Write return the given error.
func (s *StorageConditioner) InjectWriteError(callNumber int, err error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.writeErrors[callNumber] = err
}

// InjectReadError makes the nth call to Read return the given error.
func (s *StorageConditioner) InjectReadError(callNumber int, err error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.readErrors[callNumber] = err
}

// Write stores data, respecting capacity and speed limits.
func (s *StorageConditioner) Write(p []byte) (int, error) {
    s.mu.Lock()
    s.writeCalls++
    call := s.writeCalls
    if err, ok := s.writeErrors[call]; ok {
        delete(s.writeErrors, call)
        s.mu.Unlock()
        return 0, err
    }
    // Check capacity
    if s.capacity > 0 && len(s.data)+len(p) > s.capacity {
        available := s.capacity - len(s.data)
        if available <= 0 {
            s.mu.Unlock()
            return 0, errors.New("storage full")
        }
        // Write partial
        p = p[:available]
    }
    speed := s.writeSpeed
    s.mu.Unlock()

    // Simulate write speed
    if speed > 0 {
        sleepTime := time.Duration(len(p)) * time.Second / time.Duration(speed)
        time.Sleep(sleepTime)
    }

    s.mu.Lock()
    defer s.mu.Unlock()
    s.data = append(s.data, p...)
    return len(p), nil
}

// Read reads data from storage, respecting read speed.
func (s *StorageConditioner) Read(p []byte) (int, error) {
    s.mu.Lock()
    s.readCalls++
    call := s.readCalls
    if err, ok := s.readErrors[call]; ok {
        delete(s.readErrors, call)
        s.mu.Unlock()
        return 0, err
    }
    if len(s.data) == 0 {
        s.mu.Unlock()
        return 0, io.EOF
    }
    n := copy(p, s.data)
    s.data = s.data[n:]
    speed := s.readSpeed
    s.mu.Unlock()

    // Simulate read speed
    if speed > 0 {
        sleepTime := time.Duration(n) * time.Second / time.Duration(speed)
        time.Sleep(sleepTime)
    }
    return n, nil
}

// Close is a no-op.
func (s *StorageConditioner) Close() error { return nil }

// --------------------------------------------------------------------
// MemoryConditioner – simulates memory pressure and allocation failures.
// --------------------------------------------------------------------

// MemoryConditioner tracks allocations and can simulate out-of-memory conditions.
type MemoryConditioner struct {
    mu          sync.Mutex
    used        int
    limit       int           // max allocatable bytes (0 = unlimited)
    failOn      map[int]error // allocation number to fail
    allocCalls  int
}

// NewMemoryConditioner creates a new memory conditioner.
func NewMemoryConditioner() *MemoryConditioner {
    return &MemoryConditioner{
        failOn: make(map[int]error),
    }
}

// SetLimit sets the maximum allocatable memory (in bytes).
func (m *MemoryConditioner) SetLimit(limit int) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.limit = limit
}

// InjectAllocError makes the nth call to Allocate return an error.
func (m *MemoryConditioner) InjectAllocError(callNumber int, err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.failOn[callNumber] = err
}

// Allocate simulates allocating n bytes. Returns error if over limit or injected.
func (m *MemoryConditioner) Allocate(n int) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.allocCalls++
    if err, ok := m.failOn[m.allocCalls]; ok {
        delete(m.failOn, m.allocCalls)
        return err
    }
    if m.limit > 0 && m.used+n > m.limit {
        return errors.New("out of memory")
    }
    m.used += n
    return nil
}

// Free releases n bytes.
func (m *MemoryConditioner) Free(n int) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.used -= n
    if m.used < 0 {
        m.used = 0
    }
}

// Used returns the current allocated bytes.
func (m *MemoryConditioner) Used() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.used
}

// --------------------------------------------------------------------
// Helper (pseudo‑random for loss simulation)
// --------------------------------------------------------------------
import "math/rand"

func randFloat() float64 {
    return rand.Float64()
}