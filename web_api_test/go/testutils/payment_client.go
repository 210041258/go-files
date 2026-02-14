package testutils

import (
    "context"
    "crypto/tls"
    "crypto/x509"
    "fmt"
    "log"
    "os" // Replaced ioutil
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/credentials"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/resolver"
    "google.golang.org/grpc/status"

    // Import the generated protobuf package for the payment service.
    // Replace with your actual module path.
    pb "path/to/your/protobuf/package/paymentpb"

    // Import the service discovery package (e.g., etcd resolver).
    "github.com/yourproject/discovery" // adjust import path
)

// PaymentClient defines the interface for interacting with the payment service.
type PaymentClient interface {
    // CreatePayment initiates a new payment.
    CreatePayment(ctx context.Context, req *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error)
    // GetPayment retrieves payment details by ID.
    GetPayment(ctx context.Context, req *pb.GetPaymentRequest) (*pb.GetPaymentResponse, error)
    // RefundPayment processes a refund for a payment.
    RefundPayment(ctx context.Context, req *pb.RefundPaymentRequest) (*pb.RefundPaymentResponse, error)
    // Close releases the underlying connection.
    Close() error
}

// paymentClient is a concrete implementation of PaymentClient using gRPC.
type paymentClient struct {
    conn   *grpc.ClientConn
    client pb.PaymentServiceClient
}

// PaymentClientOptions holds configuration for the payment client.
type PaymentClientOptions struct {
    // ServiceName is the name used for service discovery (default: "payment-service").
    ServiceName string
    // ResolverBuilder is the resolver builder to use (e.g., etcd, dns).
    // If nil, the default resolver is used.
    ResolverBuilder resolver.Builder
    // TLS configuration. If nil, insecure credentials are used.
    TLSConfig *tls.Config
    // DialTimeout sets the timeout for establishing the connection.
    DialTimeout time.Duration
    // RPCTimeout is the default timeout for individual RPC calls.
    RPCTimeout time.Duration
    // RetryPolicy defines the retry behavior for RPCs.
    RetryPolicy *RetryPolicy
}

// RetryPolicy configures retry behavior.
type RetryPolicy struct {
    MaxAttempts       int
    InitialBackoff    time.Duration
    MaxBackoff        time.Duration
    RetryableStatuses []codes.Code // Changed type to codes.Code for type safety
}

// DefaultPaymentClientOptions returns a sensible default configuration.
func DefaultPaymentClientOptions() *PaymentClientOptions {
    return &PaymentClientOptions{
        ServiceName: "payment-service",
        DialTimeout: 5 * time.Second,
        RPCTimeout:  10 * time.Second,
        // ResolverBuilder: nil, // will use default resolver (e.g., dns)
        // TLSConfig:     nil,
        RetryPolicy: &RetryPolicy{
            MaxAttempts:       3,
            InitialBackoff:    100 * time.Millisecond,
            MaxBackoff:        2 * time.Second,
            RetryableStatuses: []codes.Code{codes.Unavailable, codes.DeadlineExceeded},
        },
    }
}

// NewPaymentClient creates a new payment client using the given target and options.
// The target can be a direct address (e.g., "localhost:50051") or a discovery URI
// (e.g., "etcd:///payment-service").
func NewPaymentClient(target string, opts *PaymentClientOptions) (PaymentClient, error) {
    if opts == nil {
        opts = DefaultPaymentClientOptions()
    }

    // Set up dial options
    dialOpts := []grpc.DialOption{
        grpc.WithBlock(), // Block until the connection is up (or timeout expires)
    }

    // Configure credentials
    if opts.TLSConfig != nil {
        creds := credentials.NewTLS(opts.TLSConfig)
        dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
    } else {
        dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
    }

    // Register custom resolver if provided
    // Note: Registering a resolver with the same scheme twice will panic.
    // Ensure this is either global (in init()) or checked here.
    if opts.ResolverBuilder != nil {
        resolver.Register(opts.ResolverBuilder)
    }

    // Add interceptors for logging, retries, and tracing
    dialOpts = append(dialOpts,
        grpc.WithChainUnaryInterceptor(
            loggingInterceptor,
            retryInterceptor(opts.RetryPolicy),
        ),
    )

    // Establish connection
    // grpc.WithTimeout is deprecated; we use context.WithTimeout instead.
    ctx, cancel := context.WithTimeout(context.Background(), opts.DialTimeout)
    defer cancel()

    conn, err := grpc.DialContext(ctx, target, dialOpts...)
    if err != nil {
        return nil, fmt.Errorf("failed to dial payment service: %w", err)
    }

    client := pb.NewPaymentServiceClient(conn)
    return &paymentClient{
        conn:   conn,
        client: client,
    }, nil
}

// CreatePayment implements PaymentClient.
func (c *paymentClient) CreatePayment(ctx context.Context, req *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error) {
    return c.client.CreatePayment(ctx, req)
}

// GetPayment implements PaymentClient.
func (c *paymentClient) GetPayment(ctx context.Context, req *pb.GetPaymentRequest) (*pb.GetPaymentResponse, error) {
    return c.client.GetPayment(ctx, req)
}

// RefundPayment implements PaymentClient.
func (c *paymentClient) RefundPayment(ctx context.Context, req *pb.RefundPaymentRequest) (*pb.RefundPaymentResponse, error) {
    return c.client.RefundPayment(ctx, req)
}

// Close implements PaymentClient.
func (c *paymentClient) Close() error {
    return c.conn.Close()
}

// loggingInterceptor logs gRPC calls.
func loggingInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
    start := time.Now()
    err := invoker(ctx, method, req, reply, cc, opts...)
    log.Printf("gRPC call: method=%s duration=%s error=%v", method, time.Since(start), err)
    return err
}

// retryInterceptor adds retry logic for transient failures.
func retryInterceptor(policy *RetryPolicy) grpc.UnaryClientInterceptor {
    return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
        if policy == nil {
            return invoker(ctx, method, req, reply, cc, opts...)
        }

        var err error
        backoff := policy.InitialBackoff

        for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
            err = invoker(ctx, method, req, reply, cc, opts...)
            if err == nil {
                return nil
            }

            // Check if the error is retryable
            if !isRetryable(err, policy.RetryableStatuses) {
                return err
            }

            log.Printf("Attempt %d failed: %v. Retrying...", attempt, err)

            if attempt == policy.MaxAttempts {
                break
            }

            // Wait for backoff or check context cancellation
            select {
            case <-ctx.Done():
                // Context cancelled or deadline exceeded during backoff
                return fmt.Errorf("context cancelled during backoff: %w", ctx.Err())
            case <-time.After(backoff):
                // Proceed to next attempt
            }

            // Exponential backoff
            backoff *= 2
            if backoff > policy.MaxBackoff {
                backoff = policy.MaxBackoff
            }
        }
        return fmt.Errorf("max retry attempts (%d) reached. Last error: %w", policy.MaxAttempts, err)
    }
}

// isRetryable determines if the gRPC error status code is retryable.
func isRetryable(err error, retryableCodes []codes.Code) bool {
    // Convert error to gRPC status
    st, ok := status.FromError(err)
    if !ok {
        // Not a gRPC status error (e.g., network error during dial)
        // Usually transport errors are considered retryable, but strictly speaking
        // if we can't parse the code, we might want to be safe.
        // However, for gRPC clients, most errors return a status.
        return false 
    }

    for _, code := range retryableCodes {
        if st.Code() == code {
            return true
        }
    }
    return false
}

// Example usage with etcd discovery.
func main() {
    // 1. Load CA Cert (if using TLS)
    // caCert, err := os.ReadFile("ca.pem")
    // if err != nil { log.Fatal(err) }
    // caCertPool := x509.NewCertPool()
    // caCertPool.AppendCertsFromPEM(caCert)
    // tlsConfig := &tls.Config{RootCAs: caCertPool}

    // 2. Initialize etcd registry (from discovery.go)
    endpoints := []string{"localhost:2379"}
    registry, err := discovery.NewEtcdRegistry(endpoints, 5*time.Second)
    if err != nil {
        log.Fatalf("Failed to create etcd registry: %v", err)
    }
    defer registry.Close()

    // 3. Register the etcd resolver builder.
    // Note: This should typically happen in an init() function or once per application lifecycle
    // to avoid panics if you create multiple clients.
    resolver.Register(registry)

    // 4. Create payment client using etcd discovery.
    // Target format: scheme:///service-name
    target := "etcd:///payment-service"
    
    opts := DefaultPaymentClientOptions()
    // opts.TLSConfig = tlsConfig // Uncomment if using TLS
    // Since resolver is registered globally, we can omit ResolverBuilder in opts 
    // unless we need to pass specific config to the builder itself.

    client, err := NewPaymentClient(target, opts)
    if err != nil {
        log.Fatalf("Failed to create payment client: %v", err)
    }
    defer client.Close()

    // Example: create a payment
    createReq := &pb.CreatePaymentRequest{
        Amount:     1000,
        Currency:   "USD",
        CustomerId: "cust_123",
    }
    
    ctx, cancel := context.WithTimeout(context.Background(), opts.RPCTimeout)
    defer cancel()
    
    resp, err := client.CreatePayment(ctx, createReq)
    if err != nil {
        log.Fatalf("CreatePayment failed: %v", err)
    }
    log.Printf("Payment created: %+v", resp)
}