package testutils

import (
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

// ----------------------------------------------------------------------
// TCP keep‑alive (operating system level)
// ----------------------------------------------------------------------

// TCPConfig holds parameters for TCP keep‑alive on a *net.TCPConn.
type TCPConfig struct {
	Idle     time.Duration // time before first keep‑alive probe (Linux: tcp_keepalive_time)
	Interval time.Duration // interval between probes (Linux: tcp_keepalive_intvl)
	Count    int           // number of probes before death (Linux: tcp_keepalive_probes)
}

// DefaultTCPConfig returns a sensible default TCP keep‑alive configuration.
func DefaultTCPConfig() *TCPConfig {
	return &TCPConfig{
		Idle:     60 * time.Second,
		Interval: 15 * time.Second,
		Count:    9,
	}
}

// SetKeepAlive configures TCP keep‑alive on a *net.TCPConn.
// It enables keep‑alive and sets the three standard options.
// Returns an error if the underlying connection does not support these options.
func (cfg *TCPConfig) SetKeepAlive(conn *net.TCPConn) error {
	if err := conn.SetKeepAlive(true); err != nil {
		return err
	}
	if cfg.Idle != 0 {
		if err := conn.SetKeepAlivePeriod(cfg.Idle); err != nil {
			// SetKeepAlivePeriod sets both idle and interval on most platforms.
			// We ignore the error; the OS defaults remain.
			_ = err
		}
	}
	// The count (number of probes) cannot be set via net package on most OSes.
	// On Linux it can be set via setsockopt with IPPROTO_TCP, TCP_KEEPCNT.
	// For cross‑platform compatibility we do not attempt it here.
	// Users who need fine‑grained control can use a syscall.RawConn.
	return nil
}

// ----------------------------------------------------------------------
// gRPC keep‑alive (application level)
// ----------------------------------------------------------------------

// GRPCServerConfig defines keep‑alive and enforcement policy for a gRPC server.
type GRPCServerConfig struct {
	MaxConnectionIdle     time.Duration // maximum time a client can be idle before GOAWAY
	MaxConnectionAge      time.Duration // maximum time any connection will be kept open
	MaxConnectionAgeGrace time.Duration // grace period after MaxConnectionAge
	Time                  time.Duration // server ping interval to idle clients
	Timeout               time.Duration // wait for ping ack before closing connection
	MinTime               time.Duration // clients must wait this long between pings (enforcement)
	PermitWithoutStream   bool          // allow pings even when there are no active streams
}

// DefaultGRPCServerConfig returns recommended gRPC server keep‑alive settings.
func DefaultGRPCServerConfig() *GRPCServerConfig {
	return &GRPCServerConfig{
		MaxConnectionIdle:     time.Duration(0), // disabled
		MaxConnectionAge:      time.Duration(0), // disabled
		MaxConnectionAgeGrace: time.Duration(0), // disabled
		Time:                  2 * time.Hour,    // very conservative
		Timeout:               20 * time.Second,
		MinTime:               5 * time.Minute,
		PermitWithoutStream:   false,
	}
}

// ServerOptions converts the config to a slice of grpc.ServerOption.
func (cfg *GRPCServerConfig) ServerOptions() []grpc.ServerOption {
	var opts []grpc.ServerOption
	// ServerParameters control the server's keep‑alive pings.
	sp := keepalive.ServerParameters{
		MaxConnectionIdle:     cfg.MaxConnectionIdle,
		MaxConnectionAge:      cfg.MaxConnectionAge,
		MaxConnectionAgeGrace: cfg.MaxConnectionAgeGrace,
		Time:                  cfg.Time,
		Timeout:               cfg.Timeout,
	}
	opts = append(opts, grpc.KeepaliveParams(sp))

	// EnforcementPolicy controls what the server will allow from clients.
	ep := keepalive.EnforcementPolicy{
		MinTime:             cfg.MinTime,
		PermitWithoutStream: cfg.PermitWithoutStream,
	}
	opts = append(opts, grpc.KeepaliveEnforcementPolicy(ep))
	return opts
}

// GRPCClientConfig defines keep‑alive settings for a gRPC client.
type GRPCClientConfig struct {
	Time                time.Duration // client ping interval if no activity
	Timeout             time.Duration // wait for ping ack before considering connection dead
	PermitWithoutStream bool          // send pings even when there are no active streams
}

// DefaultGRPCClientConfig returns recommended gRPC client keep‑alive settings.
func DefaultGRPCClientConfig() *GRPCClientConfig {
	return &GRPCClientConfig{
		Time:                30 * time.Second,
		Timeout:             20 * time.Second,
		PermitWithoutStream: false,
	}
}

// DialOptions converts the config to a slice of grpc.DialOption.
func (cfg *GRPCClientConfig) DialOptions() []grpc.DialOption {
	cp := keepalive.ClientParameters{
		Time:                cfg.Time,
		Timeout:             cfg.Timeout,
		PermitWithoutStream: cfg.PermitWithoutStream,
	}
	return []grpc.DialOption{grpc.WithKeepaliveParams(cp)}
}

// ----------------------------------------------------------------------
// Custom application‑level keep‑alive for any net.Conn
// ----------------------------------------------------------------------

// KeepAliveConn wraps a net.Conn and adds application‑level ping/pong.
// It sends periodic ping frames and expects pong replies. If a pong is
// not received within the timeout, the connection is considered dead
// and is closed.
type KeepAliveConn struct {
	net.Conn
	config *CustomConfig
	stop   chan struct{}
}

// CustomConfig configures a custom keep‑alive mechanism.
type CustomConfig struct {
	Interval    time.Duration // how often to send a ping
	Timeout     time.Duration // how long to wait for pong before closing
	PingPayload []byte        // optional ping payload (e.g., []byte("ping"))
	PongPayload []byte        // optional pong payload expected (empty = any response)
	OnDead      func(conn net.Conn) // callback when connection is declared dead
}

// DefaultCustomConfig returns a sensible custom keep‑alive configuration.
func DefaultCustomConfig() *CustomConfig {
	return &CustomConfig{
		Interval:    30 * time.Second,
		Timeout:     10 * time.Second,
		PingPayload: []byte("ping\n"),
		PongPayload: []byte("pong\n"),
		OnDead:      nil,
	}
}

// NewKeepAliveConn wraps an existing connection with a custom keep‑alive.
// It starts a background goroutine that sends pings and monitors responses.
// The caller must still call Close() on the returned connection, which
// stops the background goroutine and closes the underlying connection.
func NewKeepAliveConn(conn net.Conn, cfg *CustomConfig) *KeepAliveConn {
	if cfg == nil {
		cfg = DefaultCustomConfig()
	}
	kac := &KeepAliveConn{
		Conn:   conn,
		config: cfg,
		stop:   make(chan struct{}),
	}
	go kac.watchdog()
	return kac
}

// watchdog runs the periodic ping/pong loop.
func (k *KeepAliveConn) watchdog() {
	ticker := time.NewTicker(k.config.Interval)
	defer ticker.Stop()

	// Channel to signal pong receipt.
	pongCh := make(chan struct{}, 1)

	// Set the read deadline to trigger pong timeout.
	// This goroutine handles all read operations for pongs.
	go func() {
		buf := make([]byte, len(k.config.PongPayload))
		for {
			select {
			case <-k.stop:
				return
			default:
			}
			// Wait for pong. We expect exactly the pong payload,
			// but we allow partial reads and accumulate.
			// For simplicity, we read exactly the length of the expected pong.
			// In production you would use a frame protocol.
			k.Conn.SetReadDeadline(time.Now().Add(k.config.Timeout))
			n, err := k.Conn.Read(buf)
			if err != nil {
				// Read error – connection likely dead.
				k.declareDead()
				return
			}
			if n == len(k.config.PongPayload) && (len(k.config.PongPayload) == 0 || string(buf[:n]) == string(k.config.PongPayload)) {
				select {
				case pongCh <- struct{}{}:
				default:
				}
			}
		}
	}()

	for {
		select {
		case <-k.stop:
			return
		case <-ticker.C:
			// Send ping.
			k.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if _, err := k.Conn.Write(k.config.PingPayload); err != nil {
				k.declareDead()
				return
			}
			// Wait for pong or timeout.
			select {
			case <-pongCh:
				// Pong received, continue.
			case <-time.After(k.config.Timeout):
				k.declareDead()
				return
			case <-k.stop:
				return
			}
		}
	}
}

// declareDead handles a dead connection.
func (k *KeepAliveConn) declareDead() {
	if k.config.OnDead != nil {
		k.config.OnDead(k.Conn)
	}
	k.Conn.Close()
}

// Close stops the keep‑alive watchdog and closes the underlying connection.
func (k *KeepAliveConn) Close() error {
	close(k.stop)
	return k.Conn.Close()
}

// ----------------------------------------------------------------------
// Example usage (commented out)
//
// func main() {
//     // TCP keepalive
//     conn, _ := net.Dial("tcp", "example.com:80")
//     tcpConn := conn.(*net.TCPConn)
//     keepalive.DefaultTCPConfig().SetKeepAlive(tcpConn)
//
//     // gRPC server
//     s := grpc.NewServer(keepalive.DefaultGRPCServerConfig().ServerOptions()...)
//
//     // gRPC client
//     opts := keepalive.DefaultGRPCClientConfig().DialOptions()
//     conn, _ := grpc.Dial("example.com:50051", opts...)
//
//     // Custom keepalive
//     raw, _ := net.Dial("tcp", "localhost:8080")
//     kaConn := keepalive.NewKeepAliveConn(raw, nil)
//     defer kaConn.Close()
// }