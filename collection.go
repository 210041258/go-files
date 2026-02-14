// Package testutils provides mock and composite collectors for gathering resource statistics.
package testutils

import (
    "sync"
    "time"
)

// --------------------------------------------------------------------
// Snapshot – a point-in-time capture of all resource statistics.
// --------------------------------------------------------------------

// Snapshot holds metrics from storage, memory, network, and database at a given time.
type Snapshot struct {
    Timestamp   time.Time
    Storage     StorageStats
    Memory      MemoryStats
    Network     NetworkStats
    DB          DBStats
}

// StorageStats represents disk usage or file system metrics.
type StorageStats struct {
    TotalBytes     int64
    UsedBytes      int64
    FreeBytes      int64
    ReadOps        int64
    WriteOps       int64
    ReadErrors     int64
    WriteErrors    int64
}

// MemoryStats represents RAM usage metrics.
type MemoryStats struct {
    TotalBytes     int64
    UsedBytes      int64
    FreeBytes      int64
    CachedBytes    int64
    SwapTotalBytes int64
    SwapUsedBytes  int64
}

// NetworkStats represents network interface statistics.
type NetworkStats struct {
    BytesSent      int64
    BytesReceived  int64
    PacketsSent    int64
    PacketsReceived int64
    ErrorsSent     int64
    ErrorsReceived int64
    DroppedSent    int64
    DroppedReceived int64
}

// DBStats represents database connection pool and query metrics.
type DBStats struct {
    OpenConnections  int
    InUseConnections int
    IdleConnections  int
    WaitCount        int64
    WaitDuration     time.Duration
    MaxIdleClosed    int64
    MaxLifetimeClosed int64
    QueryCount       int64
    QueryErrors      int64
}

// --------------------------------------------------------------------
// Collection – interface for capturing a full snapshot of resources.
// --------------------------------------------------------------------

// Collection defines a single method to capture all resource statistics at once.
type Collection interface {
    // Collect returns a snapshot of current resource statistics.
    Collect() (Snapshot, error)
}

// --------------------------------------------------------------------
// MockCollection – a test double that records calls and can be programmed.
// --------------------------------------------------------------------

// MockCollection implements Collection for unit tests.
type MockCollection struct {
    mu         sync.Mutex
    snapshot   Snapshot
    err        error
    callCount  int
}

// NewMockCollection creates a new mock collection with zero snapshot and no error.
func NewMockCollection() *MockCollection {
    return &MockCollection{}
}

// SetSnapshot programs the snapshot returned by Collect.
func (m *MockCollection) SetSnapshot(snap Snapshot) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.snapshot = snap
}

// SetError programs the error returned by Collect.
func (m *MockCollection) SetError(err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.err = err
}

// Collect records the call and returns programmed snapshot and error.
func (m *MockCollection) Collect() (Snapshot, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.callCount++
    return m.snapshot, m.err
}

// CallCount returns the number of times Collect was called.
func (m *MockCollection) CallCount() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.callCount
}

// Reset clears programmed values and call count.
func (m *MockCollection) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.snapshot = Snapshot{}
    m.err = nil
    m.callCount = 0
}

// --------------------------------------------------------------------
// MultiCollector – a composite collector that gathers stats from individual providers.
// --------------------------------------------------------------------

// StorageProvider is a function that returns StorageStats and an error.
type StorageProvider func() (StorageStats, error)

// MemoryProvider is a function that returns MemoryStats and an error.
type MemoryProvider func() (MemoryStats, error)

// NetworkProvider is a function that returns NetworkStats and an error.
type NetworkProvider func() (NetworkStats, error)

// DBProvider is a function that returns DBStats and an error.
type DBProvider func() (DBStats, error)

// MultiCollector implements Collection by calling individual provider functions.
type MultiCollector struct {
    storageFn StorageProvider
    memoryFn  MemoryProvider
    networkFn NetworkProvider
    dbFn      DBProvider
}

// NewMultiCollector creates a collector from optional provider functions.
// Any nil provider returns zero stats and no error.
func NewMultiCollector(
    storage StorageProvider,
    memory MemoryProvider,
    network NetworkProvider,
    db DBProvider,
) *MultiCollector {
    return &MultiCollector{
        storageFn: storage,
        memoryFn:  memory,
        networkFn: network,
        dbFn:      db,
    }
}

// Collect calls each provider and builds a snapshot.
// If any provider returns an error, collection stops and that error is returned.
func (c *MultiCollector) Collect() (Snapshot, error) {
    var snap Snapshot
    snap.Timestamp = time.Now()

    if c.storageFn != nil {
        stats, err := c.storageFn()
        if err != nil {
            return Snapshot{}, err
        }
        snap.Storage = stats
    }

    if c.memoryFn != nil {
        stats, err := c.memoryFn()
        if err != nil {
            return Snapshot{}, err
        }
        snap.Memory = stats
    }

    if c.networkFn != nil {
        stats, err := c.networkFn()
        if err != nil {
            return Snapshot{}, err
        }
        snap.Network = stats
    }

    if c.dbFn != nil {
        stats, err := c.dbFn()
        if err != nil {
            return Snapshot{}, err
        }
        snap.DB = stats
    }

    return snap, nil
}

// --------------------------------------------------------------------
// InMemoryCollection – a simple collection that returns stored snapshots.
// --------------------------------------------------------------------

// InMemoryCollection implements Collection with an in-memory snapshot that can be updated.
type InMemoryCollection struct {
    mu       sync.RWMutex
    snapshot Snapshot
    err      error
}

// NewInMemoryCollection creates a new in-memory collection with zero snapshot.
func NewInMemoryCollection() *InMemoryCollection {
    return &InMemoryCollection{}
}

// UpdateSnapshot allows tests to modify the snapshot.
func (c *InMemoryCollection) UpdateSnapshot(update func(*Snapshot)) {
    c.mu.Lock()
    defer c.mu.Unlock()
    update(&c.snapshot)
    c.snapshot.Timestamp = time.Now() // ensure timestamp is current
}

// SetError makes Collect return the given error.
func (c *InMemoryCollection) SetError(err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.err = err
}

// Collect returns the current snapshot or error.
func (c *InMemoryCollection) Collect() (Snapshot, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if c.err != nil {
        return Snapshot{}, c.err
    }
    // Return a copy to prevent external modification
    return c.snapshot, nil
}