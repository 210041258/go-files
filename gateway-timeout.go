package testutils

import (
    "context"
    "crypto/tls"
    "fmt"
    "log"
    "os"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/resolver"

    // Import the generated protobuf package
    pb "path/to/your/protobuf/package/paymentpb"

    // Import the service discovery package
    "github.com/yourproject/discovery"
)

// PaymentClient defines the interface for interacting with the payment service.
type PaymentClient interface {
    CreatePayment(ctx context.Context, req *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error)
    GetPayment(ctx context.Context, req *pb.GetPaymentRequest) (*pb.GetPaymentResponse, error)
    RefundPayment(ctx context.Context, req *pb.RefundPaymentRequest) (*pb.RefundPaymentResponse, error)
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
    ResolverBuilder resolver.Builder
    // TLS configuration. If nil, insecure credentials are used.
    TLSConfig *tls.Config
    // DialTimeout sets the timeout for establishing the connection.
    DialTimeout time.Duration
    // RetryPolicy defines the retry behavior for RPCs. Uses the RetryPolicy struct.
    RetryPolicy *RetryPolicy
    // AuthToken is the bearer token to be sent via the AuthClientInterceptor.
    AuthToken string
}

// DefaultPaymentClientOptions returns a sensible default configuration.
func DefaultPaymentClientOptions() *PaymentClientOptions {
    return &PaymentClientOptions{
        ServiceName: "payment-service",
        DialTimeout: 5 * time.Second,
        TLSConfig:   nil,
        RetryPolicy: DefaultRetryPolicy(), // Import from retry_policy.go
        AuthToken:   "", // No auth by default
    }
}

// NewPaymentClient creates a new payment client using the given target and options.
func NewPaymentClient(target string, opts *PaymentClientOptions) (PaymentClient, error) {
    if opts == nil {
        opts = DefaultPaymentClientOptions()
    }

    // 1. Set up basic dial options
    dialOpts := []grpc.DialOption{
        grpc.WithBlock(), // Block until connection up or timeout
    }

    // 2. Configure Credentials
    if opts.TLSConfig != nil {
        creds := credentials.NewTLS(opts.TLSConfig)
        dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
    } else {
        dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
    }

    // 3. Register custom resolver if provided
    if opts.ResolverBuilder != nil {
        resolver.Register(opts.ResolverBuilder)
    }

    // 4. Build Interceptor Chain
    // Order matters: 
    // RequestID -> Auth -> Retry -> Logging (Outer)
    
    var chain []grpc.UnaryClientInterceptor

    // A. Request ID (Ensures every call has an ID)
    chain = append(chain, RequestIDClientInterceptor())

    // B. Auth (Attaches token if provided)
    if opts.AuthToken != "" {
        chain = append(chain, AuthClientInterceptor(opts.AuthToken))
    }

    // C. Retry (Handles transient failures with backoff)
    chain = append(chain, UnaryRetryInterceptor(opts.RetryPolicy))

    // D. Logging (Logs the final result of the call)
    chain = append(chain, LoggingClientInterceptor())

    dialOpts = append(dialOpts, grpc.WithChainUnaryInterceptor(chain...))

    // 5. Establish Connection with Timeout
    ctx, cancel := context.WithTimeout(context.Background(), opts.DialTimeout)
    defer cancel()

    conn, err := grpc.DialContext(ctx, target, dialOpts...)
    if err != nil {
        return nil, fmt.Errorf("failed to dial payment service at %s: %w", target, err)
    }

    client := pb.NewPaymentServiceClient(conn)
    return &paymentClient{
        conn:   conn,
        client: client,
    }, nil
}

// --- PaymentClient Interface Implementations ---

func (c *paymentClient) CreatePayment(ctx context.Context, req *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error) {
    return c.client.CreatePayment(ctx, req)
}

func (c *paymentClient) GetPayment(ctx context.Context, req *pb.GetPaymentRequest) (*pb.GetPaymentResponse, error) {
    return c.client.GetPayment(ctx, req)
}

func (c *paymentClient) RefundPayment(ctx context.Context, req *pb.RefundPaymentRequest) (*pb.RefundPaymentResponse, error) {
    return c.client.RefundPayment(ctx, req)
}

func (c *paymentClient) Close() error {
    return c.conn.Close()
}

// --- Example Usage ---

func main() {
    // 1. Setup Discovery (Optional - Direct address usage shown if discovery is unavailable)
    // target := "etcd:///payment-service" 
    target := "localhost:50051" // Fallback for local testing

    // 2. Create Options
    opts := DefaultPaymentClientOptions()
    
    // Enable advanced retrying
    opts.RetryPolicy = &RetryPolicy{
        MaxAttempts:       5,
        InitialBackoff:    100 * time.Millisecond,
        MaxBackoff:        2 * time.Second,
        BackoffMultiplier: 2.0,
    }

    // Add Auth Token
    opts.AuthToken = "Bearer super-secret-jwt-token"

    // 3. Load TLS Certificates (Optional)
    // caCert, _ := os.ReadFile("ca.crt")
    // caCertPool := x509.NewCertPool()
    // caCertPool.AppendCertsFromPEM(caCert)
    // opts.TLSConfig = &tls.Config{RootCAs: caCertPool}

    // 4. Create Client
    client, err := NewPaymentClient(target, opts)
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }
    defer client.Close()

    // 5. Make a Call
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    req := &pb.CreatePaymentRequest{
        Amount:     1000,
        Currency:   "USD",
        CustomerId: "customer_123",
    }

    resp, err := client.CreatePayment(ctx, req)
    if err != nil {
        log.Printf("RPC Error: %v", err)
        return
    }

    log.Printf("Success! Payment ID: %s, Status: %s", resp.PaymentId, resp.Status)
}