package testutils

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Handler defines the interface for handling accepted connections.
type Handler interface {
	Handle(ctx context.Context, conn net.Conn)
}

// HandlerFunc is an adapter to allow the use of ordinary functions as handlers.
type HandlerFunc func(ctx context.Context, conn net.Conn)

// Handle calls f(ctx, conn).
func (f HandlerFunc) Handle(ctx context.Context, conn net.Conn) {
	f(ctx, conn)
}

// Config holds the configuration for a Listener server.
type Config struct {
	// Addr is the TCP address to listen on, in form "host:port".
	Addr string
	// TLSConfig, if non-nil, is used to create a TLS listener.
	TLSConfig *tls.Config
	// KeepAlive enables TCP keepalive on accepted connections.
	KeepAlive bool
	// KeepAlivePeriod sets the keepalive period; zero uses default.
	KeepAlivePeriod time.Duration
	// ReadTimeout sets the read deadline on accepted connections.
	ReadTimeout time.Duration
	// WriteTimeout sets the write deadline on accepted connections.
	WriteTimeout time.Duration
	// MaxConcurrent limits the number of concurrent connections.
	// Zero means no limit.
	MaxConcurrent int32
	// MaxMessageSize limits the maximum message size (optional, application‑specific).
	MaxMessageSize int64
}

// Server represents a generic TCP server that listens and handles connections.
type Server struct {
	cfg      Config
	handler  Handler
	listener net.Listener
	wg       sync.WaitGroup
	conns    map[*trackedConn]struct{}
	connMu   sync.Mutex
	active   int32
	quit     chan struct{}
	done     chan struct{}
}

// trackedConn wraps a net.Conn with deadlines and server state.
type trackedConn struct {
	net.Conn
	server    *Server
	createdAt time.Time
}

// Option configures a Server.
type Option func(*Server)

// New creates a new Server with the given address and handler.
// It applies the provided options.
func New(addr string, handler Handler, opts ...Option) *Server {
	s := &Server{
		cfg: Config{
			Addr: addr,
		},
		handler: handler,
		conns:   make(map[*trackedConn]struct{}),
		quit:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// WithTLS sets the TLS configuration.
func WithTLS(cfg *tls.Config) Option {
	return func(s *Server) {
		s.cfg.TLSConfig = cfg
	}
}

// WithKeepAlive enables TCP keepalive with optional period.
func WithKeepAlive(period time.Duration) Option {
	return func(s *Server) {
		s.cfg.KeepAlive = true
		s.cfg.KeepAlivePeriod = period
	}
}

// WithTimeouts sets read and write deadlines.
func WithTimeouts(read, write time.Duration) Option {
	return func(s *Server) {
		s.cfg.ReadTimeout = read
		s.cfg.WriteTimeout = write
	}
}

// WithMaxConcurrent sets the maximum number of concurrent connections.
func WithMaxConcurrent(n int32) Option {
	return func(s *Server) {
		s.cfg.MaxConcurrent = n
	}
}

// WithConfig replaces the entire configuration.
func WithConfig(cfg Config) Option {
	return func(s *Server) {
		s.cfg = cfg
	}
}

// Serve starts the listener and begins accepting connections.
// It blocks until the server is stopped or a fatal error occurs.
func (s *Server) Serve() error {
	// Create listener.
	l, err := s.createListener()
	if err != nil {
		return err
	}
	s.listener = l
	defer s.closeListener()

	go s.acceptLoop()

	<-s.done
	return nil
}

// createListener creates a TCP listener, optionally with TLS.
func (s *Server) createListener() (net.Listener, error) {
	l, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return nil, err
	}
	if s.cfg.TLSConfig != nil {
		l = tls.NewListener(l, s.cfg.TLSConfig)
	}
	return l, nil
}

// acceptLoop accepts incoming connections and spawns handlers.
func (s *Server) acceptLoop() {
	defer close(s.done)

	for {
		select {
		case <-s.quit:
			return
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			// Check if the listener was closed.
			select {
			case <-s.quit:
				return
			default:
				// Otherwise, log error and continue.
				// In production, use proper logging.
				continue
			}
		}

		// Apply connection limits.
		if s.cfg.MaxConcurrent > 0 {
			if atomic.LoadInt32(&s.active) >= s.cfg.MaxConcurrent {
				conn.Close()
				continue
			}
		}

		// Configure TCP keepalive.
		if s.cfg.KeepAlive {
			if tcpConn, ok := conn.(*net.TCPConn); ok {
				tcpConn.SetKeepAlive(true)
				if s.cfg.KeepAlivePeriod > 0 {
					tcpConn.SetKeepAlivePeriod(s.cfg.KeepAlivePeriod)
				}
			}
		}

		tc := &trackedConn{
			Conn:      conn,
			server:    s,
			createdAt: time.Now(),
		}
		s.track(tc)
		s.wg.Add(1)
		go s.handleConn(tc)
	}
}

// handleConn calls the user handler with context and connection.
func (s *Server) handleConn(tc *trackedConn) {
	defer s.wg.Done()
	defer s.untrack(tc)
	defer tc.Close()

	// Set initial deadlines if configured.
	if s.cfg.ReadTimeout > 0 {
		tc.SetReadDeadline(time.Now().Add(s.cfg.ReadTimeout))
	}
	if s.cfg.WriteTimeout > 0 {
		tc.SetWriteDeadline(time.Now().Add(s.cfg.WriteTimeout))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Monitor server shutdown.
	go func() {
		select {
		case <-s.quit:
			cancel()
		case <-ctx.Done():
		}
	}()

	atomic.AddInt32(&s.active, 1)
	defer atomic.AddInt32(&s.active, -1)

	s.handler.Handle(ctx, tc)
}

// track adds a connection to the server's connection map.
func (s *Server) track(tc *trackedConn) {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	s.conns[tc] = struct{}{}
}

// untrack removes a connection from the map.
func (s *Server) untrack(tc *trackedConn) {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	delete(s.conns, tc)
}

// closeListener closes the listener.
func (s *Server) closeListener() {
	if s.listener != nil {
		s.listener.Close()
	}
}

// Stop immediately closes the listener and all active connections.
func (s *Server) Stop() {
	close(s.quit)
	s.closeListener()
	s.connMu.Lock()
	for tc := range s.conns {
		tc.Close()
	}
	s.connMu.Unlock()
	s.wg.Wait()
}

// GracefulStop stops accepting new connections and waits for active
// handlers to finish. It can be canceled via the context.
func (s *Server) GracefulStop(ctx context.Context) error {
	close(s.quit)
	s.closeListener()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		// Force close remaining connections.
		s.connMu.Lock()
		for tc := range s.conns {
			tc.Close()
		}
		s.connMu.Unlock()
		<-s.wg.WaitChan() // custom WaitChan? We can just wait again.
		return ctx.Err()
	case <-done:
		return nil
	}
}

// Addr returns the listener's network address.
func (s *Server) Addr() net.Addr {
	if s.listener != nil {
		return s.listener.Addr()
	}
	return nil
}

// ActiveConnections returns the current number of active connections.
func (s *Server) ActiveConnections() int {
	return int(atomic.LoadInt32(&s.active))
}

// Example usage (commented out):
//
// func main() {
//     handler := listener.HandlerFunc(func(ctx context.Context, conn net.Conn) {
//         defer conn.Close()
//         io.Copy(conn, conn) // echo
//     })
//
//     srv := listener.New(":8080", handler,
//         listener.WithKeepAlive(30*time.Second),
//         listener.WithTimeouts(5*time.Minute, 5*time.Minute),
//         listener.WithMaxConcurrent(100),
//     )
//
//     // Run in goroutine
//     go func() {
//         if err := srv.Serve(); err != nil {
//             log.Fatal(err)
//         }
//     }()
//
//     // Wait for signal
//     sig := make(chan os.Signal, 1)
//     signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
//     <-sig
//
//     ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
//     defer cancel()
//     if err := srv.GracefulStop(ctx); err != nil {
//         log.Printf("graceful shutdown error: %v", err)
//     }
// }