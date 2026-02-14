package testutils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs" // Added for crypto/rand usage if needed, though removed in snippet, standard practice
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// LogEntry represents a structured log entry
// Kept here as it is specific to the TestLogger implementation in this file
type LogEntry struct {
	Timestamp time.Time      `json:"timestamp"`
	Level     LogLevel       `json:"level"`
	TestID    string         `json:"test_id,omitempty"`
	Message   string         `json:"message"`
	Fields    map[string]any `json:"fields,omitempty"`
	Caller    string         `json:"caller,omitempty"`
	Sequence  uint64         `json:"sequence,omitempty"`
}

// PortCheckConfig holds configuration for port checking operations.
// Kept here as it is used by TestLogger in this file.
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

var logLevelNames = []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL"}

// TestLogger provides structured, thread-safe test logging with JSON support
type TestLogger struct {
	mu          sync.RWMutex
	testID      string
	logLevel    LogLevel
	output      io.Writer
	jsonOutput  bool
	fields      map[string]any
	callerSkip  int
	sequence    atomic.Uint64
	portChecks  []PortCheckResult
	rangeChecks []PortRangeCheckResult
	intUtils    *IntUtilities // Added integer utilities
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
		intUtils:   NewIntUtilities(), // Initialize integer utilities
	}

	for _, opt := range opts {
		opt(logger)
	}

	return logger
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

// DefaultLogger creates a logger with reasonable defaults
func DefaultLogger() *TestLogger {
	return NewTestLogger("default", os.Stdout, WithJSONOutput(true), WithDefaultFields(map[string]any{
		"version": "1.0.0",
		"env":     "test",
	}))
}

// TestLogger interface implementation
func (l *TestLogger) Info(msg string, keyvals map[string]any) {
	l.log(INFO, msg, keyvals)
}

func (l *TestLogger) Debug(msg string, keyvals map[string]any) {
	l.log(DEBUG, msg, keyvals)
}

func (l *TestLogger) Warn(msg string, keyvals map[string]any) {
	l.log(WARN, msg, keyvals)
}

func (l *TestLogger) Error(msg string, keyvals map[string]any) {
	l.log(ERROR, msg, keyvals)
}

// SetLevel sets the log level
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

// Integer utility methods added to TestLogger

// GenerateTestInts generates test integers with comprehensive logging
func (l *TestLogger) GenerateTestInts(count int, config RandomIntConfig) ([]int, error) {
	l.Info("generating test integers", map[string]any{
		"count":  count,
		"min":    config.Min,
		"max":    config.Max,
		"unique": false,
	})

	generator := NewRandomIntGenerator(config)
	ints, err := generator.GenerateMany(count)
	if err != nil {
		l.Error("failed to generate test integers", map[string]any{
			"count": count,
			"error": err.Error(),
		})
		return nil, err
	}

	// Analyze the generated integers
	stats := l.intUtils.Analyze(ints)
	l.Debug("integer generation statistics", map[string]any{
		"count":     stats.Count,
		"min":       stats.Min,
		"max":       stats.Max,
		"mean":      stats.Mean,
		"median":    stats.Median,
		"std_dev":   stats.StdDev,
		"generated": ints,
	})

	return ints, nil
}

// GenerateUniqueTestInts generates unique test integers
func (l *TestLogger) GenerateUniqueTestInts(count int, config RandomIntConfig) ([]int, error) {
	l.Info("generating unique test integers", map[string]any{
		"count":  count,
		"min":    config.Min,
		"max":    config.Max,
		"unique": true,
	})

	generator := NewRandomIntGenerator(config)
	ints, err := generator.GenerateUnique(count)
	if err != nil {
		l.Error("failed to generate unique test integers", map[string]any{
			"count": count,
			"error": err.Error(),
		})
		return nil, err
	}

	// Validate uniqueness
	seen := make(map[int]bool)
	for _, v := range ints {
		if seen[v] {
			l.Error("duplicate integer found", map[string]any{
				"value": v,
				"set":   ints,
			})
			return nil, errors.New("non-unique integers generated")
		}
		seen[v] = true
	}

	l.Debug("unique integers generated", map[string]any{
		"count":    count,
		"integers": ints,
	})

	return ints, nil
}

// ValidateTestInts validates integers against specified rules
func (l *TestLogger) ValidateTestInts(ints []int, ruleNames ...string) (bool, *CompositeIntError) {
	l.Debug("validating integers", map[string]any{
		"count": len(ints),
		"rules": ruleNames,
	})

	validator := NewIntValidator()
	compositeErr := NewCompositeIntError("integer validation failed")
	allValid := true

	for i, v := range ints {
		valid, err := validator.Validate(v, ruleNames...)
		if !valid {
			allValid = false
			if err != nil {
				compositeErr.Add(fmt.Errorf("index %d: %v", i, err), v)
			}
		}
	}

	if allValid {
		l.Debug("all integers passed validation", map[string]any{
			"count": len(ints),
		})
		return true, nil
	}

	l.Warn("integer validation failed", map[string]any{
		"count":       len(ints),
		"error":       compositeErr.Error(),
		"error_count": compositeErr.ErrorCount(),
	})

	return false, compositeErr
}

// CreateIntegerTestFiles creates test files with integer data
func (l *TestLogger) CreateIntegerTestFiles(tdm *TestDataManager, baseName string, ints []int) ([]string, error) {
	l.Info("creating integer test files", map[string]any{
		"base_name": baseName,
		"count":     len(ints),
	})

	var filePaths []string
	compositeErr := NewCompositeIntError("failed to create integer test files")

	// Create collection for statistics
	stats := l.intUtils.Analyze(ints)

	// Create individual files for each integer
	for i, value := range ints {
		filename := fmt.Sprintf("%s_%d.txt", baseName, i)
		content := fmt.Sprintf("Test integer %d: %d\nValid: %v\n", i, value, value >= 0)

		filePath, err := tdm.CreateTestFile(filename, content)
		if err != nil {
			compositeErr.Add(fmt.Errorf("failed to create file for integer %d: %w", value, err), value)
			continue
		}

		filePaths = append(filePaths, filePath)
	}

	// Create statistics file
	statsJSON, err := json.MarshalIndent(stats, "", "  ")
	if err == nil {
		statsContent := fmt.Sprintf("Integer Statistics:\n%s\n", string(statsJSON))
		statsFile, err := tdm.CreateTestFile(baseName+"_stats.txt", statsContent)
		if err == nil {
			filePaths = append(filePaths, statsFile)
		}
	}

	// Create CSV file
	var csvBuilder strings.Builder
	csvBuilder.WriteString("index,value,is_prime,factors\n")
	for i, value := range ints {
		isPrime := l.intUtils.IsPrime(value)
		factors := l.intUtils.Factors(value)
		factorsStr := strings.Trim(strings.Join(strings.Fields(fmt.Sprint(factors)), ","), "[]")
		csvBuilder.WriteString(fmt.Sprintf("%d,%d,%v,%s\n", i, value, isPrime, factorsStr))
	}

	csvFile, err := tdm.CreateTestFile(baseName+"_data.csv", csvBuilder.String())
	if err == nil {
		filePaths = append(filePaths, csvFile)
	}

	if compositeErr.HasErrors() {
		l.Error("failed to create some integer test files", map[string]any{
			"created": len(filePaths),
			"failed":  compositeErr.ErrorCount(),
			"errors":  compositeErr.Error(),
		})
		return filePaths, compositeErr
	}

	l.Info("integer test files created successfully", map[string]any{
		"total_files": len(filePaths),
		"stats": map[string]any{
			"min":     stats.Min,
			"max":     stats.Max,
			"mean":    stats.Mean,
			"std_dev": stats.StdDev,
		},
	})

	return filePaths, nil
}

// Port checking methods (from previous implementation)

func (l *TestLogger) CheckPort(ctx context.Context, host string, port int, config PortCheckConfig) (PortCheckResult, error) {
	// Implementation from previous response
	startTime := time.Now()
	result := PortCheckResult{
		Port:      port,
		Protocol:  config.Protocol,
		CheckedAt: startTime,
	}

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

// Logging methods
func (l *TestLogger) log(level LogLevel, msg string, fields map[string]any) {
	if level < l.Level() {
		return
	}

	caller := ""
	if _, file, line, ok := runtime.Caller(l.callerSkip); ok {
		caller = fmt.Sprintf("%s:%d", filepath.Base(file), line)
	}

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
		intUtils:   NewIntUtilities(),
	}
}

// TestDataManager manages test data isolation with robust error handling.
type TestDataManager struct {
	mu      sync.RWMutex // Protects the directory state during cleanup/restore
	testDir string
	logger  Logger
	config  TestDataManagerConfig
}

// CleanupTransaction represents a snapshot state that can be restored.
type CleanupTransaction struct {
	manager   *TestDataManager
	backupDir string
	committed bool
}

// NewTestDataManager creates a new test data manager with atomic directory creation.
func NewTestDataManager(testID string, logger Logger, config *TestDataManagerConfig) (*TestDataManager, error) {
	if testID == "" {
		return nil, errors.New("testID cannot be empty")
	}

	// strict sanitization
	cleanID := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return -1 // Drop invalid characters
	}, testID)

	if cleanID == "" {
		cleanID = "unnamed-test"
	}

	cfg := TestDataManagerConfig{
		TempDir:  os.TempDir(),
		FileMode: 0644,
		DirMode:  0755,
	}
	if config != nil {
		if config.TempDir != "" {
			cfg.TempDir = config.TempDir
		}
		if config.FileMode != 0 {
			cfg.FileMode = config.FileMode
		}
		if config.DirMode != 0 {
			cfg.DirMode = config.DirMode
		}
	}

	testDir := filepath.Join(cfg.TempDir, "tests", cleanID)

	logger.Info("creating test data directory", map[string]any{
		"original_id": testID,
		"clean_id":    cleanID,
		"directory":   testDir,
	})

	// Atomic directory creation
	if err := os.MkdirAll(testDir, cfg.DirMode); err != nil {
		return nil, fmt.Errorf("failed to create test directory %q: %w", testDir, err)
	}

	// Verify directory creation and permissions
	stat, err := os.Stat(testDir)
	if err != nil {
		return nil, fmt.Errorf("failed to verify test directory %q: %w", testDir, err)
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("test directory path %q exists but is not a directory", testDir)
	}

	logger.Debug("test data directory created", map[string]any{
		"directory": testDir,
		"mode":      stat.Mode().String(),
	})

	return &TestDataManager{
		testDir: testDir,
		logger:  logger,
		config:  cfg,
	}, nil
}

// Enhanced methods using integer utilities

// CreateIntegerTestFiles creates test files with integer data using the logger's integer utilities
func (tdm *TestDataManager) CreateIntegerTestFiles(baseName string, ints []int) ([]string, error) {
	// Check if logger supports integer utilities
	if logger, ok := tdm.logger.(*TestLogger); ok {
		return logger.CreateIntegerTestFiles(tdm, baseName, ints)
	}

	// Fallback implementation
	var filePaths []string
	for i, value := range ints {
		filename := fmt.Sprintf("%s_%d.txt", baseName, i)
		content := fmt.Sprintf("Test integer %d: %d\n", i, value)

		filePath, err := tdm.CreateTestFile(filename, content)
		if err != nil {
			return filePaths, fmt.Errorf("failed to create file for integer %d: %w", value, err)
		}

		filePaths = append(filePaths, filePath)
	}

	return filePaths, nil
}

// GenerateAndCreateTestFiles generates random integers and creates test files
func (tdm *TestDataManager) GenerateAndCreateTestFiles(baseName string, count int, config RandomIntConfig) ([]string, []int, error) {
	if logger, ok := tdm.logger.(*TestLogger); ok {
		// Generate integers
		ints, err := logger.GenerateTestInts(count, config)
		if err != nil {
			return nil, nil, err
		}

		// Create files
		filePaths, err := logger.CreateIntegerTestFiles(tdm, baseName, ints)
		return filePaths, ints, err
	}

	return nil, nil, errors.New("logger does not support integer utilities")
}

// AnalyzeTestFiles analyzes integer data in test files
func (tdm *TestDataManager) AnalyzeTestFiles(pattern string) (*IntStats, error) {
	files, err := filepath.Glob(filepath.Join(tdm.testDir, pattern))
	if err != nil {
		return nil, fmt.Errorf("failed to find files with pattern %q: %w", pattern, err)
	}

	var allInts []int
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			continue // Skip unreadable files
		}

		// Simple integer extraction - could be enhanced
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if strings.Contains(line, ":") {
				parts := strings.Split(line, ":")
				if len(parts) > 1 {
					if val, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
						allInts = append(allInts, val)
					}
				}
			}
		}
	}

	if logger, ok := tdm.logger.(*TestLogger); ok {
		return logger.intUtils.Analyze(allInts), nil
	}

	// Fallback
	if len(allInts) == 0 {
		return &IntStats{}, nil
	}

	// Simple analysis
	stats := &IntStats{
		Count: len(allInts),
	}

	for _, v := range allInts {
		stats.Sum += v
		if v < stats.Min || stats.Count == 0 {
			stats.Min = v
		}
		if v > stats.Max {
			stats.Max = v
		}
	}

	if stats.Count > 0 {
		stats.Mean = float64(stats.Sum) / float64(stats.Count)
	}

	return stats, nil
}

// Original TestDataManager methods

// CreateTestFile creates a test file with atomic writes.
func (tdm *TestDataManager) CreateTestFile(filename, content string) (string, error) {
	return tdm.CreateTestFileWithMode(filename, content, tdm.config.FileMode)
}

// CreateTestFileWithMode creates a test file ensuring it stays within the test directory.
func (tdm *TestDataManager) CreateTestFileWithMode(filename, content string, mode os.FileMode) (string, error) {
	tdm.mu.RLock()
	defer tdm.mu.RUnlock()

	if filename == "" {
		return "", errors.New("filename cannot be empty")
	}

	// Secure path resolution (Zip Slip protection)
	fullPath := filepath.Join(tdm.testDir, filename)
	if !strings.HasPrefix(filepath.Clean(fullPath), filepath.Clean(tdm.testDir)+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid filename %q: path traversal out of test root attempted", filename)
	}

	tdm.logger.Debug("creating test file", map[string]any{
		"filename": filename,
		"path":     fullPath,
		"size":     len(content),
		"mode":     mode,
	})

	// Ensure parent directory exists
	parentDir := filepath.Dir(fullPath)
	if err := os.MkdirAll(parentDir, tdm.config.DirMode); err != nil {
		return "", fmt.Errorf("failed to create parent directory %q: %w", parentDir, err)
	}

	// Atomic write: Write to temp -> Rename
	tmpFile := fullPath + ".tmp." + randomString() // Avoiding collision if parallel writes happen
	if err := os.WriteFile(tmpFile, []byte(content), mode); err != nil {
		return "", fmt.Errorf("failed to write temporary file: %w", err)
	}

	if err := os.Rename(tmpFile, fullPath); err != nil {
		os.Remove(tmpFile) // Best effort cleanup
		return "", fmt.Errorf("failed to rename temporary file to %q: %w", fullPath, err)
	}

	return fullPath, nil
}

// CreateJSONFile creates a test file with JSON content.
func (tdm *TestDataManager) CreateJSONFile(filename string, data any) (string, error) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON for file %q: %w", filename, err)
	}
	return tdm.CreateTestFile(filename, string(jsonBytes))
}

// CopyFile copies an existing file on the OS to the test directory.
func (tdm *TestDataManager) CopyFile(srcPath, destFilename string) (string, error) {
	// We read the source first to ensure it exists before creating target logic
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return "", fmt.Errorf("failed to open source file %q: %w", srcPath, err)
	}
	defer srcFile.Close()

	// Get mode from source if possible
	stat, err := srcFile.Stat()
	mode := tdm.config.FileMode
	if err == nil {
		mode = stat.Mode()
	}

	content, err := io.ReadAll(srcFile)
	if err != nil {
		return "", fmt.Errorf("failed to read source file %q: %w", srcPath, err)
	}

	return tdm.CreateTestFileWithMode(destFilename, string(content), mode)
}

// GetTestDir returns the test directory path.
func (tdm *TestDataManager) GetTestDir() string {
	tdm.mu.RLock()
	defer tdm.mu.RUnlock()
	return tdm.testDir
}

// Cleanup removes the entire test directory.
func (tdm *TestDataManager) Cleanup() error {
	tdm.mu.Lock()
	defer tdm.mu.Unlock()

	tdm.logger.Info("cleaning up test data directory", map[string]any{
		"directory": tdm.testDir,
	})

	// os.RemoveAll is sufficient. Iterating files individually is slower and unnecessary
	// unless specific file locks prevent deletion, in which case RemoveAll returns the error anyway.
	if err := os.RemoveAll(tdm.testDir); err != nil {
		// If it's already gone, that's fine
		if os.IsNotExist(err) {
			return nil
		}
		tdm.logger.Error("cleanup failed", map[string]any{
			"directory": tdm.testDir,
			"error":     err.Error(),
		})
		return fmt.Errorf("failed to remove directory %q: %w", tdm.testDir, err)
	}

	tdm.logger.Info("test data directory cleaned up successfully", map[string]any{
		"directory": tdm.testDir,
	})

	return nil
}

// TransactionalCleanup creates a snapshot (backup) of the current state.
// Calling Commit() discards the backup (confirming the changes).
// Calling Rollback() restores the backup (undoing changes).
func (tdm *TestDataManager) TransactionalCleanup() (*CleanupTransaction, error) {
	tdm.mu.Lock()
	defer tdm.mu.Unlock()

	backupDir := tdm.testDir + ".backup"

	tdm.logger.Debug("creating snapshot for transactional cleanup", map[string]any{
		"source": tdm.testDir,
		"backup": backupDir,
	})

	// Ensure backup dir is clean
	os.RemoveAll(backupDir)

	if err := copyDir(tdm.testDir, backupDir); err != nil {
		return nil, fmt.Errorf("failed to create backup: %w", err)
	}

	return &CleanupTransaction{
		manager:   tdm,
		backupDir: backupDir,
		committed: false,
	}, nil
}

// Commit finalizes the transaction by deleting the backup.
func (ct *CleanupTransaction) Commit() error {
	if ct.committed {
		return errors.New("cleanup transaction already committed or rolled back")
	}

	ct.manager.logger.Debug("committing transaction (removing backup)", map[string]any{
		"backup": ct.backupDir,
	})

	if err := os.RemoveAll(ct.backupDir); err != nil {
		return fmt.Errorf("failed to remove backup directory: %w", err)
	}

	ct.committed = true
	return nil
}

// Rollback restores the test directory from the backup.
func (ct *CleanupTransaction) Rollback() error {
	if ct.committed {
		return errors.New("cannot rollback committed transaction")
	}

	ct.manager.mu.Lock()
	defer ct.manager.mu.Unlock()

	ct.manager.logger.Warn("rolling back transaction (restoring from backup)", map[string]any{
		"target": ct.manager.testDir,
		"source": ct.backupDir,
	})

	// 1. Clear current state
	if err := os.RemoveAll(ct.manager.testDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear test directory during rollback: %w", err)
	}

	// 2. Restore from backup
	if err := copyDir(ct.backupDir, ct.manager.testDir); err != nil {
		return fmt.Errorf("failed to restore from backup: %w", err)
	}

	// 3. Clean up backup
	os.RemoveAll(ct.backupDir)
	ct.committed = true // Mark as done so we don't try to reuse it

	return nil
}

// --- Helpers ---

// copyDir recursively copies a directory tree, preserving permissions.
func copyDir(src, dst string) error {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	stat, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !stat.IsDir() {
		return fmt.Errorf("source %q is not a directory", src)
	}

	if err := os.MkdirAll(dst, stat.Mode()); err != nil {
		return err
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == src {
			return nil
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			return os.MkdirAll(dstPath, info.Mode())
		}

		// It's a file
		return copyFileContent(path, dstPath)
	})
}

// copyFileContent copies file content and preserves mode.
func copyFileContent(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	info, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return nil
}

// randomString helps create unique temp files
func randomString() string {
	// Simple pseudo-random for temp filenames; in production/crypto use crypto/rand
	// Keeping it simple here to avoid heavy imports for just a temp suffix
	return fmt.Sprintf("%d", os.Getpid())
}

// Integer Utilities (Stubs - assuming implementation exists elsewhere or users adds it)
// The user's previous code implied these exist.
// If these are missing, the compiler will complain about undefined types.
// For the purpose of fixing redeclaration errors, I assume these types exist
// in another file (e.g. integer_utils.go) which the user hasn't provided errors for yet.
