// Package testutils provides mock and simple in-memory resource free space checkers for testing.
package testutils

import (
    "sync"
    "time"
)

// --------------------------------------------------------------------
// FreeStats – represents available/free capacity for each resource.
// --------------------------------------------------------------------

// StorageFree represents available disk space and other storage metrics.
type StorageFree struct {
    FreeBytes      int64
    FreeInodes     int64 // optional
}

// MemoryFree represents available RAM and swap.
type MemoryFree struct {
    FreeBytes      int64
    AvailableBytes int64 // includes cached memory that can be reclaimed
    SwapFreeBytes  int64
}

// NetworkFree represents available network bandwidth or connection slots.
type NetworkFree struct {
    AvailableBandwidth int64 // bytes per second
    AvailableSockets   int
    AvailableConnections int
}

// DBFree represents available database connection pool slots and other limits.
type DBFree struct {
    AvailableConnections int
    MaxConnections       int
    AvailableThreads     int
}

// --------------------------------------------------------------------
// Free – interface for checking available resources.
// --------------------------------------------------------------------

type Free interface {
    // StorageFree returns available disk space.
    StorageFree() (StorageFree, error)
    // MemoryFree returns available memory.
    MemoryFree() (MemoryFree, error)
    // NetworkFree returns available network capacity.
    NetworkFree() (NetworkFree, error)
    // DBFree returns available database resources.
    DBFree() (DBFree, error)
}

// --------------------------------------------------------------------
// MockFree – a test double that records calls and can be programmed.
// --------------------------------------------------------------------

type MockFree struct {
    mu           sync.Mutex
    storageFree  StorageFree
    memoryFree   MemoryFree
    networkFree  NetworkFree
    dbFree       DBFree
    storageErr   error
    memoryErr    error
    networkErr   error
    dbErr        error
    storageCalls int
    memoryCalls  int
    networkCalls int
    dbCalls      int
}

func NewMockFree() *MockFree {
    return &MockFree{}
}

// SetStorageFree programs the result of StorageFree.
func (m *MockFree) SetStorageFree(free StorageFree) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.storageFree = free
}

// SetStorageError makes StorageFree return an error.
func (m *MockFree) SetStorageError(err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.storageErr = err
}

// SetMemoryFree programs the result of MemoryFree.
func (m *MockFree) SetMemoryFree(free MemoryFree) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.memoryFree = free
}

// SetMemoryError makes MemoryFree return an error.
func (m *MockFree) SetMemoryError(err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.memoryErr = err
}

// SetNetworkFree programs the result of NetworkFree.
func (m *MockFree) SetNetworkFree(free NetworkFree) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.networkFree = free
}

// SetNetworkError makes NetworkFree return an error.
func (m *MockFree) SetNetworkError(err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.networkErr = err
}

// SetDBFree programs the result of DBFree.
func (m *MockFree) SetDBFree(free DBFree) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.dbFree = free
}

// SetDBError makes DBFree return an error.
func (m *MockFree) SetDBError(err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.dbErr = err
}

func (m *MockFree) StorageFree() (StorageFree, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.storageCalls++
    return m.storageFree, m.storageErr
}

func (m *MockFree) MemoryFree() (MemoryFree, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.memoryCalls++
    return m.memoryFree, m.memoryErr
}

func (m *MockFree) NetworkFree() (NetworkFree, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.networkCalls++
    return m.networkFree, m.networkErr
}

func (m *MockFree) DBFree() (DBFree, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.dbCalls++
    return m.dbFree, m.dbErr
}

// CallCounts returns the number of calls for each method.
func (m *MockFree) CallCounts() (storage, memory, network, db int) {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.storageCalls, m.memoryCalls, m.networkCalls, m.dbCalls
}

// Reset clears all programmed values and call counts.
func (m *MockFree) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.storageFree = StorageFree{}
    m.memoryFree = MemoryFree{}
    m.networkFree = NetworkFree{}
    m.dbFree = DBFree{}
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
// InMemoryFree – a simple in-memory free space checker that can be updated.
// --------------------------------------------------------------------

type InMemoryFree struct {
    mu          sync.RWMutex
    storageFree StorageFree
    memoryFree  MemoryFree
    networkFree NetworkFree
    dbFree      DBFree
    storageErr  error
    memoryErr   error
    networkErr  error
    dbErr       error
}

func NewInMemoryFree() *InMemoryFree {
    return &InMemoryFree{}
}

// UpdateStorageFree allows tests to modify storage free values.
func (f *InMemoryFree) UpdateStorageFree(update func(*StorageFree)) {
    f.mu.Lock()
    defer f.mu.Unlock()
    update(&f.storageFree)
}

// UpdateMemoryFree allows tests to modify memory free values.
func (f *InMemoryFree) UpdateMemoryFree(update func(*MemoryFree)) {
    f.mu.Lock()
    defer f.mu.Unlock()
    update(&f.memoryFree)
}

// UpdateNetworkFree allows tests to modify network free values.
func (f *InMemoryFree) UpdateNetworkFree(update func(*NetworkFree)) {
    f.mu.Lock()
    defer f.mu.Unlock()
    update(&f.networkFree)
}

// UpdateDBFree allows tests to modify database free values.
func (f *InMemoryFree) UpdateDBFree(update func(*DBFree)) {
    f.mu.Lock()
    defer f.mu.Unlock()
    update(&f.dbFree)
}

// SetStorageError makes StorageFree return an error.
func (f *InMemoryFree) SetStorageError(err error) {
    f.mu.Lock()
    defer f.mu.Unlock()
    f.storageErr = err
}

// SetMemoryError makes MemoryFree return an error.
func (f *InMemoryFree) SetMemoryError(err error) {
    f.mu.Lock()
    defer f.mu.Unlock()
    f.memoryErr = err
}

// SetNetworkError makes NetworkFree return an error.
func (f *InMemoryFree) SetNetworkError(err error) {
    f.mu.Lock()
    defer f.mu.Unlock()
    f.networkErr = err
}

// SetDBError makes DBFree return an error.
func (f *InMemoryFree) SetDBError(err error) {
    f.mu.Lock()
    defer f.mu.Unlock()
    f.dbErr = err
}

func (f *InMemoryFree) StorageFree() (StorageFree, error) {
    f.mu.RLock()
    defer f.mu.RUnlock()
    if f.storageErr != nil {
        return StorageFree{}, f.storageErr
    }
    return f.storageFree, nil
}

func (f *InMemoryFree) MemoryFree() (MemoryFree, error) {
    f.mu.RLock()
    defer f.mu.RUnlock()
    if f.memoryErr != nil {
        return MemoryFree{}, f.memoryErr
    }
    return f.memoryFree, nil
}

func (f *InMemoryFree) NetworkFree() (NetworkFree, error) {
    f.mu.RLock()
    defer f.mu.RUnlock()
    if f.networkErr != nil {
        return NetworkFree{}, f.networkErr
    }
    return f.networkFree, nil
}

func (f *InMemoryFree) DBFree() (DBFree, error) {
    f.mu.RLock()
    defer f.mu.RUnlock()
    if f.dbErr != nil {
        return DBFree{}, f.dbErr
    }
    return f.dbFree, nil
}

// --------------------------------------------------------------------
// MultiFree – combines separate free providers into one Free interface.
// --------------------------------------------------------------------

type StorageFreeProvider func() (StorageFree, error)
type MemoryFreeProvider func() (MemoryFree, error)
type NetworkFreeProvider func() (NetworkFree, error)
type DBFreeProvider func() (DBFree, error)

type MultiFree struct {
    storageFn StorageFreeProvider
    memoryFn  MemoryFreeProvider
    networkFn NetworkFreeProvider
    dbFn      DBFreeProvider
}

func NewMultiFree(
    storage StorageFreeProvider,
    memory MemoryFreeProvider,
    network NetworkFreeProvider,
    db DBFreeProvider,
) *MultiFree {
    return &MultiFree{
        storageFn: storage,
        memoryFn:  memory,
        networkFn: network,
        dbFn:      db,
    }
}

func (m *MultiFree) StorageFree() (StorageFree, error) {
    if m.storageFn != nil {
        return m.storageFn()
    }
    return StorageFree{}, nil
}

func (m *MultiFree) MemoryFree() (MemoryFree, error) {
    if m.memoryFn != nil {
        return m.memoryFn()
    }
    return MemoryFree{}, nil
}

func (m *MultiFree) NetworkFree() (NetworkFree, error) {
    if m.networkFn != nil {
        return m.networkFn()
    }
    return NetworkFree{}, nil
}

func (m *MultiFree) DBFree() (DBFree, error) {
    if m.dbFn != nil {
        return m.dbFn()
    }
    return DBFree{}, nil
}