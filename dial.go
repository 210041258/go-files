package payment

import (
    "context"
    "crypto/tls"
    "fmt"
    "log"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    "google.golang.org/grpc/credentials/insecure"

    "yourapp/internal/discovery"
)

// DialOptions encapsulates all client-side connection options
type DialOptions struct {
    Target          string                     // e.g., "etcd:///payment-service" or "localhost:50051"
    TLSConfig       *tls.Config                // optional TLS
    RetryPolicy     *RetryPolicy               // optional retry policy
    RateLimiter     *RateLimiterConfig         // optional rate-limiter
    DialTimeout     time.Duration              // gRPC dial timeout
}

// DefaultDialOptions returns reasonable defaults
func DefaultDialOptions(target string) *DialOptions {
    return &DialOptions{
        Target:      target,
        DialTimeout: 5 * time.Second,
        RetryPolicy: &RetryPolicy{
            MaxAttempts:       3,
            InitialBackoff:    100 * time.Millisecond,
            MaxBackoff:        2 * time.Second,
            BackoffMultiplier: 2,
        },
        RateLimiter: &RateLimiterConfig{
            RequestsPerSecond: 100,
            MaxBurst:          5,
            JitterStrategy:    JitterEqual,
        },
    }
}

// DialGRPC establishes a gRPC connection with interceptors, TLS, retries, and rate-limiting
func DialGRPC(opts *DialOptions) (*grpc.ClientConn, error) {
    if opts == nil {
        return nil, fmt.Errorf("DialOptions cannot be nil")
    }

    dialOpts := []grpc.DialOption{
        grpc.WithBlock(),
    }

    // TLS / Insecure
    if opts.TLSConfig != nil {
        creds := credentials.NewTLS(opts.TLSConfig)
        dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
    } else {
        dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
    }

    // Build interceptors
    var interceptors []grpc.UnaryClientInterceptor

    if opts.RateLimiter != nil {
        interceptors = append(interceptors, RateLimitInterceptor(opts.RateLimiter))
    }

    if opts.RetryPolicy != nil {
        interceptors = append(interceptors, RetryInterceptor(opts.RetryPolicy))
    }

    interceptors = append(interceptors, LoggingInterceptor())

    dialOpts = append(dialOpts, grpc.WithChainUnaryInterceptor(interceptors...))

    // Handle context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), opts.DialTimeout)
    defer cancel()

    log.Printf("Dialing gRPC target: %s", opts.Target)
    conn, err := grpc.DialContext(ctx, opts.Target, dialOpts...)
    if err != nil {
        return nil, fmt.Errorf("failed to dial target %s: %w", opts.Target, err)
    }

    return conn, nil
}
