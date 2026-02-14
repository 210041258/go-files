package testutils

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"go_server_enterprise/internal/business"
)

// -----------------------------------------------------------------------------
// Metrics Interface for Observability
// -----------------------------------------------------------------------------

// MetricsCollector allows the initialization code to report statistics
// (e.g., to Prometheus, OpenTelemetry, or logs).
type MetricsCollector interface {
	RecordDBInit(driver string, success bool)
	SetGauge(name string, value float64, labels ...string)
}

// NoOpMetrics is a default implementation that does nothing.
type NoOpMetrics struct{}

func (m *NoOpMetrics) RecordDBInit(driver string, success bool)              {}
func (m *NoOpMetrics) SetGauge(name string, value float64, labels ...string) {}

// -----------------------------------------------------------------------------
// Domain Specific Errors
// -----------------------------------------------------------------------------

var (
	// ErrInvalidConfig indicates a configuration parsing error.
	ErrInvalidConfig = errors.New("invalid database configuration")
	// ErrConnectionFailed indicates the database could not be reached.
	ErrConnectionFailed = errors.New("database connection failed")
	// ErrPingTimeout indicates the health check timed out.
	ErrPingTimeout = errors.New("database ping timed out")
)

// -----------------------------------------------------------------------------
// Configuration Struct
// -----------------------------------------------------------------------------

// Config holds the database configuration parameters.
type Config struct {
	Driver          string
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// ToBusinessConfig converts infrastructure Config to business.Config.
func (c *Config) ToBusinessConfig() *business.Config {
	return &business.Config{
		Driver:          c.Driver,
		DSN:             c.DSN,
		MaxOpenConns:    c.MaxOpenConns,
		MaxIdleConns:    c.MaxIdleConns,
		ConnMaxLifetime: c.ConnMaxLifetime,
		ConnMaxIdleTime: c.ConnMaxIdleTime,
	}
}

// -----------------------------------------------------------------------------
// Configuration Loader
// -----------------------------------------------------------------------------

// ConfigLoader defines the contract for loading database configuration.
type ConfigLoader interface {
	Load() (*Config, error)
}

// EnvConfigLoader loads configuration from environment variables.
type EnvConfigLoader struct {
	Prefix string
}

// NewEnvConfigLoader creates a loader with an optional prefix.
func NewEnvConfigLoader(prefix string) *EnvConfigLoader {
	return &EnvConfigLoader{Prefix: prefix}
}

// Load reads and validates configuration from environment variables.
// It performs strict parsing to fail fast on invalid inputs.
func (l *EnvConfigLoader) Load() (*Config, error) {
	const (
		defaultMaxOpen = 25
		defaultMaxIdle = 10 // Optimized for connection reuse without hogging resources
		defaultMaxLife = 5 * time.Minute
		defaultMaxIdle = 5 * time.Minute
	)

	cfg := &Config{
		Driver:          l.getEnv("DRIVER", "postgres"),
		DSN:             l.getEnv("DSN", ""),
		MaxOpenConns:    l.getInt("MAX_OPEN", defaultMaxOpen),
		MaxIdleConns:    l.getInt("MAX_IDLE", defaultMaxIdle),
		ConnMaxLifetime: l.getDuration("MAX_LIFE", defaultMaxLife),
		ConnMaxIdleTime: l.getDuration("MAX_IDLE_T", defaultMaxIdle),
	}

	// Validation
	if cfg.DSN == "" {
		return nil, fmt.Errorf("%w: DSN cannot be empty", ErrInvalidConfig)
	}
	if cfg.MaxIdleConns > cfg.MaxOpenConns {
		return nil, fmt.Errorf("%w: MAX_IDLE (%d) cannot exceed MAX_OPEN (%d)",
			ErrInvalidConfig, cfg.MaxIdleConns, cfg.MaxOpenConns)
	}
	if cfg.ConnMaxLifetime < 0 {
		return nil, fmt.Errorf("%w: MAX_LIFE cannot be negative", ErrInvalidConfig)
	}

	return cfg, nil
}

func (l *EnvConfigLoader) getEnv(key, defaultValue string) string {
	if val, exists := os.LookupEnv(l.Prefix + key); exists {
		return val
	}
	return defaultValue
}

func (l *EnvConfigLoader) getInt(key string, defaultValue int) int {
	valStr := l.getEnv(key, "")
	if valStr == "" {
		return defaultValue
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		panic(fmt.Errorf("invalid integer for %s%s: %w", l.Prefix, key, err))
	}
	return val
}

func (l *EnvConfigLoader) getDuration(key string, defaultValue time.Duration) time.Duration {
	valStr := l.getEnv(key, "")
	if valStr == "" {
		return defaultValue
	}
	val, err := time.ParseDuration(valStr)
	if err != nil {
		panic(fmt.Errorf("invalid duration for %s%s: %w", l.Prefix, key, err))
	}
	return val
}

// -----------------------------------------------------------------------------
// Connector Factory
// -----------------------------------------------------------------------------

// ConnectorFactory defines the contract for creating database connections.
type ConnectorFactory interface {
	Connect(ctx context.Context, cfg *Config) (business.DB, error)
}

type stdConnectorFactory struct{}

func (f *stdConnectorFactory) Connect(ctx context.Context, cfg *Config) (business.DB, error) {
	// Note: sql.Open does not actually connect; it validates args.
	// We assume business.Connector handles the initial setup.
	connector := business.NewConnector(cfg.Driver, cfg.DSN)

	// We pass the business config here
	return connector.Open()
}

// -----------------------------------------------------------------------------
// Initialization Function
// -----------------------------------------------------------------------------

// DBWrapper wraps the DB and Manager to allow for graceful shutdown.
type DBWrapper struct {
	DB     business.DB
	TxMgr  *business.TransactionManager
	mu     sync.Mutex
	closed bool
}

// Close gracefully shuts down the database connection.
func (w *DBWrapper) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}

	if w.DB != nil {
		if err := w.DB.Close(); err != nil {
			return fmt.Errorf("error closing DB: %w", err)
		}
	}
	w.closed = true
	return nil
}

// InitDB initializes the database connection pool with comprehensive error handling and metrics.
//
// Parameters:
//   - ctx: Context for timeout/cancellation.
//   - loader: Source of configuration (e.g., Env vars).
//   - factory: Strategy to create the connection.
//   - logger: Interface for logging (can be nil).
//   - metrics: Interface for metrics (can be nil).
func InitDB(
	ctx context.Context,
	loader ConfigLoader,
	factory ConnectorFactory,
	logger Logger,
	metrics MetricsCollector,
) (*DBWrapper, error) {
	if logger == nil {
		logger = &NoOpLogger{}
	}
	if metrics == nil {
		metrics = &NoOpMetrics{}
	}
	if factory == nil {
		factory = &stdConnectorFactory{}
	}

	// 1. Load Configuration
	cfg, err := loader.Load()
	if err != nil {
		logger.Error("Config load failed", "error", err)
		metrics.RecordDBInit("unknown", false)
		return nil, fmt.Errorf("load config: %w", err)
	}

	logger.Info("Initializing database connection", "driver", cfg.Driver)

	// 2. Establish Connection
	db, err := factory.Connect(ctx, cfg)
	if err != nil {
		logger.Error("Connection failed", "error", err)
		metrics.RecordDBInit(cfg.Driver, false)
		return nil, fmt.Errorf("connect: %w: %v", ErrConnectionFailed, err)
	}

	// 3. Health Check with Timeout
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		logger.Error("Ping failed", "error", err)
		metrics.RecordDBInit(cfg.Driver, false)
		db.Close() // Cleanup
		return nil, fmt.Errorf("ping: %w: %v", ErrPingTimeout, err)
	}

	// 4. Expose Metrics (if supported by underlying DB)
	// Note: We cast to *sql.DB to access standard library stats
	if sqlDB, ok := db.(*business.SQLDB); ok {
		// Report raw connection stats
		// In a real scenario, these might be scraped periodically, but we set initial gauges here.
		stats := sqlDB.Stats()
		metrics.SetGauge("db_max_open_connections", float64(stats.MaxOpenConnections))
		metrics.SetGauge("db_idle_connections", float64(stats.Idle))
		metrics.SetGauge("db_in_use_connections", float64(stats.InUse))
	}

	logger.Info("Database connected successfully", "max_open", cfg.MaxOpenConns, "max_idle", cfg.MaxIdleConns)
	metrics.RecordDBInit(cfg.Driver, true)

	// 5. Create Transaction Manager
	txMgr := business.NewTransactionManager(db)

	return &DBWrapper{
		DB:    db,
		TxMgr: txMgr,
	}, nil
}

// InitDBWithDefaults is a convenience wrapper for simple applications.
func InitDBWithDefaults(ctx context.Context) (*DBWrapper, error) {
	loader := NewEnvConfigLoader("DB_")
	return InitDB(ctx, loader, &stdConnectorFactory{}, &NoOpLogger{}, &NoOpMetrics{})
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

// NoOpLogger implementation
type NoOpLogger struct{}

func (l *NoOpLogger) Info(msg string, fields ...interface{})  {}
func (l *NoOpLogger) Error(msg string, fields ...interface{}) {}
