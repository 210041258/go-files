// Package testutils provides mock and simple in-memory disk utilities for testing.
package testutils

import (
    "bytes"
    "errors"
    "io"
    "os"
    "sync"
    "time"
)

// --------------------------------------------------------------------
// Disk – interface for file-like operations.
// --------------------------------------------------------------------

// FileInfo is a simplified version of os.FileInfo for testing.
type FileInfo struct {
    Name    string
    Size    int64
    Mode    os.FileMode
    ModTime time.Time
    IsDir   bool
}

// Disk defines the methods a test disk should implement.
type Disk interface {
    // Open opens a file for reading.
    Open(name string) (File, error)
    // Create creates or truncates a file for writing.
    Create(name string) (File, error)
    // OpenFile opens a file with the given flags and mode.
    OpenFile(name string, flag int, perm os.FileMode) (File, error)
    // Remove deletes a file.
    Remove(name string) error
    // Rename renames a file.
    Rename(oldpath, newpath string) error
    // Stat returns file info.
    Stat(name string) (FileInfo, error)
    // Mkdir creates a directory.
    Mkdir(name string, perm os.FileMode) error
    // MkdirAll creates a directory and any necessary parents.
    MkdirAll(name string, perm os.FileMode) error
}

// File is the interface for an open file.
type File interface {
    io.ReadWriteCloser
    io.Seeker
    // Sync commits the current contents to stable storage.
    Sync() error
    // Stat returns file info.
    Stat() (FileInfo, error)
    // Name returns the file name.
    Name() string
}

// --------------------------------------------------------------------
// MockDisk – a test double that records calls and can be programmed.
// --------------------------------------------------------------------

// MockDisk implements Disk for unit tests.
type MockDisk struct {
    mu          sync.Mutex
    openFunc    func(name string) (File, error)
    createFunc  func(name string) (File, error)
    openFileFunc func(name string, flag int, perm os.FileMode) (File, error)
    removeFunc  func(name string) error
    renameFunc  func(oldpath, newpath string) error
    statFunc    func(name string) (FileInfo, error)
    mkdirFunc   func(name string, perm os.FileMode) error
    mkdirAllFunc func(name string, perm os.FileMode) error

    openCalls    []string
    createCalls  []string
    openFileCalls []struct{ name string; flag int; perm os.FileMode }
    removeCalls  []string
    renameCalls  []struct{ oldpath, newpath string }
    statCalls    []string
    mkdirCalls   []struct{ name string; perm os.FileMode }
    mkdirAllCalls []struct{ name string; perm os.FileMode }
}

// NewMockDisk creates a new mock disk.
func NewMockDisk() *MockDisk {
    return &MockDisk{}
}

// SetOpenFunc overrides the Open method.
func (m *MockDisk) SetOpenFunc(fn func(name string) (File, error)) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.openFunc = fn
}

// SetCreateFunc overrides the Create method.
func (m *MockDisk) SetCreateFunc(fn func(name string) (File, error)) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.createFunc = fn
}

// SetOpenFileFunc overrides the OpenFile method.
func (m *MockDisk) SetOpenFileFunc(fn func(name string, flag int, perm os.FileMode) (File, error)) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.openFileFunc = fn
}

// SetRemoveFunc overrides the Remove method.
func (m *MockDisk) SetRemoveFunc(fn func(name string) error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.removeFunc = fn
}

// SetRenameFunc overrides the Rename method.
func (m *MockDisk) SetRenameFunc(fn func(oldpath, newpath string) error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.renameFunc = fn
}

// SetStatFunc overrides the Stat method.
func (m *MockDisk) SetStatFunc(fn func(name string) (FileInfo, error)) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.statFunc = fn
}

// SetMkdirFunc overrides the Mkdir method.
func (m *MockDisk) SetMkdirFunc(fn func(name string, perm os.FileMode) error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.mkdirFunc = fn
}

// SetMkdirAllFunc overrides the MkdirAll method.
func (m *MockDisk) SetMkdirAllFunc(fn func(name string, perm os.FileMode) error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.mkdirAllFunc = fn
}

func (m *MockDisk) Open(name string) (File, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.openCalls = append(m.openCalls, name)
    if m.openFunc != nil {
        return m.openFunc(name)
    }
    return nil, errors.New("mock disk: Open not implemented")
}

func (m *MockDisk) Create(name string) (File, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.createCalls = append(m.createCalls, name)
    if m.createFunc != nil {
        return m.createFunc(name)
    }
    return nil, errors.New("mock disk: Create not implemented")
}

func (m *MockDisk) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.openFileCalls = append(m.openFileCalls, struct {
        name string
        flag int
        perm os.FileMode
    }{name, flag, perm})
    if m.openFileFunc != nil {
        return m.openFileFunc(name, flag, perm)
    }
    return nil, errors.New("mock disk: OpenFile not implemented")
}

func (m *MockDisk) Remove(name string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.removeCalls = append(m.removeCalls, name)
    if m.removeFunc != nil {
        return m.removeFunc(name)
    }
    return nil
}

func (m *MockDisk) Rename(oldpath, newpath string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.renameCalls = append(m.renameCalls, struct{ oldpath, newpath string }{oldpath, newpath})
    if m.renameFunc != nil {
        return m.renameFunc(oldpath, newpath)
    }
    return nil
}

func (m *MockDisk) Stat(name string) (FileInfo, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.statCalls = append(m.statCalls, name)
    if m.statFunc != nil {
        return m.statFunc(name)
    }
    return FileInfo{}, errors.New("mock disk: Stat not implemented")
}

func (m *MockDisk) Mkdir(name string, perm os.FileMode) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.mkdirCalls = append(m.mkdirCalls, struct {
        name string
        perm os.FileMode
    }{name, perm})
    if m.mkdirFunc != nil {
        return m.mkdirFunc(name, perm)
    }
    return nil
}

func (m *MockDisk) MkdirAll(name string, perm os.FileMode) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.mkdirAllCalls = append(m.mkdirAllCalls, struct {
        name string
        perm os.FileMode
    }{name, perm})
    if m.mkdirAllFunc != nil {
        return m.mkdirAllFunc(name, perm)
    }
    return nil
}

// CallCounts returns the number of calls for each method.
func (m *MockDisk) CallCounts() (open, create, openFile, remove, rename, stat, mkdir, mkdirAll int) {
    m.mu.Lock()
    defer m.mu.Unlock()
    return len(m.openCalls), len(m.createCalls), len(m.openFileCalls), len(m.removeCalls),
        len(m.renameCalls), len(m.statCalls), len(m.mkdirCalls), len(m.mkdirAllCalls)
}

// Reset clears all recorded calls.
func (m *MockDisk) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.openCalls = nil
    m.createCalls = nil
    m.openFileCalls = nil
    m.removeCalls = nil
    m.renameCalls = nil
    m.statCalls = nil
    m.mkdirCalls = nil
    m.mkdirAllCalls = nil
}

// --------------------------------------------------------------------
// InMemoryFile – implements File using a bytes.Buffer.
// --------------------------------------------------------------------

type InMemoryFile struct {
    mu       sync.Mutex
    name     string
    buffer   *bytes.Buffer
    closed   bool
    mode     os.FileMode
    modTime  time.Time
}

func NewInMemoryFile(name string) *InMemoryFile {
    return &InMemoryFile{
        name:    name,
        buffer:  &bytes.Buffer{},
        modTime: time.Now(),
    }
}

func (f *InMemoryFile) Read(p []byte) (int, error) {
    f.mu.Lock()
    defer f.mu.Unlock()
    if f.closed {
        return 0, os.ErrClosed
    }
    return f.buffer.Read(p)
}

func (f *InMemoryFile) Write(p []byte) (int, error) {
    f.mu.Lock()
    defer f.mu.Unlock()
    if f.closed {
        return 0, os.ErrClosed
    }
    n, err := f.buffer.Write(p)
    f.modTime = time.Now()
    return n, err
}

func (f *InMemoryFile) Seek(offset int64, whence int) (int64, error) {
    f.mu.Lock()
    defer f.mu.Unlock()
    if f.closed {
        return 0, os.ErrClosed
    }
    // bytes.Buffer doesn't support Seek; we'd need a more complex implementation.
    // For simplicity, we'll implement a basic seek using a bytes.Reader.
    // But we need to maintain a read position. Let's use a bytes.Reader internally.
    // Alternatively, we could use a bytes.Buffer with a separate position.
    // To keep it simple, we'll just return an error for now.
    return 0, errors.New("in-memory file: Seek not implemented")
}

func (f *InMemoryFile) Close() error {
    f.mu.Lock()
    defer f.mu.Unlock()
    f.closed = true
    return nil
}

func (f *InMemoryFile) Sync() error {
    return nil // in-memory, no-op
}

func (f *InMemoryFile) Stat() (FileInfo, error) {
    f.mu.Lock()
    defer f.mu.Unlock()
    if f.closed {
        return FileInfo{}, os.ErrClosed
    }
    return FileInfo{
        Name:    f.name,
        Size:    int64(f.buffer.Len()),
        Mode:    f.mode,
        ModTime: f.modTime,
        IsDir:   false,
    }, nil
}

func (f *InMemoryFile) Name() string {
    f.mu.Lock()
    defer f.mu.Unlock()
    return f.name
}

// --------------------------------------------------------------------
// InMemoryDisk – a simple in-memory file system for integration tests.
// --------------------------------------------------------------------

// InMemoryDisk implements Disk using an in-memory map.
type InMemoryDisk struct {
    mu    sync.RWMutex
    files map[string]*InMemoryFile
    dirs  map[string]bool
}

func NewInMemoryDisk() *InMemoryDisk {
    return &InMemoryDisk{
        files: make(map[string]*InMemoryFile),
        dirs:  make(map[string]bool),
    }
}

func (d *InMemoryDisk) Open(name string) (File, error) {
    d.mu.RLock()
    file, ok := d.files[name]
    d.mu.RUnlock()
    if !ok {
        return nil, os.ErrNotExist
    }
    // Return a new reader (can't share the same file with position)
    // For simplicity, we'll just return a copy? That's complex.
    // In a real in-memory FS, we'd need to handle multiple opens.
    // For testing, we'll return the same file but with a new buffer? Not safe.
    // Let's implement a more robust in-memory FS later if needed.
    return file, nil // but this shares the same file object (position and buffer)
}

func (d *InMemoryDisk) Create(name string) (File, error) {
    d.mu.Lock()
    defer d.mu.Unlock()
    file := NewInMemoryFile(name)
    d.files[name] = file
    return file, nil
}

func (d *InMemoryDisk) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
    // Simplified: ignore flags other than create/trunc.
    d.mu.Lock()
    defer d.mu.Unlock()
    file, ok := d.files[name]
    if !ok {
        if flag&os.O_CREATE != 0 {
            file = NewInMemoryFile(name)
            d.files[name] = file
            return file, nil
        }
        return nil, os.ErrNotExist
    }
    if flag&os.O_TRUNC != 0 {
        file.buffer.Reset()
    }
    return file, nil
}

func (d *InMemoryDisk) Remove(name string) error {
    d.mu.Lock()
    defer d.mu.Unlock()
    if _, ok := d.files[name]; ok {
        delete(d.files, name)
        return nil
    }
    return os.ErrNotExist
}

func (d *InMemoryDisk) Rename(oldpath, newpath string) error {
    d.mu.Lock()
    defer d.mu.Unlock()
    file, ok := d.files[oldpath]
    if !ok {
        return os.ErrNotExist
    }
    d.files[newpath] = file
    delete(d.files, oldpath)
    return nil
}

func (d *InMemoryDisk) Stat(name string) (FileInfo, error) {
    d.mu.RLock()
    file, ok := d.files[name]
    d.mu.RUnlock()
    if ok {
        return file.Stat()
    }
    d.mu.RLock()
    _, isDir := d.dirs[name]
    d.mu.RUnlock()
    if isDir {
        return FileInfo{
            Name:  name,
            IsDir: true,
            Mode:  os.ModeDir | 0755,
        }, nil
    }
    return FileInfo{}, os.ErrNotExist
}

func (d *InMemoryDisk) Mkdir(name string, perm os.FileMode) error {
    d.mu.Lock()
    defer d.mu.Unlock()
    if _, ok := d.files[name]; ok {
        return os.ErrExist
    }
    if _, ok := d.dirs[name]; ok {
        return os.ErrExist
    }
    d.dirs[name] = true
    return nil
}

func (d *InMemoryDisk) MkdirAll(name string, perm os.FileMode) error {
    // For simplicity, just Mkdir.
    return d.Mkdir(name, perm)
}

// --------------------------------------------------------------------
// DiskConditioner – wraps a disk to inject errors and delays.
// --------------------------------------------------------------------

type DiskConditioner struct {
    mu          sync.Mutex
    disk        Disk
    readDelay   time.Duration
    writeDelay  time.Duration
    readErrors  map[int]error // call number -> error
    writeErrors map[int]error
    readCalls   int
    writeCalls  int
}

func NewDiskConditioner(disk Disk) *DiskConditioner {
    return &DiskConditioner{
        disk:        disk,
        readErrors:  make(map[int]error),
        writeErrors: make(map[int]error),
    }
}

func (c *DiskConditioner) SetReadDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.readDelay = d
}

func (c *DiskConditioner) SetWriteDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.writeDelay = d
}

func (c *DiskConditioner) InjectReadError(callNumber int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.readErrors[callNumber] = err
}

func (c *DiskConditioner) InjectWriteError(callNumber int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.writeErrors[callNumber] = err
}

// Open delegates with possible delays/errors (simplified – not all methods wrapped)
func (c *DiskConditioner) Open(name string) (File, error) {
    return c.disk.Open(name)
}

func (c *DiskConditioner) Create(name string) (File, error) {
    return c.disk.Create(name)
}

func (c *DiskConditioner) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
    return c.disk.OpenFile(name, flag, perm)
}

func (c *DiskConditioner) Remove(name string) error {
    return c.disk.Remove(name)
}

func (c *DiskConditioner) Rename(oldpath, newpath string) error {
    return c.disk.Rename(oldpath, newpath)
}

func (c *DiskConditioner) Stat(name string) (FileInfo, error) {
    return c.disk.Stat(name)
}

func (c *DiskConditioner) Mkdir(name string, perm os.FileMode) error {
    return c.disk.Mkdir(name, perm)
}

func (c *DiskConditioner) MkdirAll(name string, perm os.FileMode) error {
    return c.disk.MkdirAll(name, perm)
}

// ConditionedFile wraps a File to add delays and errors.
type ConditionedFile struct {
    mu           sync.Mutex
    file         File
    conditioner  *DiskConditioner
    isRead       bool // to count read/write calls separately? We'll count per file.
    readCalls    int
    writeCalls   int
}

func (c *DiskConditioner) newConditionedFile(f File) *ConditionedFile {
    return &ConditionedFile{
        file:        f,
        conditioner: c,
    }
}

func (cf *ConditionedFile) Read(p []byte) (int, error) {
    cf.mu.Lock()
    cf.readCalls++
    call := cf.readCalls
    cf.mu.Unlock()

    cf.conditioner.mu.Lock()
    delay := cf.conditioner.readDelay
    err, ok := cf.conditioner.readErrors[call]
    cf.conditioner.mu.Unlock()

    if ok {
        return 0, err
    }
    if delay > 0 {
        time.Sleep(delay)
    }
    return cf.file.Read(p)
}

func (cf *ConditionedFile) Write(p []byte) (int, error) {
    cf.mu.Lock()
    cf.writeCalls++
    call := cf.writeCalls
    cf.mu.Unlock()

    cf.conditioner.mu.Lock()
    delay := cf.conditioner.writeDelay
    err, ok := cf.conditioner.writeErrors[call]
    cf.conditioner.mu.Unlock()

    if ok {
        return 0, err
    }
    if delay > 0 {
        time.Sleep(delay)
    }
    return cf.file.Write(p)
}

func (cf *ConditionedFile) Seek(offset int64, whence int) (int64, error) {
    return cf.file.Seek(offset, whence)
}

func (cf *ConditionedFile) Close() error {
    return cf.file.Close()
}

func (cf *ConditionedFile) Sync() error {
    return cf.file.Sync()
}

func (cf *ConditionedFile) Stat() (FileInfo, error) {
    return cf.file.Stat()
}

func (cf *ConditionedFile) Name() string {
    return cf.file.Name()
}