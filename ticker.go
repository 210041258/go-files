// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"time"
)

// Ticker defines the interface for a ticker that can be stopped.
// It mimics the behaviour of time.Ticker but allows for mock implementations.
type Ticker interface {
	// C returns the channel on which ticks are delivered.
	C() <-chan time.Time
	// Stop turns off the ticker. After Stop, no more ticks will be sent.
	Stop()
}

// ------------------------------------------------------------------------
// RealTicker – wrapper around time.Ticker
// ------------------------------------------------------------------------

// RealTicker wraps a standard time.Ticker to satisfy the Ticker interface.
type RealTicker struct {
	*time.Ticker
}

// C returns the ticker's channel.
func (rt *RealTicker) C() <-chan time.Time {
	return rt.Ticker.C
}

// NewRealTicker creates a new RealTicker that ticks with the given duration.
func NewRealTicker(d time.Duration) *RealTicker {
	return &RealTicker{time.NewTicker(d)}
}

// ------------------------------------------------------------------------
// MockTicker – controllable ticker for tests
// ------------------------------------------------------------------------

// MockTicker is a ticker whose ticks are controlled manually.
// It is safe for use by a single goroutine at a time.
type MockTicker struct {
	ch       chan time.Time
	stop     chan struct{}
	stopped  bool
	duration time.Duration // only used for sleep‑based simulation
}

// NewMockTicker creates a new mock ticker. The returned ticker does not
// tick automatically; you must call Tick() to send a tick.
func NewMockTicker() *MockTicker {
	return &MockTicker{
		ch:   make(chan time.Time),
		stop: make(chan struct{}),
	}
}

// NewSleepMockTicker creates a mock ticker that ticks after each duration
// by sleeping in a background goroutine. This can be useful when you want
// a ticker that behaves like a real one but you still want to control the
// time in a coarse way. Use with caution in tests; prefer manual Tick().
func NewSleepMockTicker(d time.Duration) *MockTicker {
	mt := &MockTicker{
		ch:       make(chan time.Time),
		stop:     make(chan struct{}),
		duration: d,
	}
	go mt.run()
	return mt
}

// run is the background goroutine for SleepMockTicker.
func (mt *MockTicker) run() {
	ticker := time.NewTicker(mt.duration)
	defer ticker.Stop()
	for {
		select {
		case t := <-ticker.C:
			select {
			case mt.ch <- t:
			case <-mt.stop:
				return
			}
		case <-mt.stop:
			return
		}
	}
}

// C returns the channel on which ticks are sent.
func (mt *MockTicker) C() <-chan time.Time {
	return mt.ch
}

// Stop stops the mock ticker. No more ticks will be sent, and any pending
// Tick() calls will panic.
func (mt *MockTicker) Stop() {
	if mt.stopped {
		return
	}
	mt.stopped = true
	close(mt.stop)
}

// Tick sends a tick at the current time on the ticker's channel.
// It panics if the ticker has been stopped.
func (mt *MockTicker) Tick() {
	if mt.stopped {
		panic("MockTicker: Tick called after Stop")
	}
	mt.ch <- time.Now()
}

// TickAt sends a tick at the specified time on the ticker's channel.
// It panics if the ticker has been stopped.
func (mt *MockTicker) TickAt(t time.Time) {
	if mt.stopped {
		panic("MockTicker: TickAt called after Stop")
	}
	mt.ch <- t
}

// ------------------------------------------------------------------------
// Helper: Ticker that never ticks (useful for timeouts)
// ------------------------------------------------------------------------

// NeverTicker is a ticker that never ticks. Its C channel never receives.
// Stop is a no‑op.
type NeverTicker struct{}

// C returns a channel that never receives.
func (NeverTicker) C() <-chan time.Time { return nil }

// Stop does nothing.
func (NeverTicker) Stop() {}