package testutils

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/resolver"

	// Import the generated protobuf package for the auth service.
	// Replace with your actual module path.
	pb "path/to/your/protobuf/package/authpb"

	// Import the service discovery package if needed.
	// "github.com/yourproject/discovery"
)

// AuthClient defines the interface for interacting with the authentication service.
type AuthClient interface {
	// Login authenticates a user and returns tokens.
	Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error)
	// RefreshToken obtains new access token using a refresh token.
	RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error)
	// VerifyToken validates an access token and returns its claims.
	VerifyToken(ctx context.Context, req *pb.VerifyTokenRequest) (*pb.VerifyTokenResponse, error)
	// Logout invalidates the provided tokens.
	Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error)
	// ChangePassword updates user's password.
	ChangePassword(ctx context.Context, req *pb.ChangePasswordRequest) (*pb.ChangePasswordResponse, error)
	// Close releases the underlying connection.
	Close() error
}

// authClient is a concrete implementation of AuthClient using gRPC.
type authClient struct {
	conn   *grpc.ClientConn
	client pb.AuthServiceClient
}

// AuthClientOptions holds configuration for the authentication client.
type AuthClientOptions struct {
	// ServiceName is the name used for service discovery (default: "auth-service").
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
	// TokenProvider is a function that returns the access token to inject
	// into gRPC metadata. If nil, no token is added automatically.
	TokenProvider func(ctx context.Context) (string, error)
}

// RetryPolicy configures retry behavior.
type RetryPolicy struct {
	MaxAttempts       int
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
	RetryableStatuses []uint32 // gRPC status codes that should be retried
}

// DefaultAuthClientOptions returns a sensible default configuration.
func DefaultAuthClientOptions() *AuthClientOptions {
	return &AuthClientOptions{
		ServiceName:   "auth-service",
		DialTimeout:   5 * time.Second,
		RPCTimeout:    10 * time.Second,
		ResolverBuilder: nil,
		TLSConfig:     nil,
		RetryPolicy: &RetryPolicy{
			MaxAttempts:       3,
			InitialBackoff:    100 * time.Millisecond,
			MaxBackoff:        2 * time.Second,
			RetryableStatuses: []uint32{14, 4}, // codes.Unavailable, codes.DeadlineExceeded
		},
		TokenProvider: nil, // no automatic token injection by default
	}
}

// NewAuthClient creates a new authentication client using the given target and options.
// The target can be a direct address (e.g., "localhost:50051") or a discovery URI
// (e.g., "etcd:///auth-service").
func NewAuthClient(target string, opts *AuthClientOptions) (AuthClient, error) {
	if opts == nil {
		opts = DefaultAuthClientOptions()
	}

	// Set up dial options
	dialOpts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithTimeout(opts.DialTimeout),
	}

	// Configure credentials
	if opts.TLSConfig != nil {
		creds := credentials.NewTLS(opts.TLSConfig)
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Register custom resolver if provided
	if opts.ResolverBuilder != nil {
		resolver.Register(opts.ResolverBuilder)
	}

	// Build interceptor chain
	interceptors := []grpc.UnaryClientInterceptor{
		loggingInterceptor,
		retryInterceptor(opts.RetryPolicy),
	}
	if opts.TokenProvider != nil {
		interceptors = append(interceptors, authTokenInterceptor(opts.TokenProvider))
	}

	dialOpts = append(dialOpts, grpc.WithChainUnaryInterceptor(interceptors...))

	// Establish connection
	ctx, cancel := context.WithTimeout(context.Background(), opts.DialTimeout)
	defer cancel()
	conn, err := grpc.DialContext(ctx, target, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial auth service: %w", err)
	}

	client := pb.NewAuthServiceClient(conn)
	return &authClient{
		conn:   conn,
		client: client,
	}, nil
}

// Login implements AuthClient.
func (c *authClient) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	return c.client.Login(ctx, req)
}

// RefreshToken implements AuthClient.
func (c *authClient) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	return c.client.RefreshToken(ctx, req)
}

// VerifyToken implements AuthClient.
func (c *authClient) VerifyToken(ctx context.Context, req *pb.VerifyTokenRequest) (*pb.VerifyTokenResponse, error) {
	return c.client.VerifyToken(ctx, req)
}

// Logout implements AuthClient.
func (c *authClient) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	return c.client.Logout(ctx, req)
}

// ChangePassword implements AuthClient.
func (c *authClient) ChangePassword(ctx context.Context, req *pb.ChangePasswordRequest) (*pb.ChangePasswordResponse, error) {
	return c.client.ChangePassword(ctx, req)
}

// Close implements AuthClient.
func (c *authClient) Close() error {
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
			if !isRetryable(err, policy.RetryableStatuses) {
				return err
			}
			log.Printf("retryable error on attempt %d: %v", attempt, err)
			if attempt == policy.MaxAttempts {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > policy.MaxBackoff {
				backoff = policy.MaxBackoff
			}
		}
		return err
	}
}

// authTokenInterceptor injects an access token from the provided TokenProvider
// into the outgoing gRPC metadata.
func authTokenInterceptor(provider func(ctx context.Context) (string, error)) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		token, err := provider(ctx)
		if err != nil {
			return fmt.Errorf("failed to get auth token: %w", err)
		}
		if token != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// isRetryable determines if the gRPC error status code is retryable.
// This is a placeholder; implement proper status code checking using google.golang.org/grpc/status.
func isRetryable(err error, retryableCodes []uint32) bool {
	// TODO: Extract status code and compare with retryableCodes.
	// For now, just return true for any error.
	return true
}

// Example usage with etcd discovery and token injection from context.
func main() {
	// Example token provider: reads from context.
	tokenProvider := func(ctx context.Context) (string, error) {
		// In a real application, you might retrieve the token from the context
		// (e.g., using a custom context key) or from a token manager.
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if tokens := md.Get("token"); len(tokens) > 0 {
				return tokens[0], nil
			}
		}
		// For outgoing calls, you might want to get the token from a manager.
		return "", nil // no token
	}

	// Create options
	opts := DefaultAuthClientOptions()
	opts.TokenProvider = tokenProvider
	// If using service discovery:
	// resolver.Register(etcdRegistry)
	// target := "etcd:///auth-service"

	// Direct connection for development
	target := "localhost:50051"
	client, err := NewAuthClient(target, opts)
	if err != nil {
		log.Fatalf("Failed to create auth client: %v", err)
	}
	defer client.Close()

	// Example: Login
	loginReq := &pb.LoginRequest{
		Username: "alice",
		Password: "secret",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	loginResp, err := client.Login(ctx, loginReq)
	if err != nil {
		log.Fatalf("Login failed: %v", err)
	}
	log.Printf("Login successful, access token: %s", loginResp.AccessToken)

	// Example: Verify token (with token automatically injected via interceptor)
	// For this call, we need to provide the token in context.
	// In a real scenario, you'd store the token and attach it to the context.
	verifyCtx := metadata.AppendToOutgoingContext(ctx, "token", loginResp.AccessToken)
	verifyReq := &pb.VerifyTokenRequest{
		Token: loginResp.AccessToken, // may be optional if token is in header
	}
	verifyResp, err := client.VerifyToken(verifyCtx, verifyReq)
	if err != nil {
		log.Fatalf("VerifyToken failed: %v", err)
	}
	log.Printf("Token valid, user: %s", verifyResp.UserId)
}