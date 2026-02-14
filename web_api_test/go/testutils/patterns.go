package testutils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// =============================================================================
// 1. CONCURRENCY: Worker Pool
// =============================================================================

type Job func() error

type WorkerPool struct {
	workerCount int
	jobQueue    chan Job
	wg          sync.WaitGroup
	quit        chan struct{}
}

func NewWorkerPool(workerCount, queueSize int) *WorkerPool {
	return &WorkerPool{
		workerCount: workerCount,
		jobQueue:    make(chan Job, queueSize),
		quit:        make(chan struct{}),
	}
}

func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workerCount; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()
	for {
		select {
		case job, ok := <-wp.jobQueue:
			if !ok {
				return
			}
			if err := job(); err != nil {
				fmt.Printf("[Worker %d] Job failed: %v\n", id, err)
			}
		case <-wp.quit:
			return
		}
	}
}

// Submit adds job, supports context cancellation
func (wp *WorkerPool) SubmitCtx(ctx context.Context, job Job) error {
	select {
	case wp.jobQueue <- job:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (wp *WorkerPool) Stop() {
	close(wp.quit)
	close(wp.jobQueue)
	wp.wg.Wait()
}

// =============================================================================
// 2. STRUCTURAL: Functional Options Pattern
// =============================================================================

type ServerConfig struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type Option func(*ServerConfig)

func WithAddr(addr string) Option {
	return func(c *ServerConfig) {
		c.Addr = addr
	}
}

func WithTimeouts(read, write, idle time.Duration) Option {
	return func(c *ServerConfig) {
		c.ReadTimeout = read
		c.WriteTimeout = write
		c.IdleTimeout = idle
	}
}

func NewServerConfig(opts ...Option) *ServerConfig {
	cfg := &ServerConfig{
		Addr:         ":8080",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// =============================================================================
// 3. HTTP: Middleware & Graceful Shutdown
// =============================================================================

type Middleware func(http.Handler) http.Handler

func Chain(mws ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
}

func GracefulShutdown(server *http.Server, timeout time.Duration) error {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	fmt.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return err
	}

	fmt.Println("Server gracefully stopped")
	return nil
}

// =============================================================================
// 4. ERROR HANDLING: Sentinel & Wrapping
// =============================================================================

var (
	ErrNotFound     = errors.New("resource not found")
	ErrUnauthorized = errors.New("unauthorized access")
	ErrInvalidInput = errors.New("invalid input")
)

func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// =============================================================================
// 5. RESOURCE MANAGEMENT: sync.Pool
// =============================================================================

var BufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

func GetBuffer() *bytes.Buffer {
	return BufferPool.Get().(*bytes.Buffer)
}

func PutBuffer(buf *bytes.Buffer) {
	if buf != nil {
		buf.Reset()
		BufferPool.Put(buf)
	}
}

// =============================================================================
// 6. CONTEXT: Request-scoped Values
// =============================================================================

type Key string

const (
	UserIDKey    Key = "userID"
	RequestIDKey Key = "requestID"
)

func SetUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

func GetUserID(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDKey).(string)
	return userID, ok
}

// =============================================================================
// 7. RETRY & BACKOFF
// =============================================================================

func Retry(attempts int, sleep time.Duration, fn func() error) error {
	for i := 0; i < attempts; i++ {
		if err := fn(); err != nil {
			time.Sleep(sleep)
			sleep *= 2
		} else {
			return nil
		}
	}
	return errors.New("max retries exceeded")
}

// =============================================================================
// 8. LOGGING PATTERN
// =============================================================================

type Logger interface {
	Info(args ...interface{})
	Error(args ...interface{})
}

func StdLogger() Logger {
	return &stdLogger{}
}

type stdLogger struct{}

func (l *stdLogger) Info(args ...interface{}) {
	fmt.Println("[INFO]", fmt.Sprint(args...))
}

func (l *stdLogger) Error(args ...interface{}) {
	fmt.Println("[ERROR]", fmt.Sprint(args...))
}
