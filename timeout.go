package testutils

import (
    "context"
    "fmt"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

// TimeoutConfig defines the timeout behavior for the client.
type TimeoutConfig struct {
    // DefaultTimeout is the maximum time allowed for an RPC if the caller
    // does not set a specific deadline in the context.
    DefaultTimeout time.Duration
    
    // PerMethodTimeouts allows overriding the default timeout for specific gRPC methods.
    // Map key is the full gRPC method name (e.g., "/paymentpb.PaymentService/CreatePayment").
    PerMethodTimeouts map[string]time.Duration
}

// NewDefaultTimeoutConfig returns a standard configuration.
func NewDefaultTimeoutConfig() *TimeoutConfig {
    return &TimeoutConfig{
        DefaultTimeout: 5 * time.Second, // Conservative default
        PerMethodTimeouts: map[string]time.Duration{
            // Example: Payment creation might take longer due to bank processing
            // "/paymentpb.PaymentService/CreatePayment": 10 * time.Second,
        },
    }
}

// ClientTimeoutInterceptor returns a UnaryClientInterceptor that enforces timeouts.
// It ensures that if the parent context has no deadline, one is applied automatically.
// If the parent context already has a deadline, it is respected.
func ClientTimeoutInterceptor(cfg *TimeoutConfig) grpc.UnaryClientInterceptor {
    return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
        // Determine the target timeout for this specific method
        timeout := cfg.DefaultTimeout
        if customTimeout, ok := cfg.PerMethodTimeouts[method]; ok {
            timeout = customTimeout
        }

        // Logic: Only wrap the context if a deadline does NOT already exist.
        // If the user passed context.WithTimeout(...) to the client method, we respect that.
        // If the user passed context.Background(), we apply the DefaultTimeout.
        
        var cancel context.CancelFunc
        if _, ok := ctx.Deadline(); !ok {
            ctx, cancel = context.WithTimeout(ctx, timeout)
            defer cancel()
        } else {
            // A deadline exists, but check if it's longer than our allowed max (optional safety check)
            if deadline, ok := ctx.Deadline(); ok {
                remaining := time.Until(deadline)
                // If the remaining time is longer than our configured max for this method,
                // we might choose to shrink it. Here we just trust the caller.
                _ = remaining
            }
        }

        // Invoke the RPC
        err := invoker(ctx, method, req, reply, cc, opts...)

        // Wrap errors for clarity
        if err != nil && ctx.Err() == context.DeadlineExceeded {
            // Determine if the error came from the RPC or the timeout
            if st, ok := status.FromError(err); ok && st.Code() == codes.DeadlineExceeded {
                return fmt.Errorf("client timeout exceeded (%v) for method %s: %w", timeout, method, err)
            }
            return fmt.Errorf("context deadline exceeded before completion of method %s: %w", method, ctx.Err())
        }

        return err
    }
}

// EnsureTimeout is a helper function to ensure a context has a timeout.
// It is useful for internal logic outside of interceptors.
func EnsureTimeout(ctx context.Context, defaultTimeout time.Duration) (context.Context, context.CancelFunc) {
    if _, ok := ctx.Deadline(); !ok {
        return context.WithTimeout(ctx, defaultTimeout)
    }
    return ctx, func() {} // No-op cancel function if context already has a deadline
}

// CheckDeadline is a helper for server-side logic or long-running loops.
// It returns an error immediately if the context has been cancelled/expired.
func CheckDeadline(ctx context.Context) error {
    select {
    case <-ctx.Done():
        // Convert context error to gRPC status
        if ctx.Err() == context.DeadlineExceeded {
            return status.Error(codes.DeadlineExceeded, "server operation timed out")
        }
        return status.Error(codes.Canceled, "operation canceled by client")
    default:
        return nil
    }
}

// --- Example Usage ---

/*
// In your client setup (gateway-timeout.go):

// 1. Define Timeout Rules
timeoutCfg := &TimeoutConfig{
    DefaultTimeout: 2 * time.Second,
    PerMethodTimeouts: map[string]time.Duration{
        "/paymentpb.PaymentService/CreatePayment": 5 * time.Second, // Give more time for creation
        "/paymentpb.PaymentService/GetPayment":    1 * time.Second, // Fast lookup
    },
}

// 2. Create Interceptor
timeoutInterceptor := ClientTimeoutInterceptor(timeoutCfg)

// 3. Add to Dial Options
dialOpts := []grpc.DialOption{
    grpc.WithTransportCredentials(insecure.NewCredentials()),
    grpc.WithChainUnaryInterceptor(
        timeoutInterceptor, // Should generally be the first/outermost wrapper
        UnaryRetryInterceptor(retryPolicy),
        // ...
    ),
}

// Usage Scenarios:

// Case A: No timeout provided by caller
// ctx := context.Background()
// client.CreatePayment(ctx, req) 
// Result: Uses DefaultTimeout (2s) or Method Override (5s).

// Case B: Caller provides specific timeout
// ctx,