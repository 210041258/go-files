package testutils

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// TCPServer represents a TCP server instance.
type TCPServer struct {
	addr    string
	listener net.Listener
	wg      sync.WaitGroup
	quit    chan struct{}
}

// NewTCPServer creates a new TCP server listening on the given address.
func NewTCPServer(addr string) *TCPServer {
	return &TCPServer{
		addr: addr,
		quit: make(chan struct{}),
	}
}

// Start begins listening and accepting connections.
func (s *TCPServer) Start(ctx context.Context) error {
	l, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.addr, err)
	}
	s.listener = l
	log.Printf("TCP server listening on %s", s.addr)

	go s.acceptLoop(ctx)
	return nil
}

// acceptLoop accepts incoming connections and handles them in goroutines.
func (s *TCPServer) acceptLoop(ctx context.Context) {
	for {
		select {
		case <-s.quit:
			return
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			// Check if the error is due to the listener being closed
			select {
			case <-s.quit:
				return
			default:
				log.Printf("accept error: %v", err)
			}
			continue
		}

		s.wg.Add(1)
		go s.handleConnection(ctx, conn)
	}
}

// handleConnection processes a single client connection.
func (s *TCPServer) handleConnection(ctx context.Context, conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	// Set read/write deadlines to prevent hanging connections
	if err := conn.SetDeadline(time.Now().Add(5 * time.Minute)); err != nil {
		log.Printf("set deadline error: %v", err)
		return
	}

	remoteAddr := conn.RemoteAddr().String()
	log.Printf("accepted connection from %s", remoteAddr)

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("[%s] received: %s", remoteAddr, line)

		// Echo the line back
		_, err := fmt.Fprintf(conn, "echo: %s\n", line)
		if err != nil {
			log.Printf("[%s] write error: %v", remoteAddr, err)
			return
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[%s] scan error: %v", remoteAddr, err)
	}
	log.Printf("connection from %s closed", remoteAddr)
}

// Stop gracefully shuts down the server.
func (s *TCPServer) Stop() {
	close(s.quit)

	if s.listener != nil {
		s.listener.Close()
	}

	// Wait for active connections to finish with a timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("all connections closed gracefully")
	case <-time.After(10 * time.Second):
		log.Println("shutdown timeout: forcing exit")
	}
}

func main() {
	// Read port from environment, default to 8080
	port := os.Getenv("TCP_PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port

	server := NewTCPServer(addr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the server
	if err := server.Start(ctx); err != nil {
		log.Fatal(err)
	}

	// Wait for interrupt signal to gracefully shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("shutting down TCP server...")

	server.Stop()
	log.Println("server stopped")
}