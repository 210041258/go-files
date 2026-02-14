// Package testutils provides mock and simple in‑memory statistics providers
// for testing storage, memory, network, and database components. It allows
// tests to simulate various metric values and verify that the system
// reacts appropriately.
package testutils

import (
    "sync"
    "time"
)

// --------------------------------------------------------------------
// Stats – aggregated statistics for all components.
// --------------------------------------------------------------------

// Stats holds a snapshot of metrics from all system resources.
type Stats struct {
    Storage   StorageStats `json:"storage"`
    Memory    MemoryStats  `json:"memory"`
    Network   NetworkStats `json:"network"`
    DB        DBStats      `json:"db"`
    Timestamp time.Time    `json:"timestamp"`
}

// StorageStats represents disk usage and I/O metrics.
type StorageStats struct {
    TotalBytes  int64 `json:"total_bytes"`
    UsedBytes   int64 `json:"used_bytes"`
    FreeBytes   int64 `json:"free_bytes"`
    ReadOps     int64 `json:"read_ops"`
    WriteOps    int64 `json:"write_ops"`
    ReadErrors  int64 `json:"read_errors"`
    WriteErrors int64 `json:"write_errors"`
}

// MemoryStats represents RAM and swap metrics.
type MemoryStats struct {
    TotalBytes     int64 `json:"total_bytes"`
    UsedBytes      int64 `json:"used_bytes"`
    FreeBytes      int64 `json:"free_bytes"`
    CachedBytes    int64 `json:"cached_bytes"`
    SwapTotalBytes int64 `json:"swap_total_bytes"`
    SwapUsedBytes  int64 `json:"swap_used_bytes"`
}

// NetworkStats represents network interface metrics.
type NetworkStats struct {
    BytesSent       int64 `json:"bytes_sent"`
    BytesReceived   int64 `json:"bytes_received"`
    PacketsSent     int64 `json:"packets_sent"`
    PacketsReceived int64 `json:"packets_received"`
    ErrorsSent      int64 `json:"errors_sent"`
    ErrorsReceived  int64 `json:"errors_received"`
    DroppedSent     int64 `json:"dropped_sent"`
    DroppedReceived int64 `json:"dropped_received"`
}

// DBStats represents database connection pool and query metrics.
type DBStats struct {
    OpenConnections   int           `json:"open_connections"`
    InUseConnections  int           `json:"in_use_connections"`
    IdleConnections   int           `json:"idle_connections"`
    WaitCount         int64         `json:"wait_count"`
    WaitDuration      time.Duration `json:"wait_duration"`
    MaxIdleClosed     int64         `json:"max_idle_closed"`
    MaxLifetimeClosed int64         `json:"max_lifetime_closed"`
    QueryCount        int64         `json:"query_count"`
    QueryErrors       int64         `json:"query_errors"`
}

// --------------------------------------------------------------------
// StatsProvider – interface for obtaining statistics.
// --------------------------------------------------------------------

// StatsProvider defines a method to retrieve current statistics.
type StatsProvider interface {
    // Stats returns a snapshot of the current statistics.
    Stats() (Stats, error)
}

// --------------------------------------------------------------------
// MockStatsProvider – a test double that records calls and can be programmed.
// --------------------------------------------------------------------

// MockStatsProvider implements StatsProvider for unit tests.
type MockStatsProvider struct {
    mu        sync.Mutex
    stats     Stats
    err       error
    callCount int
    callFunc  func() (Stats, error) // optional custom behavior
}

// NewMockStatsProvider creates a new mock provider with zeroed stats.
func NewMockStatsProvider() *MockStatsProvider {
    return &MockStatsProvider{}
}

// SetStats programs the stats returned by Stats.
func (m *MockStatsProvider) SetStats(stats Stats) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.stats = stats
}

// SetError programs the error returned by Stats.
func (m *MockStatsProvider) SetError(err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.err = err
}

// SetCallFunc overrides the Stats method with custom behavior.
func (m *MockStatsProvider) SetCallFunc(fn func() (Stats, error)) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.callFunc = fn
}

// Stats records the call and returns programmed stats/error or calls custom function.
func (m *MockStatsProvider) Stats() (Stats, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.callCount++
    if m.callFunc != nil {
        return m.callFunc()
    }
    return m.stats, m.err
}

// CallCount returns the number of times Stats was called.
func (m *MockStatsProvider) CallCount() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.callCount
}

// Reset clears programmed values and call count.
func (m *MockStatsProvider) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.stats = Stats{}
    m.err = nil
    m.callCount = 0
    m.callFunc = nil
}

// --------------------------------------------------------------------
// InMemoryStatsProvider – a simple mutable stats provider.
// --------------------------------------------------------------------

// InMemoryStatsProvider implements StatsProvider with a mutable stats struct.
type InMemoryStatsProvider struct {
    mu    sync.RWMutex
    stats Stats
    err   error
}

// NewInMemoryStatsProvider creates a new provider with zeroed stats.
func NewInMemoryStatsProvider() *InMemoryStatsProvider {
    return &InMemoryStatsProvider{}
}

// Update allows tests to modify the stats via a callback.
func (p *InMemoryStatsProvider) Update(update func(*Stats)) {
    p.mu.Lock()
    defer p.mu.Unlock()
    update(&p.stats)
    p.stats.Timestamp = time.Now()
}

// SetError makes Stats return an error.
func (p *InMemoryStatsProvider) SetError(err error) {
    p.mu.Lock()
    defer p.mu.Unlock()
    p.err = err
}

// Stats returns the current stats or error.
func (p *InMemoryStatsProvider) Stats() (Stats, error) {
    p.mu.RLock()
    defer p.mu.RUnlock()
    if p.err != nil {
        return Stats{}, p.err
    }
    return p.stats, nil
}

// --------------------------------------------------------------------
// MultiStatsProvider – aggregates multiple named stats providers.
// --------------------------------------------------------------------

// MultiStatsProvider collects stats from several providers and returns
// a map of component names to their stats, along with a combined Stats
// where fields are summed (or combined as appropriate).
type MultiStatsProvider struct {
    mu        sync.RWMutex
    providers map[string]StatsProvider
}

// NewMultiStatsProvider creates a new empty multi‑provider.
func NewMultiStatsProvider() *MultiStatsProvider {
    return &MultiStatsProvider{
        providers: make(map[string]StatsProvider),
    }
}

// AddProvider adds or replaces a provider with the given name.
func (m *MultiStatsProvider) AddProvider(name string, provider StatsProvider) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.providers[name] = provider
}

// RemoveProvider removes a provider by name.
func (m *MultiStatsProvider) RemoveProvider(name string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    delete(m.providers, name)
}

// Stats calls each provider and returns a combined Stats where each metric
// is the sum of the individual providers' metrics. If any provider returns
// an error, the combined Stats is zeroed and the first error is returned.
// Additionally, it returns a map of individual component stats.
func (m *MultiStatsProvider) Stats() (Stats, map[string]Stats, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    combined := Stats{Timestamp: time.Now()}
    components := make(map[string]Stats)

    for name, provider := range m.providers {
        stats, err := provider.Stats()
        if err != nil {
            return Stats{}, nil, err
        }
        components[name] = stats

        // Sum numeric fields (simplistic aggregation)
        combined.Storage.TotalBytes += stats.Storage.TotalBytes
        combined.Storage.UsedBytes += stats.Storage.UsedBytes
        combined.Storage.FreeBytes += stats.Storage.FreeBytes
        combined.Storage.ReadOps += stats.Storage.ReadOps
        combined.Storage.WriteOps += stats.Storage.WriteOps
        combined.Storage.ReadErrors += stats.Storage.ReadErrors
        combined.Storage.WriteErrors += stats.Storage.WriteErrors

        combined.Memory.TotalBytes += stats.Memory.TotalBytes
        combined.Memory.UsedBytes += stats.Memory.UsedBytes
        combined.Memory.FreeBytes += stats.Memory.FreeBytes
        combined.Memory.CachedBytes += stats.Memory.CachedBytes
        combined.Memory.SwapTotalBytes += stats.Memory.SwapTotalBytes
        combined.Memory.SwapUsedBytes += stats.Memory.SwapUsedBytes

        combined.Network.BytesSent += stats.Network.BytesSent
        combined.Network.BytesReceived += stats.Network.BytesReceived
        combined.Network.PacketsSent += stats.Network.PacketsSent
        combined.Network.PacketsReceived += stats.Network.PacketsReceived
        combined.Network.ErrorsSent += stats.Network.ErrorsSent
        combined.Network.ErrorsReceived += stats.Network.ErrorsReceived
        combined.Network.DroppedSent += stats.Network.DroppedSent
        combined.Network.DroppedReceived += stats.Network.DroppedReceived

        combined.DB.OpenConnections += stats.DB.OpenConnections
        combined.DB.InUseConnections += stats.DB.InUseConnections
        combined.DB.IdleConnections += stats.DB.IdleConnections
        combined.DB.WaitCount += stats.DB.WaitCount
        combined.DB.WaitDuration += stats.DB.WaitDuration
        combined.DB.MaxIdleClosed += stats.DB.MaxIdleClosed
        combined.DB.MaxLifetimeClosed += stats.DB.MaxLifetimeClosed
        combined.DB.QueryCount += stats.DB.QueryCount
        combined.DB.QueryErrors += stats.DB.QueryErrors
    }

    return combined, components, nil
}

// --------------------------------------------------------------------
// StatsConditioner – wraps a StatsProvider to add delays and per‑call errors.
// --------------------------------------------------------------------

// StatsConditioner adds configurable delays and error injection to a StatsProvider.
type StatsConditioner struct {
    mu        sync.Mutex
    provider  StatsProvider
    delay     time.Duration
    errors    map[int]error
    callCount int
}

// NewStatsConditioner creates a conditioner around an existing StatsProvider.
func NewStatsConditioner(provider StatsProvider) *StatsConditioner {
    return &StatsConditioner{
        provider: provider,
        errors:   make(map[int]error),
    }
}

// SetDelay adds a fixed delay before every Stats call.
func (c *StatsConditioner) SetDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.delay = d
}

// InjectError makes the nth call to Stats return the given error.
func (c *StatsConditioner) InjectError(callNumber int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.errors[callNumber] = err
}

// Stats implements StatsProvider with delay and error injection.
func (c *StatsConditioner) Stats() (Stats, error) {
    c.mu.Lock()
    c.callCount++
    call := c.callCount
    delay := c.delay
    err, ok := c.errors[call]
    if ok {
        delete(c.errors, call)
        c.mu.Unlock()
        return Stats{}, err
    }
    c.mu.Unlock()

    if delay > 0 {
        time.Sleep(delay)
    }
    return c.provider.Stats()
}

// --------------------------------------------------------------------
// StatsAssertions – helper functions for testing with StatsProvider.
// --------------------------------------------------------------------

type testingT interface {
    Error(args ...interface{})
    Errorf(format string, args ...interface{})
}

// StatsAssertions provides convenience methods for verifying statistics.
type StatsAssertions struct {
    t testingT
}

// NewStatsAssertions creates a new assertion helper.
func NewStatsAssertions(t testingT) *StatsAssertions {
    return &StatsAssertions{t: t}
}

// AssertStorageBytes asserts that the storage usage metrics match expected values.
func (a *StatsAssertions) AssertStorageBytes(provider StatsProvider, total, used, free int64) {
    stats, err := provider.Stats()
    if err != nil {
        a.t.Errorf("unexpected error: %v", err)
        return
    }
    if stats.Storage.TotalBytes != total {
        a.t.Errorf("expected storage total bytes %d, got %d", total, stats.Storage.TotalBytes)
    }
    if stats.Storage.UsedBytes != used {
        a.t.Errorf("expected storage used bytes %d, got %d", used, stats.Storage.UsedBytes)
    }
    if stats.Storage.FreeBytes != free {
        a.t.Errorf("expected storage free bytes %d, got %d", free, stats.Storage.FreeBytes)
    }
}

// AssertMemoryBytes asserts that the memory usage metrics match expected values.
func (a *StatsAssertions) AssertMemoryBytes(provider StatsProvider, total, used, free int64) {
    stats, err := provider.Stats()
    if err != nil {
        a.t.Errorf("unexpected error: %v", err)
        return
    }
    if stats.Memory.TotalBytes != total {
        a.t.Errorf("expected memory total bytes %d, got %d", total, stats.Memory.TotalBytes)
    }
    if stats.Memory.UsedBytes != used {
        a.t.Errorf("expected memory used bytes %d, got %d", used, stats.Memory.UsedBytes)
    }
    if stats.Memory.FreeBytes != free {
        a.t.Errorf("expected memory free bytes %d, got %d", free, stats.Memory.FreeBytes)
    }
}

// AssertNetworkTraffic asserts that the network traffic metrics match expected values.
func (a *StatsAssertions) AssertNetworkTraffic(provider StatsProvider, sent, received int64) {
    stats, err := provider.Stats()
    if err != nil {
        a.t.Errorf("unexpected error: %v", err)
        return
    }
    if stats.Network.BytesSent != sent {
        a.t.Errorf("expected network bytes sent %d, got %d", sent, stats.Network.BytesSent)
    }
    if stats.Network.BytesReceived != received {
        a.t.Errorf("expected network bytes received %d, got %d", received, stats.Network.BytesReceived)
    }
}

// AssertDBConnections asserts that the database connection metrics match expected values.
func (a *StatsAssertions) AssertDBConnections(provider StatsProvider, open, inUse, idle int) {
    stats, err := provider.Stats()
    if err != nil {
        a.t.Errorf("unexpected error: %v", err)
        return
    }
    if stats.DB.OpenConnections != open {
        a.t.Errorf("expected DB open connections %d, got %d", open, stats.DB.OpenConnections)
    }
    if stats.DB.InUseConnections != inUse {
        a.t.Errorf("expected DB in‑use connections %d, got %d", inUse, stats.DB.InUseConnections)
    }
    if stats.DB.IdleConnections != idle {
        a.t.Errorf("expected DB idle connections %d, got %d", idle, stats.DB.IdleConnections)
    }
}