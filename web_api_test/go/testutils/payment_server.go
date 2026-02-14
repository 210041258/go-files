package testutils

import (
    "context"
    "fmt"
    "log"
    "net"
    "os"
    "sync"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/credentials"
    "google.golang.org/grpc/health"
    "google.golang.org/grpc/health/grpc_health_v1"
    "google.golang.org/grpc/peer"
    "google.golang.org/grpc/reflection"
    "google.golang.org/grpc/status"

)

// ServerConfig holds the configuration for the payment server.
type ServerConfig struct {
    // Port is the TCP port to listen on (e.g., ":50051").
    Port string
    // TLSCertFile is the path to the TLS certificate file.
    TLSCertFile string
    // TLSKeyFile is the path to the TLS key file.
    TLSKeyFile string
    // EtcdEndpoints are the endpoints for the service discovery registry.
    EtcdEndpoints []string
    // ServiceName is the name registered in service discovery.
    ServiceName string
    // ServiceAddr is the public address/IP advertised to discovery (e.g., "192.168.1.5:50051").
    // If empty, it defaults to the local listening address.
    ServiceAddr string
}

// paymentServer implements the pb.PaymentServiceServer interface.
type paymentServer struct {
    pb.UnimplementedPaymentServiceServer

    // In-memory storage for simplicity. In a real app, use a database.
    payments map[string]*pb.Payment
    mu       sync.RWMutex
}

// NewPaymentServer creates a new instance of the payment server logic.
func NewPaymentServer() *paymentServer {
    return &paymentServer{
        payments: make(map[string]*pb.Payment),
    }
}

// CreatePayment handles the creation of a new payment.
func (s *paymentServer) CreatePayment(ctx context.Context, req *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error) {
    // Basic validation
    if req.Amount <= 0 {
        return nil, status.Error(codes.InvalidArgument, "amount must be positive")
    }

    // Generate a unique ID (In reality, use UUID)
    paymentID := fmt.Sprintf("pay_%d", time.Now().UnixNano())

    payment := &pb.Payment{
        Id:         paymentID,
        Amount:     req.Amount,
        Currency:   req.Currency,
        CustomerId: req.CustomerId,
        Status:     "PENDING", // Initial status
        CreatedAt:  time.Now().Format(time.RFC3339),
    }

    // Save to store
    s.mu.Lock()
    s.payments[paymentID] = payment
    s.mu.Unlock()

    log.Printf("Created payment: ID=%s, Amount=%d %s", paymentID, req.Amount, req.Currency)

    return &pb.CreatePaymentResponse{
        PaymentId: paymentID,
        Status:    payment.Status,
    }, nil
}

// GetPayment retrieves a payment by ID.
func (s *paymentServer) GetPayment(ctx context.Context, req *pb.GetPaymentRequest) (*pb.GetPaymentResponse, error) {
    s.mu.RLock()
    payment, exists := s.payments[req.PaymentId]
    s.mu.RUnlock()

    if !exists {
        return nil, status.Error(codes.NotFound, "payment not found")
    }

    // Log client peer info for debugging
    if p, ok := peer.FromContext(ctx); ok {
        log.Printf("GetPayment requested by %s for ID %s", p.Addr, req.PaymentId)
    }

    return &pb.GetPaymentResponse{
        Payment: payment,
    }, nil
}

// RefundPayment processes a refund.
func (s *paymentServer) RefundPayment(ctx context.Context, req *pb.RefundPaymentRequest) (*pb.RefundPaymentResponse, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    payment, exists := s.payments[req.PaymentId]
    if !exists {
        return nil, status.Error(codes.NotFound, "payment not found")
    }

    if payment.Status == "REFUNDED" {
        return nil, status.Error(codes.FailedPrecondition, "payment already refunded")
    }

    if payment.Status != "COMPLETED" {
        return nil, status.Error(codes.FailedPrecondition, fmt.Sprintf("cannot refund payment with status %s", payment.Status))
    }

    // Update status
    payment.Status = "REFUNDED"

    log.Printf("Refunded payment: ID=%s", req.PaymentId)

    return &pb.RefundPaymentResponse{
        PaymentId: req.PaymentId,
        Status:    "REFUNDED",
    }, nil
}

// RunPaymentServer starts the gRPC server.
func RunPaymentServer(cfg *ServerConfig) error {
    // 1. Configure Listener
    lis, err := net.Listen("tcp", cfg.Port)
    if err != nil {
        return fmt.Errorf("failed to listen on %s: %w", cfg.Port, err)
    }

    // 2. Configure TLS
    var opts []grpc.ServerOption
    if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
        creds, err := credentials.NewServerTLSFromFile(cfg.TLSCertFile, cfg.TLSKeyFile)
        if err != nil {
            return fmt.Errorf("failed to load TLS credentials: %w", err)
        }
        opts = append(opts, grpc.Creds(creds))
        log.Println("TLS enabled")
    } else {
        log.Println("Warning: Running in insecure mode (no TLS)")
    }

    // 3. Add Interceptors (Logging, Recovery)
    opts = append(opts,
        grpc.ChainUnaryInterceptor(
            loggingServerInterceptor,
            recoveryServerInterceptor,
        ),
    )

    // 4. Create gRPC Server
    s := grpc.NewServer(opts...)

    // 5. Register Services
    paymentSvc := NewPaymentServer()
    pb.RegisterPaymentServiceServer(s, paymentSvc)

    // Register Health Service
    healthServer := health.NewServer()
    healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
    grpc_health_v1.RegisterHealthServer(s, healthServer)

    // Register Reflection (for grpcurl/cli debugging)
    reflection.Register(s)

    // 6. Register with Service Discovery (e.g., etcd)
    // Determine advertised address
    addr := cfg.ServiceAddr
    if addr == "" {
        // Fallback to the listener address if not explicitly set
        addr = lis.Addr().String()
    }

    var registry *discovery.EtcdRegistry
    if len(cfg.EtcdEndpoints) > 0 {
        registry, err = discovery.NewEtcdRegistry(cfg.EtcdEndpoints, 5*time.Second)
        if err != nil {
            return fmt.Errorf("failed to connect to discovery registry: %w", err)
        }
        
        // Register service instance
        // Assuming Register method exists in discovery package
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        
        // TTL usually handled by lease keep-alive in real implementations
        err = registry.Register(ctx, cfg.ServiceName, addr) 
        if err != nil {
            return fmt.Errorf("failed to register service: %w", err)
        }
        log.Printf("Registered service '%s' at '%s' with discovery", cfg.ServiceName, addr)
        
        // Ensure we deregister on shutdown
        defer registry.Deregister(context.Background(), cfg.ServiceName, addr)
        defer registry.Close()
    }

    // 7. Start Serving
    log.Printf("Payment server listening on %s", cfg.Port)
    return s.Serve(lis)
}

// loggingServerInterceptor logs incoming gRPC requests.
func loggingServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
    start := time.Now()
    
    // Call the handler
    resp, err := handler(ctx, req)
    
    // Log after handling
    duration := time.Since(start)
    code := codes.OK
    if err != nil {
        st, ok := status.FromError(err)
        if ok {
            code = st.Code()
        }
    }
    
    log.Printf("gRPC Request: method=%s duration=%s status=%s error=%v", info.FullMethod, duration, code, err)
    
    return resp, err
}

// recoveryServerInterceptor prevents panics from crashing the server.
func recoveryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("Panic recovered in %s: %v", info.FullMethod, r)
            err = status.Error(codes.Internal, "internal server error")
        }
    }()
    return handler(ctx, req)
}

// Example main function to run the server.
func serverMain() {
    cfg := &ServerConfig{
        Port:          ":50051",
        // Uncomment below to enable TLS
        // TLSCertFile:   "server.crt",
        // TLSKeyFile:    "server.key",
        EtcdEndpoints: []string{"localhost:2379"},
        ServiceName:   "payment-service",
        ServiceAddr:   "localhost:50051", // Use actual IP if running in Docker/K8s
    }

    if err := RunPaymentServer(cfg); err != nil {
        log.Fatalf("Failed to run server: %v", err)
    }
}