package testutils

import (
    "context"
    "fmt"
    "log"
    "net"
    "sync"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/health/grpc_health_v1"
    "google.golang.org/grpc/status"

)

// TestServer wraps the gRPC server to allow graceful shutdown and state manipulation during tests.
type TestServer struct {
    server   *grpc.Server
    impl     *paymentServer // Pointer to the concrete implementation to manipulate state
    listener net.Listener
    port     int
}

// Start initializes the gRPC server with health checks and starts listening.
func (s *TestServer) Start() error {
    var err error
    // Listen on a random available port (":0" lets OS choose)
    s.listener, err = net.Listen("tcp", "127.0.0.1:0")
    if err != nil {
        return fmt.Errorf("failed to listen: %w", err)
    }

    s.port = s.listener.Addr().(*net.TCPAddr).Port

    // Create server with interceptors
    s.server = grpc.NewServer(
        grpc.ChainUnaryInterceptor(loggingServerInterceptor, recoveryServerInterceptor),
    )

    // Initialize the actual service implementation
    s.impl = NewPaymentServer()

    // Register services
    pb.RegisterPaymentServiceServer(s.server, s.impl)

    // Register Health Service
    healthServer := health.NewServer()
    healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
    grpc_health_v1.RegisterHealthServer(s.server, healthServer)

    // Start serving in a goroutine so it doesn't block
    go func() {
        log.Printf("Test Server listening on 127.0.0.1:%d", s.port)
        if err := s.server.Serve(s.listener); err != nil {
            log.Printf("Test server stopped: %v", err)
        }
    }()

    return nil
}

// Stop gracefully shuts down the server.
func (s *TestServer) Stop() {
    if s.server != nil {
        s.server.GracefulStop()
    }
    if s.listener != nil {
        s.listener.Close()
    }
}

// Addr returns the address the server is listening on.
func (s *TestServer) Addr() string {
    return fmt.Sprintf("127.0.0.1:%d", s.port)
}

// ForceSetPaymentStatus allows tests to bypass business logic and set a payment status.
// This is useful for testing workflows like Refund that require a "COMPLETED" status.
func (s *TestServer) ForceSetPaymentStatus(paymentID, newStatus string) {
    s.impl.mu.Lock()
    defer s.impl.mu.Unlock()
    if p, ok := s.impl.payments[paymentID]; ok {
        p.Status = newStatus
    }
}

// PaymentTestPathway manages the lifecycle of a client-server connection for testing.
type PaymentTestPathway struct {
    Server *TestServer
    Client PaymentClient
}

// NewPaymentTestPathway sets up the server and client, ensuring the server is ready.
func NewPaymentTestPathway(ctx context.Context) (*PaymentTestPathway, error) {
    // 1. Start Server
    testServer := &TestServer{}
    if err := testServer.Start(); err != nil {
        return nil, fmt.Errorf("failed to start test server: %w", err)
    }

    // 2. Wait for server to be ready (Health Check)
    if err := waitForServerReady(ctx, testServer.Addr()); err != nil {
        testServer.Stop()
        return nil, fmt.Errorf("server health check failed: %w", err)
    }

    // 3. Create Client
    opts := DefaultPaymentClientOptions()
    opts.DialTimeout = 2 * time.Second
    // Ensure we use insecure credentials for local loopback tests if TLS isn't set up
    if opts.TLSConfig == nil {
        // Explicitly setting insecure for the test pathway
    }

    client, err := NewPaymentClient(testServer.Addr(), opts)
    if err != nil {
        testServer.Stop()
        return nil, fmt.Errorf("failed to create client: %w", err)
    }

    return &PaymentTestPathway{
        Server: testServer,
        Client: client,
    }, nil
}

// Close tears down the pathway.
func (p *PaymentTestPathway) Close() {
    if p.Client != nil {
        p.Client.Close()
    }
    if p.Server != nil {
        p.Server.Stop()
    }
}

// RunHappyPath executes a standard lifecycle: Create -> Get -> Force Complete -> Refund.
func (p *PaymentTestPathway) RunHappyPath(ctx context.Context) error {
    req := &pb.CreatePaymentRequest{
        Amount:     10000, // $100.00
        Currency:   "USD",
        CustomerId: "customer_happy_path",
    }

    log.Println("--- Step 1: Create Payment ---")
    createResp, err := p.Client.CreatePayment(ctx, req)
    if err != nil {
        return fmt.Errorf("CreatePayment failed: %w", err)
    }
    log.Printf("Created Payment ID: %s, Status: %s", createResp.PaymentId, createResp.Status)

    log.Println("--- Step 2: Get Payment ---")
    getResp, err := p.Client.GetPayment(ctx, &pb.GetPaymentRequest{PaymentId: createResp.PaymentId})
    if err != nil {
        return fmt.Errorf("GetPayment failed: %w", err)
    }
    if getResp.Payment.Id != createResp.PaymentId {
        return fmt.Errorf("ID mismatch: expected %s, got %s", createResp.PaymentId, getResp.Payment.Id)
    }
    log.Printf("Retrieved Payment: %+v", getResp.Payment)

    log.Println("--- Step 3: Simulate External Completion (Bank callback) ---")
    // In a real scenario, a webhook would call the server to update status.
    // Here we use the TestServer backdoor to simulate that event.
    p.Server.ForceSetPaymentStatus(createResp.PaymentId, "COMPLETED")
    log.Printf("Payment %s status forced to COMPLETED", createResp.PaymentId)

    log.Println("--- Step 4: Refund Payment ---")
    refundResp, err := p.Client.RefundPayment(ctx, &pb.RefundPaymentRequest{PaymentId: createResp.PaymentId})
    if err != nil {
        return fmt.Errorf("RefundPayment failed: %w", err)
    }
    if refundResp.Status != "REFUNDED" {
        return fmt.Errorf("Expected REFUNDED status, got %s", refundResp.Status)
    }
    log.Printf("Payment Refunded. Status: %s", refundResp.Status)

    return nil
}

// RunStressTest executes the replay logic against the live test server.
func (p *PaymentTestPathway) RunStressTest(ctx context.Context, concurrency, totalRequests int) (*ReplayStats, error) {
    req := &pb.CreatePaymentRequest{
        Amount:     5000,
        Currency:   "USD",
        CustomerId: "customer_load_test",
    }

    replayer := NewReplayer(p.Client)
    opts := &ReplayOptions{
        Concurrency:   concurrency,
        TotalRequests: totalRequests,
        RateLimit:     0, // Full speed
    }

    log.Printf("--- Starting Stress Test: Concurrency=%d, Total=%d ---", concurrency, totalRequests)
    return replayer.RunCreatePaymentReplay(ctx, req, opts)
}

// waitForServerReady polls the health check endpoint until the server responds.
func waitForServerReady(ctx context.Context, addr string) error {
    conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        return err
    }
    defer conn.Close()

    client := grpc_health_v1.NewHealthClient(conn)
    
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            resp, err := client.Check(ctx, &grpc_health_v1.HealthCheckRequest{Service: ""})
            if err == nil && resp.Status == grpc_health_v1.HealthCheckResponse_SERVING {
                return nil
            }
        }
    }
}

// Example usage of the Pathway
func pathwayExampleMain() {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    pathway, err := NewPaymentTestPathway(ctx)
    if err != nil {
        log.Fatalf("Failed to initialize pathway: %v", err)
    }
    defer pathway.Close()

    // 1. Run Happy Path
    if err := pathway.RunHappyPath(ctx); err != nil {
        log.Fatalf("Happy Path failed: %v", err)
    }
    log.Println("Happy Path Passed!")

    // 2. Run Stress Test (small scale)
    stats, err := pathway.RunStressTest(ctx, 5, 20)
    if err != nil {
        log.Printf("Stress test finished with error: %v", err)
    } else {
        log.Printf("Stress test stats: %+v", stats)
    }
}