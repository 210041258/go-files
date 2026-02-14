package testutils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// LogLevel represents the logging level
type LogLevel int

const (
	TRACE LogLevel = iota
	DEBUG
	INFO
	WARN
	ERROR
	FATAL
)

// Logger defines the interface required for logging.
type Logger interface {
	Info(msg string, keyvals map[string]any)
	Debug(msg string, keyvals map[string]any)
	Warn(msg string, keyvals map[string]any)
	Error(msg string, keyvals map[string]any)
}

// String implements the Stringer interface for readable logs
func (l LogLevel) String() string {
	switch l {
	case TRACE:
		return "TRACE"
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// ParseLogLevel parses a string into LogLevel
func ParseLogLevel(s string) (LogLevel, error) {
	switch strings.ToUpper(s) {
	case "TRACE":
		return TRACE, nil
	case "DEBUG":
		return DEBUG, nil
	case "INFO":
		return INFO, nil
	case "WARN":
		return WARN, nil
	case "ERROR":
		return ERROR, nil
	case "FATAL":
		return FATAL, nil
	default:
		return INFO, fmt.Errorf("unknown log level: %s", s)
	}
}

// UnmarshalText implements encoding.TextUnmarshaler for JSON/YAML string parsing
func (l *LogLevel) UnmarshalText(text []byte) error {
	level, err := ParseLogLevel(string(text))
	if err != nil {
		return err
	}
	*l = level
	return nil
}

// MarshalText implements encoding.TextMarshaler
func (l LogLevel) MarshalText() ([]byte, error) {
	return []byte(l.String()), nil
}

// Protocol represents network protocol
type Protocol string

const (
	TCP  Protocol = "tcp"
	TCP4 Protocol = "tcp4"
	TCP6 Protocol = "tcp6"
	UDP  Protocol = "udp"
	UDP4 Protocol = "udp4"
	UDP6 Protocol = "udp6"
)

// String returns string representation
func (p Protocol) String() string {
	return string(p)
}

// ParseProtocol parses string to Protocol
func ParseProtocol(s string) (Protocol, error) {
	switch strings.ToLower(s) {
	case "tcp":
		return TCP, nil
	case "tcp4":
		return TCP4, nil
	case "tcp6":
		return TCP6, nil
	case "udp":
		return UDP, nil
	case "udp4":
		return UDP4, nil
	case "udp6":
		return UDP6, nil
	default:
		return TCP, fmt.Errorf("unknown protocol: %s", s)
	}
}

// IPVersion represents IP version preference
type IPVersion int

const (
	IPv4  IPVersion = 4
	IPv6  IPVersion = 6
	AnyIP IPVersion = 0
)

// String returns string representation
func (v IPVersion) String() string {
	switch v {
	case IPv4:
		return "IPv4"
	case IPv6:
		return "IPv6"
	case AnyIP:
		return "Any"
	default:
		return fmt.Sprintf("IPVersion(%d)", v)
	}
}

// ParseIPVersion parses string to IPVersion
func ParseIPVersion(s string) (IPVersion, error) {
	switch strings.ToLower(s) {
	case "ipv4", "4":
		return IPv4, nil
	case "ipv6", "6":
		return IPv6, nil
	case "any", "auto", "":
		return AnyIP, nil
	default:
		return AnyIP, fmt.Errorf("unknown IP version: %s", s)
	}
}

// Config holds global configuration for test utilities
type Config struct {
	AppName        string                `json:"app_name" yaml:"app_name" env:"APP_NAME"`
	AppVersion     string                `json:"app_version" yaml:"app_version" env:"APP_VERSION"`
	Environment    string                `json:"environment" yaml:"environment" env:"ENVIRONMENT"`
	Logger         LoggerConfig          `json:"logger" yaml:"logger" env:"LOGGER"`
	PortChecker    PortCheckerConfig     `json:"port_checker" yaml:"port_checker" env:"PORT_CHECKER"`
	Retry          RetryConfig           `json:"retry" yaml:"retry" env:"RETRY"`
	TestData       TestDataManagerConfig `json:"test_data" yaml:"test_data" env:"TEST_DATA"`
	Timer          TimerConfig           `json:"timer" yaml:"timer" env:"TIMER"`
	SafeExecute    SafeExecuteConfig     `json:"safe_execute" yaml:"safe_execute" env:"SAFE_EXECUTE"`
	IntegerUtils   IntegerUtilsConfig    `json:"integer_utils" yaml:"integer_utils" env:"INTEGER_UTILS"`
	FileOperations FileOperationsConfig  `json:"file_operations" yaml:"file_operations" env:"FILE_OPERATIONS"`
	Concurrency    ConcurrencyConfig     `json:"concurrency" yaml:"concurrency" env:"CONCURRENCY"`
	Metrics        MetricsConfig         `json:"metrics" yaml:"metrics" env:"METRICS"`
	Paths          PathsConfig           `json:"paths" yaml:"paths" env:"PATHS"`
}

// LoggerConfig holds logger configuration
type LoggerConfig struct {
	DefaultLevel    LogLevel               `json:"default_level" yaml:"default_level" env:"DEFAULT_LEVEL"`
	JSONOutput      bool                   `json:"json_output" yaml:"json_output" env:"JSON_OUTPUT"`
	CallerSkip      int                    `json:"caller_skip" yaml:"caller_skip" env:"CALLER_SKIP"`
	EnableCaller    bool                   `json:"enable_caller" yaml:"enable_caller" env:"ENABLE_CALLER"`
	EnableTimestamp bool                   `json:"enable_timestamp" yaml:"enable_timestamp" env:"ENABLE_TIMESTAMP"`
	TimestampFormat string                 `json:"timestamp_format" yaml:"timestamp_format" env:"TIMESTAMP_FORMAT"`
	OutputFile      string                 `json:"output_file" yaml:"output_file" env:"OUTPUT_FILE"`
	MaxFileSize     int64                  `json:"max_file_size" yaml:"max_file_size" env:"MAX_FILE_SIZE"`
	MaxBackups      int                    `json:"max_backups" yaml:"max_backups" env:"MAX_BACKUPS"`
	MaxAge          int                    `json:"max_age" yaml:"max_age" env:"MAX_AGE"`
	Compress        bool                   `json:"compress" yaml:"compress" env:"COMPRESS"`
	DefaultFields   map[string]interface{} `json:"default_fields" yaml:"default_fields" env:"DEFAULT_FIELDS"`
	EnableColors    bool                   `json:"enable_colors" yaml:"enable_colors" env:"ENABLE_COLORS"`
	LevelOverrides  map[string]LogLevel    `json:"level_overrides" yaml:"level_overrides" env:"LEVEL_OVERRIDES"`
}

// PortCheckerConfig holds port checker configuration
type PortCheckerConfig struct {
	Protocol         Protocol      `json:"protocol" yaml:"protocol" env:"PROTOCOL"`
	IPVersion        IPVersion     `json:"ip_version" yaml:"ip_version" env:"IP_VERSION"`
	DialTimeout      time.Duration `json:"dial_timeout" yaml:"dial_timeout" env:"DIAL_TIMEOUT"`
	ReadTimeout      time.Duration `json:"read_timeout" yaml:"read_timeout" env:"READ_TIMEOUT"`
	WriteTimeout     time.Duration `json:"write_timeout" yaml:"write_timeout" env:"WRITE_TIMEOUT"`
	RetryInterval    time.Duration `json:"retry_interval" yaml:"retry_interval" env:"RETRY_INTERVAL"`
	MaxRetries       int           `json:"max_retries" yaml:"max_retries" env:"MAX_RETRIES"`
	MaxConcurrency   int           `json:"max_concurrency" yaml:"max_concurrency" env:"MAX_CONCURRENCY"`
	Workers          int           `json:"workers" yaml:"workers" env:"WORKERS"`
	JitterEnabled    bool          `json:"jitter_enabled" yaml:"jitter_enabled" env:"JITTER_ENABLED"`
	BackoffFactor    float64       `json:"backoff_factor" yaml:"backoff_factor" env:"BACKOFF_FACTOR"`
	ValidatePorts    bool          `json:"validate_ports" yaml:"validate_ports" env:"VALIDATE_PORTS"`
	MinPort          int           `json:"min_port" yaml:"min_port" env:"MIN_PORT"`
	MaxPort          int           `json:"max_port" yaml:"max_port" env:"MAX_PORT"`
	OperationTimeout time.Duration `json:"operation_timeout" yaml:"operation_timeout" env:"OPERATION_TIMEOUT"`
	WaitTimeout      time.Duration `json:"wait_timeout" yaml:"wait_timeout" env:"WAIT_TIMEOUT"`
	EnableStats      bool          `json:"enable_stats" yaml:"enable_stats" env:"ENABLE_STATS"`
	Deterministic    bool          `json:"deterministic" yaml:"deterministic" env:"DETERMINISTIC"`
}

// RetryConfig holds retry configuration
type RetryConfig struct {
	Attempts        int           `json:"attempts" yaml:"attempts" env:"ATTEMPTS"`
	InitialDelay    time.Duration `json:"initial_delay" yaml:"initial_delay" env:"INITIAL_DELAY"`
	MaxDelay        time.Duration `json:"max_delay" yaml:"max_delay" env:"MAX_DELAY"`
	Multiplier      float64       `json:"multiplier" yaml:"multiplier" env:"MULTIPLIER"`
	JitterFactor    float64       `json:"jitter_factor" yaml:"jitter_factor" env:"JITTER_FACTOR"`
	Timeout         time.Duration `json:"timeout" yaml:"timeout" env:"TIMEOUT"`
	BackoffStrategy string        `json:"backoff_strategy" yaml:"backoff_strategy" env:"BACKOFF_STRATEGY"`
	RetryOnErrors   []string      `json:"retry_on_errors" yaml:"retry_on_errors" env:"RETRY_ON_ERRORS"`
	RetryOnPanics   []string      `json:"retry_on_panics" yaml:"retry_on_panics" env:"RETRY_ON_PANICS"`
	MaxElapsedTime  time.Duration `json:"max_elapsed_time" yaml:"max_elapsed_time" env:"MAX_ELAPSED_TIME"`
	EnableMetrics   bool          `json:"enable_metrics" yaml:"enable_metrics" env:"ENABLE_METRICS"`
}

// TestDataManagerConfig holds test data manager configuration
type TestDataManagerConfig struct {
	TempDir        string      `json:"temp_dir" yaml:"temp_dir" env:"TEMP_DIR"`
	BaseDir        string      `json:"base_dir" yaml:"base_dir" env:"BASE_DIR"`
	FileMode       os.FileMode `json:"file_mode" yaml:"file_mode" env:"FILE_MODE"`
	DirMode        os.FileMode `json:"dir_mode" yaml:"dir_mode" env:"DIR_MODE"`
	EnableCache    bool        `json:"enable_cache" yaml:"enable_cache" env:"ENABLE_CACHE"`
	CleanupOnExit  bool        `json:"cleanup_on_exit" yaml:"cleanup_on_exit" env:"CLEANUP_ON_EXIT"`
	MaxFileSize    int64       `json:"max_file_size" yaml:"max_file_size" env:"MAX_FILE_SIZE"`
	AllowSymlinks  bool        `json:"allow_symlinks" yaml:"allow_symlinks" env:"ALLOW_SYMLINKS"`
	PreserveMode   bool        `json:"preserve_mode" yaml:"preserve_mode" env:"PRESERVE_MODE"`
	AtomicWrites   bool        `json:"atomic_writes" yaml:"atomic_writes" env:"ATOMIC_WRITES"`
	MaxDirectories int         `json:"max_directories" yaml:"max_directories" env:"MAX_DIRECTORIES"`
	MaxFiles       int         `json:"max_files" yaml:"max_files" env:"MAX_FILES"`
}

// TimerConfig holds timer configuration
type TimerConfig struct {
	DefaultPrecision time.Duration `json:"default_precision" yaml:"default_precision" env:"DEFAULT_PRECISION"`
	EnableLaps       bool          `json:"enable_laps" yaml:"enable_laps" env:"ENABLE_LAPS"`
	MaxLaps          int           `json:"max_laps" yaml:"max_laps" env:"MAX_LAPS"`
	ReportFormat     string        `json:"report_format" yaml:"report_format" env:"REPORT_FORMAT"`
	EnableStats      bool          `json:"enable_stats" yaml:"enable_stats" env:"ENABLE_STATS"`
}

// SafeExecuteConfig holds safe execution configuration
type SafeExecuteConfig struct {
	PanicRecovery    bool          `json:"panic_recovery" yaml:"panic_recovery" env:"PANIC_RECOVERY"`
	LogPanic         bool          `json:"log_panic" yaml:"log_panic" env:"LOG_PANIC"`
	ReturnOnContext  bool          `json:"return_on_context" yaml:"return_on_context" env:"RETURN_ON_CONTEXT"`
	MaxStackDepth    int           `json:"max_stack_depth" yaml:"max_stack_depth" env:"MAX_STACK_DEPTH"`
	DefaultTimeout   time.Duration `json:"default_timeout" yaml:"default_timeout" env:"DEFAULT_TIMEOUT"`
	EnableCallerInfo bool          `json:"enable_caller_info" yaml:"enable_caller_info" env:"ENABLE_CALLER_INFO"`
	PanicHandler     string        `json:"panic_handler" yaml:"panic_handler" env:"PANIC_HANDLER"`
}

// IntegerUtilsConfig holds integer utilities configuration
type IntegerUtilsConfig struct {
	RandomSeed       int64         `json:"random_seed" yaml:"random_seed" env:"RANDOM_SEED"`
	DefaultMin       int           `json:"default_min" yaml:"default_min" env:"DEFAULT_MIN"`
	DefaultMax       int           `json:"default_max" yaml:"default_max" env:"DEFAULT_MAX"`
	AllowZero        bool          `json:"allow_zero" yaml:"allow_zero" env:"ALLOW_ZERO"`
	AllowNegative    bool          `json:"allow_negative" yaml:"allow_negative" env:"ALLOW_NEGATIVE"`
	MaxRetries       int           `json:"max_retries" yaml:"max_retries" env:"MAX_RETRIES"`
	DefaultTimeout   time.Duration `json:"default_timeout" yaml:"default_timeout" env:"DEFAULT_TIMEOUT"`
	EnableStatistics bool          `json:"enable_statistics" yaml:"enable_statistics" env:"ENABLE_STATISTICS"`
	CacheSize        int           `json:"cache_size" yaml:"cache_size" env:"CACHE_SIZE"`
	PrimeCacheLimit  int           `json:"prime_cache_limit" yaml:"prime_cache_limit" env:"PRIME_CACHE_LIMIT"`
}

// FileOperationsConfig holds file operations configuration
type FileOperationsConfig struct {
	BufferSize          int64         `json:"buffer_size" yaml:"buffer_size" env:"BUFFER_SIZE"`
	CopyConcurrency     int           `json:"copy_concurrency" yaml:"copy_concurrency" env:"COPY_CONCURRENCY"`
	PreservePermissions bool          `json:"preserve_permissions" yaml:"preserve_permissions" env:"PRESERVE_PERMISSIONS"`
	FollowSymlinks      bool          `json:"follow_symlinks" yaml:"follow_symlinks" env:"FOLLOW_SYMLINKS"`
	SkipHidden          bool          `json:"skip_hidden" yaml:"skip_hidden" env:"SKIP_HIDDEN"`
	MaxFileSize         int64         `json:"max_file_size" yaml:"max_file_size" env:"MAX_FILE_SIZE"`
	EnableProgress      bool          `json:"enable_progress" yaml:"enable_progress" env:"ENABLE_PROGRESS"`
	ProgressInterval    time.Duration `json:"progress_interval" yaml:"progress_interval" env:"PROGRESS_INTERVAL"`
	ChecksumAlgorithm   string        `json:"checksum_algorithm" yaml:"checksum_algorithm" env:"CHECKSUM_ALGORITHM"`
	AtomicOperations    bool          `json:"atomic_operations" yaml:"atomic_operations" env:"ATOMIC_OPERATIONS"`
}

// ConcurrencyConfig holds concurrency configuration
type ConcurrencyConfig struct {
	MaxGoroutines           int           `json:"max_goroutines" yaml:"max_goroutines" env:"MAX_GOROUTINES"`
	DefaultPoolSize         int           `json:"default_pool_size" yaml:"default_pool_size" env:"DEFAULT_POOL_SIZE"`
	QueueSize               int           `json:"queue_size" yaml:"queue_size" env:"QUEUE_SIZE"`
	ShutdownTimeout         time.Duration `json:"shutdown_timeout" yaml:"shutdown_timeout" env:"SHUTDOWN_TIMEOUT"`
	EnableDeadlockDetection bool          `json:"enable_deadlock_detection" yaml:"enable_deadlock_detection" env:"ENABLE_DEADLOCK_DETECTION"`
	WorkerIdleTimeout       time.Duration `json:"worker_idle_timeout" yaml:"worker_idle_timeout" env:"WORKER_IDLE_TIMEOUT"`
	MaxTaskDuration         time.Duration `json:"max_task_duration" yaml:"max_task_duration" env:"MAX_TASK_DURATION"`
}

// MetricsConfig holds metrics configuration
type MetricsConfig struct {
	Enabled          bool              `json:"enabled" yaml:"enabled" env:"ENABLED"`
	CollectInterval  time.Duration     `json:"collect_interval" yaml:"collect_interval" env:"COLLECT_INTERVAL"`
	ExportInterval   time.Duration     `json:"export_interval" yaml:"export_interval" env:"EXPORT_INTERVAL"`
	MetricsPort      int               `json:"metrics_port" yaml:"metrics_port" env:"METRICS_PORT"`
	EnableHTTP       bool              `json:"enable_http" yaml:"enable_http" env:"ENABLE_HTTP"`
	EnablePrometheus bool              `json:"enable_prometheus" yaml:"enable_prometheus" env:"ENABLE_PROMETHEUS"`
	EnableStatsD     bool              `json:"enable_statsd" yaml:"enable_statsd" env:"ENABLE_STATSD"`
	StatsDAddress    string            `json:"statsd_address" yaml:"statsd_address" env:"STATSD_ADDRESS"`
	DefaultLabels    map[string]string `json:"default_labels" yaml:"default_labels" env:"DEFAULT_LABELS"`
	HistogramBuckets []float64         `json:"histogram_buckets" yaml:"histogram_buckets" env:"HISTOGRAM_BUCKETS"`
}

// PathsConfig holds paths configuration
type PathsConfig struct {
	TempDir        string   `json:"temp_dir" yaml:"temp_dir" env:"TEMP_DIR"`
	LogDir         string   `json:"log_dir" yaml:"log_dir" env:"LOG_DIR"`
	DataDir        string   `json:"data_dir" yaml:"data_dir" env:"DATA_DIR"`
	CacheDir       string   `json:"cache_dir" yaml:"cache_dir" env:"CACHE_DIR"`
	ConfigDir      string   `json:"config_dir" yaml:"config_dir" env:"CONFIG_DIR"`
	TestDir        string   `json:"test_dir" yaml:"test_dir" env:"TEST_DIR"`
	ImportPaths    []string `json:"import_paths" yaml:"import_paths" env:"IMPORT_PATHS"`
	AllowedPaths   []string `json:"allowed_paths" yaml:"allowed_paths" env:"ALLOWED_PATHS"`
	ForbiddenPaths []string `json:"forbidden_paths" yaml:"forbidden_paths" env:"FORBIDDEN_PATHS"`
}

// DefaultConfig returns a default configuration with sane defaults
func DefaultConfig() *Config {
	return &Config{
		AppName:     "testutils",
		AppVersion:  "1.0.0",
		Environment: "development",
		Logger: LoggerConfig{
			DefaultLevel:    INFO,
			JSONOutput:      false,
			CallerSkip:      3,
			EnableCaller:    true,
			EnableTimestamp: true,
			TimestampFormat: "2006-01-02 15:04:05.000",
			OutputFile:      "",
			MaxFileSize:     100 * 1024 * 1024, // 100MB
			MaxBackups:      10,
			MaxAge:          30, // days
			Compress:        true,
			DefaultFields: map[string]interface{}{
				"app":     "testutils",
				"version": "1.0.0",
			},
			EnableColors:   false,
			LevelOverrides: make(map[string]LogLevel),
		},
		PortChecker: PortCheckerConfig{
			Protocol:         TCP,
			IPVersion:        AnyIP,
			DialTimeout:      2 * time.Second,
			ReadTimeout:      1 * time.Second,
			WriteTimeout:     1 * time.Second,
			RetryInterval:    500 * time.Millisecond,
			MaxRetries:       3,
			MaxConcurrency:   100,
			Workers:          10,
			JitterEnabled:    true,
			BackoffFactor:    1.5,
			ValidatePorts:    true,
			MinPort:          1,
			MaxPort:          65535,
			OperationTimeout: 30 * time.Second,
			WaitTimeout:      5 * time.Minute,
			EnableStats:      true,
			Deterministic:    false,
		},
		Retry: RetryConfig{
			Attempts:        3,
			InitialDelay:    100 * time.Millisecond,
			MaxDelay:        5 * time.Second,
			Multiplier:      2.0,
			JitterFactor:    0.1,
			Timeout:         30 * time.Second,
			BackoffStrategy: "exponential",
			RetryOnErrors:   []string{},
			RetryOnPanics:   []string{},
			MaxElapsedTime:  5 * time.Minute,
			EnableMetrics:   true,
		},
		TestData: TestDataManagerConfig{
			TempDir:        os.TempDir(),
			BaseDir:        "",
			FileMode:       0o644,
			DirMode:        0o755,
			EnableCache:    true,
			CleanupOnExit:  true,
			MaxFileSize:    100 * 1024 * 1024, // 100MB
			AllowSymlinks:  false,
			PreserveMode:   true,
			AtomicWrites:   true,
			MaxDirectories: 100,
			MaxFiles:       1000,
		},
		Timer: TimerConfig{
			DefaultPrecision: time.Microsecond,
			EnableLaps:       true,
			MaxLaps:          100,
			ReportFormat:     "human",
			EnableStats:      true,
		},
		SafeExecute: SafeExecuteConfig{
			PanicRecovery:    true,
			LogPanic:         true,
			ReturnOnContext:  true,
			MaxStackDepth:    32,
			DefaultTimeout:   30 * time.Second,
			EnableCallerInfo: true,
			PanicHandler:     "default",
		},
		IntegerUtils: IntegerUtilsConfig{
			RandomSeed:       0,
			DefaultMin:       0,
			DefaultMax:       100,
			AllowZero:        true,
			AllowNegative:    false,
			MaxRetries:       1000,
			DefaultTimeout:   5 * time.Second,
			EnableStatistics: true,
			CacheSize:        1000,
			PrimeCacheLimit:  1000000,
		},
		FileOperations: FileOperationsConfig{
			BufferSize:          32 * 1024, // 32KB
			CopyConcurrency:     runtime.NumCPU(),
			PreservePermissions: true,
			FollowSymlinks:      false,
			SkipHidden:          false,
			MaxFileSize:         1024 * 1024 * 1024, // 1GB
			EnableProgress:      false,
			ProgressInterval:    1 * time.Second,
			ChecksumAlgorithm:   "sha256",
			AtomicOperations:    true,
		},
		Concurrency: ConcurrencyConfig{
			MaxGoroutines:           10000,
			DefaultPoolSize:         runtime.NumCPU() * 2,
			QueueSize:               1000,
			ShutdownTimeout:         30 * time.Second,
			EnableDeadlockDetection: false,
			WorkerIdleTimeout:       1 * time.Minute,
			MaxTaskDuration:         5 * time.Minute,
		},
		Metrics: MetricsConfig{
			Enabled:          false,
			CollectInterval:  10 * time.Second,
			ExportInterval:   30 * time.Second,
			MetricsPort:      9090,
			EnableHTTP:       true,
			EnablePrometheus: true,
			EnableStatsD:     false,
			StatsDAddress:    "localhost:8125",
			DefaultLabels: map[string]string{
				"app":     "testutils",
				"version": "1.0.0",
			},
			HistogramBuckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5, 10},
		},
		Paths: PathsConfig{
			TempDir:        os.TempDir(),
			LogDir:         "",
			DataDir:        "",
			CacheDir:       "",
			ConfigDir:      "",
			TestDir:        "",
			ImportPaths:    []string{},
			AllowedPaths:   []string{},
			ForbiddenPaths: []string{"/", "/etc", "/bin", "/sbin", "/usr/bin", "/usr/sbin"},
		},
	}
}

// LoadConfig loads configuration from a file
func LoadConfig(filePath string) (*Config, error) {
	config := DefaultConfig()

	if filePath == "" {
		return config, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported config file format: %s", ext)
	}

	// Load environment variables
	config.LoadFromEnv()

	// Validate
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	// Expand paths
	config.ExpandPaths()

	return config, nil
}

// LoadFromEnv loads configuration from environment variables
func (c *Config) LoadFromEnv() {
	c.loadStructFromEnv("TESTUTILS_", reflect.ValueOf(c).Elem())
}

// loadStructFromEnv recursively loads struct fields from environment variables
func (c *Config) loadStructFromEnv(prefix string, v reflect.Value) {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Get env tag
		envTag := fieldType.Tag.Get("env")
		if envTag == "" || envTag == "-" {
			continue
		}

		envKey := prefix + envTag
		envValue, exists := os.LookupEnv(envKey)
		if !exists {
			// Check for nested struct
			if field.Kind() == reflect.Struct {
				c.loadStructFromEnv(prefix+envTag+"_", field)
			}
			continue
		}

		// Set field value based on type
		if err := c.setFieldFromEnv(field, fieldType, envValue); err != nil {
			// Log error but continue
			fmt.Printf("Failed to set field %s from env %s: %v\n",
				fieldType.Name, envKey, err)
		}
	}
}

// setFieldFromEnv sets a field value from environment variable
func (c *Config) setFieldFromEnv(field reflect.Value, fieldType reflect.StructField, envValue string) error {
	if !field.CanSet() {
		return fmt.Errorf("field %s cannot be set", fieldType.Name)
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(envValue)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if val, err := strconv.ParseInt(envValue, 10, 64); err != nil {
			return err
		} else {
			field.SetInt(val)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if val, err := strconv.ParseUint(envValue, 10, 64); err != nil {
			return err
		} else {
			field.SetUint(val)
		}
	case reflect.Float32, reflect.Float64:
		if val, err := strconv.ParseFloat(envValue, 64); err != nil {
			return err
		} else {
			field.SetFloat(val)
		}
	case reflect.Bool:
		if val, err := strconv.ParseBool(envValue); err != nil {
			return err
		} else {
			field.SetBool(val)
		}
	case reflect.Slice:
		// Handle comma-separated values for slices
		values := strings.Split(envValue, ",")
		slice := reflect.MakeSlice(field.Type(), len(values), len(values))
		for i, v := range values {
			elem := slice.Index(i)
			// Handle string slices
			if elem.Kind() == reflect.String {
				elem.SetString(strings.TrimSpace(v))
			}
			// TODO: Handle other slice types
		}
		field.Set(slice)
	case reflect.Map:
		// For maps, we might need a special format
		// For now, skip complex map types from env
		return nil
	default:
		// Handle custom types
		switch field.Type() {
		case reflect.TypeOf(time.Duration(0)):
			if val, err := time.ParseDuration(envValue); err != nil {
				return err
			} else {
				field.Set(reflect.ValueOf(val))
			}
		case reflect.TypeOf(LogLevel(0)):
			if val, err := ParseLogLevel(envValue); err != nil {
				return err
			} else {
				field.Set(reflect.ValueOf(val))
			}
		case reflect.TypeOf(Protocol("")):
			if val, err := ParseProtocol(envValue); err != nil {
				return err
			} else {
				field.Set(reflect.ValueOf(val))
			}
		case reflect.TypeOf(IPVersion(0)):
			if val, err := ParseIPVersion(envValue); err != nil {
				return err
			} else {
				field.Set(reflect.ValueOf(val))
			}
		case reflect.TypeOf(os.FileMode(0)):
			if val, err := strconv.ParseUint(envValue, 8, 32); err != nil {
				return err
			} else {
				field.Set(reflect.ValueOf(os.FileMode(val)))
			}
		default:
			return fmt.Errorf("unsupported field type: %s", field.Type())
		}
	}

	return nil
}

// ExpandPaths expands paths in the configuration
func (c *Config) ExpandPaths() {
	// Helper function to expand path
	expand := func(path string) string {
		if path == "" {
			return path
		}
		if strings.HasPrefix(path, "~/") {
			home, err := os.UserHomeDir()
			if err == nil {
				return filepath.Join(home, path[2:])
			}
		}
		return os.ExpandEnv(path)
	}

	// Expand paths in PathsConfig
	c.Paths.TempDir = expand(c.Paths.TempDir)
	c.Paths.LogDir = expand(c.Paths.LogDir)
	c.Paths.DataDir = expand(c.Paths.DataDir)
	c.Paths.CacheDir = expand(c.Paths.CacheDir)
	c.Paths.ConfigDir = expand(c.Paths.ConfigDir)
	c.Paths.TestDir = expand(c.Paths.TestDir)

	// Expand paths in TestDataManagerConfig
	if c.TestData.TempDir == "" {
		c.TestData.TempDir = c.Paths.TempDir
	}
	c.TestData.TempDir = expand(c.TestData.TempDir)
	c.TestData.BaseDir = expand(c.TestData.BaseDir)

	// Expand paths in LoggerConfig
	c.Logger.OutputFile = expand(c.Logger.OutputFile)
}

// Validate checks the configuration for errors and sanity
func (c *Config) Validate() error {
	var errors []string

	// App validation
	if c.AppName == "" {
		errors = append(errors, "AppName cannot be empty")
	}

	// Logger validation
	if c.Logger.MaxFileSize < 0 {
		errors = append(errors, "Logger MaxFileSize must be >= 0")
	}
	if c.Logger.MaxBackups < 0 {
		errors = append(errors, "Logger MaxBackups must be >= 0")
	}
	if c.Logger.MaxAge < 0 {
		errors = append(errors, "Logger MaxAge must be >= 0")
	}

	// PortChecker validation
	if c.PortChecker.MaxConcurrency <= 0 {
		errors = append(errors, "PortChecker MaxConcurrency must be > 0")
	}
	if c.PortChecker.Workers <= 0 {
		errors = append(errors, "PortChecker Workers must be > 0")
	}
	if c.PortChecker.MinPort < 1 || c.PortChecker.MinPort > 65535 {
		errors = append(errors, "PortChecker MinPort must be between 1 and 65535")
	}
	if c.PortChecker.MaxPort < 1 || c.PortChecker.MaxPort > 65535 {
		errors = append(errors, "PortChecker MaxPort must be between 1 and 65535")
	}
	if c.PortChecker.MinPort > c.PortChecker.MaxPort {
		errors = append(errors, "PortChecker MinPort must be <= MaxPort")
	}
	if c.PortChecker.BackoffFactor < 1.0 {
		errors = append(errors, "PortChecker BackoffFactor must be >= 1.0")
	}

	// Retry validation
	if c.Retry.Attempts < 1 {
		errors = append(errors, "Retry Attempts must be >= 1")
	}
	if c.Retry.Multiplier < 1.0 {
		errors = append(errors, "Retry Multiplier must be >= 1.0")
	}
	if c.Retry.MaxDelay < c.Retry.InitialDelay {
		errors = append(errors, "Retry MaxDelay must be >= InitialDelay")
	}
	if c.Retry.JitterFactor < 0 || c.Retry.JitterFactor > 1 {
		errors = append(errors, "Retry JitterFactor must be between 0 and 1")
	}

	// TestData validation
	if c.TestData.MaxFileSize < 0 {
		errors = append(errors, "TestData MaxFileSize must be >= 0")
	}
	if c.TestData.MaxDirectories < 0 {
		errors = append(errors, "TestData MaxDirectories must be >= 0")
	}
	if c.TestData.MaxFiles < 0 {
		errors = append(errors, "TestData MaxFiles must be >= 0")
	}

	// IntegerUtils validation
	if c.IntegerUtils.MaxRetries < 0 {
		errors = append(errors, "IntegerUtils MaxRetries must be >= 0")
	}
	if c.IntegerUtils.CacheSize < 0 {
		errors = append(errors, "IntegerUtils CacheSize must be >= 0")
	}
	if c.IntegerUtils.PrimeCacheLimit < 0 {
		errors = append(errors, "IntegerUtils PrimeCacheLimit must be >= 0")
	}

	// Concurrency validation
	if c.Concurrency.MaxGoroutines <= 0 {
		errors = append(errors, "Concurrency MaxGoroutines must be > 0")
	}
	if c.Concurrency.DefaultPoolSize <= 0 {
		errors = append(errors, "Concurrency DefaultPoolSize must be > 0")
	}
	if c.Concurrency.QueueSize <= 0 {
		errors = append(errors, "Concurrency QueueSize must be > 0")
	}

	// Metrics validation
	if c.Metrics.Enabled {
		if c.Metrics.MetricsPort < 1 || c.Metrics.MetricsPort > 65535 {
			errors = append(errors, "Metrics Port must be between 1 and 65535")
		}
	}

	// Timer validation
	if c.Timer.DefaultPrecision <= 0 {
		errors = append(errors, "Timer DefaultPrecision must be > 0")
	}
	if c.Timer.MaxLaps < 0 {
		errors = append(errors, "Timer MaxLaps must be >= 0")
	}

	// FileOperations validation
	if c.FileOperations.BufferSize <= 0 {
		errors = append(errors, "FileOperations BufferSize must be > 0")
	}
	if c.FileOperations.CopyConcurrency <= 0 {
		errors = append(errors, "FileOperations CopyConcurrency must be > 0")
	}
	if c.FileOperations.MaxFileSize < 0 {
		errors = append(errors, "FileOperations MaxFileSize must be >= 0")
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// Save saves the configuration to a file
func (c *Config) Save(filePath string) error {
	ext := strings.ToLower(filepath.Ext(filePath))

	var data []byte
	var err error

	switch ext {
	case ".json":
		data, err = json.MarshalIndent(c, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
	case ".yaml", ".yml":
		data, err = yaml.Marshal(c)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML: %w", err)
		}
	default:
		return fmt.Errorf("unsupported file format: %s", ext)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Merge merges another configuration into this one
func (c *Config) Merge(other *Config) {
	if other == nil {
		return
	}

	// Use reflection to merge structs
	c.mergeStructs(reflect.ValueOf(c).Elem(), reflect.ValueOf(other).Elem())
}

// mergeStructs recursively merges two structs
func (c *Config) mergeStructs(dst, src reflect.Value) {
	if dst.Kind() != reflect.Struct || src.Kind() != reflect.Struct {
		return
	}

	for i := 0; i < dst.NumField(); i++ {
		dstField := dst.Field(i)
		srcField := src.Field(i)

		if !srcField.IsValid() || srcField.IsZero() {
			continue
		}

		switch dstField.Kind() {
		case reflect.Struct:
			c.mergeStructs(dstField, srcField)
		case reflect.Slice:
			if srcField.Len() > 0 {
				dstField.Set(srcField)
			}
		case reflect.Map:
			if srcField.Len() > 0 {
				if dstField.IsNil() {
					dstField.Set(reflect.MakeMap(dstField.Type()))
				}
				iter := srcField.MapRange()
				for iter.Next() {
					dstField.SetMapIndex(iter.Key(), iter.Value())
				}
			}
		default:
			dstField.Set(srcField)
		}
	}
}

// Clone creates a deep copy of the configuration
func (c *Config) Clone() (*Config, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	var cloned Config
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cloned, nil
}

// GetEnvironment returns the current environment
func (c *Config) GetEnvironment() string {
	return c.Environment
}

// IsDevelopment returns true if environment is development
func (c *Config) IsDevelopment() bool {
	return strings.ToLower(c.Environment) == "development"
}

// IsProduction returns true if environment is production
func (c *Config) IsProduction() bool {
	return strings.ToLower(c.Environment) == "production"
}

// IsTest returns true if environment is test
func (c *Config) IsTest() bool {
	return strings.ToLower(c.Environment) == "test"
}

// GetEffectiveLogLevel returns effective log level for a component
func (c *Config) GetEffectiveLogLevel(component string) LogLevel {
	// Check for component-specific override
	if level, exists := c.Logger.LevelOverrides[component]; exists {
		return level
	}

	// Return default level
	return c.Logger.DefaultLevel
}

// GetTempDir returns the effective temp directory
func (c *Config) GetTempDir() string {
	if c.TestData.TempDir != "" {
		return c.TestData.TempDir
	}
	if c.Paths.TempDir != "" {
		return c.Paths.TempDir
	}
	return os.TempDir()
}

// String returns a string representation of the configuration
func (c *Config) String() string {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error marshaling config: %v", err)
	}
	return string(data)
}

// ShortString returns a concise string representation
func (c *Config) ShortString() string {
	return fmt.Sprintf("Config[App=%s v%s Env=%s]",
		c.AppName, c.AppVersion, c.Environment)
}

// ExampleUsage demonstrates how to use the configuration
func ExampleUsage() {
	// Load configuration from file
	config, err := LoadConfig("config.yaml")
	if err != nil {
		// Fall back to defaults
		config = DefaultConfig()
	}

	// Override with environment variables
	config.LoadFromEnv()

	// Validate
	if err := config.Validate(); err != nil {
		panic(err)
	}

	// Use configuration
	fmt.Printf("Application: %s v%s\n", config.AppName, config.AppVersion)
	fmt.Printf("Environment: %s\n", config.Environment)
	fmt.Printf("Log Level: %s\n", config.Logger.DefaultLevel)

	// Save configuration
	if err := config.Save("config-backup.yaml"); err != nil {
		fmt.Printf("Failed to save config: %v\n", err)
	}
}
