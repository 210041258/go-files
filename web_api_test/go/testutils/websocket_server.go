package testutils

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketServer holds the server state and active connections.
type WebSocketServer struct {
	addr        string
	upgrader    websocket.Upgrader
	connections map[*websocket.Conn]bool
	mu          sync.Mutex
	wg          sync.WaitGroup
	quit        chan struct{}
}

// NewWebSocketServer creates a new WebSocket server instance.
func NewWebSocketServer(addr string) *WebSocketServer {
	return &WebSocketServer{
		addr: addr,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			// Allow all origins; customize in production.
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		connections: make(map[*websocket.Conn]bool),
		quit:        make(chan struct{}),
	}
}

// handleConnection processes an individual WebSocket connection.
func (s *WebSocketServer) handleConnection(conn *websocket.Conn) {
	defer func() {
		s.mu.Lock()
		delete(s.connections, conn)
		s.mu.Unlock()
		conn.Close()
		s.wg.Done()
	}()

	// Set initial read deadline and pong handler.
	conn.SetReadLimit(512 * 1024) // 512KB max message size
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Start a goroutine to send periodic pings.
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()
	go func() {
		for {
			select {
			case <-s.quit:
				return
			case <-pingTicker.C:
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()

	// Main message loop: echo received messages.
	for {
		select {
		case <-s.quit:
			// Server is shutting down; close connection.
			conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "server shutdown"))
			return
		default:
		}

		msgType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("read error: %v", err)
			}
			break
		}

		log.Printf("received: %s", message)

		// Echo the message back.
		if err := conn.WriteMessage(msgType, message); err != nil {
			log.Printf("write error: %v", err)
			break
		}
	}
}

// ServeHTTP implements http.Handler for WebSocket upgrades.
func (s *WebSocketServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Optionally authenticate the connection here (e.g., check headers, query params).
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		return
	}

	s.mu.Lock()
	s.connections[conn] = true
	s.mu.Unlock()
	s.wg.Add(1)

	go s.handleConnection(conn)
}

// Start begins listening for HTTP requests.
func (s *WebSocketServer) Start(ctx context.Context) error {
	server := &http.Server{
		Addr:    s.addr,
		Handler: s,
	}

	go func() {
		<-ctx.Done()
		log.Println("Shutting down WebSocket server...")
		close(s.quit) // signal all connections to close

		// Wait for active connections to finish with a timeout.
		done := make(chan struct{})
		go func() {
			s.wg.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			log.Println("Shutdown timeout: forcing connections closed")
		}

		// Shutdown HTTP server.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
	}()

	log.Printf("WebSocket server listening on %s", s.addr)
	return server.ListenAndServe()
}

func main() {
	// Read port from environment variable, default to 8080.
	port := os.Getenv("WS_PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port

	wsServer := NewWebSocketServer(addr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals for graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	if err := wsServer.Start(ctx); err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}

	log.Println("Server stopped gracefully.")
}