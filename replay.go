package testutils

import (
    "context"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "sync"
    "sync/atomic"
    "time"

    "google.golang.org/protobuf/encoding/protojson"
    "google.golang.org/protobuf/types/known/durationpb"

)

// ReplayStats holds the metrics from a replay session.
type ReplayStats struct {
    TotalRequests   int64
    SuccessCount    int64
    FailureCount    int64
    TotalLatency    int64 // in milliseconds
    MinLatency      int64 // in milliseconds
    MaxLatency      int64 // in milliseconds
    ErrorDistribution map[string]int64
}

// ReplayOptions configures how the replay is executed.
type ReplayOptions struct {
    // Concurrency is the number of simultaneous goroutines making requests.
    Concurrency int
    // TotalRequests is the total number of requests to send.
    TotalRequests int
    // RequestPerSecond limits the rate (0 = no limit).
    RateLimit int
}

// DefaultReplayOptions returns a safe default configuration.
func DefaultReplayOptions() *ReplayOptions {
    return &ReplayOptions{
        Concurrency:    10,
        TotalRequests:  100,
        RateLimit:      0, // Unlimited
    }
}

// Replayer handles the execution of replay scenarios against the PaymentClient.
type Replayer struct {
    client PaymentClient
}

// NewReplayer creates a new Replayer instance.
func NewReplayer(client PaymentClient) *Replayer {
    return &Replayer{client: client}
}

// RunCreatePaymentReplay executes the CreatePayment call repeatedly based on options.
func (r *Replayer) RunCreatePaymentReplay(ctx context.Context, req *pb.CreatePaymentRequest, opts *ReplayOptions) (*ReplayStats, error) {
    if opts == nil {
        opts = DefaultReplayOptions()
    }

    stats := &ReplayStats{
        ErrorDistribution: make(map[string]int64),
        MinLatency:        -1, // Initialize to -1 so first valid update sets it
    }

    var wg sync.WaitGroup
    sem := make(chan struct{}, opts.Concurrency)

    // Rate limiter ticker
    var ticker *time.Ticker
    var limiter <-chan time.Time
    if opts.RateLimit > 0 {
        interval := time.Second / time.Duration(opts.RateLimit)
        ticker = time.NewTicker(interval)
        defer ticker.Stop()
        limiter = ticker.C
    }

    // Start workers
    log.Printf("Starting replay: Concurrency=%d, Total=%d, RateLimit=%d", opts.Concurrency, opts.TotalRequests, opts.RateLimit)
    startTime := time.Now()

    for i := 0; i < opts.TotalRequests; i++ {
        // Check for context cancellation
        select {
        case <-ctx.Done():
            log.Println("Replay cancelled by context")
            wg.Wait() // Wait for current in-flight requests
            return stats, ctx.Err()
        default:
        }

        // Respect rate limit
        if opts.RateLimit > 0 {
            <-limiter
        }

        // Acquire semaphore (concurrency limit)
        sem <- struct{}{}
        wg.Add(1)

        go func() {
            defer func() { <-sem }()
            defer wg.Done()

            reqStart := time.Now()
            _, err := r.client.CreatePayment(ctx, req)
            latency := time.Since(reqStart)

            // Update Stats
            atomic.AddInt64(&stats.TotalRequests, 1)
            latencyMs := latency.Milliseconds()
            atomic.AddInt64(&stats.TotalLatency, latencyMs)

            // Update Min/Max Latency (Need a CAS loop or mutex for precision, using atomic approx here for simplicity)
            // For strict accuracy in high contention, a mutex would be better, but atomic is faster for benchmarks.
            for {
                oldMin := atomic.LoadInt64(&stats.MinLatency)
                if oldMin == -1 || latencyMs < oldMin {
                    if atomic.CompareAndSwapInt64(&stats.MinLatency, oldMin, latencyMs) {
                        break
                    }
                } else {
                    break
                }
            }
            for {
                oldMax := atomic.LoadInt64(&stats.MaxLatency)
                if latencyMs > oldMax {
                    if atomic.CompareAndSwapInt64(&stats.MaxLatency, oldMax, latencyMs) {
                        break
                    }
                } else {
                    break
                }
            }

            if err != nil {
                atomic.AddInt64(&stats.FailureCount, 1)
                errStr := fmt.Sprintf("%v", err)
                // Limit error string length for map key
                if len(errStr) > 50 {
                    errStr = errStr[:50]
                }
                atomic.AddInt64(&stats.ErrorDistribution[errStr], 1)
            } else {
                atomic.AddInt64(&stats.SuccessCount, 1)
            }
        }()
    }

    wg.Wait()
    totalDuration := time.Since(startTime)
    
    log.Printf("Replay finished in %v", totalDuration)
    log.Printf("Stats - Success: %d, Failed: %d, Avg Latency: %dms", 
        stats.SuccessCount, stats.FailureCount, stats.TotalLatency/stats.TotalRequests)

    return stats, nil
}

// LoadCreatePaymentRequestFromFile loads a JSON file and unmarshals it into a CreatePaymentRequest.
// This allows you to edit request payloads in a text editor for replay testing.
func LoadCreatePaymentRequestFromFile(filepath string) (*pb.CreatePaymentRequest, error) {
    // Read file
    data, err := os.ReadFile(filepath)
    if err != nil {
        return nil, fmt.Errorf("failed to read file %s: %w", filepath, err)
    }

    // Use protojson for protobuf-compatible JSON unmarshalling
    req := &pb.CreatePaymentRequest{}
    unmarshaler := protojson.UnmarshalOptions{
        DiscardUnknown: true, // Ignore fields in JSON that aren't in the proto
    }

    if err := unmarshaler.Unmarshal(data, req); err != nil {
        return nil, fmt.Errorf("failed to unmarshal proto JSON: %w", err)
    }

    return req, nil
}

// SaveCreatePaymentRequestToFile saves the current request to a JSON file.
// Useful for capturing a live request to replay later.
func SaveCreatePaymentRequestToFile(req *pb.CreatePaymentRequest, filepath string) error {
    marshaler := protojson.MarshalOptions{
        Indent:          "  ",
        UseProtoNames:   false, // Use JSON field names (camelCase) instead of proto names (snake_case)
        EmitUnpopulated: true,
    }

    data, err := marshaler.Marshal(req)
    if err != nil {
        return fmt.Errorf("failed to marshal request to JSON: %w", err)
    }

    if err := os.WriteFile(filepath, data, 0644); err != nil {
        return fmt.Errorf("failed to write file %s: %w", filepath, err)
    }

    return nil
}

// Example usage of the Replay utilities
func replayExampleMain() {
    // 1. Initialize the client (as shown in gateway-timeout.go)
    target := "localhost:50051"
    client, err := NewPaymentClient(target, DefaultPaymentClientOptions())
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }
    defer client.Close()

    replayer := NewReplayer(client)

    // Scenario A: Replay a hardcoded request
    req := &pb.CreatePaymentRequest{
        Amount:     5000,
        Currency:   "USD",
        CustomerId: "cust_replay_test",
        Metadata:   "stress_test_run_1",
    }

    opts := &ReplayOptions{
        Concurrency:   5,
        TotalRequests: 20,
        RateLimit:     10, // 10 requests per second
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    stats, err := replayer.RunCreatePaymentReplay(ctx, req, opts)
    if err != nil {
        log.Printf("Replay ended with error: %v", err)
    }
    fmt.Printf("Final Stats: %+v\n", stats)

    // Scenario B: Load request from JSON and replay it
    // This assumes you have a file named "request_payload.json"
    /*
    jsonReq, err := LoadCreatePaymentRequestFromFile("request_payload.json")
    if err != nil {
        log.Fatal(err)
    }
    
    // Save a modified version back to disk for inspection
    // jsonReq.Amount = 9999
    // SaveCreatePaymentRequestToFile(jsonReq, "request_payload_modified.json")
    
    replayer.RunCreatePaymentReplay(ctx, jsonReq, opts)
    */
}