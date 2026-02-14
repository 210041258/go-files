// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"reflect"
	"time"
)

// ------------------------------------------------------------------------
// Basic channel operations with timeouts
// ------------------------------------------------------------------------

// ReadWithTimeout attempts to read a value from channel ch within the given timeout.
// If a value is received, it returns the value and true.
// If the timeout elapses or the channel is closed, it returns the zero value and false.
func ReadWithTimeout[T any](ch <-chan T, timeout time.Duration) (T, bool) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case val, ok := <-ch:
		return val, ok
	case <-timer.C:
		var zero T
		return zero, false
	}
}

// WriteWithTimeout attempts to send val to channel ch within the given timeout.
// It returns true if the send succeeded, false if the timeout elapsed or the channel closed.
func WriteWithTimeout[T any](ch chan<- T, val T, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case ch <- val:
		return true
	case <-timer.C:
		return false
	}
}

// ------------------------------------------------------------------------
// Draining and collecting
// ------------------------------------------------------------------------

// Drain reads all values from a channel until it is closed.
// It returns a slice of all received values. If the channel never closes,
// Drain will block forever. For a bounded wait, use CollectWithTimeout.
func Drain[T any](ch <-chan T) []T {
	var result []T
	for v := range ch {
		result = append(result, v)
	}
	return result
}

// Collect reads values from a channel for at most the given duration,
// or until the channel is closed (whichever comes first).
// It returns a slice of all values received during that window.
func Collect[T any](ch <-chan T, d time.Duration) []T {
	deadline := time.NewTimer(d)
	defer deadline.Stop()
	var result []T
	for {
		select {
		case v, ok := <-ch:
			if !ok {
				return result // channel closed
			}
			result = append(result, v)
		case <-deadline.C:
			return result // time's up
		}
	}
}

// ------------------------------------------------------------------------
// Channel state inspection
// ------------------------------------------------------------------------

// IsClosed checks whether a channel is closed without blocking.
// It uses a non‑blocking receive and returns true if the channel is closed.
// Note: This only works for channels that have been closed; an active channel
// that has no values ready will return false.
func IsClosed[T any](ch <-chan T) bool {
	select {
	case _, ok := <-ch:
		return !ok
	default:
		return false
	}
}

// WaitForClosed waits for the channel to be closed, or until the timeout elapses.
// It returns true if the channel was closed before the timeout, false otherwise.
func WaitForClosed[T any](ch <-chan T, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return true
			}
		case <-timer.C:
			return false
		}
	}
}

// ------------------------------------------------------------------------
// ChannelRecorder – records all values sent/received on a channel
// ------------------------------------------------------------------------

// ChannelRecorder wraps a channel and records every value that passes through it.
// It is useful for verifying that expected values were sent, or for capturing
// output in tests. The recorder can be used as a drop‑in replacement for a
// channel when you need to inspect the data flow.
type ChannelRecorder[T any] struct {
	// In is the send‑only side of the recorder. Send to this channel to record.
	In chan<- T
	// Out is the receive‑only side of the recorder. Receive from this channel
	// to get the original values (after recording).
	Out <-chan T
	// Recorded returns a copy of all values that have passed through the channel
	// (in order). The channel must be closed before calling Recorded, or you may
	// get an incomplete snapshot. For ongoing recording, use Snapshot.
	Recorded func() []T
	// Snapshot returns a copy of the values recorded so far without closing.
	Snapshot func() []T
	// stop is used internally to signal the goroutine to stop when the channel is closed.
	stop chan struct{}
}

// NewChannelRecorder creates a new ChannelRecorder that wraps an unbuffered channel.
// The returned recorder's In and Out channels are the send and receive ends.
// The recorder runs a goroutine that forwards values while recording them.
// The caller must close the original channel (by closing the In end) when done.
func NewChannelRecorder[T any]() *ChannelRecorder[T] {
	in := make(chan T)
	out := make(chan T)
	stop := make(chan struct{})
	var recorded []T
	go func() {
		defer close(out)
		for {
			select {
			case val, ok := <-in:
				if !ok {
					return // input closed, we're done
				}
				recorded = append(recorded, val)
				select {
				case out <- val:
				case <-stop:
					return
				}
			case <-stop:
				return
			}
		}
	}()
	return &ChannelRecorder[T]{
		In:  in,
		Out: out,
		Recorded: func() []T {
			// Create a copy.
			return append([]T(nil), recorded...)
		},
		Snapshot: func() []T {
			// Create a copy.
			return append([]T(nil), recorded...)
		},
		stop: stop,
	}
}

// Close stops the recorder's goroutine and closes the In channel.
// It is safe to call multiple times.
func (cr *ChannelRecorder[T]) Close() {
	close(cr.stop)
	close(cr.In)
}

// ------------------------------------------------------------------------
// Re‑export from generic.go for convenience
// ------------------------------------------------------------------------

// MergeChannels merges multiple channels of the same type into one.
// The returned channel is closed when all input channels are closed.
func MergeChannels[T any](chans ...<-chan T) <-chan T {
	return mergeChannelsGeneric(chans...)
}

// mergeChannelsGeneric is the internal implementation (copied from generic.go).
// We duplicate it here to avoid import cycles.
func mergeChannelsGeneric[T any](chans ...<-chan T) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		if len(chans) == 0 {
			return
		}
		cases := make([]reflect.SelectCase, len(chans))
		for i, ch := range chans {
			cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
		}
		for len(cases) > 0 {
			chosen, val, ok := reflect.Select(cases)
			if !ok {
				cases = append(cases[:chosen], cases[chosen+1:]...)
				continue
			}
			out <- val.Interface().(T)
		}
	}()
	return out
}

// Take reads up to n values from ch and returns them.
// It blocks until n values are received or ch is closed.
func Take[T any](ch <-chan T, n int) []T {
	result := make([]T, 0, n)
	for i := 0; i < n; i++ {
		v, ok := <-ch
		if !ok {
			break
		}
		result = append(result, v)
	}
	return result
}

// First returns the first value received from ch, or the zero value and false
// if the channel is closed without sending.
func First[T any](ch <-chan T) (T, bool) {
	v, ok := <-ch
	return v, ok
}