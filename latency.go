package testutils

import (
    "fmt"
    "math"
    "sync"
    "sync/atomic"
    "time"

    "google.golang.org/grpc"
)

// DefaultLatencyBuckets defines the standard latency boundaries for microservices.
// Ranges from 0.5ms to 10s.
var DefaultLatencyBuckets = []float64{
    0.0005, // 0.5ms
    0.001,  // 1ms
    0.005,  // 5ms
    0.01,   // 10ms
    0.025,  // 25ms
    0.05,   // 50ms
    0.1,    // 100ms
    0.25,   // 250ms
    0.5,    // 500ms
    1.0,    // 1s
    2.5,    // 2.5s
    5.0,    // 5s
    10.0,   // 10s
}

// LatencyTracker records duration observations into a histogram.
// It is thread-safe and optimized for high throughput.
type LatencyTracker struct {
    buckets []float64       // The upper bounds of the buckets
    counts  []uint64        // Atomic counters for each bucket
    sum     atomic.Uint64   // Total duration in microseconds (for Average)
    total   atomic.Uint64   // Total count of observations
    min     atomic.Int64    // Minimum duration in nanoseconds
    max     atomic.Int64    // Maximum duration in nanoseconds
}

// NewLatencyTracker creates a new tracker with default buckets.
func NewLatencyTracker() *LatencyTracker {
    return NewLatencyTrackerWithBuckets(DefaultLatencyBuckets)
}

// NewLatencyTrackerWithBuckets creates a tracker with custom boundaries.
func NewLatencyTrackerWithBuckets(buckets []float64) *LatencyTracker {
    return &LatencyTracker{
        buckets: buckets,
        counts:  make([]uint64, len(buckets)),
        min:     atomic.Int64{} // Initialized to 0
        max:     atomic.Int64{},
    }
}

// Observe records a duration.
func (lt *LatencyTracker) Observe(d time.Duration) {
    dSec := d.Seconds()
    dMicro := uint64(d.Microseconds())
    dNano := int64(d.Nanoseconds())

    // 1. Update Sum and Total
    lt.sum.Add(dMicro)
    lt.total.Add(1)

    // 2. Update Min
    for {
        current := lt.min.Load()
        if current == 0 || dNano < current {
            if lt.min.CompareAndSwap(current, dNano) {
                break
            }
        } else {
            break
        }
    }

    // 3. Update Max
    for {
        current := lt.max.Load()
        if dNano > current {
            if lt.max.CompareAndSwap(current, dNano) {
                break
            }
        } else {
            break
        }
    }

    // 4. Find Bucket and Increment
    // Since we are using atomic ops on specific indices, we don't need a global lock.
    // However, finding the index requires reading the slice (safe in Go).
    idx := -1
    for i, bound := range lt.buckets {
        if dSec <= bound {
            idx = i
            break
        }
    }

    if idx != -1 {
        atomic.AddUint64(&lt.counts[idx], 1)
    } else {
        // It exceeded the last defined bucket, effectively we could add an "Inf" bucket logic here
        // For this impl, we just ignore if it exceeds max bounds or add it to the last one if desired.
        // Let's increment the last bucket as overflow
        lastIdx := len(lt.counts) - 1
        atomic.AddUint64(&lt.counts[lastIdx], 1)
    }
}

// Snapshot captures a consistent view of the current metrics.
type Snapshot struct {
    Total    uint64
    Sum      uint64 // microseconds
    Avg      float64 // milliseconds
    Min      float64 // milliseconds
    Max      float64 // milliseconds
    P50      float64 // milliseconds
    P90      float64 // milliseconds
    P99      float64 // milliseconds
    Buckets  map[float64]uint64
}

// Snapshot calculates the current statistics.
// This is not strictly atomic across all fields, but sufficient for monitoring.
func (lt *LatencyTracker) Snapshot() *Snapshot {
    total := lt.total.Load()
    if total == 0 {
        return &Snapshot{}
    }

    sum := lt.sum.Load()
    
    // Calculate percentiles by walking buckets
    // We need a copy of counts to avoid index out of bounds or race during calculation
    // (though atomic reads are safe, the loop logic relies on consistency)
    countsCopy := make([]uint64, len(lt.counts))
    for i := range lt.counts {
        countsCopy[i] = atomic.LoadUint64(&lt.counts[i])
    }

    bucketMap := make(map[float64]uint64)
    for i, count := range countsCopy {
        bucketMap[lt.buckets[i]] = count
    }

    snap := &Snapshot{
        Total:   total,
        Sum:     sum,
        Avg:     float64(sum) / float64(total),
        Min:     float64(lt.min.Load()) / 1e6,
        Max:     float64(lt.max.Load()) / 1e6,
        Buckets: bucketMap,
    }

    snap.P50 = lt.calculatePercentile(0.50, countsCopy)
    snap.P90 = lt.calculatePercentile(0.90, countsCopy)
    snap.P99 = lt.calculatePercentile(0.99, countsCopy)

    return snap
}

// calculatePercentile estimates the value at the given percentile (0.0 - 1.0).
// Returns the upper bound of the bucket where the percentile falls.
func (lt *LatencyTracker) calculatePercentile(p float64, counts []uint64) float64 {
    total := lt.total.Load()
    if total == 0 {
        return 0
    }

    rank := uint64(float64(total) * p)
    var accumulated uint64

    for i, count := range counts {
        accumulated += count
        if accumulated >= rank {
            // Return the upper bound of this bucket in milliseconds
            return lt.buckets[i] * 1000
        }
    }
    
    // If we run out of buckets, return the max bound
    return lt.buckets[len(lt.buckets)-1] * 1000
}

// Reset clears all metrics.
func (lt *LatencyTracker) Reset() {
    lt.total.Store(0)
    lt.sum.Store(0)
    lt.min.Store(0)
    lt.max.Store(0)
    for i := range lt.counts {
        atomic.StoreUint64(&lt.counts[i], 0)
    }
}

// String returns a formatted report.
func (s *Snapshot) String() string {
    if s.Total == 0 {
        return "No data recorded yet."
    }
    return fmt.Sprintf(
        "Req: %d | Avg: %.2fms | Min: %.2fms | Max: %.2fms | P50: %.2fms | P90: %.2fms | P99: %.2fms",
        s.Total, s.Avg, s.Min, s.Max, s.P50, s.P90, s.P99,
    )
}

// --- Interceptor ---

// LatencyInterceptor returns a client interceptor that automatically records
// the latency of every RPC call into the provided tracker.
func LatencyInterceptor(tracker *LatencyTracker) grpc.UnaryClientInterceptor {
    return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
        start := time.Now()
        err := invoker(ctx, method, req, reply, cc, opts...)
        elapsed := time.Since(start)
        
        tracker.Observe(elapsed)
        
        return err
    }
}

// --- Example Usage (Monitoring Loop) ---

/*
func monitorExample() {
    tracker := NewLatencyTracker()

    // 1. Add interceptor to client
    dialOpts := []grpc.DialOption{
        grpc.WithChainUnaryInterceptor(
            LatencyInterceptor(tracker),
            // ... other interceptors
        ),
    }
    // ... create client ...

    // 2. Start a background monitor
    go func() {
        ticker := time.NewTicker(5 * time.Second)
        for range ticker.C {
            snap := tracker.Snapshot()
            log.Printf("[METRICS] %s", snap.String())
            
            // Optional: Reset every interval to get "windowed" stats
            // tracker.Reset() 
        }
    }()

    // 3. Run traffic...
}
*/