package testutils


import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Logger defines a minimal logging interface.
// It can be satisfied by *log.Logger, log/slog, or a custom logger.
type Logger interface {
	Printf(format string, v ...interface{})
}

// DB is an interface that abstracts the standard sql.DB methods.
// It returns custom Tx and Stmt interfaces instead of concrete types.
type DB interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	PrepareContext(ctx context.Context, query string) (Stmt, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error)
	PingContext(ctx context.Context) error
	Close() error
}

// Tx represents a database transaction with the same methods as sql.Tx,
// but returns our Stmt interface.
type Tx interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	PrepareContext(ctx context.Context, query string) (Stmt, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	Commit() error
	Rollback() error
}

// Stmt represents a prepared statement with the same methods as sql.Stmt.
type Stmt interface {
	ExecContext(ctx context.Context, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, args ...interface{}) *sql.Row
	Close() error
}

// SQLDB wraps *sql.DB and implements the DB interface.
type SQLDB struct {
	db *sql.DB
}

// NewSQLDB wraps an *sql.DB into a SQLDB that satisfies the DB interface.
func NewSQLDB(db *sql.DB) *SQLDB {
	return &SQLDB{db: db}
}

func (s *SQLDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return s.db.ExecContext(ctx, query, args...)
}

func (s *SQLDB) PrepareContext(ctx context.Context, query string) (Stmt, error) {
	stmt, err := s.db.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	return &sqlStmt{stmt: stmt}, nil
}

func (s *SQLDB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, query, args...)
}

func (s *SQLDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return s.db.QueryRowContext(ctx, query, args...)
}

func (s *SQLDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	tx, err := s.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &sqlTx{tx: tx}, nil
}

func (s *SQLDB) PingContext(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *SQLDB) Close() error {
	return s.db.Close()
}

// sqlTx wraps *sql.Tx and implements the Tx interface.
type sqlTx struct {
	tx *sql.Tx
}

func (s *sqlTx) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return s.tx.ExecContext(ctx, query, args...)
}

func (s *sqlTx) PrepareContext(ctx context.Context, query string) (Stmt, error) {
	stmt, err := s.tx.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	return &sqlStmt{stmt: stmt}, nil
}

func (s *sqlTx) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return s.tx.QueryContext(ctx, query, args...)
}

func (s *sqlTx) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return s.tx.QueryRowContext(ctx, query, args...)
}

func (s *sqlTx) Commit() error {
	return s.tx.Commit()
}

func (s *sqlTx) Rollback() error {
	return s.tx.Rollback()
}

// sqlStmt wraps *sql.Stmt and implements the Stmt interface.
type sqlStmt struct {
	stmt *sql.Stmt
}

func (s *sqlStmt) ExecContext(ctx context.Context, args ...interface{}) (sql.Result, error) {
	return s.stmt.ExecContext(ctx, args...)
}

func (s *sqlStmt) QueryContext(ctx context.Context, args ...interface{}) (*sql.Rows, error) {
	return s.stmt.QueryContext(ctx, args...)
}

func (s *sqlStmt) QueryRowContext(ctx context.Context, args ...interface{}) *sql.Row {
	return s.stmt.QueryRowContext(ctx, args...)
}

func (s *sqlStmt) Close() error {
	return s.stmt.Close()
}

// Config holds database connection parameters.
// Use the With* methods to create a copy with modified settings.
type Config struct {
	driver          string
	dsn             string
	maxOpenConns    int
	maxIdleConns    int
	connMaxLifetime time.Duration
	connMaxIdleTime time.Duration
	pingTimeout     time.Duration
}

// DefaultConfig returns a Config with safe connection pool defaults.
func DefaultConfig() *Config {
	return &Config{
		maxOpenConns:    25,
		maxIdleConns:    5,
		connMaxLifetime: 5 * time.Minute,
		connMaxIdleTime: 1 * time.Minute,
		pingTimeout:     5 * time.Second,
	}
}

// WithDriver sets the driver name and returns a new Config.
func (c *Config) WithDriver(driver string) *Config {
	cfg := *c
	cfg.driver = driver
	return &cfg
}

// WithDSN sets the data source name and returns a new Config.
func (c *Config) WithDSN(dsn string) *Config {
	cfg := *c
	cfg.dsn = dsn
	return &cfg
}

// WithMaxOpenConns sets the maximum number of open connections.
func (c *Config) WithMaxOpenConns(n int) *Config {
	cfg := *c
	cfg.maxOpenConns = n
	return &cfg
}

// WithMaxIdleConns sets the maximum number of idle connections.
func (c *Config) WithMaxIdleConns(n int) *Config {
	cfg := *c
	cfg.maxIdleConns = n
	return &cfg
}

// WithConnMaxLifetime sets the maximum lifetime of a connection.
func (c *Config) WithConnMaxLifetime(d time.Duration) *Config {
	cfg := *c
	cfg.connMaxLifetime = d
	return &cfg
}

// WithConnMaxIdleTime sets the maximum idle time of a connection.
func (c *Config) WithConnMaxIdleTime(d time.Duration) *Config {
	cfg := *c
	cfg.connMaxIdleTime = d
	return &cfg
}

// WithPingTimeout sets the timeout for the initial ping.
// If zero, no timeout is applied.
func (c *Config) WithPingTimeout(d time.Duration) *Config {
	cfg := *c
	cfg.pingTimeout = d
	return &cfg
}

// Connector is an immutable builder for database connections.
type Connector struct {
	driver string
	dsn    string
	config *Config
	logger Logger
}

// NewConnector creates a new Connector with the given driver and DSN.
// It uses the default configuration.
func NewConnector(driver, dsn string) *Connector {
	return &Connector{
		driver: driver,
		dsn:    dsn,
		config: DefaultConfig(),
	}
}

// WithConfig returns a new Connector with the given configuration.
// The configuration is deeply copied to prevent external modifications.
func (c *Connector) WithConfig(cfg *Config) *Connector {
	// Create a shallow copy of the Config (all fields are immutable after construction)
	cfgCopy := *cfg
	return &Connector{
		driver: c.driver,
		dsn:    c.dsn,
		config: &cfgCopy,
		logger: c.logger,
	}
}

// WithLogger returns a new Connector with the given logger.
func (c *Connector) WithLogger(logger Logger) *Connector {
	return &Connector{
		driver: c.driver,
		dsn:    c.dsn,
		config: c.config,
		logger: logger,
	}
}

// log formats and prints a message using the logger, if set.
func (c *Connector) log(format string, v ...interface{}) {
	if c.logger != nil {
		c.logger.Printf(format, v...)
	}
}

// Open initializes the database connection, configures the connection pool,
// and verifies connectivity with a ping.
func (c *Connector) Open(ctx context.Context) (DB, error) {
	// Use driver and dsn from Connector, but allow config to override.
	driver := c.driver
	dsn := c.dsn
	if c.config.driver != "" {
		driver = c.config.driver
	}
	if c.config.dsn != "" {
		dsn = c.config.dsn
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(c.config.maxOpenConns)
	db.SetMaxIdleConns(c.config.maxIdleConns)
	db.SetConnMaxLifetime(c.config.connMaxLifetime)
	db.SetConnMaxIdleTime(c.config.connMaxIdleTime)

	c.log("Database opened, verifying connection...")

	// Apply ping timeout if the context has no deadline and pingTimeout > 0
	if c.config.pingTimeout > 0 {
		if _, ok := ctx.Deadline(); !ok {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, c.config.pingTimeout)
			defer cancel()
		}
	}

	if err := db.PingContext(ctx); err != nil {
		if cerr := db.Close(); cerr != nil {
			return nil, fmt.Errorf("ping failed: %w, also failed to close: %v", err, cerr)
		}
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	c.log("Database connection verified successfully.")
	return NewSQLDB(db), nil
}

// MustOpen calls Open and panics if an error occurs.
// The panic value preserves the original error.
func (c *Connector) MustOpen(ctx context.Context) DB {
	db, err := c.Open(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to open DB: %w", err))
	}
	return db
}
