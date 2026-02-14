package testutils

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	// Replace with your actual generated protobuf package
	pb "path/to/your/protobuf/package"
)

// server implements the Greeter service.
type server struct {
	pb.UnimplementedGreeterServer
}

// SayHello implements helloworld.GreeterServer
func (s *server) SayHello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
	log.Printf("Received: %v", req.GetName())
	return &pb.HelloReply{Message: "Hello " + req.GetName()}, nil
}

func main() {
	// Configure gRPC port from environment variable, default to 50051
	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = "50051"
	}

	// Create TCP listener
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Create gRPC server with interceptors
	s := grpc.NewServer(
		grpc.UnaryInterceptor(loggingInterceptor),
		grpc.ChainUnaryInterceptor(recoveryInterceptor),
	)

	// Register your service
	pb.RegisterGreeterServer(s, &server{})

	// Register health service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// Register reflection service on gRPC server (useful for grpcurl etc.)
	reflection.Register(s)

	// Start serving in a goroutine
	go func() {
		log.Printf("gRPC server listening on port %s", port)
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down gRPC server...")

	// We don't have Shutdown method in grpc.Server, we use GracefulStop
	s.GracefulStop()
	log.Println("gRPC server stopped gracefully")
}

// loggingInterceptor logs each incoming gRPC call
func loggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	log.Printf("method=%s duration=%s error=%v", info.FullMethod, time.Since(start), err)
	return resp, err
}

// recoveryInterceptor recovers from panics in handlers
func recoveryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered from panic: %v", r)
			err = grpc.Errorf(codes.Internal, "internal error")
		}
	}()
	return handler(ctx, req)
}