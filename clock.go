// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"sync"
	"time"
)

// Clock defines the interface for time operations.
// Using this interface allows tests to control time without relying on the real system clock.
type Clock interface {
	// Now returns the current time.
	Now() time.Time
	// After returns a channel that receives after the duration d.
	After(d time.Duration) <-chan time.Time
	// NewTicker creates a new Ticker that ticks every d.
	NewTicker(d time.Duration) Ticker
	// NewTimer creates a new Timer that fires after d.
	NewTimer(d time.Duration) Timer
	// Sleep pauses the current goroutine for at least d duration.
	Sleep(d time.Duration)
}

// Ticker defines the interface for a ticker.
type Ticker interface {
	// C returns the channel on which ticks are delivered.
	C() <-chan time.Time
	// Stop turns off the ticker. No more ticks will be sent.
	Stop()
}

// Timer defines the interface for a timer.
type Timer interface {
	// C returns the channel on which the time is delivered when the timer fires.
	C() <-chan time.Time
	// Stop prevents the timer from firing. It returns true if the call stops the timer,
	// false if the timer has already expired or been stopped.
	Stop() bool
	// Reset changes the timer to expire after duration d. It returns true if the timer
	// had been active, false if the timer had already fired or been stopped.
	Reset(d time.Duration) bool
}

// ------------------------------------------------------------------------
// RealClock – implementation using the standard time package
// ------------------------------------------------------------------------

// RealClock is a Clock that delegates to the standard library.
type RealClock struct{}

// Now returns time.Now().
func (RealClock) Now() time.Time { return time.Now() }

// After calls time.After.
func (RealClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// NewTicker returns a RealTicker wrapping time.NewTicker.
func (RealClock) NewTicker(d time.Duration) Ticker {
	return &realTicker{time.NewTicker(d)}
}

// NewTimer returns a RealTimer wrapping time.NewTimer.
func (RealClock) NewTimer(d time.Duration) Timer {
	return &realTimer{time.NewTimer(d)}
}

// Sleep calls time.Sleep.
func (RealClock) Sleep(d time.Duration) { time.Sleep(d) }

type realTicker struct {
	*time.Ticker
}

func (rt *realTicker) C() <-chan time.Time { return rt.Ticker.C }

type realTimer struct {
	*time.Timer
}

func (rt *realTimer) C() <-chan time.Time { return rt.Timer.C }

// ------------------------------------------------------------------------
// MockClock – controllable clock for testing
// ------------------------------------------------------------------------

// MockClock is a Clock that allows tests to control the passage of time.
// All time‑based operations are simulated without real waiting.
type MockClock struct {
	mu      sync.Mutex
	now     time.Time
	timers  []*mockTimer
	tickers []*mockTicker
}

// NewMockClock creates a new mock clock set to the given start time.
// If start.IsZero(), time.Unix(0, 0) is used.
func NewMockClock(start time.Time) *MockClock {
	if start.IsZero() {
		start = time.Unix(0, 0)
	}
	return &MockClock{
		now: start,
	}
}

// Now returns the current mock time.
func (mc *MockClock) Now() time.Time {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	return mc.now
}

// After returns a channel that will receive the current time after the mock
// duration d has passed (i.e., after Advance is called enough times).
func (mc *MockClock) After(d time.Duration) <-chan time.Time {
	return mc.NewTimer(d).C()
}

// NewTicker creates a new mock ticker that ticks every d in mock time.
func (mc *MockClock) NewTicker(d time.Duration) Ticker {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	t := &mockTicker{
		clock:  mc,
		period: d,
		ch:     make(chan time.Time),
		next:   mc.now.Add(d),
	}
	mc.tickers = append(mc.tickers, t)
	return t
}

// NewTimer creates a new mock timer that fires after d in mock time.
func (mc *MockClock) NewTimer(d time.Duration) Timer {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	t := &mockTimer{
		clock: mc,
		ch:    make(chan time.Time, 1),
		when:  mc.now.Add(d),
	}
	mc.timers = append(mc.timers, t)
	return t
}

// Sleep advances the mock time by d. It does not block; instead it triggers
// any timers or tickers that become due.
func (mc *MockClock) Sleep(d time.Duration) {
	mc.Advance(d)
}

// Advance moves the mock time forward by d and fires any timers/tickers that
// expire during this period.
func (mc *MockClock) Advance(d time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.now = mc.now.Add(d)

	// Fire timers that have expired.
	remaining := mc.timers[:0]
	for _, t := range mc.timers {
		if !t.when.After(mc.now) {
			// Non‑blocking send: the channel has capacity 1.
			select {
			case t.ch <- mc.now:
			default:
			}
		} else {
			remaining = append(remaining, t)
		}
	}
	mc.timers = remaining

	// Fire tickers that have passed their next tick.
	for _, t := range mc.tickers {
		for !t.next.After(mc.now) {
			select {
			case t.ch <- mc.now:
			default:
			}
			t.next = t.next.Add(t.period)
		}
	}
}

// ------------------------------------------------------------------------
// mockTimer – internal implementation of Timer for MockClock
// ------------------------------------------------------------------------

type mockTimer struct {
	clock *MockClock
	ch    chan time.Time
	when  time.Time
}

func (mt *mockTimer) C() <-chan time.Time { return mt.ch }

func (mt *mockTimer) Stop() bool {
	mt.clock.mu.Lock()
	defer mt.clock.mu.Unlock()
	for i, t := range mt.clock.timers {
		if t == mt {
			mt.clock.timers = append(mt.clock.timers[:i], mt.clock.timers[i+1:]...)
			return true
		}
	}
	return false
}

func (mt *mockTimer) Reset(d time.Duration) bool {
	mt.clock.mu.Lock()
	defer mt.clock.mu.Unlock()
	active := true
	// Check if timer is already in the list.
	found := false
	for _, t := range mt.clock.timers {
		if t == mt {
			found = true
			break
		}
	}
	if !found {
		active = false
		mt.clock.timers = append(mt.clock.timers, mt)
	}
	mt.when = mt.clock.now.Add(d)
	// Clear any pending value in the channel.
	select {
	case <-mt.ch:
	default:
	}
	return active
}

// ------------------------------------------------------------------------
// mockTicker – internal implementation of Ticker for MockClock
// ------------------------------------------------------------------------

type mockTicker struct {
	clock  *MockClock
	period time.Duration
	ch     chan time.Time
	next   time.Time
}

func (mt *mockTicker) C() <-chan time.Time { return mt.ch }

func (mt *mockTicker) Stop() {
	mt.clock.mu.Lock()
	defer mt.clock.mu.Unlock()
	for i, t := range mt.clock.tickers {
		if t == mt {
			mt.clock.tickers = append(mt.clock.tickers[:i], mt.clock.tickers[i+1:]...)
			close(mt.ch)
			return
		}
	}
}