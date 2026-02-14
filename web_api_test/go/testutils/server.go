package testutils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ServerConfig holds all configuration for a managed server process.
type ServerConfig struct {
	// Process configuration
	Path    string            // Working directory
	Command string            // Binary to execute
	Args    []string          // Command arguments
	EnvVars map[string]string // Custom environment variables (overrides system)

	// Health checking
	HealthEndpoint      string         // e.g., "/health"
	HealthCheckInterval time.Duration  // Polling frequency (default: 500ms)
	HealthCheckTimeout  time.Duration  // Per-attempt HTTP timeout (default: 2s)
	HealthCheckSuccess  func(int) bool // Custom status code validator
	StartupTimeout      time.Duration  // Max time to wait for healthy (default: 30s)

	// Shutdown configuration
	ShutdownTimeout time.Duration // Max time to wait for graceful stop (default: 10s)
	ShutdownSignal  os.Signal     // Signal for graceful shutdown (default: os.Interrupt)

	// Diagnostics
	LogOutput            bool   // Capture stdout/stderr to logger
	CaptureStderrOnError bool   // Include stderr in startup failure errors
	PortCheckHost        string // Host to check for port conflicts (e.g., "localhost")
	PortCheckPort        int    // Port to verify is free before starting
}

// ServerManager handles application server lifecycle with full concurrency safety.
type ServerManager struct {
	mu      sync.Mutex
	config  ServerConfig
	logger  Logger
	cmd     *exec.Cmd
	baseURL string
	done    chan error // signals process exit
}

// Logger defines the minimal logging interface required by ServerManager.
type Logger interface {
	Info(msg string, keyvals ...interface{})
	Debug(msg string, keyvals ...interface{})
	Warn(msg string, keyvals ...interface{})
	Error(msg string, keyvals ...interface{})
	Writer() io.Writer
}

// DefaultConfig returns a ServerConfig populated with sane defaults.
func DefaultConfig() ServerConfig {
	return ServerConfig{
		HealthCheckInterval:  500 * time.Millisecond,
		HealthCheckTimeout:   2 * time.Second,
		StartupTimeout:       30 * time.Second,
		ShutdownTimeout:      10 * time.Second,
		ShutdownSignal:       os.Interrupt,
		HealthCheckSuccess:   defaultHealthCheckSuccess,
		CaptureStderrOnError: true,
	}
}

// defaultHealthCheckSuccess accepts 2xx and 3xx status codes.
func defaultHealthCheckSuccess(statusCode int) bool {
	return statusCode >= 200 && statusCode < 400
}

// NewServerManager creates a new server manager instance with validation.
func NewServerManager(cfg ServerConfig, baseURL string, logger Logger) (*ServerManager, error) {
	// Apply defaults for zero values
	if cfg.HealthCheckInterval <= 0 {
		cfg.HealthCheckInterval = 500 * time.Millisecond
	}
	if cfg.HealthCheckTimeout <= 0 {
		cfg.HealthCheckTimeout = 2 * time.Second
	}
	if cfg.StartupTimeout <= 0 {
		cfg.StartupTimeout = 30 * time.Second
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = 10 * time.Second
	}
	if cfg.ShutdownSignal == nil {
		cfg.ShutdownSignal = os.Interrupt
	}
	if cfg.HealthCheckSuccess == nil {
		cfg.HealthCheckSuccess = defaultHealthCheckSuccess
	}

	// Validate required fields
	if cfg.Path == "" {
		return nil, fmt.Errorf("server path cannot be empty")
	}
	if cfg.Command == "" {
		return nil, fmt.Errorf("server command cannot be empty")
	}
	if baseURL == "" {
		return nil, fmt.Errorf("base URL cannot be empty")
	}

	// Verify working directory exists
	info, err := os.Stat(cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("server path '%s' invalid: %w", cfg.Path, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("server path '%s' is not a directory", cfg.Path)
	}

	// Verify command exists
	if _, err := exec.LookPath(cfg.Command); err != nil {
		return nil, fmt.Errorf("command '%s' not found in PATH: %w", cfg.Command, err)
	}

	// Optional: pre-flight port check
	if cfg.PortCheckPort > 0 {
		if err := checkPortAvailable(cfg.PortCheckHost, cfg.PortCheckPort); err != nil {
			return nil, fmt.Errorf("port availability check failed: %w", err)
		}
	}

	return &ServerManager{
		config:  cfg,
		logger:  logger,
		baseURL: baseURL,
	}, nil
}

// Start launches the application server and waits for it to become healthy.
// It accepts a context to allow cancellation during the startup phase.
func (sm *ServerManager) Start(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Prevent double-start
	if sm.cmd != nil && sm.cmd.Process != nil {
		return fmt.Errorf("server already running with PID %d", sm.cmd.Process.Pid)
	}

	sm.logger.Info("Starting server",
		"command", sm.config.Command,
		"args", sm.config.Args,
		"dir", sm.config.Path)

	// Prepare command
	sm.cmd = exec.CommandContext(ctx, sm.config.Command, sm.config.Args...)
	sm.cmd.Dir = sm.config.Path
	sm.cmd.Env = sm.getEnvironmentVariables()

	// Capture stderr for debugging if requested
	var stderrBuf bytes.Buffer
	if sm.config.CaptureStderrOnError {
		sm.cmd.Stderr = &stderrBuf
	}

	// Redirect stdout/stderr to logger if configured
	if sm.config.LogOutput {
		if sm.cmd.Stdout == nil {
			sm.cmd.Stdout = sm.logger.Writer()
		}
		if sm.cmd.Stderr == nil {
			sm.cmd.Stderr = io.MultiWriter(sm.logger.Writer(), &stderrBuf)
		}
	}

	// Start the process
	if err := sm.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server process: %w", err)
	}

	sm.logger.Info("Server process started", "pid", sm.cmd.Process.Pid)

	// Start process reaper
	sm.done = make(chan error, 1)
	go func() {
		err := sm.cmd.Wait()
		sm.mu.Lock()
		defer sm.mu.Unlock()
		select {
		case sm.done <- err:
		default:
		}
		sm.logger.Debug("Process exited", "pid", sm.cmd.Process.Pid, "err", err)
	}()

	// Wait for health check
	healthURL := sm.baseURL + sm.config.HealthEndpoint
	if err := sm.waitForHealth(ctx, healthURL); err != nil {
		// Capture stderr before killing
		var stderrMsg string
		if sm.config.CaptureStderrOnError && stderrBuf.Len() > 0 {
			stderrMsg = fmt.Sprintf("\nstderr:\n%s", stderrBuf.String())
		}

		// Clean up the process
		_ = sm.killProcessLocked()
		return fmt.Errorf("server health check failed: %w%s", err, stderrMsg)
	}

	sm.logger.Info("Server started successfully", "url", sm.baseURL)
	return nil
}

// Stop gracefully terminates the server with configurable signal and timeout.
func (sm *ServerManager) Stop(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.cmd == nil || sm.cmd.Process == nil {
		sm.logger.Info("Server process not running, skipping stop")
		return nil
	}

	sm.logger.Info("Stopping server gracefully...", "pid", sm.cmd.Process.Pid)

	// Send configured shutdown signal
	if err := sm.cmd.Process.Signal(sm.config.ShutdownSignal); err != nil {
		sm.logger.Error("Failed to send shutdown signal", "error", err)
		return fmt.Errorf("failed to signal process: %w", err)
	}

	// Wait for graceful termination with timeout
	select {
	case err := <-sm.done:
		sm.logger.Info("Server terminated gracefully")
		sm.cmd = nil
		return err
	case <-time.After(sm.config.ShutdownTimeout):
		sm.logger.Warn("Server shutdown timeout exceeded, forcing termination")
		return sm.killProcessLocked()
	case <-ctx.Done():
		sm.logger.Warn("Context cancelled during server stop, forcing termination")
		return sm.killProcessLocked()
	}
}

// killProcessLocked forces the server process to stop (SIGKILL).
// Must be called with sm.mu held.
func (sm *ServerManager) killProcessLocked() error {
	if sm.cmd == nil || sm.cmd.Process == nil {
		return nil
	}

	pid := sm.cmd.Process.Pid
	sm.logger.Warn("Force-killing server process", "pid", pid)

	if err := sm.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to force kill process %d: %w", pid, err)
	}

	// Wait for process to be reaped
	if sm.done != nil {
		select {
		case err := <-sm.done:
			sm.logger.Debug("Process killed", "pid", pid, "err", err)
		case <-time.After(5 * time.Second):
			sm.logger.Warn("Timeout waiting for killed process", "pid", pid)
		}
	}

	sm.cmd = nil
	return nil
}

// getEnvironmentVariables merges system env with custom vars, with deduplication.
func (sm *ServerManager) getEnvironmentVariables() []string {
	// Start with system environment as a map for deduplication
	envMap := make(map[string]string)

	for _, entry := range os.Environ() {
		if key, val, ok := strings.Cut(entry, "="); ok {
			envMap[key] = val
		}
	}

	// Override with custom variables
	for key, val := range sm.config.EnvVars {
		envMap[key] = val
	}

	// Convert back to slice
	env := make([]string, 0, len(envMap))
	for key, val := range envMap {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}
	return env
}

// waitForHealth polls the health endpoint until success or timeout.
func (sm *ServerManager) waitForHealth(ctx context.Context, url string) error {
	// Reusable HTTP client with per-attempt timeout
	client := &http.Client{
		Timeout: sm.config.HealthCheckTimeout,
	}

	deadline := time.Now().Add(sm.config.StartupTimeout)
	ticker := time.NewTicker(sm.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("health check cancelled: %w", ctx.Err())
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("health check timed out after %v", sm.config.StartupTimeout)
			}

			// Create request with context
			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				sm.logger.Debug("Failed to create health request", "error", err)
				continue
			}

			resp, err := client.Do(req)
			if err != nil {
				// Connection refused/reset is normal during startup
				sm.logger.Debug("Health check failed (connection error)", "error", err)
				continue
			}
			resp.Body.Close()

			if sm.config.HealthCheckSuccess(resp.StatusCode) {
				sm.logger.Debug("Health check succeeded", "status", resp.StatusCode)
				return nil
			}

			sm.logger.Debug("Health check failed (status code)", "status", resp.StatusCode)
		}
	}
}

// checkPortAvailable verifies a TCP port is free on the given host.
func checkPortAvailable(host string, port int) error {
	if host == "" {
		host = "localhost"
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("port %d on %s is already in use: %w", port, host, err)
	}
	ln.Close()
	return nil
}

// Pid returns the process ID of the managed server, or 0 if not running.
func (sm *ServerManager) Pid() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.cmd != nil && sm.cmd.Process != nil {
		return sm.cmd.Process.Pid
	}
	return 0
}

// IsRunning returns true if the server process is still alive.
func (sm *ServerManager) IsRunning() bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.cmd == nil || sm.cmd.Process == nil {
		return false
	}

	// Signal with signal 0 tests process existence
	return sm.cmd.Process.Signal(os.Signal(nil)) == nil
}
