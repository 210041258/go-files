package testutils

import (
    "google.golang.org/grpc"
)

// ClientChainConfig aggregates all the distinct interceptor configurations
// into a single struct for easier management.
type ClientChainConfig struct {
    // Timeout configuration from timeout.go
    Timeout *TimeoutConfig

    // Retry configuration from RetryPolicy.go
    Retry *RetryPolicy

    // Rate Limiting configuration from gateway-jitter.go
    RateLimit *RateLimiterConfig

    // Authentication Token for interceptors.go
    AuthToken string

    // Feature Flags
    EnableRequestID bool
    EnableLogging   bool
}

// DefaultClientChainConfig returns a configuration with all standard
// features enabled (Timeout, Retry, Logging, RequestID).
func DefaultClientChainConfig() *ClientChainConfig {
    return &ClientChainConfig{
        Timeout:         NewDefaultTimeoutConfig(),
        Retry:           DefaultRetryPolicy(),
        RateLimit:       nil, // Disabled by default to avoid limiting local dev
        AuthToken:       "",
        EnableRequestID: true,
        EnableLogging:   true,
    }
}

// NewClientChain constructs the interceptor chain and returns a grpc.DialOption.
//
// Order of Execution (Outermost -> Innermost):
// 1. Timeout (Fails fast if deadline exceeded)
// 2. RequestID (Ensures tracing ID exists for all logs)
// 3. Auth (Injects credentials)
// 4. RateLimit (Throttles traffic, applies to retries as well)
// 5. Retry (Handles transient failures)
// 6. Logging (Logs the final result of the call)
func NewClientChain(cfg *ClientChainConfig) grpc.DialOption {
    var interceptors []grpc.UnaryClientInterceptor

    // 1. Timeout
    // We apply this first so that even the overhead of other interceptors
    // is counted against the deadline if desired, or strictly to fail fast.
    if cfg.Timeout != nil {
        interceptors = append(interceptors, ClientTimeoutInterceptor(cfg.Timeout))
    }

    // 2. Request ID
    if cfg.EnableRequestID {
        interceptors = append(interceptors, RequestIDClientInterceptor())
    }

    // 3. Auth
    if cfg.AuthToken != "" {
        interceptors = append(interceptors, AuthClientInterceptor(cfg.AuthToken))
    }

    // 4. Rate Limit
    if cfg.RateLimit != nil {
        interceptors = append(interceptors, RateLimitInterceptor(cfg.RateLimit))
    }

    // 5. Retry
    // Retry wraps the actual invoker. It handles backoff and re-invocation.
    if cfg.Retry != nil {
        interceptors = append(interceptors, UnaryRetryInterceptor(cfg.Retry))
    }

    // 6. Logging
    // Logging is usually innermost to capture the exact duration and final status code,
    // or outermost to capture the total wall-clock time including retries.
    // Here we place it innermost (closest to the actual RPC) to log the attempt outcome,
    // but often outer is preferred for "Client" perspective. 
    // Given previous files used Logging as the final wrapper, we place it here.
    if cfg.EnableLogging {
        interceptors = append(interceptors, LoggingClientInterceptor())
    }

    return grpc.WithChainUnaryInterceptor(interceptors...)
}

// --- Example: How this simplifies your Gateway setup ---

/*
// In your main() or client factory:

func CreateProductionClient(target string) (PaymentClient, error) {
    // Define all your policies in one place
    cfg := &ClientChainConfig{
        Timeout: &TimeoutConfig{
            DefaultTimeout: 2 * time.Second,
            PerMethodTimeouts: map[string]time.Duration{
                "/paymentpb.PaymentService/CreatePayment": 5 * time.Second,
            },
        },
        Retry: &RetryPolicy{
            MaxAttempts:       5,
            InitialBackoff:    100 * time.Millisecond,
            BackoffMultiplier: 2.0,
            MaxBackoff:        5 * time.Second,
        },
        RateLimit: &RateLimiterConfig{
            RequestsPerSecond: 500, // Limit to 500 RPS
            MaxBurst:          10,
            JitterStrategy:    JitterEqual,
        },
        AuthToken:       os.Getenv("SERVICE_JWT_TOKEN"),
        EnableRequestID: true,
        EnableLogging:   true,
    }

    dialOpts := []grpc.DialOption{
        grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
        NewClientChain(cfg), // <--- Inject the entire stack cleanly
    }

    conn, err := grpc.Dial(target, dialOpts...)
    // ... rest of client setup
}
*/