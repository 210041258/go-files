// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"reflect"
	"sync"
)

// ------------------------------------------------------------------------
// Merge – combine multiple channels into one
// ------------------------------------------------------------------------

// Merge merges several channels of the same type into a single channel.
// It spawns one goroutine per input channel. The output channel is closed
// after all input channels have been closed.
func Merge[T any](chans ...<-chan T) <-chan T {
	out := make(chan T)
	var wg sync.WaitGroup
	wg.Add(len(chans))
	for _, ch := range chans {
		go func(c <-chan T) {
			for v := range c {
				out <- v
			}
			wg.Done()
		}(ch)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// ------------------------------------------------------------------------
// FanOut – duplicate values to multiple channels
// ------------------------------------------------------------------------

// FanOut creates n copies of the input channel. Every value sent to in
// will be sent to each of the returned channels (in the same order).
// The output channels are closed when in is closed.
func FanOut[T any](in <-chan T, n int) []<-chan T {
	if n <= 0 {
		return nil
	}
	outs := make([]chan T, n)
	for i := 0; i < n; i++ {
		outs[i] = make(chan T)
	}
	go func() {
		for v := range in {
			for _, out := range outs {
				out <- v
			}
		}
		for _, out := range outs {
			close(out)
		}
	}()
	result := make([]<-chan T, n)
	for i, ch := range outs {
		result[i] = ch
	}
	return result
}

// ------------------------------------------------------------------------
// Distribute – round‑robin distribution to multiple channels
// ------------------------------------------------------------------------

// Distribute reads values from in and sends them to the output channels
// in round‑robin order. It blocks until in is closed, then closes all outputs.
// The number of output channels is determined by the length of outs.
func Distribute[T any](in <-chan T, outs []chan<- T) {
	if len(outs) == 0 {
		return
	}
	go func() {
		defer func() {
			for _, out := range outs {
				close(out)
			}
		}()
		i := 0
		for v := range in {
			outs[i] <- v
			i = (i + 1) % len(outs)
		}
	}()
}

// ------------------------------------------------------------------------
// FirstOf – receive from the first ready channel
// ------------------------------------------------------------------------

// FirstOf receives a value from the first channel that becomes ready.
// It returns the value, the index of the channel that supplied it, and
// a boolean indicating whether a value was actually received (false if
// the channel was closed without delivering a value).
// If all channels are closed immediately, it returns zero value, -1, false.
func FirstOf[T any](chans ...<-chan T) (T, int, bool) {
	cases := make([]reflect.SelectCase, len(chans))
	for i, ch := range chans {
		cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
	}
	chosen, val, ok := reflect.Select(cases)
	if !ok {
		var zero T
		return zero, -1, false
	}
	return val.Interface().(T), chosen, true
}

// ------------------------------------------------------------------------
// AllOf – receive one value from each channel
// ------------------------------------------------------------------------

// AllOf attempts to read exactly one value from each input channel.
// It returns a slice of values in the same order as the input channels,
// and a boolean indicating whether all channels delivered a value before
// any of them closed. If any channel is closed without sending, it returns
// nil, false.
func AllOf[T any](chans ...<-chan T) ([]T, bool) {
	result := make([]T, len(chans))
	for i, ch := range chans {
		v, ok := <-ch
		if !ok {
			return nil, false
		}
		result[i] = v
	}
	return result, true
}