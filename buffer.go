// Package testutils provides mock and simple in-memory buffer implementations
// suitable for testing storage, memory, network, and database components.
package testutils

import (
    "bytes"
    "errors"
    "io"
    "sync"
    "time"
)

// --------------------------------------------------------------------
// Buffer – core interface for all buffers.
// --------------------------------------------------------------------

// Buffer defines the operations common to all buffer types.
type Buffer interface {
    io.ReadWriteCloser

    // Len returns the number of bytes currently buffered.
    Len() int

    // Cap returns the total capacity of the buffer (if bounded).
    Cap() int

    // Reset discards any buffered data and resets the buffer to its initial state.
    Reset()

    // Bytes returns a slice of the buffered data (the slice is valid only until
    // the next write/modification). Use with care; for tests only.
    Bytes() []byte
}

// --------------------------------------------------------------------
// MockBuffer – a test double that records all operations.
// --------------------------------------------------------------------

// MockBuffer implements Buffer for unit tests.
// It records every call and can be programmed to return specific errors
// on a per‑call basis (1‑based).
type MockBuffer struct {
    mu          sync.Mutex
    data        []byte                     // simulated buffered data
    readCalls   int
    writeCalls  int
    closeCalls  int
    resetCalls  int
    bytesCalls  int
    readErrors  map[int]error               // call number -> error for Read
    writeErrors map[int]error                // call number -> error for Write
    closeErrors map[int]error                // call number -> error for Close
    // readData can be set to control what Read returns (simulate incoming data).
    readData    []byte
}

// NewMockBuffer creates a new mock buffer with no programmed errors.
func NewMockBuffer() *MockBuffer {
    return &MockBuffer{
        readErrors:  make(map[int]error),
        writeErrors: make(map[int]error),
        closeErrors: make(map[int]error),
    }
}

// SetReadData sets the data that will be returned by subsequent Read calls.
// Each Read call will consume bytes from this slice until exhausted.
func (m *MockBuffer) SetReadData(data []byte) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.readData = append([]byte(nil), data...)
}

// InjectReadError makes the nth call to Read return the given error.
func (m *MockBuffer) InjectReadError(callNumber int, err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.readErrors[callNumber] = err
}

// InjectWriteError makes the nth call to Write return the given error.
func (m *MockBuffer) InjectWriteError(callNumber int, err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.writeErrors[callNumber] = err
}

// InjectCloseError makes the nth call to Close return the given error.
func (m *MockBuffer) InjectCloseError(callNumber int, err error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.closeErrors[callNumber] = err
}

// Read implements io.Reader.
func (m *MockBuffer) Read(p []byte) (int, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.readCalls++
    if err, ok := m.readErrors[m.readCalls]; ok {
        delete(m.readErrors, m.readCalls)
        return 0, err
    }
    if len(m.readData) == 0 {
        return 0, io.EOF
    }
    n := copy(p, m.readData)
    m.readData = m.readData[n:]
    return n, nil
}

// Write implements io.Writer.
func (m *MockBuffer) Write(p []byte) (int, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.writeCalls++
    if err, ok := m.writeErrors[m.writeCalls]; ok {
        delete(m.writeErrors, m.writeCalls)
        return 0, err
    }
    m.data = append(m.data, p...)
    return len(p), nil
}

// Close implements io.Closer.
func (m *MockBuffer) Close() error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.closeCalls++
    if err, ok := m.closeErrors[m.closeCalls]; ok {
        delete(m.closeErrors, m.closeCalls)
        return err
    }
    return nil
}

// Len returns the length of the buffered data.
func (m *MockBuffer) Len() int {
    m.mu.Lock()
    defer m.mu.Unlock()
    return len(m.data)
}

// Cap returns the capacity (simulated as unlimited, returns Len() for simplicity).
func (m *MockBuffer) Cap() int {
    return m.Len()
}

// Reset discards all buffered data.
func (m *MockBuffer) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.resetCalls++
    m.data = nil
}

// Bytes returns the buffered data.
func (m *MockBuffer) Bytes() []byte {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.bytesCalls++
    return m.data
}

// CallCounts returns the number of calls to each method.
func (m *MockBuffer) CallCounts() (read, write, close, reset, bytes int) {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.readCalls, m.writeCalls, m.closeCalls, m.resetCalls, m.bytesCalls
}

// ResetCalls clears all recorded call counts and injected errors.
func (m *MockBuffer) ResetCalls() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.readCalls = 0
    m.writeCalls = 0
    m.closeCalls = 0
    m.resetCalls = 0
    m.bytesCalls = 0
    m.readErrors = make(map[int]error)
    m.writeErrors = make(map[int]error)
    m.closeErrors = make(map[int]error)
    m.readData = nil
}

// --------------------------------------------------------------------
// InMemoryBuffer – a simple in‑memory buffer backed by bytes.Buffer.
// --------------------------------------------------------------------

// InMemoryBuffer implements Buffer using bytes.Buffer.
type InMemoryBuffer struct {
    mu   sync.Mutex
    buf  *bytes.Buffer
    cap  int // 0 means unlimited
}

// NewInMemoryBuffer creates an unbounded in‑memory buffer.
func NewInMemoryBuffer() *InMemoryBuffer {
    return &InMemoryBuffer{
        buf: new(bytes.Buffer),
    }
}

// NewBoundedBuffer creates an in‑memory buffer with a fixed capacity.
// Writes that would exceed the capacity return an error.
func NewBoundedBuffer(capacity int) *InMemoryBuffer {
    return &InMemoryBuffer{
        buf: bytes.NewBuffer(make([]byte, 0, capacity)),
        cap: capacity,
    }
}

// Read implements io.Reader.
func (b *InMemoryBuffer) Read(p []byte) (int, error) {
    b.mu.Lock()
    defer b.mu.Unlock()
    return b.buf.Read(p)
}

// Write implements io.Writer. If the buffer is bounded and the write would
// exceed capacity, it returns an error and writes nothing.
func (b *InMemoryBuffer) Write(p []byte) (int, error) {
    b.mu.Lock()
    defer b.mu.Unlock()
    if b.cap > 0 {
        available := b.cap - b.buf.Len()
        if len(p) > available {
            return 0, errors.New("in-memory buffer: capacity exceeded")
        }
    }
    return b.buf.Write(p)
}

// Close is a no‑op; always returns nil.
func (b *InMemoryBuffer) Close() error {
    return nil
}

// Len returns the number of bytes buffered.
func (b *InMemoryBuffer) Len() int {
    b.mu.Lock()
    defer b.mu.Unlock()
    return b.buf.Len()
}

// Cap returns the maximum capacity (0 means unlimited).
func (b *InMemoryBuffer) Cap() int {
    return b.cap
}

// Reset discards all buffered data.
func (b *InMemoryBuffer) Reset() {
    b.mu.Lock()
    defer b.mu.Unlock()
    if b.cap > 0 {
        b.buf = bytes.NewBuffer(make([]byte, 0, b.cap))
    } else {
        b.buf = new(bytes.Buffer)
    }
}

// Bytes returns the buffered data. The slice is valid only until the next write.
func (b *InMemoryBuffer) Bytes() []byte {
    b.mu.Lock()
    defer b.mu.Unlock()
    return b.buf.Bytes()
}

// --------------------------------------------------------------------
// BufferConditioner – wraps a Buffer to inject delays and per‑call errors.
// --------------------------------------------------------------------

// BufferConditioner adds configurable delays and error injection to any Buffer.
type BufferConditioner struct {
    mu          sync.Mutex
    buffer      Buffer
    readDelay   time.Duration
    writeDelay  time.Duration
    readErrors  map[int]error
    writeErrors map[int]error
    closeErrors map[int]error
    readCalls   int
    writeCalls  int
    closeCalls  int
}

// NewBufferConditioner creates a conditioner around an existing Buffer.
func NewBufferConditioner(buf Buffer) *BufferConditioner {
    return &BufferConditioner{
        buffer:      buf,
        readErrors:  make(map[int]error),
        writeErrors: make(map[int]error),
        closeErrors: make(map[int]error),
    }
}

// SetReadDelay adds a fixed delay before every Read.
func (c *BufferConditioner) SetReadDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.readDelay = d
}

// SetWriteDelay adds a fixed delay before every Write.
func (c *BufferConditioner) SetWriteDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.writeDelay = d
}

// InjectReadError makes the nth call to Read return the given error.
func (c *BufferConditioner) InjectReadError(callNumber int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.readErrors[callNumber] = err
}

// InjectWriteError makes the nth call to Write return the given error.
func (c *BufferConditioner) InjectWriteError(callNumber int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.writeErrors[callNumber] = err
}

// InjectCloseError makes the nth call to Close return the given error.
func (c *BufferConditioner) InjectCloseError(callNumber int, err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.closeErrors[callNumber] = err
}

// Read implements io.Reader with delays and error injection.
func (c *BufferConditioner) Read(p []byte) (int, error) {
    c.mu.Lock()
    c.readCalls++
    call := c.readCalls
    delay := c.readDelay
    err, ok := c.readErrors[call]
    if ok {
        delete(c.readErrors, call)
    }
    c.mu.Unlock()

    if ok {
        return 0, err
    }
    if delay > 0 {
        time.Sleep(delay)
    }
    return c.buffer.Read(p)
}

// Write implements io.Writer with delays and error injection.
func (c *BufferConditioner) Write(p []byte) (int, error) {
    c.mu.Lock()
    c.writeCalls++
    call := c.writeCalls
    delay := c.writeDelay
    err, ok := c.writeErrors[call]
    if ok {
        delete(c.writeErrors, call)
    }
    c.mu.Unlock()

    if ok {
        return 0, err
    }
    if delay > 0 {
        time.Sleep(delay)
    }
    return c.buffer.Write(p)
}

// Close implements io.Closer with error injection.
func (c *BufferConditioner) Close() error {
    c.mu.Lock()
    c.closeCalls++
    call := c.closeCalls
    err, ok := c.closeErrors[call]
    if ok {
        delete(c.closeErrors, call)
    }
    c.mu.Unlock()

    if ok {
        return err
    }
    return c.buffer.Close()
}

// Len delegates to the underlying buffer.
func (c *BufferConditioner) Len() int {
    return c.buffer.Len()
}

// Cap delegates to the underlying buffer.
func (c *BufferConditioner) Cap() int {
    return c.buffer.Cap()
}

// Reset delegates to the underlying buffer.
func (c *BufferConditioner) Reset() {
    c.buffer.Reset()
}

// Bytes delegates to the underlying buffer.
func (c *BufferConditioner) Bytes() []byte {
    return c.buffer.Bytes()
}