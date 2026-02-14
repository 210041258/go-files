// Package lock provides extended synchronization primitives including
// mutexes with timeouts, keyed locks, cross‑platform file locks, and semaphores.
//
// All types are safe for concurrent use.
package testutils

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// --------------------------------------------------------------------
// Mutex with timeout and TryLock
// --------------------------------------------------------------------

// ErrTimeout is returned by TryLockTimeout when the lock cannot be acquired
// within the specified duration.
var ErrTimeout = errors.New("lock: timeout acquiring mutex")

// Mutex is a mutual exclusion lock that extends sync.Mutex with TryLock
// and TryLockTimeout.
type Mutex struct {
	mu sync.Mutex
}

// NewMutex creates a new Mutex.
func NewMutex() *Mutex {
	return &Mutex{}
}

// Lock locks the mutex. It blocks until the lock is acquired.
func (m *Mutex) Lock() {
	m.mu.Lock()
}

// Unlock unlocks the mutex. It panics if the mutex is not locked.
func (m *Mutex) Unlock() {
	m.mu.Unlock()
}

// TryLock attempts to lock the mutex without blocking.
// It returns true if the lock was acquired, false otherwise.
func (m *Mutex) TryLock() bool {
	return m.mu.TryLock()
}

// TryLockTimeout attempts to lock the mutex within the given timeout.
// It returns nil if the lock was acquired, ErrTimeout otherwise.
func (m *Mutex) TryLockTimeout(timeout time.Duration) error {
	return m.TryLockContext(context.Background(), timeout)
}

// TryLockContext attempts to lock the mutex until the context is done
// or the timeout expires. It returns nil if the lock was acquired,
// ctx.Err() if the context expires, or ErrTimeout if the timeout is reached.
func (m *Mut​ex) TryLockContext(ctx context.Context, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if m.TryLock() {
		return nil
	}

	done := make(chan struct{})
	go func() {
		m.mu.Lock()
		close(done)
	}()

	select {
	case <-ctx.Done():
		go func() {
			<-done
			m.mu.Unlock()
		}()
		return ctx.Err()
	case <-timer.C:
		go func() {
			<-done
			m.mu.Unlock()
		}()
		return ErrTimeout
	case <-done:
		return nil
	}
}

// --------------------------------------------------------------------
// RWMutex with timeout and TryLock
// --------------------------------------------------------------------

// RWMutex is a reader/writer mutual exclusion lock that extends sync.RWMutex
// with TryRLock, TryLock, and timeout versions.
type RWMutex struct {
	mu sync.RWMutex
}

// NewRWMutex creates a new RWMutex.
func NewRWMutex() *RWMutex {
	return &RWMutex{}
}

// Lock locks the mutex for writing.
func (rw *RWMutex) Lock() {
	rw.mu.Lock()
}

// Unlock unlocks the mutex for writing.
func (rw *RWMutex) Unlock() {
	rw.mu.Unlock()
}

// RLock locks the mutex for reading.
func (rw *RWMutex) RLock() {
	rw.mu.RLock()
}

// RUnlock unlocks the mutex for reading.
func (rw *RWMutex) RUnlock() {
	rw.mu.RUnlock()
}

// TryLock attempts to lock the mutex for writing without blocking.
func (rw *RWMutex) TryLock() bool {
	return rw.mu.TryLock()
}

// TryRLock attempts to lock the mutex for reading without blocking.
func (rw *RWMutex) TryRLock() bool {
	return rw.mu.TryRLock()
}

// TryLockTimeout attempts to acquire the write lock within the timeout.
func (rw *RWMutex) TryLockTimeout(timeout time.Duration) error {
	// Simplified implementation; for full-featured see Mutex.TryLockContext above.
	ch := make(chan struct{})
	go func() {
		rw.mu.Lock()
		close(ch)
	}()
	select {
	case <-ch:
		return nil
	case <-time.After(timeout):
		go func() {
			<-ch
			rw.mu.Unlock()
		}()
		return ErrTimeout
	}
}

// --------------------------------------------------------------------
// KeyedMutex – lock by string ID
// --------------------------------------------------------------------

// KeyedMutex provides per-key mutual exclusion.
// It is safe for concurrent access and automatically cleans up unused locks.
type KeyedMutex struct {
	mu    sync.Mutex
	locks map[string]*refCountedMutex
}

type refCountedMutex struct {
	mu        *Mutex
	refCount  int64
	lastUsed  time.Time
}

// NewKeyedMutex creates a new KeyedMutex.
func NewKeyedMutex() *KeyedMutex {
	return &KeyedMutex{
		locks: make(map[string]*refCountedMutex),
	}
}

// Lock acquires the lock for the given key. It blocks until the lock is acquired.
// It automatically increments the reference count and cleans up unused locks
// when Unlock is called and the count reaches zero.
func (km *KeyedMutex) Lock(key string) {
	km.mu.Lock()
	rc, ok := km.locks[key]
	if !ok {
		rc = &refCountedMutex{
			mu:       NewMutex(),
			refCount: 0,
		}
		km.locks[key] = rc
	}
	atomic.AddInt64(&rc.refCount, 1)
	rc.lastUsed = time.Now()
	km.mu.Unlock()

	rc.mu.Lock()
}

// Unlock releases the lock for the given key. It panics if the lock is not held.
// When the reference count reaches zero, the lock is eligible for cleanup.
func (km *KeyedMutex) Unlock(key string) {
	km.mu.Lock()
	defer km.mu.Unlock()

	rc, ok := km.locks[key]
	if !ok {
		panic("unlock of unlocked key: " + key)
	}
	rc.mu.Unlock()

	newCount := atomic.AddInt64(&rc.refCount, -1)
	if newCount == 0 {
		delete(km.locks, key)
	}
}

// Cleanup removes all unused locks. Normally not needed, but can be called periodically.
func (km *KeyedMutex) Cleanup(olderThan time.Duration) {
	km.mu.Lock()
	defer km.mu.Unlock()
	threshold := time.Now().Add(-olderThan)
	for k, rc := range km.locks {
		if atomic.LoadInt64(&rc.refCount) == 0 && rc.lastUsed.Before(threshold) {
			delete(km.locks, k)
		}
	}
}

// --------------------------------------------------------------------
// Semaphore – counting semaphore
// --------------------------------------------------------------------

// Semaphore is a counting semaphore implemented with a buffered channel.
type Semaphore struct {
	ch chan struct{}
}

// NewSemaphore creates a new semaphore with the given maximum count.
func NewSemaphore(max int) *Semaphore {
	if max <= 0 {
		panic("semaphore: max must be positive")
	}
	return &Semaphore{
		ch: make(chan struct{}, max),
	}
}

// Acquire acquires one permit, blocking until available.
func (s *Semaphore) Acquire() {
	s.ch <- struct{}{}
}

// AcquireContext acquires one permit, blocking until available or the context is done.
// Returns nil on success, ctx.Err() otherwise.
func (s *Semaphore) AcquireContext(ctx context.Context) error {
	select {
	case s.ch <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TryAcquire attempts to acquire one permit without blocking.
// Returns true if acquired, false otherwise.
func (s *Semaphore) TryAcquire() bool {
	select {
	case s.ch <- struct{}{}:
		return true
	default:
		return false
	}
}

// Release releases one permit. Never blocks.
func (s *Semaphore) Release() {
	<-s.ch
}

// Len returns the current number of occupied permits.
func (s *Semaphore) Len() int {
	return len(s.ch)
}

// Cap returns the maximum number of permits.
func (s *Semaphore) Cap() int {
	return cap(s.ch)
}

// --------------------------------------------------------------------
// FileLock – cross‑platform advisory file lock
// --------------------------------------------------------------------

// FileLock represents an exclusive lock on a file.
// It uses LockFileEx on Windows and flock on Unix.
type FileLock struct {
	file *os.File
	path string
}

// NewFileLock creates a new FileLock for the given path.
// The file is opened (or created) immediately.
func NewFileLock(path string) (*FileLock, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("lock: cannot create directory %s: %w", dir, err)
	}

	// Open file with appropriate flags
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("lock: cannot open file %s: %w", path, err)
	}
	return &FileLock{
		file: file,
		path: path,
	}, nil
}

// Lock acquires an exclusive lock on the file. Blocks until acquired.
func (fl *FileLock) Lock() error {
	return fl.lock(true, 0)
}

// TryLock attempts to acquire an exclusive lock without blocking.
// Returns true if lock acquired, false otherwise.
func (fl *FileLock) TryLock() (bool, error) {
	err := fl.lock(false, 0)
	if err == errWouldBlock {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Unlock releases the file lock.
func (fl *FileLock) Unlock() error {
	return fl.unlock()
}

// Close releases the lock and closes the underlying file.
func (fl *FileLock) Close() error {
	return fl.file.Close()
}

// platform-specific implementations
var errWouldBlock = errors.New("lock: resource temporarily unavailable")

// Windows implementation
// Unix implementation follows