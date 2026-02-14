package testutils

import (
    "context"
    "fmt"
    "math/rand"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

// RetryPolicy defines the configuration for retrying gRPC requests.
type RetryPolicy struct {
    // MaxAttempts is the maximum number of calls (including the initial attempt).
    MaxAttempts int
    // InitialBackoff is the amount of time to wait before the first retry.
    InitialBackoff time.Duration
    // MaxBackoff is the maximum amount of time to wait between retries.
    MaxBackoff time.Duration
    // BackoffMultiplier is the factor by which the backoff increases (e.g., 2.0 for exponential).
    BackoffMultiplier float64
    // RetryableStatuses is a list of gRPC status codes that trigger a retry.
    // If empty, it defaults to standard transient errors (Unavailable, DeadlineExceeded, etc.).
    RetryableStatuses []codes.Code
    // PerCallTimeout overrides the context timeout for each individual attempt.
    // If 0, the parent context's deadline is used.
    PerCallTimeout time.Duration
}

// DefaultRetryPolicy returns a standard policy suitable for most network conditions.
func DefaultRetryPolicy() *RetryPolicy {
    return &RetryPolicy{
        MaxAttempts:       4,
        InitialBackoff:    100 * time.Millisecond,
        MaxBackoff:        5 * time.Second,
        BackoffMultiplier: 2.0,
        RetryableStatuses: []codes.Code{
            codes.Unavailable,
            codes.DeadlineExceeded,
            codes.ResourceExhausted,
            codes.Aborted,
        },
        PerCallTimeout: 0, // Inherit from parent
    }
}

// NoRetryPolicy returns a policy that disables retries.
func NoRetryPolicy() *RetryPolicy {
    return &RetryPolicy{
        MaxAttempts: 1,
    }
}

// UnaryRetryInterceptor returns a grpc.UnaryClientInterceptor that implements the retry logic.
func UnaryRetryInterceptor(policy *RetryPolicy) grpc.UnaryClientInterceptor {
    return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
        // Ensure we have a valid policy
        if policy == nil {
            policy = DefaultRetryPolicy()
        }

        var lastErr error
        
        for attempt := 0; attempt < policy.MaxAttempts; attempt++ {
            // If this is not the first attempt, wait for backoff
            if attempt > 0 {
                backoff := calculateBackoff(policy, attempt)
                
                // Log retry attempt (optional, helpful for debugging)
                // fmt.Printf("Attempt %d failed. Retrying in %v...\n", attempt, backoff)
                
                select {
                case <-time.After(backoff):
                    // Proceed with retry
                case <-ctx.Done():
                    // Context cancelled while waiting for backoff
                    return status.Errorf(codes.DeadlineExceeded, "context cancelled before retry: %v", ctx.Err())
                }
            }

            // Set a timeout for this specific call if PerCallTimeout is configured
            callCtx := ctx
            if policy.PerCallTimeout > 0 {
                var cancel context.CancelFunc
                callCtx, cancel = context.WithTimeout(ctx, policy.PerCallTimeout)
                defer cancel()
            }

            // Invoke the RPC
            lastErr = invoker(callCtx, method, req, reply, cc, opts...)

            if lastErr == nil {
                // Success!
                return nil
            }

            // Check if the error is retryable
            if !isRetryableError(lastErr, policy.RetryableStatuses) {
                // Non-retryable error, bail out immediately
                return lastErr
            }
        }

        // We exhausted all retries
        return fmt.Errorf("retry policy exhausted: last error: %w", lastErr)
    }
}

// calculateBackoff computes the wait duration using Exponential Backoff with Jitter.
// Formula: min(cap, base * multiplier^(attempt-1)) + random_jitter
func calculateBackoff(policy *RetryPolicy, attempt int) time.Duration {
    if attempt < 1 {
        return 0
    }

    // Calculate exponential backoff
    backoff := float64(policy.InitialBackoff) 
    for i := 1; i < attempt; i++ {
        backoff *= policy.BackoffMultiplier
    }

    // Cap at MaxBackoff
    if backoff > float64(policy.MaxBackoff) {
        backoff = float64(policy.MaxBackoff)
    }

    // Add Jitter (randomization) to synchronize retries less aggressively
    // Full jitter: random(0, backoff)
    // Decorrelated jitter: random(base, backoff * 3)
    // Here we use a simple jitter of +/- 20%
    jitter := backoff * 0.2 * (rand.Float64()*2 - 1)

    return time.Duration(backoff + jitter)
}

// isRetryableError checks if the gRPC error should trigger a retry.
func isRetryableError(err error, customCodes []codes.Code) bool {
    st, ok := status.FromError(err)
    if !ok {
        // If it's not a gRPC status error (e.g., connection refused), we generally retry it.
        // However, if the context was cancelled, we should not retry.
        if err == context.Canceled || err == context.DeadlineExceeded {
            return false
        }
        return true
    }

    code := st.Code()

    // If the context was cancelled (even if wrapped in a status), don't retry.
    // gRPC often wraps context.Canceled as status Code(Canceled).
    if code == codes.Canceled {
        return false
    }

    // Check against custom list
    if len(customCodes) > 0 {
        for _, c := range customCodes {
            if code == c {
                return true
            }
        }
        return false
    }

    // Default behavior: Retry on specific transient codes if no custom list provided
    switch code {
    case codes.DeadlineExceeded, codes.Unavailable, codes.ResourceExhausted, codes.Aborted, codes.Internal:
        // Note: Be careful retrying 'Internal'. It might hide bugs. 
        // Usually only retry Internal if it's known to be transient (like stream reset).
        return true
    default:
        return false
    }
}

/*
// USAGE EXAMPLE:

// In your client setup (gateway-timeout.go):

retryPolicy := &testutils.RetryPolicy{
    MaxAttempts: 5,
    InitialBackoff: 200 * time.Millisecond,
    MaxBackoff: 10 * time.Second,
    RetryableStatuses: []codes.Code{codes.Unavailable, codes.DeadlineExceeded},
}

dialOpts := []grpc.DialOption{
    grpc.WithChainUnaryInterceptor(
        testutils.UnaryRetryInterceptor(retryPolicy),
    ),
}

*/