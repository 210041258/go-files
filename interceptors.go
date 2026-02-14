package testutils

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "runtime"
    "strings"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/metadata"
    "google.golang.org/grpc/status"
)

// --- Context Keys ---

// contextKey is a custom type to avoid collisions in context.Context
type contextKey string

const (
    // RequestIDKey is the key used to store the Request ID in the context.
    RequestIDKey contextKey = "requestID"
)

// --- Interfaces ---

// Validator allows messages to define their own validation logic.
// Protobuf generated structs can implement this if needed.
type Validator interface {
    Validate() error
}

// --- Server Interceptors ---

// RequestIDServerInterceptor checks for an existing Request ID in metadata
// or generates a new one, then adds it to the context.
func RequestIDServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
    var requestID string

    // Check incoming metadata
    md, ok := metadata.FromIncomingContext(ctx)
    if ok {
        // x-request-id is a standard header
        if ids := md.Get("x-request-id"); len(ids) > 0 {
            requestID = ids[0]
        }
    }

    // If not found, generate one
    if requestID == "" {
        requestID = generateShortID()
    }

    // Add to context
    ctx = context.WithValue(ctx, RequestIDKey, requestID)

    // Add to outgoing metadata (so if this server calls another service, it forwards the ID)
    // Note: In a real scenario, you might use a dedicated forwarder interceptor.
    if !ok {
        md = metadata.Pairs("x-request-id", requestID)
        ctx = metadata.NewOutgoingContext(ctx, md)
    } else {
        // Ensure it exists in outgoing context
        ctx = metadata.NewOutgoingContext(ctx, md)
    }

    return handler(ctx, req)
}

// ValidationServerInterceptor checks if the request implements the Validator interface.
// If it does, it calls Validate() before proceeding.
func ValidationServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
    if v, ok := req.(Validator); ok {
        if err := v.Validate(); err != nil {
            // Return InvalidArgument if validation fails
            return nil, status.Error(codes.InvalidArgument, err.Error())
        }
    }
    return handler(ctx, req)
}

// LoggingServerInterceptor logs request details including the Request ID.
func LoggingServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
    start := time.Now()
    
    // Extract Request ID if available
    requestID := "unknown"
    if id, ok := ctx.Value(RequestIDKey).(string); ok {
        requestID = id
    }

    // Call handler
    resp, err := handler(ctx, req)

    // Determine status code
    code := codes.OK
    if err != nil {
        if s, ok := status.FromError(err); ok {
            code = s.Code()
        }
    }

    // Log
    log.Printf("[SERVER] ReqID=%s Method=%s Status=%s Duration=%s Error=%v", 
        requestID, info.FullMethod, code, time.Since(start), err)

    return resp, err
}

// RecoveryServerInterceptor catches panics and converts them to gRPC Internal errors.
func RecoveryServerInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
    defer func() {
        if r := recover(); r != nil {
            // Log stack trace
            buf := make([]byte, 4096)
            n := runtime.Stack(buf, false)
            log.Printf("[PANIC] Method=%s Stack=%s", info.FullMethod, string(buf[:n]))

            // Return error to client
            err = status.Error(codes.Internal, "internal server error")
        }
    }()
    return handler(ctx, req)
}

// --- Client Interceptors ---

// AuthClientInterceptor automatically attaches an authorization token to every request.
func AuthClientInterceptor(token string) grpc.UnaryClientInterceptor {
    return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
        // Append token to metadata
        md := metadata.Pairs("authorization", token)
        
        // Create new context with metadata
        ctx = metadata.NewOutgoingContext(ctx, md)
        
        return invoker(ctx, method, req, reply, cc, opts...)
    }
}

// RequestIDClientInterceptor ensures the client sends a Request ID if one isn't already in the context.
func RequestIDClientInterceptor() grpc.UnaryClientInterceptor {
    return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
        // Check if Request ID already exists in context (passed down from a server context perhaps)
        if _, ok := ctx.Value(RequestIDKey).(string); !ok {
            requestID := generateShortID()
            ctx = context.WithValue(ctx, RequestIDKey, requestID)
            
            // Also add to metadata for the server to see
            md, _ := metadata.FromOutgoingContext(ctx)
            if md == nil {
                md = metadata.Pairs("x-request-id", requestID)
            } else {
                md = md.Copy()
                md.Set("x-request-id", requestID)
            }
            ctx = metadata.NewOutgoingContext(ctx, md)
        }
        
        return invoker(ctx, method, req, reply, cc, opts...)
    }
}

// LoggingClientInterceptor logs the client-side latency and status.
func LoggingClientInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
    start := time.Now()
    
    // Extract ID for logging
    requestID := "unknown"
    if id, ok := ctx.Value(RequestIDKey).(string); ok {
        requestID = id
    }

    err := invoker(ctx, method, req, reply, cc, opts...)

    code := codes.OK
    if err != nil {
        if s, ok := status.FromError(err); ok {
            code = s.Code()
        }
    }

    log.Printf("[CLIENT] ReqID=%s Method=%s Status=%s Duration=%s", 
        requestID, method, code, time.Since(start))

    return err
}

// --- Helpers ---

// generateShortID creates a simple unique string for logging/tracing.
// In production, use UUIDs (google.golang.org/protobuf/types/known/wrapperspb or github.com/google/uuid).
func generateShortID() string {
    return fmt.Sprintf("%d", time.Now().UnixNano())
}

// --- Example Usage Integration ---

/*
// HOW TO USE IN SERVER (payment_server.go):

serverOpts := []grpc.ServerOption{
    grpc.ChainUnaryInterceptor(
        RequestIDServerInterceptor,
        RecoveryServerInterceptor,
        ValidationServerInterceptor, // Optional: if you implement Validate() on requests
        LoggingServerInterceptor,
    ),
}
s := grpc.NewServer(serverOpts...)

// HOW TO USE IN CLIENT (gateway-timeout.go):

clientOpts := []grpc.DialOption{
    grpc.WithChainUnaryInterceptor(
        RequestIDClientInterceptor(),
        AuthClientInterceptor("Bearer my-secret-token"), // Adds Auth header
        LoggingClientInterceptor,
    ),
}
*/