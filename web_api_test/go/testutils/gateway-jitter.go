package testutils

import (
    "context"
    "fmt"
    "math"
    "math/rand"
    "sync"
    "time"

    "google.golang.org/grpc"
)

// JitterStrategy defines the algorithm used to calculate randomness.
type JitterStrategy string

const (
    // JitterFull sets the delay to a random value between 0 and the cap.
    // Best for: Initial retries to spread out load immediately.
    JitterFull JitterStrategy = "full"

    // JitterEqual splits the delay: (base / 2) + random(0, base / 2).
    // Best for: General purpose retry/backoff (balances mean delay and spread).
    JitterEqual JitterStrategy = "equal"

    // JitterDecorrelated sets delay to random(base, cap * 3).
    // Best for: Long-running connections, prevents correlated spikes.
    JitterDecorrelated JitterStrategy = "decorrelated"
)

// JitterCalculator computes the wait duration based on strategy.
type JitterCalculator struct {
    Strategy JitterStrategy
    // RandomSource allows overriding the random generator (useful for testing)
    RandomSource *rand.Rand
}

// NewJitterCalculator creates a new calculator with a default strategy.
func NewJitterCalculator(strategy JitterStrategy) *JitterCalculator {
    return &JitterCalculator{
        Strategy:     strategy,
        RandomSource: rand.New(rand.NewSource(time.Now().UnixNano())),
    }
}

// Duration calculates the jittered wait time.
func (j *JitterCalculator) Duration(base time.Duration, cap time.Duration) time.Duration {
    if base <= 0 {
        return 0
    }

    var delay float64
    switch j.Strategy {
    case JitterFull:
        // Random between 0 and base
        delay = j.RandomSource.Float64() * float64(base)

    case JitterEqual:
        // base/2 + random between 0 and base/2
        half := float64(base) / 2.0
        delay = half + (j.RandomSource.Float64() * half)

    case JitterDecorrelated:
        // random(base, 3*base)
        // This tends to increase rapidly and then jitter around the cap
        tempBase := float64(base)
        delay = tempBase + (j.RandomSource.Float64() * (tempBase * 2)) 

    default:
        delay = float64(base)
    }

    // Apply Cap
    if cap > 0 && delay > float64(cap) {
        delay = float64(cap)
    }

    return time.Duration(delay)
}

// --- Rate Limiter (Client-Side Traffic Shaping) ---

// RateLimiterConfig configures the client-side rate limiter.
type RateLimiterConfig struct {
    // RequestsPerSecond is the target throughput.
    RequestsPerSecond float64
    // MaxBurst allows for short bursts above the rate limit.
    MaxBurst int
    // JitterStrategy adds randomness to the sleep time to smooth out traffic.
    JitterStrategy JitterStrategy
}

// DefaultRateLimiterConfig returns a standard configuration.
func DefaultRateLimiterConfig() *RateLimiterConfig {
    return &RateLimiterConfig{
        RequestsPerSecond: 100, // 100 RPS
        MaxBurst:          5,
        JitterStrategy:    JitterEqual,
    }
}

// tokenBucket is a thread-safe implementation of the Token Bucket algorithm.
type tokenBucket struct {
    rate       float64       // tokens per second
    capacity   float64       // max tokens
    tokens     float64       // current tokens
    lastUpdate time.Time     // last time tokens were added
    mu         sync.Mutex    // mutex for thread safety
    jitter     *JitterCalculator
}

func newTokenBucket(rps float64, burst int, strategy JitterStrategy) *tokenBucket {
    return &tokenBucket{
        rate:       rps,
        capacity:   float64(burst),
        tokens:     float64(burst), // Start full
        lastUpdate: time.Now(),
        jitter:     NewJitterCalculator(strategy),
    }
}

// wait calculates the time required to wait until a token is available.
func (tb *tokenBucket) wait() time.Duration {
    tb.mu.Lock()
    defer tb.mu.Unlock()

    now := time.Now()
    elapsed := now.Sub(tb.lastUpdate).Seconds()
    
    // Add tokens based on elapsed time
    tb.tokens += elapsed * tb.rate
    if tb.tokens > tb.capacity {
        tb.tokens = tb.capacity
    }
    tb.lastUpdate = now

    if tb.tokens >= 1.0 {
        tb.tokens -= 1.0
        return 0 // No wait needed
    }

    // Calculate wait time needed for 1 token
    needed := 1.0 - tb.tokens
    waitSeconds := needed / tb.rate
    waitDuration := time.Duration(waitSeconds * float64(time.Second))

    // Apply Jitter to the wait duration to prevent synchronized thundering herds across many clients
    // We cap the jitter base to a reasonable amount (e.g., 100ms) so we don't delay short requests too much
    jitteredDuration := tb.jitter.Duration(waitDuration, 100*time.Millisecond)

    return jitteredDuration
}

// RateLimitInterceptor is a gRPC client interceptor that throttles requests
// based on a token bucket algorithm with jitter.
func RateLimitInterceptor(cfg *RateLimiterConfig) grpc.UnaryClientInterceptor {
    bucket := newTokenBucket(cfg.RequestsPerSecond, cfg.MaxBurst, cfg.JitterStrategy)

    return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
        // Calculate wait time
        waitTime := bucket.wait()

        if waitTime > 0 {
            // Wait before invoking, or respect context cancellation
            select {
            case <-time.After(waitTime):
                // Proceed
            case <-ctx.Done():
                return ctx.Err()
            }
        }

        return invoker(ctx, method, req, reply, cc, opts...)
    }
}

// Example Usage in client setup:

/*
func NewJitteredGatewayClient(target string, rps float64) (PaymentClient, error) {
    
    // 1. Define Rate Limiting (Traffic Shaping)
    rateCfg := &RateLimiterConfig{
        RequestsPerSecond: rps,
        MaxBurst:          10,
        JitterStrategy:    JitterDecorrelated,
    }

    // 2. Define Retry Policy (Failure Recovery)
    retryCfg := &RetryPolicy{
        MaxAttempts:       3,
        InitialBackoff:    50 * time.Millisecond,
        MaxBackoff:        1 * time.Second,
        BackoffMultiplier: 2.0,
    }

    // 3. Dial Options
    dialOpts := []grpc.DialOption{
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithChainUnaryInterceptor(
            RateLimitInterceptor(rateCfg), // Shape traffic going out
            UnaryRetryInterceptor(retryCfg), // Handle traffic failing
            RequestIDClientInterceptor(),
            LoggingClientInterceptor(),
        ),
    }

    conn, err := grpc.Dial(target, dialOpts...)
    if err != nil {
        return nil, err
    }

    return &paymentClient{
        conn:   conn,
        client: pb.NewPaymentServiceClient(conn),
    }, nil
}
*/