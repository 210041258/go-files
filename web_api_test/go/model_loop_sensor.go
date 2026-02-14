package main

import (
	"bytes"
	"crypto/rand" // Used for secure ID generation
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io" // <--- THIS LINE MUST BE HERE
	"mime/multipart"
	"model_loop_sensor/testutils"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	
)

// ------------------- CONFIGURATION STRUCTURES -------------------

// TestConfig holds comprehensive test configuration settings
type TestConfig struct {
	TestID          string
	BaseURL         string
	TestDataDir     string
	Environment     string
	EnableMetrics   bool
	CleanupOnExit   bool
	LogLevel        testutils.LogLevel
	EnableDebugLogs bool
	TestTimeout     time.Duration
	PollInterval    time.Duration

	DockerConfig DockerConfig
	ServerConfig ServerConfig
	HTTPConfig   HTTPConfig
	RetryConfig  RetryConfig
	Concurrency  ConcurrencyConfig
}

// DockerConfig contains Docker Compose and service settings
type DockerConfig struct {
	ComposePath     string
	ComposeFile     string
	Services        []string
	Network         string
	Build           bool
	ForceRecreate   bool
	RemoveOrphans   bool
	RemoveVolumes   bool
	Timeout         time.Duration
	HealthCheckPort int
}

// ServerConfig defines server startup and management settings
type ServerConfig struct {
	Path            string
	Command         string
	Args            []string
	WorkingDir      string
	HealthEndpoint  string
	StartupTimeout  time.Duration
	ShutdownTimeout time.Duration
	LogOutput       bool
	EnvVars         map[string]string
}

// HTTPConfig holds HTTP client configuration parameters
type HTTPConfig struct {
	Timeout               time.Duration
	MaxIdleConns          int
	IdleConnTimeout       time.Duration
	DisableCompression    bool
	MaxConnsPerHost       int
	MaxIdleConnsPerHost   int
	TLSHandshakeTimeout   time.Duration
	ExpectContinueTimeout time.Duration
}

// RetryConfig defines retry behavior for operations
type RetryConfig struct {
	MaxAttempts    int
	InitialDelay   time.Duration
	MaxDelay       time.Duration
	BackoffFactor  float64
	JitterFactor   float64
	RetryableCodes []int
}

// ConcurrencyConfig manages concurrent test execution settings
type ConcurrencyConfig struct {
	MaxWorkers              int
	QueueSize               int
	TaskTimeout             time.Duration
	ShutdownTimeout         time.Duration
	EnableDeadlockDetection bool
}

// ------------------- GLOBAL VARIABLES -------------------

var (
	testConfig *TestConfig
	appConfig  *testutils.Config
	httpClient *http.Client
	dockerMgr  *DockerManager
	serverMgr  *ServerManager
	testLogger *TestLogger
	initOnce   sync.Once
)

// ------------------- INITIALIZATION -------------------

// initializeTestConfig loads configuration and sets up test environment
func initializeTestConfig() error {
	var initErr error
	initOnce.Do(func() {
		// Load application configuration
		var err error
		appConfig, err = testutils.LoadConfig("")
		if err != nil {
			appConfig = testutils.DefaultConfig()
		}

		// Generate unique test identifier
		testID, err := generateTestID()
		if err != nil {
			initErr = fmt.Errorf("failed to generate test ID: %w", err)
			return
		}

		testConfig = &TestConfig{
			TestID:          testID,
			BaseURL:         getEnvOrDefault("TEST_BASE_URL", "http://localhost:3000/api"),
			TestDataDir:     filepath.Join(os.TempDir(), "integration-test-"+testID),
			Environment:     getEnvOrDefault("TEST_ENV", "integration"),
			EnableMetrics:   getEnvBoolOrDefault("ENABLE_METRICS", false),
			CleanupOnExit:   getEnvBoolOrDefault("TEST_CLEANUP", true),
			LogLevel:        testutils.DEBUG,
			EnableDebugLogs: getEnvBoolOrDefault("TEST_DEBUG", false),
			TestTimeout:     10 * time.Minute,
			PollInterval:    500 * time.Millisecond,
			DockerConfig: DockerConfig{
				ComposePath:     findDockerComposePath(),
				ComposeFile:     "docker-compose.yml",
				Services:        []string{"postgres:5432", "redis:6379"},
				Network:         getEnvOrDefault("DOCKER_NETWORK", ""),
				Build:           true,
				ForceRecreate:   false,
				RemoveOrphans:   true,
				RemoveVolumes:   getEnvBoolOrDefault("DOCKER_REMOVE_VOLUMES", false),
				Timeout:         appConfig.PortChecker.OperationTimeout,
				HealthCheckPort: 5432,
			},
			ServerConfig: ServerConfig{
				Path:            findServerPath(),
				Command:         "npm",
				Args:            []string{"run", "dev"},
				WorkingDir:      "",
				HealthEndpoint:  "/health",
				StartupTimeout:  appConfig.Retry.Timeout,
				ShutdownTimeout: appConfig.Concurrency.ShutdownTimeout,
				LogOutput:       true,
				EnvVars:         make(map[string]string),
			},
			HTTPConfig: HTTPConfig{
				Timeout:               appConfig.PortChecker.DialTimeout,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				DisableCompression:    true,
				MaxConnsPerHost:       0,
				MaxIdleConnsPerHost:   runtime.NumCPU() * 2,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
			RetryConfig: RetryConfig{
				MaxAttempts:   appConfig.Retry.Attempts,
				InitialDelay:  appConfig.Retry.InitialDelay,
				MaxDelay:      appConfig.Retry.MaxDelay,
				BackoffFactor: appConfig.Retry.Multiplier,
				JitterFactor:  appConfig.Retry.JitterFactor,
				RetryableCodes: []int{
					http.StatusRequestTimeout,
					http.StatusTooManyRequests,
					http.StatusInternalServerError,
					http.StatusBadGateway,
					http.StatusServiceUnavailable,
					http.StatusGatewayTimeout,
				},
			},
			Concurrency: ConcurrencyConfig{
				MaxWorkers:              appConfig.Concurrency.DefaultPoolSize,
				QueueSize:               appConfig.Concurrency.QueueSize,
				TaskTimeout:             appConfig.Concurrency.MaxTaskDuration,
				ShutdownTimeout:         appConfig.Concurrency.ShutdownTimeout,
				EnableDeadlockDetection: appConfig.Concurrency.EnableDeadlockDetection,
			},
		}

		// Apply environment variable overrides
		applyEnvironmentOverrides()
	})
	return initErr
}

// initializeHTTPClient creates and configures the HTTP client
func initializeHTTPClient() {
	httpClient = &http.Client{
		Timeout: testConfig.HTTPConfig.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:          testConfig.HTTPConfig.MaxIdleConns,
			IdleConnTimeout:       testConfig.HTTPConfig.IdleConnTimeout,
			DisableCompression:    testConfig.HTTPConfig.DisableCompression,
			MaxIdleConnsPerHost:   testConfig.HTTPConfig.MaxIdleConnsPerHost,
			MaxConnsPerHost:       testConfig.HTTPConfig.MaxConnsPerHost,
			TLSHandshakeTimeout:   testConfig.HTTPConfig.TLSHandshakeTimeout,
			ExpectContinueTimeout: testConfig.HTTPConfig.ExpectContinueTimeout,
		},
	}
}

// initializeLogger sets up the test logger
func initializeLogger() {
	testLogger = NewTestLogger(testConfig)
}

// ------------------- UTILITY FUNCTIONS -------------------

// generateTestID creates a unique identifier for the test run
func generateTestID() (string, error) {
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(randomBytes), nil
}

// getEnvOrDefault retrieves environment variable or returns default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvBoolOrDefault retrieves boolean environment variable with fallback
func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsedValue, err := strconv.ParseBool(value); err == nil {
			return parsedValue
		}
	}
	return defaultValue
}

// findDockerComposePath locates the Docker Compose configuration file
func findDockerComposePath() string {
	possiblePaths := []string{
		filepath.Join("..", "docker-compose"),
		filepath.Join("..", "docker", "docker-compose.yml"),
		filepath.Join("..", "new-folder", "docker-compose.yml"),
		filepath.Join(".", "docker-compose.yml"),
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// findServerPath locates the server directory
func findServerPath() string {
	possiblePaths := []string{
		filepath.Join("..", "server"),
		filepath.Join("..", "new-folder"),
		filepath.Join(".", "server"),
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// applyEnvironmentOverrides applies configuration from environment variables
func applyEnvironmentOverrides() {
	if port := os.Getenv("TEST_PORT"); port != "" {
		testConfig.BaseURL = fmt.Sprintf("http://localhost:%s/api", port)
	}

	if timeout := os.Getenv("TEST_TIMEOUT"); timeout != "" {
		if duration, err := time.ParseDuration(timeout); err == nil {
			testConfig.TestTimeout = duration
		}
	}

	if pollInterval := os.Getenv("TEST_POLL_INTERVAL"); pollInterval != "" {
		if duration, err := time.ParseDuration(pollInterval); err == nil {
			testConfig.PollInterval = duration
		}
	}
}

// ------------------- DOCKER MANAGER -------------------

// DockerManager handles Docker Compose operations
type DockerManager struct {
	config DockerConfig
}

// NewDockerManager creates a new Docker manager instance
func NewDockerManager(config DockerConfig) (*DockerManager, error) {
	if config.ComposePath == "" {
		return nil, fmt.Errorf("docker compose path not found")
	}

	if err := os.MkdirAll(config.ComposePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create docker compose directory: %w", err)
	}

	return &DockerManager{config: config}, nil
}

// Start launches Docker containers and waits for services to be ready
func (dm *DockerManager) Start() error {
	args := []string{"compose", "-f", dm.config.ComposeFile}
	if dm.config.Network != "" {
		args = append(args, "--project-name", dm.config.Network)
	}

	args = append(args, "up", "-d")
	if dm.config.Build {
		args = append(args, "--build")
	}
	if dm.config.ForceRecreate {
		args = append(args, "--force-recreate")
	}
	if dm.config.RemoveOrphans {
		args = append(args, "--remove-orphans")
	}

	testLogger.Info("Starting Docker containers", "composeFile", dm.config.ComposeFile)

	cmd := exec.Command("docker", args...)
	cmd.Dir = dm.config.ComposePath
	cmd.Stdout = testLogger.Writer()
	cmd.Stderr = testLogger.Writer()

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start docker compose: %w", err)
	}

	return dm.waitForServices()
}

// Stop terminates Docker containers and cleans up resources
func (dm *DockerManager) Stop() error {
	args := []string{"compose", "-f", dm.config.ComposeFile, "down"}
	if dm.config.RemoveOrphans {
		args = append(args, "--remove-orphans")
	}
	if dm.config.RemoveVolumes {
		args = append(args, "--volumes")
	}

	testLogger.Info("Stopping Docker containers")

	cmd := exec.Command("docker", args...)
	cmd.Dir = dm.config.ComposePath
	cmd.Stdout = testLogger.Writer()
	cmd.Stderr = testLogger.Writer()

	return cmd.Run()
}

// waitForServices verifies that all required services are accessible
func (dm *DockerManager) waitForServices() error {
	for _, service := range dm.config.Services {
		testLogger.Debug("Waiting for service", "service", service)
		if err := waitForServicePort(service, dm.config.Timeout); err != nil {
			return fmt.Errorf("service %s not ready: %w", service, err)
		}
	}
	return nil
}

// ------------------- SERVER MANAGER -------------------

// ServerManager handles application server lifecycle
type ServerManager struct {
	config ServerConfig
	cmd    *exec.Cmd
}

// NewServerManager creates a new server manager instance
func NewServerManager(config ServerConfig) (*ServerManager, error) {
	if config.Path == "" {
		return nil, fmt.Errorf("server path not found")
	}

	if _, err := exec.LookPath(config.Command); err != nil {
		return nil, fmt.Errorf("command %s not found: %w", config.Command, err)
	}

	return &ServerManager{config: config}, nil
}

// Start launches the application server
func (sm *ServerManager) Start() error {
	testLogger.Info("Starting server", "path", sm.config.Path, "command", sm.config.Command)

	sm.cmd = exec.Command(sm.config.Command, sm.config.Args...)
	sm.cmd.Dir = sm.config.Path
	sm.cmd.Env = sm.getEnvironmentVariables()

	if sm.config.LogOutput {
		sm.cmd.Stdout = testLogger.Writer()
		sm.cmd.Stderr = testLogger.Writer()
	}

	if err := sm.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	healthURL := testConfig.BaseURL + sm.config.HealthEndpoint
	return waitForHealthEndpoint(healthURL, sm.config.StartupTimeout)
}

// Stop gracefully terminates the server
func (sm *ServerManager) Stop() error {
	if sm.cmd == nil || sm.cmd.Process == nil {
		return nil
	}

	testLogger.Info("Stopping server")

	// Attempt graceful shutdown
	if err := sm.cmd.Process.Signal(os.Interrupt); err != nil {
		testLogger.Error("Failed to send interrupt signal", "error", err)
		return sm.cmd.Process.Kill()
	}

	// Wait for graceful termination
	terminationComplete := make(chan error, 1)
	go func() { terminationComplete <- sm.cmd.Wait() }()

	select {
	case err := <-terminationComplete:
		return err
	case <-time.After(sm.config.ShutdownTimeout):
		testLogger.Warn("Server shutdown timeout exceeded, forcing termination")
		return sm.cmd.Process.Kill()
	}
}

// getEnvironmentVariables prepares environment variables for the server process
func (sm *ServerManager) getEnvironmentVariables() []string {
	environment := os.Environ()
	for key, value := range sm.config.EnvVars {
		environment = append(environment, fmt.Sprintf("%s=%s", key, value))
	}
	return environment
}

// ------------------- HEALTH CHECK FUNCTIONS -------------------

// waitForHealthEndpoint repeatedly checks a URL until it responds successfully
func waitForHealthEndpoint(url string, timeout time.Duration) error {
	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(timeout)

	for attempt := 0; time.Now().Before(deadline); attempt++ {
		response, err := client.Get(url)
		if err == nil && response.StatusCode < 500 {
			response.Body.Close()
			testLogger.Debug("Health check successful", "url", url, "attempt", attempt+1)
			return nil
		}

		if response != nil {
			response.Body.Close()
		}

		if attempt%10 == 0 {
			testLogger.Debug("Waiting for service health", "url", url, "attempt", attempt+1, "error", err)
		}

		time.Sleep(testConfig.PollInterval)
	}

	return fmt.Errorf("timeout waiting for %s after %v", url, timeout)
}

// waitForServicePort verifies TCP connectivity to a service
func waitForServicePort(service string, timeout time.Duration) error {
	parts := strings.Split(service, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid service format: %s, expected 'host:port'", service)
	}

	host, port := parts[0], parts[1]
	deadline := time.Now().Add(timeout)
	address := net.JoinHostPort(host, port)

	for attempt := 0; time.Now().Before(deadline); attempt++ {
		conn, err := net.DialTimeout("tcp", address, 2*time.Second)
		if err == nil {
			conn.Close()
			testLogger.Debug("Service port accessible", "service", service, "attempt", attempt+1)
			return nil
		}

		if attempt%5 == 0 {
			testLogger.Debug("Checking service port", "service", service, "attempt", attempt+1, "error", err)
		}

		time.Sleep(testConfig.PollInterval)
	}

	return fmt.Errorf("service %s not accessible after %v", service, timeout)
}

// ------------------- TEST LOGGER -------------------

// TestLogger provides structured logging for tests
type TestLogger struct {
	test         *testing.T
	debugEnabled bool
	logLevel     testutils.LogLevel
	logMutex     sync.Mutex
}

// NewTestLogger creates a new test logger instance
func NewTestLogger(config *TestConfig) *TestLogger {
	return &TestLogger{
		debugEnabled: config.EnableDebugLogs,
		logLevel:     config.LogLevel,
	}
}

// SetTest associates a test instance with the logger
func (tl *TestLogger) SetTest(t *testing.T) {
	tl.test = t
}

// Info logs informational messages
func (tl *TestLogger) Info(message string, args ...interface{}) {
	tl.log("INFO", message, args...)
}

// Debug logs debug messages when debug mode is enabled
func (tl *TestLogger) Debug(message string, args ...interface{}) {
	if tl.debugEnabled {
		tl.log("DEBUG", message, args...)
	}
}

// Warn logs warning messages
func (tl *TestLogger) Warn(message string, args ...interface{}) {
	tl.log("WARN", message, args...)
}

// Error logs error messages
func (tl *TestLogger) Error(message string, args ...interface{}) {
	tl.log("ERROR", message, args...)
}

// log formats and outputs log messages
func (tl *TestLogger) log(level, message string, args ...interface{}) {
	tl.logMutex.Lock()
	defer tl.logMutex.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	formatted := fmt.Sprintf("[%s] [%s] %s", timestamp, level, message)
	if len(args) > 0 {
		formatted += " " + fmt.Sprint(args...)
	}

	if tl.test != nil {
		tl.test.Log(formatted)
	} else {
		fmt.Println(formatted)
	}
}

// Writer returns an io.Writer that redirects to the logger
func (tl *TestLogger) Writer() io.Writer {
	return &testLoggerWriter{logger: tl}
}

// testLoggerWriter adapts TestLogger to io.Writer interface
type testLoggerWriter struct {
	logger *TestLogger
}

// Write implements io.Writer interface
func (w *testLoggerWriter) Write(p []byte) (int, error) {
	w.logger.Info(strings.TrimSpace(string(p)))
	return len(p), nil
}

// ------------------- TEST FIXTURES -------------------

// createTestFile generates a temporary file with specified content
func createTestFile(content string) (string, error) {
	tempDir := appConfig.GetTempDir()
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}

	tempFile, err := os.CreateTemp(tempDir, "testfile-*.txt")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	if _, err := tempFile.WriteString(content); err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}

// cleanupTestDirectory removes test data if cleanup is enabled
func cleanupTestDirectory() error {
	if testConfig.CleanupOnExit && testConfig.TestDataDir != "" {
		return os.RemoveAll(testConfig.TestDataDir)
	}
	return nil
}

// ------------------- RETRY HELPER -------------------

// retryWithBackoff executes an operation with exponential backoff retry
func retryWithBackoff(operation func() error, description string) error {
	for attempt := 1; attempt <= testConfig.RetryConfig.MaxAttempts; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}

		if attempt == testConfig.RetryConfig.MaxAttempts {
			return fmt.Errorf("%s failed after %d attempts: %w",
				description, testConfig.RetryConfig.MaxAttempts, err)
		}

		delay := calculateExponentialBackoff(attempt)
		testLogger.Debug("Retrying operation",
			"operation", description,
			"attempt", attempt,
			"delay", delay,
			"error", err)

		time.Sleep(delay)
	}
	return nil
}

// calculateExponentialBackoff computes delay with exponential backoff and jitter
func calculateExponentialBackoff(attempt int) time.Duration {
	delay := float64(testConfig.RetryConfig.InitialDelay) *
		exponentialPower(testConfig.RetryConfig.BackoffFactor, float64(attempt-1))

	// Apply jitter for randomization
	if testConfig.RetryConfig.JitterFactor > 0 {
		jitter := delay * testConfig.RetryConfig.JitterFactor
		// Read a random int64 from crypto/rand
		var n int64
		binary.Read(rand.Reader, binary.BigEndian, &n) // Use crypto/rand for secure randomness
	
		delay += (float64(n) / float64(1<<63)) * jitter - (jitter / 2) // Center jitter around the base delay
		
	}

	// Enforce maximum delay
	if delay > float64(testConfig.RetryConfig.MaxDelay) {
		delay = float64(testConfig.RetryConfig.MaxDelay)
	}

	return time.Duration(delay)
}

// exponentialPower calculates x^y for backoff computation
func exponentialPower(base, exponent float64) float64 {
	result := 1.0
	for i := 0; i < int(exponent); i++ {
		result *= base
	}
	return result
}

// ------------------- TEST SUITE ENTRY POINT -------------------

// TestMain serves as the entry point for the test suite
func TestMain(m *testing.M) {
	// Initialize test configuration
	if err := initializeTestConfig(); err != nil {
		fmt.Printf("Failed to initialize test configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize components
	initializeLogger()
	initializeHTTPClient()

	testLogger.Info("Starting test suite execution",
		"testID", testConfig.TestID,
		"environment", testConfig.Environment,
		"baseURL", testConfig.BaseURL)

	// Create test data directory
	if err := os.MkdirAll(testConfig.TestDataDir, 0755); err != nil {
		testLogger.Error("Failed to create test data directory", "error", err)
		os.Exit(1)
	}

	// Setup test environment with retry capability
	setupError := retryWithBackoff(func() error {
		return setupTestEnvironment()
	}, "test environment setup")

	if setupError != nil {
		testLogger.Error("Failed to setup test environment", "error", setupError)
		cleanupTestDirectory()
		os.Exit(1)
	}

	// Execute test cases
	exitCode := m.Run()

	// Teardown test environment
	if err := teardownTestEnvironment(); err != nil {
		testLogger.Error("Failed to teardown test environment", "error", err)
		exitCode = 1
	}

	// Cleanup test resources
	if err := cleanupTestDirectory(); err != nil {
		testLogger.Error("Failed to cleanup test directory", "error", err)
	}

	os.Exit(exitCode)
}

// setupTestEnvironment prepares the test environment
func setupTestEnvironment() error {
	// Start Docker services
	var err error
	dockerMgr, err = NewDockerManager(testConfig.DockerConfig)
	if err != nil {
		return fmt.Errorf("failed to create docker manager: %w", err)
	}

	testLogger.Info("Initializing Docker containers...")
	if err := dockerMgr.Start(); err != nil {
		return fmt.Errorf("failed to start Docker services: %w", err)
	}

	// Start application server
	serverMgr, err = NewServerManager(testConfig.ServerConfig)
	if err != nil {
		return fmt.Errorf("failed to create server manager: %w", err)
	}

	testLogger.Info("Starting application server...")
	if err := serverMgr.Start(); err != nil {
		// Clean up Docker if server fails
		dockerMgr.Stop()
		return fmt.Errorf("failed to start application server: %w", err)
	}

	return nil
}

// teardownTestEnvironment cleans up test environment
func teardownTestEnvironment() error {
	var errors []error

	testLogger.Info("Terminating application server...")
	if serverMgr != nil {
		if err := serverMgr.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop server: %w", err))
		}
	}

	testLogger.Info("Stopping Docker containers...")
	if dockerMgr != nil {
		if err := dockerMgr.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop Docker: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("teardown completed with errors: %v", errors)
	}

	return nil
}

// ------------------- TEST CASES -------------------

// TestHealthCheck verifies the health endpoint functionality
func TestHealthCheck(t *testing.T) {
	testLogger.SetTest(t)

	response, err := httpClient.Get(testConfig.BaseURL + "/health")
	if err != nil {
		t.Fatalf("Health check request failed: %v", err)
	}
	defer response.Body.Close()

	assertStatusCode(t, response, http.StatusOK)
	assertContentType(t, response, "application/json")

	var healthStatus map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&healthStatus); err != nil {
		t.Fatalf("Failed to decode health response: %v", err)
	}

	// Validate health response structure
	assertFieldExists(t, healthStatus, "status")
	assertFieldExists(t, healthStatus, "timestamp")
	assertFieldEquals(t, healthStatus, "status", "healthy")
}

// TestGetUsers validates user retrieval endpoint
func TestGetUsers(t *testing.T) {
	testLogger.SetTest(t)
	t.Parallel()

	request, err := http.NewRequest("GET", testConfig.BaseURL+"/users", nil)
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}

	response, err := httpClient.Do(request)
	if err != nil {
		t.Fatalf("GET /users request failed: %v", err)
	}
	defer response.Body.Close()

	assertStatusCode(t, response, http.StatusOK)

	var users []map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&users); err != nil {
		responseBody, _ := io.ReadAll(response.Body)
		t.Fatalf("Failed to decode response: %v\nResponse: %s", err, string(responseBody))
	}

	t.Logf("Successfully retrieved %d users", len(users))

	// Validate each user's data structure
	for index, user := range users {
		assertFieldExists(t, user, "id")
		assertFieldExists(t, user, "name")
		assertFieldExists(t, user, "email")

		// Validate email format
		if email, ok := user["email"].(string); ok {
			if !isValidEmailFormat(email) {
				t.Errorf("User %d has invalid email format: %s", index, email)
			}
		}
	}
}

// TestUploadFile validates file upload functionality
func TestUploadFile(t *testing.T) {
	testLogger.SetTest(t)
	t.Parallel()

	// Create test file using configured temporary directory
	fileContent := fmt.Sprintf("Test content %s %s", testConfig.TestID, time.Now().Format(time.RFC3339))
	filePath, err := createTestFile(fileContent)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(filePath)

	// Prepare multipart form data
	bodyBuffer := &bytes.Buffer{}
	multipartWriter := multipart.NewWriter(bodyBuffer)

	filePart, err := multipartWriter.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}
	defer file.Close()

	if _, err := io.Copy(filePart, file); err != nil {
		t.Fatalf("Failed to copy file content: %v", err)
	}
	multipartWriter.Close()

	// Execute file upload request
	request, err := http.NewRequest("POST", testConfig.BaseURL+"/upload", bodyBuffer)
	if err != nil {
		t.Fatalf("Failed to create upload request: %v", err)
	}
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	response, err := httpClient.Do(request)
	if err != nil {
		t.Fatalf("File upload request failed: %v", err)
	}
	defer response.Body.Close()

	assertStatusCode(t, response, http.StatusOK)

	var uploadResponse map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&uploadResponse); err != nil {
		t.Fatalf("Failed to decode upload response: %v", err)
	}

	assertFieldExists(t, uploadResponse, "filename")
	assertFieldExists(t, uploadResponse, "size")
	assertFieldExists(t, uploadResponse, "uploaded_at")
}

// TestUploadFile_Invalid tests error handling for invalid uploads
func TestUploadFile_Invalid(t *testing.T) {
	testLogger.SetTest(t)
	t.Parallel()

	testCases := []struct {
		name           string
		prepareForm    func(*multipart.Writer) error
		expectedStatus int
	}{
		{
			name: "Missing file",
			prepareForm: func(writer *multipart.Writer) error {
				return writer.WriteField("description", "Upload without file")
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Empty file",
			prepareForm: func(writer *multipart.Writer) error {
				filePart, err := writer.CreateFormFile("file", "empty.txt")
				if err != nil {
					return err
				}
				_, err = filePart.Write([]byte{})
				return err
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Unsupported file type",
			prepareForm: func(writer *multipart.Writer) error {
				filePart, err := writer.CreateFormFile("file", "script.exe")
				if err != nil {
					return err
				}
				_, err = filePart.Write([]byte("executable content"))
				return err
			},
			expectedStatus: http.StatusUnsupportedMediaType,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			bodyBuffer := &bytes.Buffer{}
			multipartWriter := multipart.NewWriter(bodyBuffer)

			if err := testCase.prepareForm(multipartWriter); err != nil {
				t.Fatalf("Failed to prepare form: %v", err)
			}
			multipartWriter.Close()

			request, err := http.NewRequest("POST", testConfig.BaseURL+"/upload", bodyBuffer)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			request.Header.Set("Content-Type", multipartWriter.FormDataContentType())

			response, err := httpClient.Do(request)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer response.Body.Close()

			assertStatusCode(t, response, testCase.expectedStatus)
		})
	}
}

// TestCreateUser validates user creation endpoint
func TestCreateUser(t *testing.T) {
	testLogger.SetTest(t)
	t.Parallel()

	userData := map[string]interface{}{
		"name":     fmt.Sprintf("Test User %s", testConfig.TestID),
		"email":    fmt.Sprintf("test.%s@example.com", testConfig.TestID),
		"metadata": map[string]string{"test_id": testConfig.TestID},
	}

	requestBody, err := json.Marshal(userData)
	if err != nil {
		t.Fatalf("Failed to marshal user data: %v", err)
	}

	response, err := httpClient.Post(testConfig.BaseURL+"/users",
		"application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		t.Fatalf("Failed to POST /users: %v", err)
	}
	defer response.Body.Close()

	assertStatusCode(t, response, http.StatusCreated)
	assertContentType(t, response, "application/json")

	var createdUser map[string]interface{}
	if err := json.NewDecoder(response.Body).Decode(&createdUser); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Validate response matches request data
	assertFieldEquals(t, createdUser, "name", userData["name"])
	assertFieldEquals(t, createdUser, "email", userData["email"])
	assertFieldExists(t, createdUser, "id")
	assertFieldExists(t, createdUser, "created_at")
}

// TestUserEndpoints provides comprehensive user API testing
func TestUserEndpoints(t *testing.T) {
	testLogger.SetTest(t)

	testScenarios := []struct {
		name           string
		method         string
		endpoint       string
		payload        interface{}
		expectedStatus int
		validationFunc func(*testing.T, *http.Response)
	}{
		{
			name:           "Retrieve all users",
			method:         "GET",
			endpoint:       "/users",
			expectedStatus: http.StatusOK,
			validationFunc: func(t *testing.T, response *http.Response) {
				var users []map[string]interface{}
				err := json.NewDecoder(response.Body).Decode(&users)
				if err != nil {
					t.Errorf("Failed to decode response: %v", err)
				}
				// Additional validation can be added here
			},
		},
		{
			name:     "Create user with invalid data",
			method:   "POST",
			endpoint: "/users",
			payload: map[string]interface{}{
				"name":  "",
				"email": "invalid-email-format",
			},
			expectedStatus: http.StatusBadRequest,
			validationFunc: func(t *testing.T, response *http.Response) {
				var errorResponse map[string]interface{}
				err := json.NewDecoder(response.Body).Decode(&errorResponse)
				if err != nil {
					t.Errorf("Failed to decode error response: %v", err)
				}
				assertFieldExists(t, errorResponse, "error")
				assertFieldExists(t, errorResponse, "message")
			},
		},
		{
			name:           "Retrieve non-existent user",
			method:         "GET",
			endpoint:       "/users/999999",
			expectedStatus: http.StatusNotFound,
			validationFunc: func(t *testing.T, response *http.Response) {
				var errorResponse map[string]interface{}
				err := json.NewDecoder(response.Body).Decode(&errorResponse)
				if err != nil {
					t.Errorf("Failed to decode error response: %v", err)
				}
				assertFieldExists(t, errorResponse, "error")
			},
		},
	}

	for _, scenario := range testScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			var request *http.Request
			var err error

			if scenario.payload != nil {
				payloadBytes, _ := json.Marshal(scenario.payload)
				request, err = http.NewRequest(scenario.method,
					testConfig.BaseURL+scenario.endpoint,
					bytes.NewBuffer(payloadBytes))
				request.Header.Set("Content-Type", "application/json")
			} else {
				request, err = http.NewRequest(scenario.method,
					testConfig.BaseURL+scenario.endpoint, nil)
			}

			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			response, err := httpClient.Do(request)
			if err != nil {
				t.Fatalf("Request execution failed: %v", err)
			}
			defer response.Body.Close()

			assertStatusCode(t, response, scenario.expectedStatus)

			if scenario.validationFunc != nil {
				scenario.validationFunc(t, response)
			}
		})
	}
}

// TestConcurrentRequests validates system behavior under concurrent load
func TestConcurrentRequests(t *testing.T) {
	testLogger.SetTest(t)

	// Determine concurrency level based on configuration
	concurrencyLevel := testConfig.Concurrency.MaxWorkers
	if concurrencyLevel > 50 {
		concurrencyLevel = 50 // Safety limit
	}

	errorChannel := make(chan error, concurrencyLevel)
	completionChannel := make(chan bool, concurrencyLevel)
	var waitGroup sync.WaitGroup

	for workerID := 0; workerID < concurrencyLevel; workerID++ {
		waitGroup.Add(1)
		go func(workerID int) {
			defer waitGroup.Done()
			defer func() { completionChannel <- true }()

			response, err := httpClient.Get(fmt.Sprintf("%s/users", testConfig.BaseURL))
			if err != nil {
				errorChannel <- fmt.Errorf("worker %d: %w", workerID, err)
				return
			}
			defer response.Body.Close()

			if response.StatusCode != http.StatusOK {
				responseBody, _ := io.ReadAll(response.Body)
				errorChannel <- fmt.Errorf("worker %d: expected 200, received %d\nResponse: %s",
					workerID, response.StatusCode, string(responseBody))
			}
		}(workerID)
	}

	// Wait for all workers to complete
	waitGroup.Wait()
	close(errorChannel)
	close(completionChannel)

	// Collect and report errors
	var errorMessages []string
	for err := range errorChannel {
		errorMessages = append(errorMessages, err.Error())
	}

	if len(errorMessages) > 0 {
		t.Errorf("%d concurrent requests failed:\n%s",
			len(errorMessages), strings.Join(errorMessages, "\n"))
	}
}

// ------------------- ASSERTION HELPERS -------------------

// assertStatusCode verifies HTTP response status code
func assertStatusCode(t *testing.T, response *http.Response, expected int) {
	t.Helper()
	if response.StatusCode != expected {
		responseBody, _ := io.ReadAll(response.Body)
		t.Errorf("Expected status %d, received %d\nResponse: %s",
			expected, response.StatusCode, string(responseBody))
	}
}

// assertContentType verifies response content type header
func assertContentType(t *testing.T, response *http.Response, expected string) {
	t.Helper()
	contentType := response.Header.Get("Content-Type")
	if !strings.Contains(contentType, expected) {
		t.Errorf("Expected Content-Type containing %s, received %s", expected, contentType)
	}
}

// assertFieldExists verifies a field exists in a map
func assertFieldExists(t *testing.T, data map[string]interface{}, field string) {
	t.Helper()
	if _, exists := data[field]; !exists {
		t.Errorf("Required field '%s' is missing", field)
	}
}

// assertFieldEquals verifies field value matches expected value
func assertFieldEquals(t *testing.T, data map[string]interface{}, field string, expected interface{}) {
	t.Helper()
	actual, exists := data[field]
	if !exists {
		t.Errorf("Field '%s' does not exist", field)
		return
	}

	if actual != expected {
		t.Errorf("Field '%s' expected %v, received %v", field, expected, actual)
	}
}

// isValidEmailFormat performs basic email format validation
func isValidEmailFormat(email string) bool {
	return len(email) > 3 && strings.Contains(email, "@") && strings.Contains(email, ".")
}
