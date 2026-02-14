// Package signalutil provides high‑level utilities for OS signal handling.
// It simplifies graceful shutdown, context cancellation on signals, and
// running functions with signal‑aware timeouts.
//
// Example:
//
//	ctx, stop := signalutil.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
//	defer stop()
//	<-ctx.Done() // blocks until SIGINT or SIGTERM
package testutils

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// --------------------------------------------------------------------
// Basic signal waiting
// --------------------------------------------------------------------

// WaitForSignal blocks until one of the specified signals is received.
// It returns the signal that was received.
//
// Example:
//
//	sig := signalutil.WaitForSignal(syscall.SIGINT, syscall.SIGTERM)
//	log.Printf("Received %v, shutting down...", sig)
func WaitForSignal(signals ...os.Signal) os.Signal {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, signals...)
	defer signal.Stop(ch)
	return <-ch
}

// WaitForInterrupt blocks until SIGINT or SIGTERM is received.
// It is a convenience wrapper for WaitForSignal.
func WaitForInterrupt() os.Signal {
	return WaitForSignal(syscall.SIGINT, syscall.SIGTERM)
}

// --------------------------------------------------------------------
// Context integration
// --------------------------------------------------------------------

// NotifyContext returns a copy of the parent context that is cancelled
// when any of the specified signals arrive. It is similar to
// signal.NotifyContext but with additional flexibility.
//
// The returned stop function unregisters the signal behaviour and closes
// the signal channel. It may be called concurrently with a signal arrival.
//
// Example:
//
//	ctx, stop := signalutil.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
//	defer stop()
//	<-ctx.Done() // blocks until signal
func NotifyContext(parent context.Context, signals ...os.Signal) (ctx context.Context, stop context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, signals...)
	go func() {
		select {
		case <-ch:
			cancel()
		case <-ctx.Done():
		}
	}()
	stop = func() {
		signal.Stop(ch)
		cancel()
	}
	return ctx, stop
}

// --------------------------------------------------------------------
// Run with signal cancellation
// --------------------------------------------------------------------

// RunWithSignalHandler runs the given function and waits for it to complete
// or for a signal to arrive. If a signal arrives before the function finishes,
// the signal is returned and the function continues running in the background.
//
// This is useful for running a long‑lived operation that should be notified
// of shutdown signals but may need cleanup time.
func RunWithSignalHandler(fn func(), signals ...os.Signal) os.Signal {
	done := make(chan struct{})
	go func() {
		fn()
		close(done)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, signals...)
	defer signal.Stop(sigCh)

	select {
	case <-done:
		return nil
	case sig := <-sigCh:
		return sig
	}
}

// RunWithTimeout runs the given function with both a timeout and signal
// cancellation. It returns an error if the timeout expires or a signal is
// received before the function completes.
//
// Example:
//
//	err := signalutil.RunWithTimeout(context.Background(), 30*time.Second, func(ctx context.Context) error {
//		// long operation that respects ctx.Done()
//	}, syscall.SIGINT, syscall.SIGTERM)
func RunWithTimeout(parent context.Context, timeout time.Duration, fn func(ctx context.Context) error, signals ...os.Signal) error {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	sigCtx, stop := NotifyContext(ctx, signals...)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- fn(sigCtx)
	}()

	select {
	case <-sigCtx.Done():
		return sigCtx.Err()
	case err := <-errCh:
		return err
	}
}

// --------------------------------------------------------------------
// Signal sets
// --------------------------------------------------------------------

// CommonSignals returns a slice of common termination signals:
// SIGINT and SIGTERM.
func CommonSignals() []os.Signal {
	return []os.Signal{syscall.SIGINT, syscall.SIGTERM}
}

// CommonSignalsWithHUP returns common termination signals plus SIGHUP.
func CommonSignalsWithHUP() []os.Signal {
	return []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP}
}

// AllSignals returns all signals that can be caught on the current platform.
// This is useful for debugging but rarely needed in production.
func AllSignals() []os.Signal {
	return []os.Signal{
		syscall.SIGABRT,
		syscall.SIGALRM,
		syscall.SIGBUS,
		syscall.SIGCHLD,
		syscall.SIGCONT,
		syscall.SIGFPE,
		syscall.SIGHUP,
		syscall.SIGILL,
		syscall.SIGINT,
		syscall.SIGIO,
		syscall.SIGIOT,
		syscall.SIGKILL, // cannot be caught, but included for completeness
		syscall.SIGPIPE,
		syscall.SIGPROF,
		syscall.SIGQUIT,
		syscall.SIGSEGV,
		syscall.SIGSTOP, // cannot be caught
		syscall.SIGSYS,
		syscall.SIGTERM,
		syscall.SIGTRAP,
		syscall.SIGTSTP,
		syscall.SIGTTIN,
		syscall.SIGTTOU,
		syscall.SIGURG,
		syscall.SIGUSR1,
		syscall.SIGUSR2,
		syscall.SIGVTALRM,
		syscall.SIGWINCH,
		syscall.SIGXCPU,
		syscall.SIGXFSZ,
	}
}
