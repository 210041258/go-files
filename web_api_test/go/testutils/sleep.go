// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"sync"
	"time"
)

// Sleeper defines the interface for sleeping.
// It allows tests to replace real time.Sleep with mock implementations.
type Sleeper interface {
	// Sleep pauses the current goroutine for at least d duration.
	Sleep(d time.Duration)
}

// ------------------------------------------------------------------------
// RealSleeper – uses time.Sleep
// ------------------------------------------------------------------------

// RealSleeper is a Sleeper that calls time.Sleep.
type RealSleeper struct{}

// Sleep pauses for the given duration using time.Sleep.
func (RealSleeper) Sleep(d time.Duration) {
	time.Sleep(d)
}

// ------------------------------------------------------------------------
// MockSleeper – records sleep calls without actually sleeping
// ------------------------------------------------------------------------

// MockSleeper records all sleep calls and does not block.
// Useful for asserting that specific sleeps occurred.
type MockSleeper struct {
	mu         sync.Mutex
	totalSlept time.Duration
	calls      []time.Duration
}

// Sleep records the duration and returns immediately.
func (m *MockSleeper) Sleep(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalSlept += d
	m.calls = append(m.calls, d)
}

// Total returns the sum of all slept durations.
func (m *MockSleeper) Total() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.totalSlept
}

// Calls returns a copy of the recorded sleep durations.
func (m *MockSleeper) Calls() []time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	cpy := make([]time.Duration, len(m.calls))
	copy(cpy, m.calls)
	return cpy
}

// Reset clears all recorded sleeps.
func (m *MockSleeper) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalSlept = 0
	m.calls = nil
}

// ------------------------------------------------------------------------
// ControllableSleeper – blocks until explicitly woken
// ------------------------------------------------------------------------

// ControllableSleeper is a Sleeper that blocks on Sleep until Wake is called.
// It is useful for simulating time passing in tests without actually sleeping.
type ControllableSleeper struct {
	mu    sync.Mutex
	cond  *sync.Cond
	waiting int // number of goroutines currently waiting
}

// NewControllableSleeper creates a new controllable sleeper.
func NewControllableSleeper() *ControllableSleeper {
	return &ControllableSleeper{
		cond: sync.NewCond(&sync.Mutex{}),
	}
}

// Sleep blocks until Wake is called. The duration d is ignored; the method
// returns only when Wake is invoked. This allows tests to control the exact
// moment when Sleep returns.
func (c *ControllableSleeper) Sleep(d time.Duration) {
	c.cond.L.Lock()
	c.waiting++
	c.cond.Wait() // blocks until Broadcast is called
	c.waiting--
	c.cond.L.Unlock()
}

// Wake unblocks all goroutines currently blocked in Sleep.
func (c *ControllableSleeper) Wake() {
	c.cond.L.Lock()
	c.cond.Broadcast()
	c.cond.L.Unlock()
}

// Waiting returns the number of goroutines currently blocked in Sleep.
func (c *ControllableSleeper) Waiting() int {
	c.cond.L.Lock()
	defer c.cond.L.Unlock()
	return c.waiting
}

// ------------------------------------------------------------------------
// SleepFunc – functional adapter
// ------------------------------------------------------------------------

// SleepFunc is a function type that implements Sleeper.
type SleepFunc func(d time.Duration)

// Sleep calls the underlying function.
func (f SleepFunc) Sleep(d time.Duration) {
	f(d)
}