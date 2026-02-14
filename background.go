package testutils

import (
    "context"
    "fmt"
    "log"
    "sync"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/health/grpc_health_v1"
)

// HealthStatus represents the current state of the connection.
type HealthStatus string

const (
    StatusHealthy   HealthStatus = "HEALTHY"
    StatusUnhealthy HealthStatus = "UNHEALTHY"
    StatusUnknown   HealthStatus = "UNKNOWN"
)

// HealthMonitorConfig configures the background health checker.
type HealthMonitorConfig struct {
    // CheckInterval is how often to ping the server.
    CheckInterval time.Duration
    // UnhealthyThreshold is the number of consecutive failures before marking as Unhealthy.
    UnhealthyThreshold int
    // HealthyThreshold is the number of consecutive successes before marking as Healthy.
    HealthyThreshold int
    // OnStatusChange is a callback invoked when the status changes.
    OnStatusChange func(status HealthStatus)
}

// DefaultHealthMonitorConfig returns a standard configuration.
func DefaultHealthMonitorConfig() *HealthMonitorConfig {
    return &HealthMonitorConfig{
        CheckInterval:      5 * time.Second,
        UnhealthyThreshold: 2, // Fail twice before alerting
        HealthyThreshold:   1, // Pass once to recover
        OnStatusChange:     func(status HealthStatus) { log.Printf("Health Status Changed: %s", status) },
    }
}

// HealthMonitor runs a background loop checking the gRPC health endpoint.
type HealthMonitor struct {
    client     grpc_health_v1.HealthClient
    config     *HealthMonitorConfig
    current    HealthStatus
    failCount  int
    successCount int
    mu         sync.RWMutex
    ctx        context.Context
    cancel     context.CancelFunc
    wg         sync.WaitGroup
}

// NewHealthMonitor creates a new monitor attached to a connection.
func NewHealthMonitor(conn *grpc.ClientConn, cfg *HealthMonitorConfig) *HealthMonitor {
    if cfg == nil {
        cfg = DefaultHealthMonitorConfig()
    }
    ctx, cancel := context.WithCancel(context.Background())
    return &HealthMonitor{
        client:  grpc_health_v1.NewHealthClient(conn),
        config:  cfg,
        current: StatusUnknown,
        ctx:     ctx,
        cancel:  cancel,
    }
}

// Start begins the monitoring loop in a background goroutine.
func (hm *HealthMonitor) Start() {
    hm.wg.Add(1)
    go func() {
        defer hm.wg.Done()
        ticker := time.NewTicker(hm.config.CheckInterval)
        defer ticker.Stop()

        for {
            select {
            case <-hm.ctx.Done():
                return
            case <-ticker.C:
                hm.checkHealth()
            }
        }
    }()
}

// Stop gracefully shuts down the monitor.
func (hm *HealthMonitor) Stop() {
    hm.cancel()
    hm.wg.Wait()
}

// checkHealth performs the actual RPC check and updates state.
func (hm *HealthMonitor) checkHealth() {
    ctx, cancel := context.WithTimeout(hm.ctx, 2*time.Second)
    defer cancel()

    req := &grpc_health_v1.HealthCheckRequest{Service: ""}
    resp, err := hm.client.Check(ctx, req)

    hm.mu.Lock()
    defer hm.mu.Unlock()

    isHealthy := (err == nil && resp.Status == grpc_health_v1.HealthCheckResponse_SERVING)

    if isHealthy {
        hm.successCount++
        hm.failCount = 0
    } else {
        hm.failCount++
        hm.successCount = 0
    }

    previousStatus := hm.current
    newStatus := hm.current

    // Determine State Transition
    if isHealthy {
        if hm.current != StatusHealthy {
            if hm.successCount >= hm.config.HealthyThreshold {
                newStatus = StatusHealthy
            }
        }
    } else {
        if hm.current != StatusUnhealthy {
            if hm.failCount >= hm.config.UnhealthyThreshold {
                newStatus = StatusUnhealthy
            }
        }
    }

    // Trigger Callback if changed
    if previousStatus != newStatus {
        hm.current = newStatus
        if hm.config.OnStatusChange != nil {
            // Run callback in goroutine to avoid blocking the monitor loop
            go hm.config.OnStatusChange(newStatus)
        }
    }
}

// GetStatus returns the current cached health status.
func (hm *HealthMonitor) GetStatus() HealthStatus {
    hm.mu.RLock()
    defer hm.mu.RUnlock()
    return hm.current
}

// --- Async Task Manager ---

// AsyncTask represents a job to be done in the background.
type AsyncTask struct {
    Name string
    Func func() error
}

// TaskResult contains the outcome of an async task.
type TaskResult struct {
    Name string
    Err  error
}

// AsyncTaskManager manages a pool of background workers.
type AsyncTaskManager struct {
    queue chan AsyncTask
    wg    sync.WaitGroup
    // Result channel is optional; if nil, results are just logged.
    ResultChan chan TaskResult 
}

// NewAsyncTaskManager creates a manager with a specific buffer size.
func NewAsyncTaskManager(queueSize int, resultChanSize int) *AsyncTaskManager {
    return &AsyncTaskManager{
        queue:      make(chan AsyncTask, queueSize),
        ResultChan: make(chan TaskResult, resultChanSize),
    }
}

// Start launches the worker goroutines.
func (am *AsyncTaskManager) Start(numWorkers int) {
    for i := 0; i < numWorkers; i++ {
        am.wg.Add(1)
        go am.worker(i)
    }
}

// Stop waits for all queued tasks to finish.
func (am *AsyncTaskManager) Stop() {
    close(am.queue)
    am.wg.Wait()
    close(am.ResultChan)
}

// Submit adds a task to the queue. Non-blocking if queue is full.
func (am *AsyncTaskManager) Submit(task AsyncTask) error {
    select {
    case am.queue <- task:
        return nil
    default:
        return fmt.Errorf("task queue full, cannot submit task: %s", task.Name)
    }
}

// worker processes tasks from the queue.
func (am *AsyncTaskManager) worker(id int) {
    defer am.wg.Done()
    for task := range am.queue {
        log.Printf("[Worker %d] Starting task: %s", id, task.Name)
        
        // Panic recovery for individual tasks
        func() {
            defer func() {
                if r := recover(); r != nil {
                    log.Printf("[Worker %d] Panic in task %s: %v", id, task.Name, r)
                    if am.ResultChan != nil {
                        am.ResultChan <- TaskResult{Name: task.Name, Err: fmt.Errorf("panic: %v", r)}
                    }
                }
            }()

            err := task.Func()
            if am.ResultChan != nil {
                am.ResultChan <- TaskResult{Name: task.Name, Err: err}
            }

            if err != nil {
                log.Printf("[Worker %d] Task %s failed: %v", id, task.Name, err)
            } else {
                log.Printf("[Worker %d] Task %s completed successfully", id, task.Name)
            }
        }()
    }
}

// --- Polling Utility ---

// Poller executes a condition function periodically until it returns true or an error occurs.
func Poller(ctx context.Context, interval time.Duration, condition func() (bool, error)) error {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            done, err := condition()
            if err != nil {
                return err
            }
            if done {
                return nil
            }
        }
    }
}

// --- Example Usage ---

/*
func backgroundExampleMain() {
    // 1. Setup Client
    conn, _ := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
    
    // 2. Start Health Monitor
    monitorCfg := &HealthMonitorConfig{
        CheckInterval: 2 * time.Second,
        OnStatusChange: func(s HealthStatus) {
            if s == StatusUnhealthy {
                log.Println("ALERT: PAYMENT SERVICE IS DOWN!")
                // Trigger circuit breaker logic here
            }
        },
    }
    monitor := NewHealthMonitor(conn, monitorCfg)
    monitor.Start()
    defer monitor.Stop()

    // 3. Start Async Task Manager (e.g., for sending webhooks or emails)
    taskMgr := NewAsyncTaskManager(100, 100) // Queue 100, Buffer 100 results
    taskMgr.Start(5) // 5 Background workers
    defer taskMgr.Stop()

    // Submit a background job
    taskMgr.Submit(AsyncTask{