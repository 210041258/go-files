// Package socket provides utilities for creating and configuring network sockets.
// It offers a convenient way to set common socket options and create listeners
// and connections with consistent configuration.
package testutils

import (
	"context"
	"net"
	"syscall"
	"time"
)

// ----------------------------------------------------------------------
// Common socket options
// ----------------------------------------------------------------------

// Config holds socket configuration that can be applied to a listener or connection.
type Config struct {
	// KeepAlive enables TCP keep‑alive and sets the keep‑alive period.
	KeepAlive time.Duration

	// NoDelay enables TCP_NODELAY (disables Nagle's algorithm).
	NoDelay bool

	// Linger sets the SO_LINGER option. If negative, lingering is disabled.
	// If zero, the connection is aborted on close. If positive, that many
	// seconds are allowed for pending data to be sent.
	Linger int

	// ReuseAddr enables SO_REUSEADDR (allows binding to an address in TIME_WAIT).
	// Only effective on listeners.
	ReuseAddr bool

	// ReusePort enables SO_REUSEPORT (allows multiple processes to bind the same port).
	// Only effective on listeners and supported on some platforms (Linux, BSD).
	ReusePort bool

	// SendBuf sets the socket send buffer size (SO_SNDBUF).
	SendBuf int

	// RecvBuf sets the socket receive buffer size (SO_RCVBUF).
	RecvBuf int

	// ReadTimeout sets the read deadline on accepted connections.
	ReadTimeout time.Duration

	// WriteTimeout sets the write deadline on accepted connections.
	WriteTimeout time.Duration
}

// DefaultConfig returns a default configuration with conservative settings.
func DefaultConfig() *Config {
	return &Config{
		KeepAlive: 0,      // disabled
		NoDelay:   false,  // disabled (Nagle enabled)
		Linger:    -1,     // disabled
		ReuseAddr: false,
		ReusePort: false,
		SendBuf:   0, // use system default
		RecvBuf:   0, // use system default
	}
}

// ----------------------------------------------------------------------
// TCP helpers
// ----------------------------------------------------------------------

// ListenTCP creates a TCP listener with the given address and applies the
// configuration to the listener's socket.
func ListenTCP(addr string, cfg *Config) (*net.TCPListener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if err := applyToListener(l, cfg); err != nil {
		l.Close()
		return nil, err
	}
	return l.(*net.TCPListener), nil
}

// DialTCP connects to a TCP server with the given address and applies the
// configuration to the connection.
func DialTCP(ctx context.Context, addr string, cfg *Config) (*net.TCPConn, error) {
	d := net.Dialer{}
	if cfg != nil {
		if cfg.KeepAlive > 0 {
			d.KeepAlive = cfg.KeepAlive
		}
	}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if err := applyToConn(conn, cfg); err != nil {
		conn.Close()
		return nil, err
	}
	return conn.(*net.TCPConn), nil
}

// ----------------------------------------------------------------------
// Unix domain socket helpers
// ----------------------------------------------------------------------

// ListenUnix creates a Unix domain socket listener with the given path
// and applies the configuration.
func ListenUnix(path string, cfg *Config) (*net.UnixListener, error) {
	// Remove any existing socket file.
	syscall.Unlink(path)
	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if err := applyToListener(l, cfg); err != nil {
		l.Close()
		return nil, err
	}
	return l.(*net.UnixListener), nil
}

// DialUnix connects to a Unix domain socket.
func DialUnix(ctx context.Context, path string, cfg *Config) (*net.UnixConn, error) {
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "unix", path)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if err := applyToConn(conn, cfg); err != nil {
		conn.Close()
		return nil, err
	}
	return conn.(*net.UnixConn), nil
}

// ----------------------------------------------------------------------
// Applying options (internal)
// ----------------------------------------------------------------------

// applyToListener applies configuration options to a listener.
func applyToListener(l net.Listener, cfg *Config) error {
	// Extract the underlying raw connection.
	raw, err := getRawConn(l)
	if err != nil {
		return err
	}
	return applyToRawConn(raw, cfg)
}

// applyToConn applies configuration options to a connection.
func applyToConn(conn net.Conn, cfg *Config) error {
	// Apply TCP specific options.
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		if cfg.NoDelay {
			if err := tcpConn.SetNoDelay(true); err != nil {
				return err
			}
		}
		if cfg.KeepAlive > 0 {
			if err := tcpConn.SetKeepAlive(true); err != nil {
				return err
			}
			if err := tcpConn.SetKeepAlivePeriod(cfg.KeepAlive); err != nil {
				return err
			}
		}
		if cfg.Linger >= 0 {
			if err := tcpConn.SetLinger(cfg.Linger); err != nil {
				return err
			}
		}
	}
	// Apply timeouts.
	if cfg.ReadTimeout > 0 {
		if err := conn.SetReadDeadline(time.Now().Add(cfg.ReadTimeout)); err != nil {
			return err
		}
	}
	if cfg.WriteTimeout > 0 {
		if err := conn.SetWriteDeadline(time.Now().Add(cfg.WriteTimeout)); err != nil {
			return err
		}
	}
	// Socket options that require raw syscalls.
	raw, err := getRawConn(conn)
	if err != nil {
		return err
	}
	return applyToRawConn(raw, cfg)
}

// applyToRawConn applies socket options via raw control.
func applyToRawConn(rc syscall.RawConn, cfg *Config) error {
	if cfg == nil {
		return nil
	}
	var opErr error
	err := rc.Control(func(fd uintptr) {
		// ReuseAddr / ReusePort
		if cfg.ReuseAddr {
			if err := setSockOptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
				opErr = err
				return
			}
		}
		if cfg.ReusePort {
			if err := setReusePort(int(fd), 1); err != nil {
				opErr = err
				return
			}
		}
		// Send/Receive buffer sizes.
		if cfg.SendBuf > 0 {
			if err := setSockOptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_SNDBUF, cfg.SendBuf); err != nil {
				opErr = err
				return
			}
		}
		if cfg.RecvBuf > 0 {
			if err := setSockOptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF, cfg.RecvBuf); err != nil {
				opErr = err
				return
			}
		}
	})
	if err != nil {
		return err
	}
	return opErr
}

// getRawConn extracts a syscall.RawConn from a net.Conn or net.Listener.
func getRawConn(any interface{}) (syscall.RawConn, error) {
	type rawConn interface {
		SyscallConn() (syscall.RawConn, error)
	}
	if rc, ok := any.(rawConn); ok {
		return rc.SyscallConn()
	}
	return nil, syscall.EINVAL
}

// ----------------------------------------------------------------------
// Platform‑specific socket option helpers
// ----------------------------------------------------------------------

// setSockOptInt wraps syscall.SetsockoptInt in a way that handles platform differences.
func setSockOptInt(fd, level, opt, value int) error {
	return syscall.SetsockoptInt(fd, level, opt, value)
}

// setReusePort attempts to set SO_REUSEPORT.
// It is defined as a variable so that it can be overridden on platforms that do not support it.
var setReusePort = func(fd, value int) error {
	// SO_REUSEPORT is not defined on all platforms; we try the common values.
	// On Linux, it's 15.
	const SO_REUSEPORT = 15
	err := setSockOptInt(fd, syscall.SOL_SOCKET, SO_REUSEPORT, value)
	if err != nil {
		// On some BSD systems, it's a different constant; try alternative.
		const SO_REUSEPORT_BSD = 0x200 // FreeBSD, etc.
		err = setSockOptInt(fd, syscall.SOL_SOCKET, SO_REUSEPORT_BSD, value)
	}
	return err
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     cfg := &socket.Config{
//         KeepAlive: 30 * time.Second,
//         NoDelay:   true,
//         ReuseAddr: true,
//     }
//     l, err := socket.ListenTCP(":8080", cfg)
//     if err != nil {
//         log.Fatal(err)
//     }
//     defer l.Close()
//
//     conn, err := l.Accept()
//     if err != nil {
//         log.Fatal(err)
//     }
//     // conn is a *net.TCPConn with keep‑alive and no delay already set.
// }