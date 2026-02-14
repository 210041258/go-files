// Package testutils provides mock and in‑memory mode managers and mode‑aware wrappers
// for storage, memory, network, and database components. It allows tests to simulate
// different operational modes (normal, degraded, offline, read‑only) and verify that
// the system behaves correctly.
package testutils

import (
    "errors"
    "sync"
    "time"
)

// --------------------------------------------------------------------
// Mode – defines the operational mode of a resource or the whole gateway.
// --------------------------------------------------------------------

type Mode string

const (
    // ModeNormal – full read/write access, normal latency, no errors.
    ModeNormal Mode = "normal"

    // ModeDegraded – resource is slow or partially available.
    ModeDegraded Mode = "degraded"

    // ModeReadOnly – only read operations are allowed; writes fail.
    ModeReadOnly Mode = "readonly"

    // ModeOffline – resource is completely unavailable; all operations fail.
    ModeOffline Mode = "offline"

    // ModeFlaky – operations fail intermittently (useful for network/db).
    ModeFlaky Mode = "flaky"

    // ModeMaintenance – similar to offline but with a specific message.
    ModeMaintenance Mode = "maintenance"
)

// --------------------------------------------------------------------
// ModeManager – interface for setting and getting the current mode.
// --------------------------------------------------------------------

type ModeManager interface {
    // CurrentMode returns the current mode.
    CurrentMode() Mode

    // SetMode changes the mode. Implementations may notify watchers.
    SetMode(mode Mode)

    // Watch returns a channel that receives the new mode whenever it changes.
    // The channel should be closed when the manager is closed.
    Watch() <-chan Mode

    // Close stops any watcher goroutines.
    Close() error
}

// --------------------------------------------------------------------
// MockModeManager – a test double that records calls and can be programmed.
// --------------------------------------------------------------------

type MockModeManager struct {
    mu          sync.Mutex
    currentMode Mode
    watchers    []chan Mode
    setCalls    []Mode
    watchCalls  int
    closeCalls  int
    closeErr    error
}

func NewMockModeManager(initial Mode) *MockModeManager {
    return &MockModeManager{
        currentMode: initial,
    }
}

func (m *MockModeManager) CurrentMode() Mode {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.currentMode
}

func (m *MockModeManager) SetMode(mode Mode) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.setCalls = append(m.setCalls, mode)
    m.currentMode = mode
    for _, ch := range m.watchers {
        select {
        case ch <- mode:
        default:
            // Non‑blocking; if channel is full, skip (tests should ensure buffer size)
        }
    }
}

func (m *MockModeManager) Watch() <-chan Mode {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.watchCalls++
    ch := make(chan Mode, 10) // buffered to avoid blocking
    m.watchers = append(m.watchers, ch)
    // Send current mode immediately
    ch <- m.currentMode
    return ch
}

func (m *MockModeManager) Close() error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.closeCalls++
    for _, ch := range m.watchers {
        close(ch)
    }
    m.watchers = nil
    return m.closeErr
}

// SetCloseError programs the error returned by Close.
func (m *MockModeManager) SetCloseError(err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.closeErr = err
}

// SetCalls returns a copy of the modes passed to SetMode.
func (m *MockModeManager) SetCalls() []Mode {
    m.mu.Lock()
    defer m.mu.Unlock()
    cp := make([]Mode, len(m.setCalls))
    copy(cp, m.setCalls)
    return cp
}

// WatchCalls returns the number of times Watch was called.
func (m *MockModeManager) WatchCalls() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.watchCalls
}

// CloseCalls returns the number of times Close was called.
func (m *MockModeManager) CloseCalls() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.closeCalls
}

// Reset clears recorded calls and watchers.
func (m *MockModeManager) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.setCalls = nil
    m.watchCalls = 0
    m.closeCalls = 0
    m.closeErr = nil
    for _, ch := range m.watchers {
        close(ch)
    }
    m.watchers = nil
}

// --------------------------------------------------------------------
// InMemoryModeManager – a simple manager that holds the mode and notifies watchers.
// --------------------------------------------------------------------

type InMemoryModeManager struct {
    mu       sync.Mutex
    mode     Mode
    watchers []chan Mode
    closed   bool
}

func NewInMemoryModeManager(initial Mode) *InMemoryModeManager {
    return &InMemoryModeManager{
        mode: initial,
    }
}

func (m *InMemoryModeManager) CurrentMode() Mode {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.mode
}

func (m *InMemoryModeManager) SetMode(mode Mode) {
    m.mu.Lock()
    defer m.mu.Unlock()
    if m.closed {
        return
    }
    m.mode = mode
    for _, ch := range m.watchers {
        select {
        case ch <- mode:
        default:
            // Non‑blocking; tests should ensure adequate buffer size
        }
    }
}

func (m *InMemoryModeManager) Watch() <-chan Mode {
    m.mu.Lock()
    defer m.mu.Unlock()
    if m.closed {
        ch := make(chan Mode)
        close(ch)
        return ch
    }
    ch := make(chan Mode, 10)
    ch <- m.mode
    m.watchers = append(m.watchers, ch)
    return ch
}

func (m *InMemoryModeManager) Close() error {
    m.mu.Lock()
    defer m.mu.Unlock()
    if m.closed {
        return nil
    }
    m.closed = true
    for _, ch := range m.watchers {
        close(ch)
    }
    m.watchers = nil
    return nil
}

// --------------------------------------------------------------------
// ModeAwareDisk – wraps a Disk and enforces mode semantics.
// --------------------------------------------------------------------

// ModeAwareDisk implements the Disk interface, but checks the current mode
// before delegating to the underlying disk. Depending on the mode, it may
// return errors (e.g., writes in read‑only mode) or add delays.
type ModeAwareDisk struct {
    disk  Disk
    mgr   ModeManager
    mu    sync.Mutex
    // For ModeFlaky: control failure probability (0.0–1.0)
    flakyRate float64
}

func NewModeAwareDisk(disk Disk, mgr ModeManager) *ModeAwareDisk {
    return &ModeAwareDisk{
        disk: disk,
        mgr:  mgr,
    }
}

// SetFlakyRate sets the probability (0.0–1.0) that an operation fails in ModeFlaky.
func (d *ModeAwareDisk) SetFlakyRate(rate float64) {
    d.mu.Lock()
    defer d.mu.Unlock()
    d.flakyRate = rate
}

func (d *ModeAwareDisk) checkMode(write bool) error {
    mode := d.mgr.CurrentMode()
    switch mode {
    case ModeNormal:
        return nil
    case ModeDegraded:
        // Add a small delay to simulate slowness
        time.Sleep(50 * time.Millisecond)
        return nil
    case ModeReadOnly:
        if write {
            return errors.New("mode aware disk: write denied in read‑only mode")
        }
        return nil
    case ModeOffline, ModeMaintenance:
        return errors.New("mode aware disk: resource unavailable (" + string(mode) + ")")
    case ModeFlaky:
        d.mu.Lock()
        rate := d.flakyRate
        d.mu.Unlock()
        if rate > 0 && randFloat() < rate {
            return errors.New("mode aware disk: flaky error")
        }
        return nil
    default:
        return nil
    }
}

func (d *ModeAwareDisk) Open(name string) (File, error) {
    if err := d.checkMode(false); err != nil {
        return nil, err
    }
    f, err := d.disk.Open(name)
    if err != nil {
        return nil, err
    }
    // Wrap the file to also check mode on read/write
    return &modeAwareFile{file: f, disk: d}, nil
}

func (d *ModeAwareDisk) Create(name string) (File, error) {
    if err := d.checkMode(true); err != nil {
        return nil, err
    }
    f, err := d.disk.Create(name)
    if err != nil {
        return nil, err
    }
    return &modeAwareFile{file: f, disk: d}, nil
}

func (d *ModeAwareDisk) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
    write := (flag&os.O_WRONLY != 0) || (flag&os.O_RDWR != 0)
    if err := d.checkMode(write); err != nil {
        return nil, err
    }
    f, err := d.disk.OpenFile(name, flag, perm)
    if err != nil {
        return nil, err
    }
    return &modeAwareFile{file: f, disk: d}, nil
}

func (d *ModeAwareDisk) Remove(name string) error {
    if err := d.checkMode(true); err != nil {
        return err
    }
    return d.disk.Remove(name)
}

func (d *ModeAwareDisk) Rename(oldpath, newpath string) error {
    if err := d.checkMode(true); err != nil {
        return err
    }
    return d.disk.Rename(oldpath, newpath)
}

func (d *ModeAwareDisk) Stat(name string) (FileInfo, error) {
    if err := d.checkMode(false); err != nil {
        return FileInfo{}, err
    }
    return d.disk.Stat(name)
}

func (d *ModeAwareDisk) Mkdir(name string, perm os.FileMode) error {
    if err := d.checkMode(true); err != nil {
        return err
    }
    return d.disk.Mkdir(name, perm)
}

func (d *ModeAwareDisk) MkdirAll(name string, perm os.FileMode) error {
    if err := d.checkMode(true); err != nil {
        return err
    }
    return d.disk.MkdirAll(name, perm)
}

// modeAwareFile wraps a File to check mode on read/write.
type modeAwareFile struct {
    file File
    disk *ModeAwareDisk
}

func (f *modeAwareFile) Read(p []byte) (int, error) {
    if err := f.disk.checkMode(false); err != nil {
        return 0, err
    }
    return f.file.Read(p)
}

func (f *modeAwareFile) Write(p []byte) (int, error) {
    if err := f.disk.checkMode(true); err != nil {
        return 0, err
    }
    return f.file.Write(p)
}

func (f *modeAwareFile) Seek(offset int64, whence int) (int64, error) {
    // Seek is generally a read operation (doesn't change content)
    if err := f.disk.checkMode(false); err != nil {
        return 0, err
    }
    return f.file.Seek(offset, whence)
}

func (f *modeAwareFile) Close() error {
    return f.file.Close()
}

func (f *modeAwareFile) Sync() error {
    // Sync is a write operation (flushes to disk)
    if err := f.disk.checkMode(true); err != nil {
        return err
    }
    return f.file.Sync()
}

func (f *modeAwareFile) Stat() (FileInfo, error) {
    if err := f.disk.checkMode(false); err != nil {
        return FileInfo{}, err
    }
    return f.file.Stat()
}

func (f *modeAwareFile) Name() string {
    return f.file.Name()
}

// --------------------------------------------------------------------
// ModeAwareCollector – wraps a Collector and enforces mode semantics.
// --------------------------------------------------------------------

type ModeAwareCollector struct {
    coll Collector
    mgr  ModeManager
    mu   sync.Mutex
    flakyRate float64
}

func NewModeAwareCollector(coll Collector, mgr ModeManager) *ModeAwareCollector {
    return &ModeAwareCollector{
        coll: coll,
        mgr:  mgr,
    }
}

func (c *ModeAwareCollector) SetFlakyRate(rate float64) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.flakyRate = rate
}

func (c *ModeAwareCollector) checkMode() error {
    mode := c.mgr.CurrentMode()
    switch mode {
    case ModeNormal:
        return nil
    case ModeDegraded:
        time.Sleep(50 * time.Millisecond)
        return nil
    case ModeReadOnly, ModeOffline, ModeMaintenance:
        return errors.New("mode aware collector: collection unavailable (" + string(mode) + ")")
    case ModeFlaky:
        c.mu.Lock()
        rate := c.flakyRate
        c.mu.Unlock()
        if rate > 0 && randFloat() < rate {
            return errors.New("mode aware collector: flaky error")
        }
        return nil
    default:
        return nil
    }
}

func (c *ModeAwareCollector) CollectStorage() (StorageStats, error) {
    if err := c.checkMode(); err != nil {
        return StorageStats{}, err
    }
    return c.coll.CollectStorage()
}

func (c *ModeAwareCollector) CollectMemory() (MemoryStats, error) {
    if err := c.checkMode(); err != nil {
        return MemoryStats{}, err
    }
    return c.coll.CollectMemory()
}

func (c *ModeAwareCollector) CollectNetwork() (NetworkStats, error) {
    if err := c.checkMode(); err != nil {
        return NetworkStats{}, err
    }
    return c.coll.CollectNetwork()
}

func (c *ModeAwareCollector) CollectDB() (DBStats, error) {
    if err := c.checkMode(); err != nil {
        return DBStats{}, err
    }
    return c.coll.CollectDB()
}

// --------------------------------------------------------------------
// ModeAwareFree – wraps a Free checker and enforces mode semantics.
// --------------------------------------------------------------------

type ModeAwareFree struct {
    free Free
    mgr  ModeManager
    mu   sync.Mutex
    flakyRate float64
}

func NewModeAwareFree(free Free, mgr ModeManager) *ModeAwareFree {
    return &ModeAwareFree{
        free: free,
        mgr:  mgr,
    }
}

func (f *ModeAwareFree) SetFlakyRate(rate float64) {
    f.mu.Lock()
    defer f.mu.Unlock()
    f.flakyRate = rate
}

func (f *ModeAwareFree) checkMode() error {
    mode := f.mgr.CurrentMode()
    switch mode {
    case ModeNormal:
        return nil
    case ModeDegraded:
        time.Sleep(50 * time.Millisecond)
        return nil
    case ModeReadOnly, ModeOffline, ModeMaintenance:
        return errors.New("mode aware free: resource unavailable (" + string(mode) + ")")
    case ModeFlaky:
        f.mu.Lock()
        rate := f.flakyRate
        f.mu.Unlock()
        if rate > 0 && randFloat() < rate {
            return errors.New("mode aware free: flaky error")
        }
        return nil
    default:
        return nil
    }
}

func (f *ModeAwareFree) StorageFree() (StorageFree, error) {
    if err := f.checkMode(); err != nil {
        return StorageFree{}, err
    }
    return f.free.StorageFree()
}

func (f *ModeAwareFree) MemoryFree() (MemoryFree, error) {
    if err := f.checkMode(); err != nil {
        return MemoryFree{}, err
    }
    return f.free.MemoryFree()
}

func (f *ModeAwareFree) NetworkFree() (NetworkFree, error) {
    if err := f.checkMode(); err != nil {
        return NetworkFree{}, err
    }
    return f.free.NetworkFree()
}

func (f *ModeAwareFree) DBFree() (DBFree, error) {
    if err := f.checkMode(); err != nil {
        return DBFree{}, err
    }
    return f.free.DBFree()
}

// --------------------------------------------------------------------
// ModeAwareBuffer – wraps a Buffer and enforces mode semantics.
// --------------------------------------------------------------------

type ModeAwareBuffer struct {
    buf  Buffer
    mgr  ModeManager
    mu   sync.Mutex
    flakyRate float64
}

func NewModeAwareBuffer(buf Buffer, mgr ModeManager) *ModeAwareBuffer {
    return &ModeAwareBuffer{
        buf: buf,
        mgr: mgr,
    }
}

func (b *ModeAwareBuffer) SetFlakyRate(rate float64) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.flakyRate = rate
}

func (b *ModeAwareBuffer) checkMode(write bool) error {
    mode := b.mgr.CurrentMode()
    switch mode {
    case ModeNormal:
        return nil
    case ModeDegraded:
        time.Sleep(50 * time.Millisecond)
        return nil
    case ModeReadOnly:
        if write {
            return errors.New("mode aware buffer: write denied in read‑only mode")
        }
        return nil
    case ModeOffline, ModeMaintenance:
        return errors.New("mode aware buffer: unavailable (" + string(mode) + ")")
    case ModeFlaky:
        b.mu.Lock()
        rate := b.flakyRate
        b.mu.Unlock()
        if rate > 0 && randFloat() < rate {
            return errors.New("mode aware buffer: flaky error")
        }
        return nil
    default:
        return nil
    }
}

func (b *ModeAwareBuffer) Read(p []byte) (int, error) {
    if err := b.checkMode(false); err != nil {
        return 0, err
    }
    return b.buf.Read(p)
}

func (b *ModeAwareBuffer) Write(p []byte) (int, error) {
    if err := b.checkMode(true); err != nil {
        return 0, err
    }
    return b.buf.Write(p)
}

func (b *ModeAwareBuffer) Close() error {
    // Close is usually allowed even in read‑only/offline? We'll allow.
    return b.buf.Close()
}

func (b *ModeAwareBuffer) Len() int {
    // Len is a read operation
    if err := b.checkMode(false); err != nil {
        return 0
    }
    return b.buf.Len()
}

func (b *ModeAwareBuffer) Cap() int {
    if err := b.checkMode(false); err != nil {
        return 0
    }
    return b.buf.Cap()
}

func (b *ModeAwareBuffer) Reset() {
    // Reset is a write operation (clears data)
    if err := b.checkMode(true); err != nil {
        return
    }
    b.buf.Reset()
}

func (b *ModeAwareBuffer) Bytes() []byte {
    if err := b.checkMode(false); err != nil {
        return nil
    }
    return b.buf.Bytes()
}

// --------------------------------------------------------------------
// Helper (pseudo‑random for flaky mode)
// --------------------------------------------------------------------
import "math/rand"

func randFloat() float64 {
    return rand.Float64()
}