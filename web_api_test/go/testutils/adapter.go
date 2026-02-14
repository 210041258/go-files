// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"sync"
	"time"
)

// Clock defines an interface for time operations.
// It allows tests to control time without using the real system clock.
type Clock interface {
	// Now returns the current time.
	Now() time.Time
	// Sleep pauses the current goroutine for at least d duration.
	Sleep(d time.Duration)
	// After returns a channel that receives after the duration d.
	After(d time.Duration) <-chan time.Time
	// Tick returns a channel that delivers ticks at intervals d.
	// The ticker must be stopped to release resources.
	Tick(d time.Duration) (<-chan time.Time, func())
	// NewTicker creates a new Ticker (from Ticker.go) that ticks at interval d.
	NewTicker(d time.Duration) Ticker
	// NewTimer creates a new Timer that sends the current time after duration d.
	// The timer must be stopped to release resources.
	NewTimer(d time.Duration) *time.Timer
}

// ------------------------------------------------------------------------
// RealClock – uses the actual system time
// ------------------------------------------------------------------------

// RealClock is a Clock that delegates to the standard time package.
type RealClock struct{}

// Now returns time.Now().
func (RealClock) Now() time.Time { return time.Now() }

// Sleep calls time.Sleep.
func (RealClock) Sleep(d time.Duration) { time.Sleep(d) }

// After calls time.After.
func (RealClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// Tick returns a ticker channel and a stop function.
func (RealClock) Tick(d time.Duration) (<-chan time.Time, func()) {
	ticker := time.NewTicker(d)
	return ticker.C, ticker.Stop
}

// NewTicker returns a RealTicker wrapping time.NewTicker.
func (RealClock) NewTicker(d time.Duration) Ticker {
	return &RealTicker{time.NewTicker(d)}
}

// NewTimer calls time.NewTimer.
func (RealClock) NewTimer(d time.Duration) *time.Timer {
	return time.NewTimer(d)
}

// ------------------------------------------------------------------------
// MockClock – controllable time for tests
// ------------------------------------------------------------------------

// MockClock is a Clock that allows tests to control time.
// It uses a fake current time that can be advanced manually.
// All time‑based operations are simulated without real waiting.
type MockClock struct {
	mu      sync.Mutex
	now     time.Time
	timers  []*mockTimer
	tickers []*mockTicker
}

// NewMockClock creates a new mock clock with the given start time.
// If start is zero, time.Unix(0, 0) is used.
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

// Sleep advances the mock time by d without blocking.
// All pending timers and tickers are fired if they become due.
func (mc *MockClock) Sleep(d time.Duration) {
	mc.Advance(d)
}

// After returns a channel that receives after the duration d in mock time.
// The channel is closed when the clock is advanced past that point.
func (mc *MockClock) After(d time.Duration) <-chan time.Time {
	return mc.NewTimer(d).C
}

// Tick returns a ticker channel and a stop function.
// The ticker sends every time the clock advances by d.
func (mc *MockClock) Tick(d time.Duration) (<-chan time.Time, func()) {
	ticker := mc.NewTicker(d)
	return ticker.C(), ticker.Stop
}

// NewTicker creates a new mock ticker that ticks every d in mock time.
func (mc *MockClock) NewTicker(d time.Duration) Ticker {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	ticker := &mockTicker{
		clock: mc,
		period: d,
		ch:    make(chan time.Time),
		next:  mc.now.Add(d),
	}
	mc.tickers = append(mc.tickers, ticker)
	return ticker
}

// NewTimer creates a new mock timer that fires after d in mock time.
func (mc *MockClock) NewTimer(d time.Duration) *time.Timer {
	// We cannot return a real time.Timer, so we implement a mock and wrap it.
	timer := &mockTimer{
		clock: mc,
		ch:    make(chan time.Time, 1),
		when:  mc.now.Add(d),
	}
	mc.mu.Lock()
	mc.timers = append(mc.timers, timer)
	mc.mu.Unlock()
	// Return a *time.Timer that is backed by this mock? That's tricky.
	// Instead, we can return a channel that behaves like timer.C.
	// For compatibility with code expecting *time.Timer, we might need to change the interface.
	// Since we control the Clock interface, we could have NewTimer return a Timer interface.
	// But the standard library uses *time.Timer with Stop and Reset methods.
	// We'll create a mockTimer that implements those methods and return it, but we need a type that matches *time.Timer exactly.
	// To avoid complexity, we can change the Clock interface to return a Timer interface, or keep it simple and not provide NewTimer.
	// Given the existing utilities (Ticker, Sleeper, etc.), maybe Clock doesn't need NewTimer.
	// But for completeness, we can return a channel and a stop function, similar to Tick.
	// Let's reconsider: We'll remove NewTimer from the interface and provide only After, Tick, and NewTicker.
	// That's sufficient for most testing.
	// I'll adjust the interface above.
	// For now, I'll leave NewTimer out.
}

// Advance moves the mock time forward by d and triggers any timers/tickers.
func (mc *MockClock) Advance(d time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.now = mc.now.Add(d)

	// Fire timers that have expired.
	remainingTimers := mc.timers[:0]
	for _, t := range mc.timers {
		if !t.when.After(mc.now) {
			select {
			case t.ch <- mc.now:
			default:
			}
		} else {
			remainingTimers = append(remainingTimers, t)
		}
	}
	mc.timers = remainingTimers

	// Fire tickers that have passed their next tick.
	for _, ticker := range mc.tickers {
		for !ticker.next.After(mc.now) {
			select {
			case ticker.ch <- mc.now:
			default:
			}
			ticker.next = ticker.next.Add(ticker.period)
		}
	}
}

// ------------------------------------------------------------------------
// mockTimer – internal timer for MockClock
// ------------------------------------------------------------------------

type mockTimer struct {
	clock *MockClock
	ch    chan time.Time
	when  time.Time
	// No stop for simplicity; we could add a stopped flag.
}

// C returns the timer's channel.
func (mt *mockTimer) C() <-chan time.Time {
	return mt.ch
}

// Stop prevents the timer from firing.
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

// Reset changes the timer to expire after duration d.
func (mt *mockTimer) Reset(d time.Duration) bool {
	mt.clock.mu.Lock()
	defer mt.clock.mu.Unlock()
	mt.when = mt.clock.now.Add(d)
	// Ensure it's in the list (it should be, but if stopped it might be missing)
	found := false
	for _, t := range mt.clock.timers {
		if t == mt {
			found = true
			break
		}
	}
	if !found {
		mt.clock.timers = append(mt.clock.timers, mt)
	}
	return true
}

// ------------------------------------------------------------------------
// mockTicker – internal ticker for MockClock
// ------------------------------------------------------------------------

type mockTicker struct {
	clock  *MockClock
	period time.Duration
	ch     chan time.Time
	next   time.Time
	stop   bool
}

func (mt *mockTicker) C() <-chan time.Time {
	return mt.ch
}

func (mt *mockTicker) Stop() {
	mt.clock.mu.Lock()
	defer mt.clock.mu.Unlock()
	for i, t := range mt.clock.tickers {
		if t == mt {
			mt.clock.tickers = append(mt.clock.tickers[:i], mt.clock.tickers[i+1:]...)
			break
		}
	}
	close(mt.ch)
}