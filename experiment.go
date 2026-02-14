package testutils

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	// Replace these import paths with your actual module path.
	// Example: "github.com/yourname/yourproject/discovery"
	"github.com/yourproject/discovery"
	"github.com/yourproject/hub"
	"github.com/yourproject/lifecycle"
	"github.com/yourproject/roundrobin"

	// Generated protobuf package – adjust to your own.
	pb "github.com/yourproject/proto/experimentpb"
)

// ----------------------------------------------------------------------
// 1. Define a simple gRPC service for the experiment.
// ----------------------------------------------------------------------

type experimentServer struct {
	pb.UnimplementedExperimentServiceServer
	hub *hub.Hub
}

func (s *experimentServer) Echo(ctx context.Context, req *pb.EchoRequest) (*pb.EchoResponse, error) {
	log.Printf("gRPC Echo called: %s", req.Message)
	return &pb.EchoResponse{Message: req.Message}, nil
}

func (s *experimentServer) Broadcast(ctx context.Context, req *pb.BroadcastRequest) (*pb.BroadcastResponse, error) {
	// Forward the broadcast message to all WebSocket clients.
	msg := hub.Message{
		Type:    "broadcast",
		Payload: json.RawMessage(fmt.Sprintf(`{"text":"%s"}`, req.Text)),
		Time:    time.Now(),
	}
	s.hub.Broadcast <- msg
	return &pb.BroadcastResponse{Success: true}, nil
}

// ----------------------------------------------------------------------
// 2. Main experiment – orchestrates all components.
// ----------------------------------------------------------------------

func main() {
	// Parse command line flags for easy configuration.
	var (
		httpPort  = flag.String("http", "8080", "HTTP/WebSocket port")
		grpcPort  = flag.String("grpc", "50051", "gRPC port")
		etcdEnd   = flag.String("etcd", "localhost:2379", "etcd endpoints (comma separated)")
		svcName   = flag.String("name", "experiment-service", "Service name for discovery")
		register  = flag.Bool("register", true, "Register with etcd")
	)
	flag.Parse()

	// --------------------------------------------------------------
	// 2.1 Set up the lifecycle manager.
	// --------------------------------------------------------------
	mgr := lifecycle.New()

	// --------------------------------------------------------------
	// 2.2 Create the WebSocket hub and HTTP server.
	// --------------------------------------------------------------
	h := hub.NewHub()
	go h.Run()

	// WebSocket endpoint.
	wsHandler := h.WebSocketHandler(websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	})

	// HTTP mux.
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", wsHandler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Experiment server. WebSocket endpoint: /ws")
	})

	httpServer := &http.Server{
		Addr:         ":" + *httpPort,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	mgr.AddFunc("http-server",
		func(ctx context.Context) error {
			log.Printf("HTTP/WebSocket server starting on port %s", *httpPort)
			go func() {
				<-ctx.Done()
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				httpServer.Shutdown(shutdownCtx)
			}()
			return httpServer.ListenAndServe()
		},
		func(ctx context.Context) error {
			return httpServer.Shutdown(ctx)
		},
	)

	// --------------------------------------------------------------
	// 2.3 Create the gRPC server and register our service.
	// --------------------------------------------------------------
	grpcSrv := grpc.NewServer()
	expSrv := &experimentServer{hub: h}
	pb.RegisterExperimentServiceServer(grpcSrv, expSrv)

	// Register health service.
	healthSrv := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcSrv, healthSrv)
	healthSrv.SetServingStatus(*svcName, grpc_health_v1.HealthCheckResponse_SERVING)

	// Reflection (useful for grpcurl).
	reflection.Register(grpcSrv)

	// gRPC listener.
	lis, err := net.Listen("tcp", ":"+*grpcPort)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	mgr.AddFunc("grpc-server",
		func(ctx context.Context) error {
			log.Printf("gRPC server starting on port %s", *grpcPort)
			go func() {
				<-ctx.Done()
				grpcSrv.GracefulStop()
			}()
			return grpcSrv.Serve(lis)
		},
		func(ctx context.Context) error {
			grpcSrv.GracefulStop()
			return nil
		},
	)

	// --------------------------------------------------------------
	// 2.4 Service discovery: register with etcd.
	// --------------------------------------------------------------
	if *register {
		// Parse etcd endpoints.
		endpoints := []string{*etcdEnd}
		registry, err := discovery.NewEtcdRegistry(endpoints, 5*time.Second)
		if err != nil {
			log.Fatalf("Failed to create etcd registry: %v", err)
		}

		// Service info.
		svc := discovery.ServiceInfo{
			Name:    *svcName,
			Addr:    net.JoinHostPort(getLocalIP(), *grpcPort),
			Version: "v1.0.0",
			Meta: map[string]string{
				"http_port": *httpPort,
				"protocol":  "grpc+websocket",
			},
		}

		// Register and keep alive.
		mgr.AddFunc("service-registry",
			func(ctx context.Context) error {
				if err := registry.Register(ctx, svc, 10); err != nil {
					return err
				}
				log.Printf("Registered with etcd: %s at %s", svc.Name, svc.Addr)
				// Keep the service alive until context is done.
				<-ctx.Done()
				return nil
			},
			func(ctx context.Context) error {
				return registry.Deregister(ctx, svc)
			},
		)

		// Optionally: register the weighted round‑robin balancer.
		roundrobin.RegisterWeightedRoundRobin()
		log.Println("Weighted round‑robin balancer registered.")
	}

	// --------------------------------------------------------------
	// 2.5 Run everything.
	// --------------------------------------------------------------
	log.Println("Experiment starting. Press Ctrl+C to stop.")
	if err := mgr.Run(context.Background()); err != nil {
		log.Fatalf("Lifecycle error: %v", err)
	}
	log.Println("Experiment stopped gracefully.")
}

// ----------------------------------------------------------------------
// 3. Helper: get a non‑loopback local IP address.
// ----------------------------------------------------------------------
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "localhost"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return "localhost"
}

// ----------------------------------------------------------------------
// 4. Proto definition reference (for completeness).
//    The corresponding experiment.proto file would be:
//
//    syntax = "proto3";
//    package experiment;
//    service ExperimentService {
//        rpc Echo (EchoRequest) returns (EchoResponse);
//        rpc Broadcast (BroadcastRequest) returns (BroadcastResponse);
//    }
//    message EchoRequest { string message = 1; }
//    message EchoResponse { string message = 1; }
//    message BroadcastRequest { string text = 1; }
//    message BroadcastResponse { bool success = 1; }
//
//    Generate with: protoc --go_out=. --go-grpc_out=. experiment.proto
// ----------------------------------------------------------------------