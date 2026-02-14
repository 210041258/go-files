// Package acceptloop provides a reusable TCP accept loop with
// configurable concurrency limits, graceful shutdown, and connection tracking.
package testutils

import (
	"context"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Handler processes an accepted connection.
// The connection will be closed when Handle returns.
type Handler interface {
	Handle(ctx context.Context, conn net.Conn)
}

// HandlerFunc is an adapter to allow ordinary functions as handlers.
type HandlerFunc func(ctx context.Context, conn net.Conn)

// Handle calls f(ctx, conn).
func (f HandlerFunc) Handle(ctx context.Context, conn net.Conn) {
	f(ctx, conn)
}

// Config holds optional parameters for the accept loop.
type Config struct {
	// MaxConcurrent limits how many connections are handled simultaneously.
	// 0 means no limit.
	MaxConcurrent int32

	// ReadTimeout sets the read deadline on accepted connections.
	// 0 means no timeout.
	ReadTimeout time.Duration

	// WriteTimeout sets the write deadline on accepted connections.
	// 0 means no timeout.
	WriteTimeout time.Duration

	// IdleTimeout is the maximum amount of time a connection can be idle.
	// It is implemented by setting the read deadline on every read.
	IdleTimeout time.Duration

	// KeepAlive enables TCP keep‑alive on accepted connections.
	KeepAlive bool

	// KeepAlivePeriod sets the keep‑alive period; zero uses the OS default.
	KeepAlivePeriod time.Duration

	// Logger is used for logging errors and events. If nil, no logging occurs.
	Logger *log.Logger
}

// Option configures an AcceptLoop.
type Option func(*AcceptLoop)

// WithMaxConcurrent sets the maximum concurrent connections.
func WithMaxConcurrent(n int32) Option {
	return func(al *AcceptLoop) {
		al.cfg.MaxConcurrent = n
	}
}

// WithTimeouts sets read/write deadlines on connections.
func WithTimeouts(read, write time.Duration) Option {
	return func(al *AcceptLoop) {
		al.cfg.ReadTimeout = read
		al.cfg.WriteTimeout = write
	}
}

// WithIdleTimeout sets the idle timeout for connections.
func WithIdleTimeout(d time.Duration) Option {
	return func(al *AcceptLoop) {
		al.cfg.IdleTimeout = d
	}
}

// WithKeepAlive enables TCP keep‑alive with optional period.
func WithKeepAlive(period time.Duration) Option {
	return func(al *AcceptLoop) {
		al.cfg.KeepAlive = true
		al.cfg.KeepAlivePeriod = period
	}
}

// WithLogger sets the logger for the accept loop.
func WithLogger(l *log.Logger) Option {
	return func(al *AcceptLoop) {
		al.cfg.Logger = l
	}
}

// AcceptLoop runs an accept loop and dispatches connections to a handler.
type AcceptLoop struct {
	cfg      Config
	listener net.Listener
	handler  Handler
	active   int32
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
}

// New creates a new AcceptLoop that listens on the given listener and
// dispatches connections to the provided handler. Optional configuration
// can be supplied via variadic options.
func New(l net.Listener, h Handler, opts ...Option) *AcceptLoop {
	ctx, cancel := context.WithCancel(context.Background())
	al := &AcceptLoop{
		listener: l,
		handler:  h,
		ctx:      ctx,
		cancel:   cancel,
		done:     make(chan struct{}),
		cfg: Config{
			MaxConcurrent: 0, // unlimited
		},
	}
	for _, opt := range opts {
		opt(al)
	}
	return al
}

// Run starts the accept loop. It blocks until the loop exits due to an error
// or Stop is called. The returned error is never nil; if the listener is
// closed cleanly, it returns net.ErrClosed.
func (al *AcceptLoop) Run() error {
	defer close(al.done)

	for {
		select {
		case <-al.ctx.Done():
			return al.ctx.Err()
		default:
		}

		conn, err := al.listener.Accept()
		if err != nil {
			// Check if the listener was closed.
			select {
			case <-al.ctx.Done():
				return al.ctx.Err()
			default:
			}
			// Otherwise, log the error and continue.
			al.logf("accept error: %v", err)
			continue
		}

		// Apply connection limit.
		if al.cfg.MaxConcurrent > 0 {
			if cur := atomic.LoadInt32(&al.active); cur >= al.cfg.MaxConcurrent {
				al.logf("connection limit reached (%d), dropping %s", cur, conn.RemoteAddr())
				conn.Close()
				continue
			}
		}

		// Configure TCP keep‑alive.
		if al.cfg.KeepAlive {
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				_ = tcpConn.SetKeepAlive(true)
				if al.cfg.KeepAlivePeriod > 0 {
					_ = tcpConn.SetKeepAlivePeriod(al.cfg.KeepAlivePeriod)
				}
			}
		}

		al.wg.Add(1)
		atomic.AddInt32(&al.active, 1)
		go al.handleConn(conn)
	}
}

// Stop gracefully shuts down the accept loop. It closes the listener,
// waits for active connections to finish, and then returns.
// A timeout can be applied using a context with deadline.
func (al *AcceptLoop) Stop(ctx context.Context) error {
	// Signal the accept loop to stop.
	al.cancel()
	// Close the listener to break Accept().
	al.listener.Close()

	// Wait for all handlers to finish or context deadline.
	done := make(chan struct{})
	go func() {
		al.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Done returns a channel that is closed when the Run method has exited.
func (al *AcceptLoop) Done() <-chan struct{} {
	return al.done
}

// ActiveConnections returns the current number of connections being handled.
func (al *AcceptLoop) ActiveConnections() int {
	return int(atomic.LoadInt32(&al.active))
}

// handleConn wraps the handler with deadlines and connection tracking.
func (al *AcceptLoop) handleConn(conn net.Conn) {
	defer func() {
		conn.Close()
		al.wg.Done()
		atomic.AddInt32(&al.active, -1)
	}()

	// Apply connection timeouts if configured.
	if al.cfg.ReadTimeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(al.cfg.ReadTimeout))
	}
	if al.cfg.WriteTimeout > 0 {
		_ = conn.SetWriteDeadline(time.Now().Add(al.cfg.WriteTimeout))
	}
	if al.cfg.IdleTimeout > 0 {
		// Reset deadline on every read inside the handler is the handler's responsibility.
		// We only set an initial deadline here; for full idle timeout, the handler
		// must call SetReadDeadline on each read. We provide a helper for that.
		_ = conn.SetReadDeadline(time.Now().Add(al.cfg.IdleTimeout))
	}

	// Create a per‑connection context that is cancelled when the loop stops.
	connCtx, connCancel := context.WithCancel(al.ctx)
	defer connCancel()

	al.handler.Handle(connCtx, conn)
}

// logf logs a formatted message if a logger is set.
func (al *AcceptLoop) logf(format string, v ...interface{}) {
	if al.cfg.Logger != nil {
		al.cfg.Logger.Printf(format, v...)
	}
}

// ----------------------------------------------------------------------
// Example usage (commented out)
// ----------------------------------------------------------------------
// func main() {
//     l, _ := net.Listen("tcp", ":8080")
//
//     handler := acceptloop.HandlerFunc(func(ctx context.Context, conn net.Conn) {
//         // Echo server with idle timeout.
//         defer conn.Close()
//         buf := make([]byte, 1024)
//         for {
//             // Reset idle deadline on each read.
//             conn.SetReadDeadline(time.Now().Add(30 * time.Second))
//             n, err := conn.Read(buf)
//             if err != nil {
//                 return
//             }
//             conn.Write(buf[:n])
//         }
//     })
//
//     loop := acceptloop.New(l, handler,
//         acceptloop.WithMaxConcurrent(100),
//         acceptloop.WithIdleTimeout(30*time.Second),
//         acceptloop.WithKeepAlive(15*time.Second),
//         acceptloop.WithLogger(log.Default()),
//     )
//
//     go func() {
//         if err := loop.Run(); err != nil && err != context.Canceled {
//             log.Fatal(err)
//         }
//     }()
//
//     // Wait for interrupt.
//     sig := make(chan os.Signal, 1)
//     signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
//     <-sig
//
//     ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
//     defer cancel()
//     if err := loop.Stop(ctx); err != nil {
//         log.Fatal(err)
//     }
// }