// Package testutils provides mock and simple in‑memory health checkers for storage,
// memory, network, and database components. It allows tests to simulate various
// health statuses and verify that the system reacts appropriately.
package testutils

import (
    "errors"
    "sync"
    "time"
)

// --------------------------------------------------------------------
// HealthStatus – represents the outcome of a single resource check.
// --------------------------------------------------------------------

// HealthStatus holds the result of checking a specific resource.
type HealthStatus struct {
    // OK indicates whether the resource is healthy.
    OK bool

    // Latency is the time the check took.
    Latency time.Duration

    // Message provides additional information (e.g., error details).
    Message string

    // Timestamp is when the check was performed.
    Timestamp time.Time
}

// String returns a simple string representation.
func (h HealthStatus) String() string {
    if h.OK {
        return "OK"
    }
    return "FAIL: " + h.Message
}

// --------------------------------------------------------------------
// Checker – interface for checking resource health.
// --------------------------------------------------------------------

// Checker defines methods for checking the health of various system resources.
type Checker interface {
    // StorageCheck returns the health status of storage.
    StorageCheck() (HealthStatus, error)

    // MemoryCheck returns the health status of memory.
    MemoryCheck() (HealthStatus, error)

    // NetworkCheck returns the health status of the network.
    NetworkCheck() (HealthStatus, error)

    // DBCheck returns the health status of the database.
    DBCheck() (HealthStatus, error)
}

// --------------------------------------------------------------------
// MockChecker – a test double that records calls and can be programmed.
// --------------------------------------------------------------------

// MockChecker implements Checker for unit tests.
type MockChecker struct {
    mu            sync.Mutex
    storageStatus HealthStatus
    memoryStatus  HealthStatus
    networkStatus HealthStatus
    dbStatus      HealthStatus
    storageErr    error
    memoryErr     error
    networkErr    error
    dbErr         error
    storageCalls  int
    memoryCalls   int
    networkCalls  int
    dbCalls       int
}

// NewMockChecker creates a new mock checker with all statuses OK by default.
func NewMockChecker() *MockChecker {
    now := time.Now()
    return &MockChecker{
        storageStatus: HealthStatus{OK: true, Timestamp: now},
        memoryStatus:  HealthStatus{OK: true, Timestamp: now},
        networkStatus: HealthStatus{OK: true, Timestamp: now},
        dbStatus:      HealthStatus{OK: true, Timestamp: now},
    }
}

// SetStorageStatus programs the status returned by StorageCheck.
func (m *MockChecker) SetStorageStatus(status HealthStatus) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.storageStatus = status
}

// SetStorageError makes StorageCheck return an error.
func (m *MockChecker) SetStorageError(err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.storageErr = err
}

// SetMemoryStatus programs the status returned by MemoryCheck.
func (m *MockChecker) SetMemoryStatus(status HealthStatus) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.memoryStatus = status
}

// SetMemoryError makes MemoryCheck return an error.
func (m *MockChecker) SetMemoryError(err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.memoryErr = err
}

// SetNetworkStatus programs the status returned by NetworkCheck.
func (m *MockChecker) SetNetworkStatus(status HealthStatus) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.networkStatus = status
}

// SetNetworkError makes NetworkCheck return an error.
func (m *MockChecker) SetNetworkError(err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.networkErr = err
}

// SetDBStatus programs the status returned by DBCheck.
func (m *MockChecker) SetDBStatus(status HealthStatus) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.dbStatus = status
}

// SetDBError makes DBCheck return an error.
func (m *MockChecker) SetDBError(err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.dbErr = err
}

func (m *MockChecker) StorageCheck() (HealthStatus, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.storageCalls++
    return m.storageStatus, m.storageErr
}

func (m *MockChecker) MemoryCheck() (HealthStatus, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.memoryCalls++
    return m.memoryStatus, m.memoryErr
}

func (m *MockChecker) NetworkCheck() (HealthStatus, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.networkCalls++
    return m.networkStatus, m.networkErr
}

func (m *MockChecker) DBCheck() (HealthStatus, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.dbCalls++
    return m.dbStatus, m.dbErr
}

// CallCounts returns the number of calls for each method.
func (m *MockChecker) CallCounts() (storage, memory, network, db int) {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.storageCalls, m.memoryCalls, m.networkCalls, m.dbCalls
}

// Reset clears all programmed values and call counts.
func (m *MockChecker) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.storageStatus = HealthStatus{OK: true, Timestamp: time.Now()}
    m.memoryStatus = HealthStatus{OK: true, Timestamp: time.Now()}
    m.networkStatus = HealthStatus{OK: true, Timestamp: time.Now()}
    m.dbStatus = HealthStatus{OK: true, Timestamp: time.Now()}
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
// InMemoryChecker – a simple in‑memory checker that can be updated.
// --------------------------------------------------------------------

// InMemoryChecker implements Checker with mutable statuses.
type InMemoryChecker struct {
    mu            sync.RWMutex
    storageStatus HealthStatus
    memoryStatus  HealthStatus
    networkStatus HealthStatus
    dbStatus      HealthStatus
    storageErr    error
    memoryErr     error
    networkErr    error
    dbErr         error
}

// NewInMemoryChecker creates a new checker with all statuses OK.
func NewInMemoryChecker() *InMemoryChecker {
    now := time.Now()
    return &InMemoryChecker{
        storageStatus: HealthStatus{OK: true, Timestamp: now},
        memoryStatus:  HealthStatus{OK: true, Timestamp: now},
        networkStatus: HealthStatus{OK: true, Timestamp: now},
        dbStatus:      HealthStatus{OK: true, Timestamp: now},
    }
}

// UpdateStorage allows tests to modify the storage status.
func (c *InMemoryChecker) UpdateStorage(update func(*HealthStatus)) {
    c.mu.Lock()
    defer c.mu.Unlock()
    update(&c.storageStatus)
    c.storageStatus.Timestamp = time.Now()
}

// UpdateMemory allows tests to modify the memory status.
func (c *InMemoryChecker) UpdateMemory(update func(*HealthStatus)) {
    c.mu.Lock()
    defer c.mu.Unlock()
    update(&c.memoryStatus)
    c.memoryStatus.Timestamp = time.Now()
}

// UpdateNetwork allows tests to modify the network status.
func (c *InMemoryChecker) UpdateNetwork(update func(*HealthStatus)) {
    c.mu.Lock()
    defer c.mu.Unlock()
    update(&c.networkStatus)
    c.networkStatus.Timestamp = time.Now()
}

// UpdateDB allows tests to modify the database status.
func (c *InMemoryChecker) UpdateDB(update func(*HealthStatus)) {
    c.mu.Lock()
    defer c.mu.Unlock()
    update(&c.dbStatus)
    c.dbStatus.Timestamp = time.Now()
}

// SetStorageError makes StorageCheck return an error.
func (c *InMemoryChecker) SetStorageError(err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.storageErr = err
}

// SetMemoryError makes MemoryCheck return an error.
func (c *InMemoryChecker) SetMemoryError(err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.memoryErr = err
}

// SetNetworkError makes NetworkCheck return an error.
func (c *InMemoryChecker) SetNetworkError(err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.networkErr = err
}

// SetDBError makes DBCheck return an error.
func (c *InMemoryChecker) SetDBError(err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.dbErr = err
}

func (c *InMemoryChecker) StorageCheck() (HealthStatus, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if c.storageErr != nil {
        return HealthStatus{}, c.storageErr
    }
    return c.storageStatus, nil
}

func (c *InMemoryChecker) MemoryCheck() (HealthStatus, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if c.memoryErr != nil {
        return HealthStatus{}, c.memoryErr
    }
    return c.memoryStatus, nil
}

func (c *InMemoryChecker) NetworkCheck() (HealthStatus, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if c.networkErr != nil {
        return HealthStatus{}, c.networkErr
    }
    return c.networkStatus, nil
}

func (c *InMemoryChecker) DBCheck() (HealthStatus, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if c.dbErr != nil {
        return HealthStatus{}, c.dbErr
    }
    return c.dbStatus, nil
}

// --------------------------------------------------------------------
// MultiChecker – combines separate check providers into one Checker.
// --------------------------------------------------------------------

type StorageCheckFunc func() (HealthStatus, error)
type MemoryCheckFunc func() (HealthStatus, error)
type NetworkCheckFunc func() (HealthStatus, error)
type DBCheckFunc func() (HealthStatus, error)

// MultiChecker implements Checker by calling the provided functions.
type MultiChecker struct {
    storageFn StorageCheckFunc
    memoryFn  MemoryCheckFunc
    networkFn NetworkCheckFunc
    dbFn      DBCheckFunc
}

// NewMultiChecker creates a checker from optional functions.
func NewMultiChecker(
    storage StorageCheckFunc,
    memory MemoryCheckFunc,
    network NetworkCheckFunc,
    db DBCheckFunc,
) *MultiChecker {
    return &MultiChecker{
        storageFn: storage,
        memoryFn:  memory,
        networkFn: network,
        dbFn:      db,
    }
}

func (m *MultiChecker) StorageCheck() (HealthStatus, error) {
    if m.storageFn != nil {
        return m.storageFn()
    }
    return HealthStatus{OK: true, Timestamp: time.Now()}, nil
}

func (m *MultiChecker) MemoryCheck() (HealthStatus, error) {
    if m.memoryFn != nil {
        return m.memoryFn()
    }
    return HealthStatus{OK: true, Timestamp: time.Now()}, nil
}

func (m *MultiChecker) NetworkCheck() (HealthStatus, error) {
    if m.networkFn != nil {
        return m.networkFn()
    }
    return HealthStatus{OK: true, Timestamp: time.Now()}, nil
}

func (m *MultiChecker) DBCheck() (HealthStatus, error) {
    if m.dbFn != nil {
        return m.dbFn()
    }
    return HealthStatus{OK: true, Timestamp: time.Now()}, nil
}

// --------------------------------------------------------------------
// CheckerConditioner – wraps a Checker to add delays and per‑call errors.
// --------------------------------------------------------------------

// CheckerConditioner adds configurable delays and error injection to a Checker.
type CheckerConditioner struct {
    mu            sync.Mutex
    checker       Checker
    storageDelay  time.Duration
    memoryDelay   time.Duration
    networkDelay  time.Duration
    dbDelay       time.Duration
    storageErrors map[int]error // call number -> error
    memoryErrors  map[int]error
    networkErrors map[int]error
    dbErrors      map[int]error
    storageCalls  int
    memoryCalls   int
    networkCalls  int
    dbCalls       int
}

// NewCheckerConditioner creates a conditioner around an existing Checker.
func NewCheckerConditioner(checker Checker) *CheckerConditioner {
    return &CheckerConditioner{
        checker:       checker,
        storageErrors: make(map[int]error),
        memoryErrors:  make(map[int]error),
        networkErrors: make(map[int]error),
        dbErrors:      make(map[int]error),
    }
}

// SetStorageDelay adds a fixed delay before StorageCheck.
func (c *CheckerConditioner) SetStorageDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.storageDelay = d
}

// SetMemoryDelay adds a fixed delay before MemoryCheck.
func (c *CheckerConditioner) SetMemoryDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.memoryDelay = d
}

// SetNetworkDelay adds a fixed delay before NetworkCheck.
func (c *CheckerConditioner) SetNetworkDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.networkDelay = d
}

// SetDBDelay adds a fixed delay before DBCheck.
func (c *CheckerConditioner) SetDBDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.dbDelay = d
}

// InjectStorageError makes the nth call to StorageCheck return the given error.
func (c *CheckerConditioner) InjectStorageError(callNumber int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.storageErrors[callNumber] = err
}

// InjectMemoryError makes the nth call to MemoryCheck return the given error.
func (c *CheckerConditioner) InjectMemoryError(callNumber int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.memoryErrors[callNumber] = err
}

// InjectNetworkError makes the nth call to NetworkCheck return the given error.
func (c *CheckerConditioner) InjectNetworkError(callNumber int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.networkErrors[callNumber] = err
}

// InjectDBError makes the nth call to DBCheck return the given error.
func (c *CheckerConditioner) InjectDBError(callNumber int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.dbErrors[callNumber] = err
}

func (c *CheckerConditioner) StorageCheck() (HealthStatus, error) {
    c.mu.Lock()
    c.storageCalls++
    call := c.storageCalls
    delay := c.storageDelay
    err, ok := c.storageErrors[call]
    if ok {
        delete(c.storageErrors, call)
    }
    c.mu.Unlock()

    if ok {
        return HealthStatus{}, err
    }
    if delay > 0 {
        time.Sleep(delay)
    }
    return c.checker.StorageCheck()
}

func (c *CheckerConditioner) MemoryCheck() (HealthStatus, error) {
    c.mu.Lock()
    c.memoryCalls++
    call := c.memoryCalls
    delay := c.memoryDelay
    err, ok := c.memoryErrors[call]
    if ok {
        delete(c.memoryErrors, call)
    }
    c.mu.Unlock()

    if ok {
        return HealthStatus{}, err
    }
    if delay > 0 {
        time.Sleep(delay)
    }
    return c.checker.MemoryCheck()
}

func (c *CheckerConditioner) NetworkCheck() (HealthStatus, error) {
    c.mu.Lock()
    c.networkCalls++
    call := c.networkCalls
    delay := c.networkDelay
    err, ok := c.networkErrors[call]
    if ok {
        delete(c.networkErrors, call)
    }
    c.mu.Unlock()

    if ok {
        return HealthStatus{}, err
    }
    if delay > 0 {
        time.Sleep(delay)
    }
    return c.checker.NetworkCheck()
}

func (c *CheckerConditioner) DBCheck() (HealthStatus, error) {
    c.mu.Lock()
    c.dbCalls++
    call := c.dbCalls
    delay := c.dbDelay
    err, ok := c.dbErrors[call]
    if ok {
        delete(c.dbErrors, call)
    }
    c.mu.Unlock()

    if ok {
        return HealthStatus{}, err
    }
    if delay > 0 {
        time.Sleep(delay)
    }
    return c.checker.DBCheck()
}