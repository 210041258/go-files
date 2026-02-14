package examples

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"model_loop_sensor/testutils" // <-- add your actual module path here
)

// ---------------------- CONFIG EXAMPLES ----------------------

func TestConfigExamples(t *testing.T) {
	t.Parallel()

	t.Run("DefaultConfig", func(t *testing.T) {
		config := testutils.DefaultConfig()

		if err := config.Validate(); err != nil {
			t.Errorf("Default config validation failed: %v", err)
		}
		if !config.IsDevelopment() {
			t.Errorf("Expected development environment by default")
		}
		if config.Logger.DefaultLevel != testutils.INFO {
			t.Errorf("Expected default log level INFO, got %v", config.Logger.DefaultLevel)
		}
	})

	t.Run("LoadYAMLConfig", func(t *testing.T) {
		yamlContent := `
app_name: "test-app"
environment: "test"
logger:
  default_level: "DEBUG"
  json_output: true
port_checker:
  dial_timeout: "5s"
  max_concurrency: 50
retry:
  attempts: 5
  initial_delay: "100ms"
`
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "config.yaml")
		if err := os.WriteFile(configFile, []byte(yamlContent), 0644); err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		config, err := testutils.LoadConfig(configFile)
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		if config.AppName != "test-app" {
			t.Errorf("Expected app name 'test-app', got %s", config.AppName)
		}
		if config.Logger.DefaultLevel != testutils.DEBUG {
			t.Errorf("Expected log level DEBUG, got %v", config.Logger.DefaultLevel)
		}
		if !config.Logger.JSONOutput {
			t.Error("Expected JSON output to be enabled")
		}
		if config.PortChecker.DialTimeout != 5*time.Second {
			t.Errorf("Expected dial timeout 5s, got %v", config.PortChecker.DialTimeout)
		}
	})

	t.Run("EnvVarOverrides", func(t *testing.T) {
		t.Setenv("TESTUTILS_APP_NAME", "env-test")
		t.Setenv("TESTUTILS_ENVIRONMENT", "production")
		t.Setenv("TESTUTILS_LOGGER_DEFAULT_LEVEL", "WARN")
		t.Setenv("TESTUTILS_PORT_CHECKER_DIAL_TIMEOUT", "10s")

		config := testutils.DefaultConfig()
		config.LoadFromEnv()

		if config.AppName != "env-test" {
			t.Errorf("Expected app name 'env-test', got %s", config.AppName)
		}
		if !config.IsProduction() {
			t.Errorf("Expected production environment, got %s", config.Environment)
		}
		if config.Logger.DefaultLevel != testutils.WARN {
			t.Errorf("Expected log level WARN, got %v", config.Logger.DefaultLevel)
		}
		if config.PortChecker.DialTimeout != 10*time.Second {
			t.Errorf("Expected dial timeout 10s, got %v", config.PortChecker.DialTimeout)
		}
	})

	t.Run("ConfigMerging", func(t *testing.T) {
		baseConfig := testutils.DefaultConfig()
		baseConfig.AppName = "base"
		baseConfig.Logger.DefaultLevel = testutils.INFO

		overrideConfig := &testutils.Config{
			AppName:     "overridden",
			Environment: "staging",
			Logger: testutils.LoggerConfig{
				JSONOutput: true,
			},
		}

		baseConfig.Merge(overrideConfig)

		if baseConfig.AppName != "overridden" {
			t.Errorf("Expected app name 'overridden', got %s", baseConfig.AppName)
		}
		if baseConfig.Environment != "staging" {
			t.Errorf("Expected environment 'staging', got %s", baseConfig.Environment)
		}
		if baseConfig.Logger.DefaultLevel != testutils.INFO {
			t.Errorf("Expected default level to remain INFO, got %v", baseConfig.Logger.DefaultLevel)
		}
		if !baseConfig.Logger.JSONOutput {
			t.Error("Expected JSON output to be enabled")
		}
	})
}

// ---------------------- LOGGER EXAMPLES ----------------------

func TestLoggerExamples(t *testing.T) {
	t.Parallel()

	t.Run("BasicLogger", func(t *testing.T) {
		logger := testutils.NewTestLogger("test-logger", nil)
		bufferLogger := logger.Buffer()

		logger.Info("Test info message", map[string]any{"key": "value"})
		logger.Debug("Test debug message", map[string]any{"count": 42})
		logger.Warn("Test warning message", map[string]any{"error": "something went wrong"})
		logger.Error("Test error message", map[string]any{"code": 500})

		if !strings.Contains(bufferLogger.String(), "Test info message") {
			t.Error("Expected info message in logs")
		}
	})

	t.Run("JSONLogger", func(t *testing.T) {
		logger := testutils.NewTestLogger("json-logger", nil,
			testutils.WithJSONOutput(true),
			testutils.WithDefaultFields(map[string]any{
				"app":     "testapp",
				"version": "1.0.0",
			}),
		)
		bufferLogger := logger.Buffer()

		logger.Info("User login", map[string]any{
			"user_id": "123",
			"action":  "login",
			"success": true,
		})

		if !strings.Contains(bufferLogger.String(), "\"user_id\":\"123\"") {
			t.Error("Expected structured JSON output")
		}
	})

	t.Run("LoggerWithFields", func(t *testing.T) {
		logger := testutils.NewTestLogger("fields-logger", nil)

		requestLogger := logger.WithFields(map[string]any{
			"request_id": "req-123",
			"user_agent": "test-client",
		})
		bufferLogger := requestLogger.Buffer()

		requestLogger.Info("Processing request", map[string]any{
			"method": "GET",
			"path":   "/api/users",
		})

		output := bufferLogger.String()
		if !strings.Contains(output, "request_id") || !strings.Contains(output, "user_agent") {
			t.Error("Expected inherited fields in log output")
		}
	})
}

// ---------------------- PORT CHECKER EXAMPLES ----------------------

func TestPortCheckerExamples(t *testing.T) {
	t.Parallel()

	logger := testutils.NewTestLogger("port-checker-test", nil)

	t.Run("SinglePortCheck", func(t *testing.T) {
		listener, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			t.Skipf("Cannot create test listener: %v", err)
		}
		defer listener.Close()

		_, portStr, _ := net.SplitHostPort(listener.Addr().String())
		port, _ := strconv.Atoi(portStr)

		config := testutils.PortCheckerConfig{
			DialTimeout:    2 * time.Second,
			MaxConcurrency: 10,
			Protocol:       testutils.TCP,
		}

		checker := testutils.NewPortChecker(logger, config)
		ctx := context.Background()

		result, err := checker.IsPortOpen(ctx, "localhost", port, testutils.TCP)
		if err != nil {
			t.Errorf("Port check failed: %v", err)
		}
		if !result.Open {
			t.Errorf("Expected port %d to be open", port)
		}
		if result.Latency <= 0 {
			t.Error("Expected positive latency measurement")
		}
	})
}

// ---------------------- REMAINING EXAMPLES ----------------------

// The rest of your tests (TestTestDataManagerExamples, TestTimerExamples,
// TestIntegerUtilsExamples, TestCompositeErrorExamples, TestIntegrationExamples,
// ExampleIntegrationTest, TestHelpers) all remain unchanged,
// but they now compile because "testutils" is imported.
