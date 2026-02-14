// Package context provides patterns, examples, and best practices for using
// Go's context package effectively. It's not meant to be imported directly,
// but serves as documentation and reference for the team.
package testutils

import (
    "context"
    "errors"
    "fmt"
    "net/http"
    "runtime"
    "sync"
    "time"
)

// ----------------------------------------------------------------------
// 1. BASIC CONTEXT CREATION
// ----------------------------------------------------------------------

// BackgroundAndTODO demonstrates the two root contexts.
func BackgroundAndTODO() {
    // Background: empty root context, never cancelled, never times out.
    // Use in main(), init(), and tests.
    ctx := context.Background()
    _ = ctx

    // TODO: when you haven't decided which context to use yet.
    // Your linter will complain about this - that's the point!
    ctx = context.TODO()
    _ = ctx
}

// WithCancel demonstrates cancellation propagation.
func WithCancel() context.Context {
    ctx, cancel := context.WithCancel(context.Background())
    
    // ALWAYS call cancel to avoid leaks.
    // Defer is safe even if cancel is called earlier.
    defer cancel() // Called when function exits
    
    // Pass ctx to goroutines, operations, etc.
    _ = ctx
    
    // Call cancel() explicitly to signal cancellation.
    // cancel() is idempotent - safe to call multiple times.
    cancel()
    
    return ctx
}

// WithTimeout demonstrates automatic cancellation after a duration.
func WithTimeout() error {
    // Context will be cancelled after 5 seconds OR when cancel() is called.
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel() // Always call cancel to free resources
    
    // Simulate work
    select {
    case <-time.After(10 * time.Second):
        return nil
    case <-ctx.Done():
        return ctx.Err() // context deadline exceeded
    }
}

// WithDeadline demonstrates cancellation at a specific time.
func WithDeadline() error {
    deadline := time.Now().Add(5 * time.Second)
    ctx, cancel := context.WithDeadline(context.Background(), deadline)
    defer cancel()
    
    select {
    case <-time.After(10 * time.Second):
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

// WithValue demonstrates request-scoped context values.
func WithValue() {
    // ONLY use for request-scoped data, not optional parameters.
    // Keys should be custom types, not built-ins.
    type key string
    const userIDKey key = "userID"
    
    ctx := context.WithValue(context.Background(), userIDKey, "user-123")
    
    // Retrieve value
    if userID, ok := ctx.Value(userIDKey).(string); ok {
        fmt.Println("User ID:", userID)
    }
}

// ----------------------------------------------------------------------
// 2. CONTEXT RULES & BEST PRACTICES
// ----------------------------------------------------------------------

/*
┌─────────────────────────────────────────────────────────────┐
│                    CONTEXT RULES                            │
├─────────────────────────────────────────────────────────────┤
│ 1. ctx should be the FIRST parameter in a function         │
│ 2. NEVER store contexts in structs                         │
│ 3. ALWAYS call cancel to avoid leaks                       │
│ 4. ctx is immutable; only the goroutine that creates       │
│    the context should call cancel                          │
│ 5. Use context values sparingly - they're not type-safe    │
│ 6. Context cancellation is advisory, not mandatory         │
└─────────────────────────────────────────────────────────────┘
*/

// ✅ GOOD: Context as first parameter
func DoSomething(ctx context.Context, arg string) error {
    return nil
}

// ❌ BAD: Storing context in struct
type BadStruct struct {
    ctx context.Context // Never do this!
}

// ✅ GOOD: Context passed explicitly
type GoodStruct struct {
    // No context field
}

func (g *GoodStruct) Do(ctx context.Context) error {
    return nil
}

// ❌ BAD: Not calling cancel
func LeakyContext() {
    ctx, _ := context.WithCancel(context.Background())
    // Cancel not called - resources held until ctx is GC'd
    _ = ctx
}

// ✅ GOOD: Always call cancel
func NonLeakyContext() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    _ = ctx
}

// ----------------------------------------------------------------------
// 3. CONTEXT IN GOROUTINES
// ----------------------------------------------------------------------

// Worker demonstrates proper context handling in goroutines.
func Worker(ctx context.Context, wg *sync.WaitGroup) error {
    defer wg.Done()
    
    for {
        select {
        case <-ctx.Done():
            // Parent cancelled - clean up and exit
            fmt.Println("Worker shutting down:", ctx.Err())
            return ctx.Err()
        default:
            // Do work
            time.Sleep(100 * time.Millisecond)
        }
    }
}

// SpawnWorkers demonstrates managing multiple goroutines with context.
func SpawnWorkers() error {
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
    defer cancel()
    
    var wg sync.WaitGroup
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go Worker(ctx, &wg)
    }
    
    wg.Wait() // Wait for all workers to finish
    return nil
}

// ----------------------------------------------------------------------
// 4. HTTP CONTEXT PATTERNS
// ----------------------------------------------------------------------

// HTTPHandler demonstrates context in HTTP servers.
func HTTPHandler(w http.ResponseWriter, r *http.Request) {
    // Request carries a context that's cancelled when the client disconnects.
    ctx := r.Context()
    
    // Always check if client is still there for long operations.
    select {
    case <-time.After(2 * time.Second):
        fmt.Fprintln(w, "Done!")
    case <-ctx.Done():
        // Client disconnected
        http.Error(w, "Client disconnected", http.StatusRequestTimeout)
    }
}

// HTTPClient demonstrates context in HTTP clients.
func HTTPClient(ctx context.Context) error {
    req, err := http.NewRequestWithContext(ctx, "GET", "https://api.example.com", nil)
    if err != nil {
        return err
    }
    
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        // Could be context.Canceled or context.DeadlineExceeded
        return fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()
    
    return nil
}

// Middleware adds request-scoped values to context.
func Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        type key string
        const requestIDKey key = "requestID"
        
        // Add request ID to context
        ctx := context.WithValue(r.Context(), requestIDKey, generateRequestID())
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func generateRequestID() string {
    return fmt.Sprintf("%d", time.Now().UnixNano())
}

// ----------------------------------------------------------------------
// 5. CONTEXT IN DATABASE OPERATIONS
// ----------------------------------------------------------------------

// DatabaseQuery demonstrates context cancellation with SQL.
func DatabaseQuery(ctx context.Context, db interface {
    QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
}) error {
    // Most Go database drivers support context cancellation.
    // If the context is cancelled mid-query, the driver will abort.
    
    rows, err := db.QueryContext(ctx, "SELECT * FROM users")
    if err != nil {
        return err
    }
    defer rows.Close()
    
    // Process rows...
    return nil
}

// ----------------------------------------------------------------------
// 6. CUSTOM CONTEXT CANCELLATION
// ----------------------------------------------------------------------

// OperationWithRetry demonstrates custom cancellation handling.
func OperationWithRetry(ctx context.Context) error {
    maxRetries := 3
    backoff := 100 * time.Millisecond
    
    for attempt := 0; attempt < maxRetries; attempt++ {
        select {
        case <-ctx.Done():
            // Parent cancelled - don't retry
            return fmt.Errorf("operation cancelled: %w", ctx.Err())
        default:
            // Attempt operation
            err := tryOperation()
            if err == nil {
                return nil
            }
            
            // Exponential backoff with context awareness
            timer := time.NewTimer(backoff)
            select {
            case <-ctx.Done():
                timer.Stop()
                return ctx.Err()
            case <-timer.C:
                backoff *= 2
            }
        }
    }
    
    return errors.New("max retries exceeded")
}

func tryOperation() error {
    return nil // Simulated operation
}

// ----------------------------------------------------------------------
// 7. CONTEXT LEAK DETECTION
// ----------------------------------------------------------------------

// DetectLeaks demonstrates how to find context leaks in tests.
func DetectLeaks() func() {
    // Before test: capture goroutine count
    before := runtime.NumGoroutine()
    
    // Return cleanup function to check for leaks
    return func() {
        time.Sleep(100 * time.Millisecond) // Allow goroutines to exit
        after := runtime.NumGoroutine()
        if after > before {
            fmt.Printf("Potential goroutine leak: %d -> %d\n", before, after)
        }
    }
}

// ----------------------------------------------------------------------
// 8. CONTEXT ERROR HANDLING
// ----------------------------------------------------------------------

// HandleContextError demonstrates proper error inspection.
func HandleContextError(err error) {
    switch {
    case errors.Is(err, context.Canceled):
        fmt.Println("Operation was cancelled")
    case errors.Is(err, context.DeadlineExceeded):
        fmt.Println("Operation timed out")
    default:
        fmt.Println("Other error:", err)
    }
}

// ----------------------------------------------------------------------
// 9. CHILD CONTEXT PATTERNS
// ----------------------------------------------------------------------

// ChildWithTimeout creates a child context with stricter timeout.
func ChildWithTimeout(parent context.Context) error {
    // Child context inherits parent cancellation but has its own timeout.
    // If parent cancels, child is cancelled immediately.
    // If child times out, parent remains unaffected.
    
    ctx, cancel := context.WithTimeout(parent, 1*time.Second)
    defer cancel()
    
    // Do work with stricter deadline
    select {
    case <-time.After(2 * time.Second):
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

// ----------------------------------------------------------------------
// 10. CONTEXT WITH SYNC.ONCE
// ----------------------------------------------------------------------

// SafeCancel demonstrates idempotent cancellation with sync.Once.
type SafeContext struct {
    ctx    context.Context
    cancel context.CancelFunc
    once   sync.Once
}

func NewSafeContext() *SafeContext {
    ctx, cancel := context.WithCancel(context.Background())
    return &SafeContext{
        ctx:    ctx,
        cancel: cancel,
    }
}

// Cancel is safe to call multiple times.
func (sc *SafeContext) Cancel() {
    sc.once.Do(func() {
        sc.cancel()
    })
}

// ----------------------------------------------------------------------
// 11. REAL-WORLD EXAMPLE: SERVER SHUTDOWN
// ----------------------------------------------------------------------

// Server demonstrates graceful shutdown with context.
type Server struct {
    httpServer *http.Server
    done       chan error
}

func NewServer(addr string, handler http.Handler) *Server {
    return &Server{
        httpServer: &http.Server{
            Addr:    addr,
            Handler: handler,
        },
        done: make(chan error, 1),
    }
}

// Start runs the server in a goroutine.
func (s *Server) Start() {
    go func() {
        s.done <- s.httpServer.ListenAndServe()
    }()
}

// Shutdown gracefully stops the server with context timeout.
func (s *Server) Shutdown(ctx context.Context) error {
    return s.httpServer.Shutdown(ctx)
}

// Wait waits for server to exit or context to cancel.
func (s *Server) Wait(ctx context.Context) error {
    select {
    case err := <-s.done:
        return err
    case <-ctx.Done():
        return s.Shutdown(ctx)
    }
}

// ----------------------------------------------------------------------
// 12. COMMON PITFALLS
// ----------------------------------------------------------------------

/*
┌─────────────────────────────────────────────────────────────┐
│                  COMMON CONTEXT PITFALLS                    │
├─────────────────────────────────────────────────────────────┤
│                                                            │
│ 1. Storing contexts in structs                            │
│    ✓ Pass context explicitly, don't store it              │
│                                                            │
│ 2. Not calling cancel()                                   │
│    ✓ Always defer cancel() after WithCancel/WithTimeout   │
│                                                            │
│ 3. Ignoring ctx.Done() in long-running operations         │
│    ✓ Check ctx.Done() in loops or blocking operations     │
│                                                            │
│ 4. Using context.Value() for optional parameters          │
│    ✓ Use explicit function parameters instead             │
│                                                            │
│ 5. Passing nil context                                    │
│    ✓ Use context.Background() or context.TODO()           │
│                                                            │
│ 6. Assuming cancellation is immediate                     │
│    ✓ Resources may need time to clean up                  │
│                                                            │
│ 7. Using built-in types as context keys                   │
│    ✓ Define custom type for keys to avoid collisions      │
│                                                            │
└─────────────────────────────────────────────────────────────┘
*/

// Example of proper context key pattern
type contextKey string

const (
    UserIDKey    contextKey = "user_id"
    TraceIDKey   contextKey = "trace_id"
    RequestIDKey contextKey = "request_id"
)

func SetUserID(ctx context.Context, userID string) context.Context {
    return context.WithValue(ctx, UserIDKey, userID)
}

func GetUserID(ctx context.Context) (string, bool) {
    val := ctx.Value(UserIDKey)
    if val == nil {
        return "", false
    }
    s, ok := val.(string)
    return s, ok
}