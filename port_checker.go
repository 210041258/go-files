package testutils

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

//
// Protocol Types
// NOTE: Removed 'Protocol' type and constants (TCP, UDP, etc.) because they
// are already defined in config.go, causing a redeclaration error.
//

//
// Logger Interface
//

// Logger defines the interface required for logging.

// noopLogger prevents nil-pointer panics.
type noopLogger struct{}

func (noopLogger) Info(string, map[string]any)  {}
func (noopLogger) Debug(string, map[string]any) {}
func (noopLogger) Warn(string, map[string]any)  {}
func (noopLogger) Error(string, map[string]any) {}

//
// Configuration
//

func (c PortCheckerConfig) withDefaults() PortCheckerConfig {
	if c.Protocol == "" {
		c.Protocol = TCP
	}
	if c.DialTimeout <= 0 {
		c.DialTimeout = 2 * time.Second
	}
	if c.ReadTimeout <= 0 {
		c.ReadTimeout = 1 * time.Second
	}
	if c.WriteTimeout <= 0 {
		c.WriteTimeout = 1 * time.Second
	}
	if c.RetryInterval <= 0 {
		c.RetryInterval = 500 * time.Millisecond
	}
	if c.MaxRetries <= 0 {
		c.MaxRetries = 3
	}
	if c.BackoffFactor <= 0 {
		c.BackoffFactor = 1.5
	}
	if c.MaxConcurrency <= 0 {
		c.MaxConcurrency = 100
	}
	if c.Workers <= 0 {
		c.Workers = 10
	}
	if c.MinPort <= 0 {
		c.MinPort = 1
	}
	if c.MaxPort <= 0 {
		c.MaxPort = 65535
	}
	if c.OperationTimeout <= 0 {
		c.OperationTimeout = 30 * time.Second
	}
	if c.WaitTimeout <= 0 {
		c.WaitTimeout = 5 * time.Minute
	}
	return c
}

//
// Connection Results
//

// ConnectionResult contains detailed connection metadata.
type ConnectionResult struct {
	Host          string        `json:"host"`
	Port          int           `json:"port"`
	Protocol      Protocol      `json:"protocol"`
	Address       string        `json:"address"`
	Open          bool          `json:"open"`
	Latency       time.Duration `json:"latency"`
	Error         string        `json:"error,omitempty"`
	ErrorType     string        `json:"error_type,omitempty"`
	LocalAddr     string        `json:"local_addr,omitempty"`
	RemoteAddr    string        `json:"remote_addr,omitempty"`
	ConnectedAt   time.Time     `json:"connected_at,omitempty"`
	Attempts      int           `json:"attempts"`
	IPVersion     IPVersion     `json:"ip_version"`
	Deterministic bool          `json:"deterministic"` // For test reproducibility
}

// PortRangeResult contains results for a range of ports.
type PortRangeResult struct {
	Host         string              `json:"host"`
	StartPort    int                 `json:"start_port"`
	EndPort      int                 `json:"end_port"`
	Protocol     Protocol            `json:"protocol"`
	IPVersion    IPVersion           `json:"ip_version"`
	TotalPorts   int                 `json:"total_ports"`
	OpenPorts    []int               `json:"open_ports"`
	ClosedPorts  []int               `json:"closed_ports"`
	SuccessCount int                 `json:"success_count"`
	FailureCount int                 `json:"failure_count"`
	Duration     time.Duration       `json:"duration"`
	PerPortStats []*ConnectionResult `json:"per_port_stats,omitempty"`
	Errors       []string            `json:"errors,omitempty"`
}

// WaitResult contains wait operation results.
type WaitResult struct {
	Host      string            `json:"host"`
	Port      int               `json:"port"`
	Protocol  Protocol          `json:"protocol"`
	Success   bool              `json:"success"`
	Duration  time.Duration     `json:"duration"`
	Attempts  int               `json:"attempts"`
	Errors    []string          `json:"errors,omitempty"`
	FoundPort *ConnectionResult `json:"found_port,omitempty"`
}

//
// Port Target
//

// PortTarget defines a bulk-check target.
type PortTarget struct {
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	Protocol  Protocol  `json:"protocol,omitempty"`
	IPVersion IPVersion `json:"ip_version,omitempty"`
}

//
// Port Checker
//

// PortChecker checks port availability with support for multiple protocols.
type PortChecker struct {
	mu       sync.RWMutex
	logger   Logger
	config   PortCheckerConfig
	sem      chan struct{}
	stats    *PortCheckerStats
	sequence atomic.Uint64 // For deterministic ordering
}

// PortCheckerStats holds operational statistics.
type PortCheckerStats struct {
	mu              sync.RWMutex
	ChecksCompleted int64              `json:"checks_completed"`
	ChecksSucceeded int64              `json:"checks_succeeded"`
	ChecksFailed    int64              `json:"checks_failed"`
	TotalLatency    time.Duration      `json:"total_latency"`
	AverageLatency  time.Duration      `json:"average_latency"`
	LastCheck       time.Time          `json:"last_check"`
	PortsByProtocol map[Protocol]int64 `json:"ports_by_protocol"`
}

func NewPortCheckerStats() *PortCheckerStats {
	return &PortCheckerStats{
		PortsByProtocol: make(map[Protocol]int64),
	}
}

func (s *PortCheckerStats) Record(result *ConnectionResult) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ChecksCompleted++
	if result.Open {
		s.ChecksSucceeded++
	} else {
		s.ChecksFailed++
	}

	s.TotalLatency += result.Latency
	if s.ChecksCompleted > 0 {
		s.AverageLatency = s.TotalLatency / time.Duration(s.ChecksCompleted)
	}

	s.LastCheck = time.Now()
	s.PortsByProtocol[result.Protocol]++
}

func (s *PortCheckerStats) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ChecksCompleted = 0
	s.ChecksSucceeded = 0
	s.ChecksFailed = 0
	s.TotalLatency = 0
	s.AverageLatency = 0
	s.PortsByProtocol = make(map[Protocol]int64)
}

//
// Constructor
//

func NewPortChecker(logger Logger, config PortCheckerConfig) *PortChecker {
	if logger == nil {
		logger = noopLogger{}
	}

	cfg := config.withDefaults()

	return &PortChecker{
		logger: logger,
		config: cfg,
		sem:    make(chan struct{}, cfg.MaxConcurrency),
		stats:  NewPortCheckerStats(),
	}
}

//
// Core Port Checking
//

// IsPortOpen attempts a connection to host:port with the specified protocol.
func (pc *PortChecker) IsPortOpen(
	ctx context.Context,
	host string,
	port int,
	protocol Protocol,
) (*ConnectionResult, error) {

	// Validate port range
	if pc.config.ValidatePorts {
		if port < pc.config.MinPort || port > pc.config.MaxPort {
			return nil, fmt.Errorf("port %d outside allowed range [%d-%d]",
				port, pc.config.MinPort, pc.config.MaxPort)
		}
	}

	portStr := strconv.Itoa(port)

	// Build network address based on protocol and IP version
	network, address := pc.buildNetworkAddress(host, portStr, protocol, pc.config.IPVersion)

	start := time.Now()
	attempts := 0
	var lastError error

	pc.logger.Debug("attempting connection", map[string]any{
		"address":    address,
		"protocol":   protocol,
		"network":    network,
		"ip_version": pc.config.IPVersion,
	})

	// Retry logic
	for attempt := 0; attempt <= pc.config.MaxRetries; attempt++ {
		attempts++
		select {
		case <-ctx.Done():
			result := &ConnectionResult{
				Host:      host,
				Port:      port,
				Protocol:  protocol,
				Address:   address,
				Open:      false,
				Latency:   time.Since(start),
				Error:     ctx.Err().Error(),
				ErrorType: "context_cancelled",
				Attempts:  attempts,
				IPVersion: pc.config.IPVersion,
			}
			pc.stats.Record(result)
			return result, ctx.Err()
		default:
			// Try connection
			result, err := pc.tryConnect(ctx, network, address, host, port, protocol, start)
			if err == nil && result.Open {
				result.Attempts = attempts
				pc.stats.Record(result)
				return result, nil
			}

			lastError = err
			if result != nil {
				result.Attempts = attempts
			}

			// Apply backoff before retry
			if attempt < pc.config.MaxRetries {
				delay := pc.calculateRetryDelay(attempt)
				pc.logger.Debug("connection failed, retrying", map[string]any{
					"address": address,
					"attempt": attempt + 1,
					"delay":   delay,
					"error":   err,
				})
				time.Sleep(delay)
			}
		}
	}

	// All retries failed
	result := &ConnectionResult{
		Host:      host,
		Port:      port,
		Protocol:  protocol,
		Address:   address,
		Open:      false,
		Latency:   time.Since(start),
		Error:     lastError.Error(),
		ErrorType: "connection_failed",
		Attempts:  attempts,
		IPVersion: pc.config.IPVersion,
	}
	pc.stats.Record(result)

	return result, lastError
}

// tryConnect attempts a single connection
func (pc *PortChecker) tryConnect(
	ctx context.Context,
	network, address, host string,
	port int,
	protocol Protocol,
	start time.Time,
) (*ConnectionResult, error) {

	dialCtx, cancel := context.WithTimeout(ctx, pc.config.DialTimeout)
	defer cancel()

	var conn net.Conn
	var err error

	switch protocol {
	case TCP, TCP4, TCP6:
		var d net.Dialer
		conn, err = d.DialContext(dialCtx, network, address)
	case UDP, UDP4, UDP6:
		// For UDP, we try to establish a "connection" (sets default remote address)
		conn, err = net.DialTimeout(network, address, pc.config.DialTimeout)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}

	result := &ConnectionResult{
		Host:          host,
		Port:          port,
		Protocol:      protocol,
		Address:       address,
		Open:          err == nil,
		Latency:       time.Since(start),
		IPVersion:     pc.config.IPVersion,
		Deterministic: true,
	}

	if err != nil {
		result.Error = pc.wrapError(address, protocol, err).Error()
		result.ErrorType = pc.classifyError(err)
		return result, err
	}
	defer conn.Close()

	// For connection-oriented protocols, we might want to do additional checks
	if protocol == TCP || protocol == TCP4 || protocol == TCP6 {
		// Set read/write timeouts
		if pc.config.ReadTimeout > 0 {
			conn.SetReadDeadline(time.Now().Add(pc.config.ReadTimeout))
		}
		if pc.config.WriteTimeout > 0 {
			conn.SetWriteDeadline(time.Now().Add(pc.config.WriteTimeout))
		}
	}

	result.ConnectedAt = time.Now()
	result.LocalAddr = conn.LocalAddr().String()
	result.RemoteAddr = conn.RemoteAddr().String()

	pc.logger.Debug("connection successful", map[string]any{
		"address":  address,
		"protocol": protocol,
		"latency":  result.Latency.String(),
	})

	return result, nil
}

func (pc *PortChecker) buildNetworkAddress(host, port string, protocol Protocol, ipVersion IPVersion) (string, string) {
	network := string(protocol)

	// Handle IP version preference
	switch ipVersion {
	case IPv4:
		if protocol == TCP {
			network = string(TCP4)
		} else if protocol == UDP {
			network = string(UDP4)
		}
	case IPv6:
		if protocol == TCP {
			network = string(TCP6)
		} else if protocol == UDP {
			network = string(UDP6)
		}
	}

	// Ensure proper formatting for IPv6
	if ipVersion == IPv6 && !isIPv6(host) {
		// Host might be a hostname, let net package resolve it
		address := net.JoinHostPort(host, port)
		return network, address
	}

	// For IPv6 addresses, wrap in brackets
	if isIPv6(host) {
		address := "[" + host + "]:" + port
		return network, address
	}

	address := net.JoinHostPort(host, port)
	return network, address
}

func isIPv6(host string) bool {
	// Check if it looks like an IPv6 address
	return net.ParseIP(host) != nil && strings.Contains(host, ":")
}

func (pc *PortChecker) wrapError(address string, protocol Protocol, err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("connection to %s (%s) timed out after %v: %w",
			address, protocol, pc.config.DialTimeout, err)
	case errors.Is(err, context.Canceled):
		return fmt.Errorf("connection to %s (%s) canceled: %w", address, protocol, err)
	default:
		return fmt.Errorf("failed to connect to %s (%s): %w", address, protocol, err)
	}
}

func (pc *PortChecker) classifyError(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	default:
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return "network_timeout"
		}
		return "connection_error"
	}
}

func (pc *PortChecker) calculateRetryDelay(attempt int) time.Duration {
	delay := pc.config.RetryInterval

	// Apply exponential backoff
	if pc.config.BackoffFactor > 1.0 {
		for i := 0; i < attempt; i++ {
			delay = time.Duration(float64(delay) * pc.config.BackoffFactor)
		}
	}

	// Apply jitter if enabled
	if pc.config.JitterEnabled {
		jitter := time.Duration(float64(delay) * 0.25) // ±25% jitter
		delay += time.Duration(float64(jitter) * (2*float64(time.Now().UnixNano()%100)/100 - 1))
	}

	// Cap maximum delay
	maxDelay := 30 * time.Second
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

//
// Port Range Checking
//

// CheckPortRange checks a range of ports concurrently.
func (pc *PortChecker) CheckPortRange(
	ctx context.Context,
	host string,
	startPort, endPort int,
	protocol Protocol,
) (*PortRangeResult, error) {

	if startPort > endPort {
		startPort, endPort = endPort, startPort
	}

	startTime := time.Now()
	result := &PortRangeResult{
		Host:       host,
		StartPort:  startPort,
		EndPort:    endPort,
		Protocol:   protocol,
		IPVersion:  pc.config.IPVersion,
		TotalPorts: endPort - startPort + 1,
	}

	pc.logger.Info("starting port range check", map[string]any{
		"host":       host,
		"start_port": startPort,
		"end_port":   endPort,
		"protocol":   protocol,
		"total":      result.TotalPorts,
	})

	// Use worker pool
	type job struct {
		port int
		idx  int
	}

	type resultWithIdx struct {
		idx    int
		result *ConnectionResult
		err    error
	}

	ports := make(chan job, result.TotalPorts)
	results := make(chan resultWithIdx, result.TotalPorts)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < pc.config.Workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for job := range ports {
				select {
				case <-ctx.Done():
					results <- resultWithIdx{
						idx: job.idx,
						err: ctx.Err(),
					}
					return
				default:
					connResult, err := pc.IsPortOpen(ctx, host, job.port, protocol)
					results <- resultWithIdx{
						idx:    job.idx,
						result: connResult,
						err:    err,
					}
				}
			}
		}(i)
	}

	// Send jobs
	go func() {
		idx := 0
		for port := startPort; port <= endPort; port++ {
			ports <- job{port: port, idx: idx}
			idx++
		}
		close(ports)
	}()

	// Collect results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Process results
	perPortStats := make([]*ConnectionResult, result.TotalPorts)
	for res := range results {
		if res.err != nil {
			result.Errors = append(result.Errors, res.err.Error())
		}
		if res.result != nil {
			perPortStats[res.idx] = res.result
			if res.result.Open {
				result.OpenPorts = append(result.OpenPorts, res.result.Port)
				result.SuccessCount++
			} else {
				result.ClosedPorts = append(result.ClosedPorts, res.result.Port)
				result.FailureCount++
			}
		}
	}

	if pc.config.ValidatePorts {
		result.PerPortStats = perPortStats
	}

	result.Duration = time.Since(startTime)

	pc.logger.Info("port range check completed", map[string]any{
		"host":         host,
		"start_port":   startPort,
		"end_port":     endPort,
		"open_ports":   len(result.OpenPorts),
		"closed_ports": len(result.ClosedPorts),
		"duration":     result.Duration,
	})

	return result, nil
}

//
// Waiting Operations
//

// WaitForPort blocks until a port becomes available or timeout expires.
func (pc *PortChecker) WaitForPort(
	ctx context.Context,
	host string,
	port int,
	protocol Protocol,
) (*WaitResult, error) {

	timeoutCtx, cancel := context.WithTimeout(ctx, pc.config.WaitTimeout)
	defer cancel()

	startTime := time.Now()
	attempts := 0
	var errors []string

	pc.logger.Info("waiting for port", map[string]any{
		"host":     host,
		"port":     port,
		"protocol": protocol,
		"timeout":  pc.config.WaitTimeout,
	})

	for {
		attempts++
		select {
		case <-timeoutCtx.Done():
			result := &WaitResult{
				Host:     host,
				Port:     port,
				Protocol: protocol,
				Success:  false,
				Duration: time.Since(startTime),
				Attempts: attempts,
				Errors:   errors,
			}
			return result, timeoutCtx.Err()
		default:
			connResult, err := pc.IsPortOpen(timeoutCtx, host, port, protocol)
			if err == nil && connResult.Open {
				result := &WaitResult{
					Host:      host,
					Port:      port,
					Protocol:  protocol,
					Success:   true,
					Duration:  time.Since(startTime),
					Attempts:  attempts,
					Errors:    errors,
					FoundPort: connResult,
				}
				pc.logger.Info("port became available", map[string]any{
					"host":     host,
					"port":     port,
					"attempts": attempts,
					"duration": result.Duration,
				})
				return result, nil
			}

			if err != nil {
				errors = append(errors, err.Error())
			}

			// Wait before retry with jitter
			delay := pc.calculateRetryDelay(attempts)
			select {
			case <-timeoutCtx.Done():
				continue
			case <-time.After(delay):
				// Continue loop
			}
		}
	}
}

// WaitForAnyPort waits for any port in a range to become available.
func (pc *PortChecker) WaitForAnyPort(
	ctx context.Context,
	host string,
	startPort, endPort int,
	protocol Protocol,
) (*WaitResult, error) {

	timeoutCtx, cancel := context.WithTimeout(ctx, pc.config.WaitTimeout)
	defer cancel()

	startTime := time.Now()
	attempts := 0
	var errors []string

	pc.logger.Info("waiting for any port in range", map[string]any{
		"host":       host,
		"start_port": startPort,
		"end_port":   endPort,
		"protocol":   protocol,
		"timeout":    pc.config.WaitTimeout,
	})

	// Try ports in random order for better distribution
	ports := make([]int, endPort-startPort+1)
	for i := range ports {
		ports[i] = startPort + i
	}
	// Shuffle for deterministic but random order
	// Note: For true randomness, seed with time; for tests, use fixed seed

	for {
		attempts++
		select {
		case <-timeoutCtx.Done():
			result := &WaitResult{
				Host:     host,
				Port:     -1,
				Protocol: protocol,
				Success:  false,
				Duration: time.Since(startTime),
				Attempts: attempts,
				Errors:   errors,
			}
			return result, timeoutCtx.Err()
		default:
			for _, port := range ports {
				connResult, err := pc.IsPortOpen(timeoutCtx, host, port, protocol)
				if err == nil && connResult.Open {
					result := &WaitResult{
						Host:      host,
						Port:      port,
						Protocol:  protocol,
						Success:   true,
						Duration:  time.Since(startTime),
						Attempts:  attempts,
						Errors:    errors,
						FoundPort: connResult,
					}
					pc.logger.Info("found available port", map[string]any{
						"host":     host,
						"port":     port,
						"attempts": attempts,
						"duration": result.Duration,
					})
					return result, nil
				}

				if err != nil {
					errors = append(errors, fmt.Sprintf("port %d: %v", port, err))
				}
			}

			// Wait before retrying the entire range
			delay := pc.calculateRetryDelay(attempts)
			select {
			case <-timeoutCtx.Done():
				continue
			case <-time.After(delay):
				// Continue loop
			}
		}
	}
}

//
// Bulk Operations
//

// CheckMultiplePorts checks multiple targets concurrently.
func (pc *PortChecker) CheckMultiplePorts(
	ctx context.Context,
	targets []PortTarget,
) ([]*ConnectionResult, error) {

	results := make([]*ConnectionResult, len(targets))
	errs := make([]error, len(targets))

	var wg sync.WaitGroup

	for i, target := range targets {
		select {
		case pc.sem <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		wg.Add(1)
		go func(idx int, target PortTarget) {
			defer wg.Done()
			defer func() { <-pc.sem }()

			protocol := target.Protocol
			if protocol == "" {
				protocol = pc.config.Protocol
			}

			res, err := pc.IsPortOpen(ctx, target.Host, target.Port, protocol)
			results[idx] = res
			errs[idx] = err
		}(i, target)
	}

	wg.Wait()

	// Aggregate errors
	var compositeErr *CompositeError
	for _, err := range errs {
		if err != nil {
			if compositeErr == nil {
				compositeErr = NewCompositeError("port check errors")
			}
			compositeErr.Add(err)
		}
	}

	if compositeErr != nil && compositeErr.HasErrors() {
		return results, compositeErr
	}

	return results, nil
}

//
// Statistics
//

// GetStats returns current statistics.
func (pc *PortChecker) GetStats() *PortCheckerStats {
	return pc.stats
}

// ResetStats resets all statistics.
func (pc *PortChecker) ResetStats() {
	pc.stats.Reset()
}

//
// Helper Functions
//

// StringToProtocol converts a string to Protocol type.
func StringToProtocol(s string) Protocol {
	switch strings.ToLower(s) {
	case "tcp":
		return TCP
	case "tcp4":
		return TCP4
	case "tcp6":
		return TCP6
	case "udp":
		return UDP
	case "udp4":
		return UDP4
	case "udp6":
		return UDP6
	default:
		return TCP // Default to TCP
	}
}

// ValidatePort validates a port number.
func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port %d outside valid range [1-65535]", port)
	}
	return nil
}
