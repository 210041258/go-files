// Package testutils provides mock and simple in-memory data collectors for testing.
// It supports collecting metrics and statistics from storage, memory, network, and databases.
package testutils

import (
    "errors"
    "sync"
    "time"
)

// --------------------------------------------------------------------
// Collector interface and data types
// --------------------------------------------------------------------

// Collector defines methods for collecting data from various system resources.
type Collector interface {
    // CollectStorage returns current storage statistics.
    CollectStorage() (StorageStats, error)
    // CollectMemory returns current memory statistics.
    CollectMemory() (MemoryStats, error)
    // CollectNetwork returns current network statistics.
    CollectNetwork() (NetworkStats, error)
    // CollectDB returns current database statistics (e.g., query results, connection pool).
    CollectDB() (DBStats, error)
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
    // Additional simulated metrics
    QueryCount       int64
    QueryErrors      int64
}

// --------------------------------------------------------------------
// MockCollector – a test double that records calls and can be programmed.
// --------------------------------------------------------------------

// MockCollector implements Collector for unit tests.
type MockCollector struct {
    mu              sync.Mutex
    storageStats    StorageStats
    memoryStats     MemoryStats
    networkStats    NetworkStats
    dbStats         DBStats
    storageErr      error
    memoryErr       error
    networkErr      error
    dbErr           error
    storageCalls    int
    memoryCalls     int
    networkCalls    int
    dbCalls         int
}

// NewMockCollector creates a new mock collector with zero stats and no errors.
func NewMockCollector() *MockCollector {
    return &MockCollector{}
}

// SetStorageStats programs the stats returned by CollectStorage.
func (m *MockCollector) SetStorageStats(stats StorageStats) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.storageStats = stats
}

// SetStorageError programs the error returned by CollectStorage.
func (m *MockCollector) SetStorageError(err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.storageErr = err
}

// SetMemoryStats programs the stats returned by CollectMemory.
func (m *MockCollector) SetMemoryStats(stats MemoryStats) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.memoryStats = stats
}

// SetMemoryError programs the error returned by CollectMemory.
func (m *MockCollector) SetMemoryError(err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.memoryErr = err
}

// SetNetworkStats programs the stats returned by CollectNetwork.
func (m *MockCollector) SetNetworkStats(stats NetworkStats) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.networkStats = stats
}

// SetNetworkError programs the error returned by CollectNetwork.
func (m *MockCollector) SetNetworkError(err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.networkErr = err
}

// SetDBStats programs the stats returned by CollectDB.
func (m *MockCollector) SetDBStats(stats DBStats) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.dbStats = stats
}

// SetDBError programs the error returned by CollectDB.
func (m *MockCollector) SetDBError(err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.dbErr = err
}

// CollectStorage returns programmed stats or error.
func (m *MockCollector) CollectStorage() (StorageStats, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.storageCalls++
    return m.storageStats, m.storageErr
}

// CollectMemory returns programmed stats or error.
func (m *MockCollector) CollectMemory() (MemoryStats, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.memoryCalls++
    return m.memoryStats, m.memoryErr
}

// CollectNetwork returns programmed stats or error.
func (m *MockCollector) CollectNetwork() (NetworkStats, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.networkCalls++
    return m.networkStats, m.networkErr
}

// CollectDB returns programmed stats or error.
func (m *MockCollector) CollectDB() (DBStats, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.dbCalls++
    return m.dbStats, m.dbErr
}

// StorageCalls returns the number of times CollectStorage was called.
func (m *MockCollector) StorageCalls() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.storageCalls
}

// MemoryCalls returns the number of times CollectMemory was called.
func (m *MockCollector) MemoryCalls() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.memoryCalls
}

// NetworkCalls returns the number of times CollectNetwork was called.
func (m *MockCollector) NetworkCalls() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.networkCalls
}

// DBCalls returns the number of times CollectDB was called.
func (m *MockCollector) DBCalls() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.dbCalls
}

// Reset clears all programmed stats, errors, and call counts.
func (m *MockCollector) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.storageStats = StorageStats{}
    m.memoryStats = MemoryStats{}
    m.networkStats = NetworkStats{}
    m.dbStats = DBStats{}
    m.storageErr = nil
    m.memoryErr = nil
    m.networkErr = nil
    m.dbErr = nil
    m.storageCalls = 0
    m.memoryCalls = 0
    m.networkCalls = 0
    m.dbCalls = 0
}

// --------------------------------------------------------------------
// InMemoryCollector – a simple collector that returns simulated data.
// --------------------------------------------------------------------

// InMemoryCollector implements Collector with in-memory counters that can be updated.
// Useful for integration tests where you need to simulate changing resource usage.
type InMemoryCollector struct {
    mu            sync.RWMutex
    storageStats  StorageStats
    memoryStats   MemoryStats
    networkStats  NetworkStats
    dbStats       DBStats
    storageErr    error
    memoryErr     error
    networkErr    error
    dbErr         error
}

// NewInMemoryCollector creates a new collector with zero stats.
func NewInMemoryCollector() *InMemoryCollector {
    return &InMemoryCollector{}
}

// UpdateStorageStats allows tests to modify storage statistics.
func (c *InMemoryCollector) UpdateStorageStats(update func(*StorageStats)) {
    c.mu.Lock()
    defer c.mu.Unlock()
    update(&c.storageStats)
}

// UpdateMemoryStats allows tests to modify memory statistics.
func (c *InMemoryCollector) UpdateMemoryStats(update func(*MemoryStats)) {
    c.mu.Lock()
    defer c.mu.Unlock()
    update(&c.memoryStats)
}

// UpdateNetworkStats allows tests to modify network statistics.
func (c *InMemoryCollector) UpdateNetworkStats(update func(*NetworkStats)) {
    c.mu.Lock()
    defer c.mu.Unlock()
    update(&c.networkStats)
}

// UpdateDBStats allows tests to modify database statistics.
func (c *InMemoryCollector) UpdateDBStats(update func(*DBStats)) {
    c.mu.Lock()
    defer c.mu.Unlock()
    update(&c.dbStats)
}

// SetStorageError makes CollectStorage return the given error.
func (c *InMemoryCollector) SetStorageError(err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.storageErr = err
}

// SetMemoryError makes CollectMemory return the given error.
func (c *InMemoryCollector) SetMemoryError(err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.memoryErr = err
}

// SetNetworkError makes CollectNetwork return the given error.
func (c *InMemoryCollector) SetNetworkError(err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.networkErr = err
}

// SetDBError makes CollectDB return the given error.
func (c *InMemoryCollector) SetDBError(err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.dbErr = err
}

// CollectStorage returns current storage stats or error.
func (c *InMemoryCollector) CollectStorage() (StorageStats, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if c.storageErr != nil {
        return StorageStats{}, c.storageErr
    }
    return c.storageStats, nil
}

// CollectMemory returns current memory stats or error.
func (c *InMemoryCollector) CollectMemory() (MemoryStats, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if c.memoryErr != nil {
        return MemoryStats{}, c.memoryErr
    }
    return c.memoryStats, nil
}

// CollectNetwork returns current network stats or error.
func (c *InMemoryCollector) CollectNetwork() (NetworkStats, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if c.networkErr != nil {
        return NetworkStats{}, c.networkErr
    }
    return c.networkStats, nil
}

// CollectDB returns current database stats or error.
func (c *InMemoryCollector) CollectDB() (DBStats, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if c.dbErr != nil {
        return DBStats{}, c.dbErr
    }
    return c.dbStats, nil
}

// --------------------------------------------------------------------
// CollectorFunc – adapter to turn functions into a Collector.
// --------------------------------------------------------------------

// CollectorFunc is an adapter that allows ordinary functions to be used as a Collector.
type CollectorFunc struct {
    storageFunc func() (StorageStats, error)
    memoryFunc  func() (MemoryStats, error)
    networkFunc func() (NetworkStats, error)
    dbFunc      func() (DBStats, error)
}

// NewCollectorFromFuncs creates a Collector from optional functions.
// Any nil function will return zero stats and nil error.
func NewCollectorFromFuncs(
    storage func() (StorageStats, error),
    memory func() (MemoryStats, error),
    network func() (NetworkStats, error),
    db func() (DBStats, error),
) *CollectorFunc {
    return &CollectorFunc{
        storageFunc: storage,
        memoryFunc:  memory,
        networkFunc: network,
        dbFunc:      db,
    }
}

// CollectStorage calls the storage function or returns zero stats.
func (c *CollectorFunc) CollectStorage() (StorageStats, error) {
    if c.storageFunc != nil {
        return c.storageFunc()
    }
    return StorageStats{}, nil
}

// CollectMemory calls the memory function or returns zero stats.
func (c *CollectorFunc) CollectMemory() (MemoryStats, error) {
    if c.memoryFunc != nil {
        return c.memoryFunc()
    }
    return MemoryStats{}, nil
}

// CollectNetwork calls the network function or returns zero stats.
func (c *CollectorFunc) CollectNetwork() (NetworkStats, error) {
    if c.networkFunc != nil {
        return c.networkFunc()
    }
    return NetworkStats{}, nil
}

// CollectDB calls the database function or returns zero stats.
func (c *CollectorFunc) CollectDB() (DBStats, error) {
    if c.dbFunc != nil {
        return c.dbFunc()
    }
    return DBStats{}, nil
}