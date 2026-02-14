package testutils

import (
    "context"
    "fmt"
    "log"
    "sort"
    "sync"
    "sync/atomic"
    "time"

    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"

    // Import the generated protobuf package
    pb "path/to/your/protobuf/package/paymentpb"
)

// BurstMode defines how the burst traffic is generated.
type BurstMode string

const (
    // ModeConcurrent fires all requests as fast as possible (limited by Concurrency).
    ModeConcurrent BurstMode = "concurrent"
    // ModeSustained maintains a specific QPS (Requests Per Second) for a duration.
    ModeSustained BurstMode = "sustained"
)

// BurstConfig configures the stress test.
type BurstConfig struct {
    // TotalRequests is the number of requests to send during the burst.
    TotalRequests int
    // Duration is the time window for the burst (only used in ModeSustained).
    Duration time.Duration
    // Concurrency limits the number of simultaneous goroutines.
    Concurrency int
    // Mode determines the traffic shape.
    Mode BurstMode
}

// BurstResult contains detailed statistics about the burst run.
type BurstResult struct {
    TotalRequests   int64
    SuccessCount    int64
    FailureCount    int64
    Duration        time.Duration
    RequestsPerSec  float64
    LatencyHist     []int64 // Raw latencies in milliseconds for percentile calculation
    ErrorCounts     map[string]int64
}

// BurstStressor executes the burst test against the PaymentClient.
type BurstStressor struct {
    client PaymentClient
}

// NewBurstStressor creates a new stressor.
func NewBurstStressor(client PaymentClient) *BurstStressor {
    return &BurstStressor{client: client}
}

// Run executes the burst scenario.
func (b *BurstStressor) Run(ctx context.Context, req *pb.CreatePaymentRequest, cfg *BurstConfig) (*BurstResult, error) {
    if cfg.Concurrency <= 0 {
        cfg.Concurrency = 100 // Safe default
    }

    results := &BurstResult{
        ErrorCounts: make(map[string]int64),
        LatencyHist: make([]int64, 0, cfg.TotalRequests),
    }

    var wg sync.WaitGroup
    sem := make(chan struct{}, cfg.Concurrency)
    
    // Channels for aggregation
    latencyChan := make(chan int64, cfg.TotalRequests)
    errorChan := make(chan error, cfg.TotalRequests)

    startTime := time.Now()

    if cfg.Mode == ModeConcurrent {
        // Fire TotalRequests as fast as concurrency allows
        for i := 0; i < cfg.TotalRequests; i++ {
            wg.Add(1)
            go b.executeJob(ctx, req, sem, &wg, latencyChan, errorChan)
        }
    } else if cfg.Mode == ModeSustained {
        // Maintain QPS over Duration
        interval := time.Duration(float64(cfg.Duration) / float64(cfg.TotalRequests))
        ticker := time.NewTicker(interval)
        defer ticker.Stop()

        count := 0
        for count < cfg.TotalRequests {
            select {
            case <-ticker.C:
                wg.Add(1)
                go b.executeJob(ctx, req, sem, &wg, latencyChan, errorChan)
                count++
            case <-ctx.Done():
                log.Println("Burst cancelled by context")
                goto WaitFinish
            }
        }
    }

WaitFinish:
    wg.Wait()
    endTime := time.Now()

    // Collect results
    close(latencyChan)
    close(errorChan)

    for lat := range latencyChan {
        results.LatencyHist = append(results.LatencyHist, lat)
        atomic.AddInt64(&results.TotalRequests, 1)
    }

    for err := range errorChan {
        if err == nil {
            atomic.AddInt64(&results.SuccessCount, 1)
        } else {
            atomic.AddInt64(&results.FailureCount, 1)
            // Categorize error
            st, ok := status.FromError(err)
            errKey := "unknown"
            if ok {
                errKey = st.Code().String()
            } else {
                errKey = fmt.Sprintf("%T", err)
            }
            results.ErrorCounts[errKey]++
        }
    }

    results.Duration = endTime.Sub(startTime)
    if results.Duration.Milliseconds() > 0 {
        results.RequestsPerSec = float64(results.TotalRequests) / results.Duration.Seconds()
    }

    return results, nil
}

// executeJob performs a single RPC call and pushes metrics to channels.
func (b *BurstStressor) executeJob(ctx context.Context, req *pb.CreatePaymentRequest, sem chan struct{}, wg *sync.WaitGroup, latChan chan<- int64, errChan chan<- error) {
    defer wg.Done()
    
    // Acquire semaphore
    sem <- struct{}{}
    defer func() { <-sem }()

    start := time.Now()
    _, err := b.client.CreatePayment(ctx, req)
    latency := time.Since(start)

    // Send metrics (non-blocking)
    latChan <- latency.Milliseconds()
    errChan <- err
}

// PrintReport generates a human-readable report with percentiles.
func (r *BurstResult) PrintReport() {
    log.Println("========== BURST TEST REPORT ==========")
    log.Printf("Total Requests: %d", r.TotalRequests)
    log.Printf("Duration:       %v", r.Duration)
    log.Printf("Actual RPS:     %.2f", r.RequestsPerSec)
    log.Printf("Success:        %d", r.SuccessCount)
    log.Printf("Failure:        %d", r.FailureCount)

    if len(r.LatencyHist) > 0 {
        sort.Slice(r.LatencyHist, func(i, j int) bool { return r.LatencyHist[i] < r.LatencyHist[j] })
        
        p50 := r.LatencyHist[len(r.LatencyHist)*50/100]
        p95 := r.LatencyHist[len(r.LatencyHist)*95/100]
        p99 := r.LatencyHist[len(r.LatencyHist)*99/100]
        
        log.Println("---------- LATENCY (ms) ----------")
        log.Printf("Average: %d", average(r.LatencyHist))
        log.Printf("Min:     %d", r.LatencyHist[0])
        log.Printf("Max:     %d", r.LatencyHist[len(r.LatencyHist)-1])
        log.Printf("P50:     %d", p50)
        log.Printf("P95:     %d", p95)
        log.Printf("P99:     %d", p99)
    }

    if len(r.ErrorCounts) > 0 {
        log.Println("---------- ERRORS ----------")
        for code, count := range r.ErrorCounts {
            log.Printf("%s: %d", code, count)
        }
    }
    log.Println("=====================================")
}

func average(slice []int64) int64 {
    var total int64
    for _, v := range slice {
        total += v
    }
    return total / int64(len(slice))
}

// --- Example Usage ---

/*
func burstExampleMain() {
    // Setup client (using the chain we built earlier)
    client, _ := NewPaymentClient("localhost:50051", DefaultPaymentClientOptions())
    defer client.Close()

    stressor := NewBurstStressor(client)
    req := &pb.CreatePaymentRequest{Amount: 100, Currency: "USD", CustomerId: "burst_test"}

    // Scenario 1: Flash Crowd (Sudden Spike)
    // Send 5000 requests as fast as possible with 500 concurrent workers
    cfg := &BurstConfig{
        TotalRequests: 5000,
        Concurrency:   500,
        Mode:          ModeConcurrent,
    }

    log.Println("Starting Flash Crowd Burst...")
    result, _ := stressor.Run(context.Background(), req, cfg)
    result.PrintReport()

    // Scenario 2: High Throughput (Sustained)
    // Maintain 1000 RPS for 5 seconds
    cfg2 := &BurstConfig{
        TotalRequests: 5000, // 1000 * 5
        Duration:      5 * time.Second,
        Concurrency:   50,
        Mode:          ModeSustained,
    }

    log.Println("Starting Sustained Load Burst...")
    result2, _ := stressor.Run(context.Background(), req, cfg2)
    result2.PrintReport()
}
*/