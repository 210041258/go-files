package testutils

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "math/rand"
    "net"
    "os"
    "path/filepath"
    "runtime"
    "sort"
    "strconv"
    "strings"
    "sync"
    "sync/atomic"
    "time"
)

// NOTE: LogLevel and its constants (TRACE, DEBUG, etc.) are removed from here 
// to avoid redeclaration errors. They are now defined in config.go.

var logLevelNames = []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL"}

// LogEntry represents a structured log entry
type LogEntry struct {
    Timestamp time.Time      `json:"timestamp"`
    Level     LogLevel       `json:"level"` // Uses LogLevel from config.go
    TestID    string         `json:"test_id,omitempty"`
    Message   string         `json:"message"`
    Fields    map[string]any `json:"fields,omitempty"`
    Caller    string         `json:"caller,omitempty"`
    Sequence  uint64         `json:"sequence,omitempty"`
}

// PortCheckConfig holds configuration for port checking operations
type PortCheckConfig struct {
    Protocol      string        `json:"protocol"`
    IPVersion     string        `json:"ip_version"`
    Timeout       time.Duration `json:"timeout"`
    RetryCount    int           `json:"retry_count"`
    RetryDelay    time.Duration `json:"retry_delay"`
    JitterEnabled bool          `json:"jitter_enabled"`
    CheckAll      bool          `json:"check_all"`
}

// PortCheckResult represents the result of a port checking operation
type PortCheckResult struct {
    Port          int           `json:"port"`
    Protocol      string        `json:"protocol"`
    Network       string        `json:"network"`
    Address       string        `json:"address"`
    Success       bool          `json:"success"`
    Error         string        `json:"error,omitempty"`
    Latency       time.Duration `json:"latency,omitempty"`
    RetryCount    int           `json:"retry_count,omitempty"`
    CheckedAt     time.Time     `json:"checked_at"`
    Deterministic bool          `json:"deterministic,omitempty"`
}

// PortRangeCheckResult represents the result of checking a range of ports
type PortRangeCheckResult struct {
    StartPort    int               `json:"start_port"`
    EndPort      int               `json:"end_port"`
    Protocol     string            `json:"protocol"`
    IPVersion    string            `json:"ip_version"`
    TotalPorts   int               `json:"total_ports"`
    OpenPorts    []int             `json:"open_ports"`
    ClosedPorts  []int             `json:"closed_ports"`
    SuccessCount int               `json:"success_count"`
    FailureCount int               `json:"failure_count"`
    Duration     time.Duration     `json:"duration"`
    PerPortStats []PortCheckResult `json:"per_port_stats,omitempty"`
}

// TestLogger provides structured, thread-safe test logging with JSON support
type TestLogger struct {
    mu          sync.RWMutex
    testID      string
    logLevel    LogLevel // Uses LogLevel from config.go
    output      io.Writer
    jsonOutput  bool
    fields      map[string]any
    callerSkip  int
    sequence    atomic.Uint64
    portChecks  []PortCheckResult
    rangeChecks []PortRangeCheckResult
}

// LoggerOption configures TestLogger behavior
type LoggerOption func(*TestLogger)

func NewTestLogger(testID string, output io.Writer, opts ...LoggerOption) *TestLogger {
    if output == nil {
        output = os.Stdout
    }

    logger := &TestLogger{
        testID:     testID,
        logLevel:   INFO,
        output:     output,
        jsonOutput: false,
        fields:     make(map[string]any),
        callerSkip: 3,
    }

    for _, opt := range opts {
        opt(logger)
    }

    return logger
}

// DefaultLogger creates a logger with reasonable defaults
func DefaultLogger() *TestLogger {
    return NewTestLogger("default", os.Stdout, WithJSONOutput(true), WithDefaultFields(map[string]any{
        "version": "1.0.0",
        "env":     "test",
    }))
}

// WithJSONOutput enables JSON structured logging
func WithJSONOutput(enabled bool) LoggerOption {
    return func(l *TestLogger) {
        l.jsonOutput = enabled
    }
}

// WithDefaultFields adds default fields to all log entries
func WithDefaultFields(fields map[string]any) LoggerOption {
    return func(l *TestLogger) {
        for k, v := range fields {
            l.fields[k] = v
        }
    }
}

// WithCallerSkip adjusts the number of stack frames to skip for caller info
func WithCallerSkip(skip int) LoggerOption {
    return func(l *TestLogger) {
        l.callerSkip = skip
    }
}

// WithLevel sets the initial log level
func WithLevel(level LogLevel) LoggerOption {
    return func(l *TestLogger) {
        l.logLevel = level
    }
}

// Port checking methods

// CheckPort performs a single port check with detailed logging
func (l *TestLogger) CheckPort(ctx context.Context, host string, port int, config PortCheckConfig) (PortCheckResult, error) {
    startTime := time.Now()
    result := PortCheckResult{
        Port:      port,
        Protocol:  config.Protocol,
        CheckedAt: startTime,
    }

    // Build network address based on protocol and IP version
    network, address := l.buildNetworkAddress(host, port, config)
    result.Network = network
    result.Address = address

    l.Debug("starting port check", map[string]any{
        "host":       host,
        "port":       port,
        "protocol":   config.Protocol,
        "ip_version": config.IPVersion,
        "timeout":    config.Timeout,
    })

    var lastErr error
    for attempt := 0; attempt <= config.RetryCount; attempt++ {
        select {
        case <-ctx.Done():
            result.Error = ctx.Err().Error()
            l.logPortCheck(result, attempt)
            return result, ctx.Err()
        default:
            attemptStart := time.Now()
            conn, err := net.DialTimeout(network, address, config.Timeout)
            latency := time.Since(attemptStart)

            if err == nil {
                conn.Close()
                result.Success = true
                result.Latency = latency
                result.RetryCount = attempt
                l.logPortCheck(result, attempt)
                return result, nil
            }

            lastErr = err
            result.Error = err.Error()
            result.Latency = latency

            if attempt < config.RetryCount {
                delay := config.RetryDelay
                if config.JitterEnabled {
                    // Add ±25% jitter
                    jitter := time.Duration(float64(delay) * 0.25)
                    delay += time.Duration(float64(jitter) * (2*float64(time.Now().UnixNano()%100)/100 - 1))
                }
                l.Debug("port check failed, retrying", map[string]any{
                    "port":        port,
                    "attempt":     attempt + 1,
                    "max_retries": config.RetryCount,
                    "delay":       delay,
                    "error":       err.Error(),
                })
                time.Sleep(delay)
            }
        }
    }

    l.logPortCheck(result, config.RetryCount)
    return result, lastErr
}

// CheckPortRange checks a range of ports with deterministic results
func (l *TestLogger) CheckPortRange(ctx context.Context, host string, startPort, endPort int, config PortCheckConfig) (PortRangeCheckResult, error) {
    if startPort > endPort {
        startPort, endPort = endPort, startPort
    }

    startTime := time.Now()
    result := PortRangeCheckResult{
        StartPort:  startPort,
        EndPort:    endPort,
        Protocol:   config.Protocol,
        IPVersion:  config.IPVersion,
        TotalPorts: endPort - startPort + 1,
    }

    l.Info("starting port range check", map[string]any{
        "host":       host,
        "start_port": startPort,
        "end_port":   endPort,
        "total":      result.TotalPorts,
        "protocol":   config.Protocol,
        "check_all":  config.CheckAll,
    })

    // Use a worker pool for concurrent checks
    workerCount := runtime.NumCPU()
    if workerCount > result.TotalPorts {
        workerCount = result.TotalPorts
    }

    ports := make(chan int, result.TotalPorts)
    results := make(chan PortCheckResult, result.TotalPorts)

    // Start workers
    var wg sync.WaitGroup
    for i := 0; i < workerCount; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            for port := range ports {
                select {
                case <-ctx.Done():
                    return
                default:
                    portResult, _ := l.CheckPort(ctx, host, port, config)
                    portResult.Deterministic = true // Mark as deterministic
                    results <- portResult
                }
            }
        }(i)
    }

    // Send ports to workers
    for port := startPort; port <= endPort; port++ {
        ports <- port
    }
    close(ports)

    // Collect results
    go func() {
        wg.Wait()
        close(results)
    }()

    var perPortStats []PortCheckResult
    for portResult := range results {
        perPortStats = append(perPortStats, portResult)
        if portResult.Success {
            result.OpenPorts = append(result.OpenPorts, portResult.Port)
            result.SuccessCount++
        } else {
            result.ClosedPorts = append(result.ClosedPorts, portResult.Port)
            result.FailureCount++
        }
    }

    // Sort results for deterministic output
    sort.Ints(result.OpenPorts)
    sort.Ints(result.ClosedPorts)
    sort.Slice(perPortStats, func(i, j int) bool {
        return perPortStats[i].Port < perPortStats[j].Port
    })

    if config.CheckAll {
        result.PerPortStats = perPortStats
    }

    result.Duration = time.Since(startTime)

    // Log summary
    l.Info("port range check completed", map[string]any{
        "start_port":    startPort,
        "end_port":      endPort,
        "total_ports":   result.TotalPorts,
        "open_ports":    result.OpenPorts,
        "open_count":    result.SuccessCount,
        "closed_count":  result.FailureCount,
        "duration":      result.Duration,
        "deterministic": true,
    })

    l.rangeChecks = append(l.rangeChecks, result)
    return result, nil
}

// WaitForAnyPort waits for any port in a range to become available
func (l *TestLogger) WaitForAnyPort(ctx context.Context, host string, startPort, endPort int, config PortCheckConfig) (PortCheckResult, error) {
    l.Info("waiting for any port to become available", map[string]any{
        "host":       host,
        "start_port": startPort,
        "end_port":   endPort,
        "protocol":   config.Protocol,
    })

    ports := make([]int, endPort-startPort+1)
    for i := range ports {
        ports[i] = startPort + i
    }

    // Shuffle ports for better distribution if not deterministic
    if !config.CheckAll {
        rand.Shuffle(len(ports), func(i, j int) {
            ports[i], ports[j] = ports[j], ports[i]
        })
    }

    for _, port := range ports {
        select {
        case <-ctx.Done():
            return PortCheckResult{}, ctx.Err()
        default:
            result, err := l.CheckPort(ctx, host, port, config)
            if err == nil && result.Success {
                l.Info("found available port", map[string]any{
                    "port":     port,
                    "host":     host,
                    "protocol": config.Protocol,
                    "latency":  result.Latency,
                })
                return result, nil
            }
        }
    }

    return PortCheckResult{}, fmt.Errorf("no available ports found in range %d-%d", startPort, endPort)
}

// LogPortStats logs aggregated port statistics
func (l *TestLogger) LogPortStats() {
    l.mu.Lock()
    defer l.mu.Unlock()

    if len(l.portChecks) == 0 && len(l.rangeChecks) == 0 {
        return
    }

    stats := map[string]any{
        "total_port_checks":  len(l.portChecks),
        "total_range_checks": len(l.rangeChecks),
    }

    if len(l.portChecks) > 0 {
        var successfulChecks, totalLatency int64
        for _, check := range l.portChecks {
            if check.Success {
                successfulChecks++
                totalLatency += int64(check.Latency)
            }
        }
        stats["port_check_success_rate"] = float64(successfulChecks) / float64(len(l.portChecks))
        if successfulChecks > 0 {
            stats["average_latency"] = time.Duration(totalLatency / successfulChecks)
        }
    }

    l.Info("port check statistics", stats)
}

// Helper methods

func (l *TestLogger) buildNetworkAddress(host string, port int, config PortCheckConfig) (string, string) {
    var network, address string

    switch strings.ToLower(config.Protocol) {
    case "tcp", "tcp4", "tcp6":
        network = config.Protocol
    case "udp", "udp4", "udp6":
        network = config.Protocol
    default:
        network = "tcp"
    }

    // Handle IPv4/IPv6
    if strings.HasSuffix(network, "4") && !strings.Contains(host, ":") {
        host = host + ":v4"
    } else if strings.HasSuffix(network, "6") && !strings.Contains(host, ":") {
        host = "[" + host + "]:v6"
    }

    address = net.JoinHostPort(host, strconv.Itoa(port))
    return network, address
}

func (l *TestLogger) logPortCheck(result PortCheckResult, retryCount int) {
    fields := map[string]any{
        "port":        result.Port,
        "protocol":    result.Protocol,
        "network":     result.Network,
        "address":     result.Address,
        "success":     result.Success,
        "latency":     result.Latency,
        "retry_count": retryCount,
        "checked_at":  result.CheckedAt,
    }

    if result.Error != "" {
        fields["error"] = result.Error
    }

    if result.Success {
        l.Info(fmt.Sprintf("port check succeeded for port %d", result.Port), fields)
    } else {
        l.Warn(fmt.Sprintf("port check failed for port %d", result.Port), fields)
    }

    l.mu.Lock()
    l.portChecks = append(l.portChecks, result)
    l.mu.Unlock()
}

// Original logging methods with enhancements

func (l *TestLogger) SetLevel(level LogLevel) {
    l.mu.Lock()
    defer l.mu.Unlock()
    l.logLevel = level
}

func (l *TestLogger) Level() LogLevel {
    l.mu.RLock()
    defer l.mu.RUnlock()
    return l.logLevel
}

// WithField returns a new logger with an additional field
func (l *TestLogger) WithField(key string, value any) *TestLogger {
    l.mu.RLock()
    defer l.mu.RUnlock()

    newLogger := l.clone()
    newLogger.fields[key] = value
    return newLogger
}

// WithFields returns a new logger with additional fields
func (l *TestLogger) WithFields(fields map[string]any) *TestLogger {
    l.mu.RLock()
    defer l.mu.RUnlock()

    newLogger := l.clone()
    for k, v := range fields {
        newLogger.fields[k] = v
    }
    return newLogger
}

// clone creates a deep copy of the logger
func (l *TestLogger) clone() *TestLogger {
    fields := make(map[string]any, len(l.fields))
    for k, v := range l.fields {
        fields[k] = v
    }

    return &TestLogger{
        testID:     l.testID,
        logLevel:   l.logLevel,
        output:     l.output,
        jsonOutput: l.jsonOutput,
        fields:     fields,
        callerSkip: l.callerSkip,
        sequence:   atomic.Uint64{},
    }
}

func (l *TestLogger) log(level LogLevel, msg string, fields map[string]any) {
    if level < l.Level() {
        return
    }

    // Get caller information
    caller := ""
    if _, file, line, ok := runtime.Caller(l.callerSkip); ok {
        caller = fmt.Sprintf("%s:%d", filepath.Base(file), line)
    }

    // Merge fields
    allFields := make(map[string]any)
    for k, v := range l.fields {
        allFields[k] = v
    }
    for k, v := range fields {
        allFields[k] = v
    }

    entry := LogEntry{
        Timestamp: time.Now().UTC(),
        Level:     level,
        TestID:    l.testID,
        Message:   msg,
        Fields:    allFields,
        Caller:    caller,
        Sequence:  l.sequence.Add(1),
    }

    l.writeEntry(entry)
}

func (l *TestLogger) writeEntry(entry LogEntry) {
    var output string
    if l.jsonOutput {
        jsonBytes, err := json.Marshal(entry)
        if err != nil {
            // Fallback to plain text if JSON marshaling fails
            output = fmt.Sprintf("[ERROR] Failed to marshal log entry: %v", err)
        } else {
            output = string(jsonBytes)
        }
    } else {
        levelStr := logLevelNames[entry.Level]
        fieldsStr := ""
        if len(entry.Fields) > 0 {
            var pairs []string
            for k, v := range entry.Fields {
                pairs = append(pairs, fmt.Sprintf("%s=%v", k, v))
            }
            fieldsStr = " " + strings.Join(pairs, " ")
        }

        output = fmt.Sprintf("[%s] [%s] %s: %s%s\n",
            entry.Timestamp.Format("2006-01-02 15:04:05.000"),
            levelStr,
            entry.TestID,
            entry.Message,
            fieldsStr)
    }

    l.mu.RLock()
    defer l.mu.RUnlock()
    if l.output != nil {
        fmt.Fprint(l.output, output)
        if !strings.HasSuffix(output, "\n") {
            fmt.Fprintln(l.output)
        }
    }
}

// Logging methods with field support
func (l *TestLogger) Tracef(format string, args ...any) {
    l.log(TRACE, fmt.Sprintf(format, args...), nil)
}

func (l *TestLogger) Debugf(format string, args ...any) {
    l.log(DEBUG, fmt.Sprintf(format, args...), nil)
}

func (l *TestLogger) Infof(format string, args ...any) {
    l.log(INFO, fmt.Sprintf(format, args...), nil)
}

func (l *TestLogger) Warnf(format string, args ...any) {
    l.log(WARN, fmt.Sprintf(format, args...), nil)
}

func (l *TestLogger) Errorf(format string, args ...any) {
    l.log(ERROR, fmt.Sprintf(format, args...), nil)
}

func (l *TestLogger) Fatalf(format string, args ...any) {
    l.log(FATAL, fmt.Sprintf(format, args...), nil)
    os.Exit(1)
}

func (l *TestLogger) Trace(msg string, fields map[string]any) {
    l.log(TRACE, msg, fields)
}

func (l *TestLogger) Debug(msg string, fields map[string]any) {
    l.log(DEBUG, msg, fields)
}

func (l *TestLogger) Info(msg string, fields map[string]any) {
    l.log(INFO, msg, fields)
}

func (l *TestLogger) Warn(msg string, fields map[string]any) {
    l.log(WARN, msg, fields)
}

func (l *TestLogger) Error(msg string, fields map[string]any) {
    l.log(ERROR, msg, fields)
}

// Buffer returns a logger that writes to a buffer
func (l *TestLogger) Buffer() *BufferLogger {
    return &BufferLogger{
        parent: l,
        buffer: &bytes.Buffer{},
    }
}

// GetPortCheckHistory returns all port checks performed
func (l *TestLogger) GetPortCheckHistory() []PortCheckResult {
    l.mu.RLock()
    defer l.mu.RUnlock()

    history := make([]PortCheckResult, len(l.portChecks))
    copy(history, l.portChecks)
    return history
}

// GetPortRangeCheckHistory returns all port range checks performed
func (l *TestLogger) GetPortRangeCheckHistory() []PortRangeCheckResult {
    l.mu.RLock()
    defer l.mu.RUnlock()

    history := make([]PortRangeCheckResult, len(l.rangeChecks))
    copy(history, l.rangeChecks)
    return history
}

// ClearHistory clears all port check history
func (l *TestLogger) ClearHistory() {
    l.mu.Lock()
    defer l.mu.Unlock()
    l.portChecks = nil
    l.rangeChecks = nil
}

// BufferLogger writes logs to a buffer
type BufferLogger struct {
    parent *TestLogger
    buffer *bytes.Buffer
    mu     sync.Mutex
}

func (bl *BufferLogger) Write(p []byte) (n int, err error) {
    bl.mu.Lock()
    defer bl.mu.Unlock()
    return bl.buffer.Write(p)
}

func (bl *BufferLogger) String() string {
    bl.mu.Lock()
    defer bl.mu.Unlock()
    return bl.buffer.String()
}

func (bl *BufferLogger) Clear() {
    bl.mu.Lock()
    defer bl.mu.Unlock()
    bl.buffer.Reset()
}

// Example usage function
func ExamplePortChecking() {
    logger := DefaultLogger()

    // Configure port check
    config := PortCheckConfig{
        Protocol:      "tcp",
        IPVersion:     "ipv4",
        Timeout:       5 * time.Second,
        RetryCount:    3,
        RetryDelay:    1 * time.Second,
        JitterEnabled: true,
        CheckAll:      true,
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Single port check
    result, err := logger.CheckPort(ctx, "localhost", 8080, config)
    if err != nil {
        logger.Error("port check failed", map[string]any{"error": err})
    }

    // Port range check
    rangeResult, err := logger.CheckPortRange(ctx, "localhost", 8000, 8010, config)
    if err != nil {
        logger.Error("port range check failed", map[string]any{"error": err})
    }

    // Wait for any port
    waitResult, err := logger.WaitForAnyPort(ctx, "localhost", 9000, 9010, config)
    if err == nil {
        logger.Info("found available port", map[string]any{
            "port":    waitResult.Port,
            "latency": waitResult.Latency,
        })
    }

    // Log statistics
    logger.LogPortStats()

    // Output history as JSON
    history := logger.GetPortCheckHistory()
    if jsonBytes, err := json.MarshalIndent(history, "", "  "); err == nil {
        logger.Debug("port check history", map[string]any{"history": string(jsonBytes)})
    }
}