package testutils

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	// Example imports – uncomment and adjust paths when using other packages.
	// "yourmodule/broadcaster"
	// "yourmodule/discovery"
	// "yourmodule/frame"
	// "yourmodule/keepalive"
	// "yourmodule/lifecycle"
	// "yourmodule/listener"
)

/*
scratch.go is a scratchpad for quick experiments and examples.

It can be used to test code snippets, try out the various packages
(http, grpc, tcp, websocket, discovery, etc.) in a self‑contained way.

The code below shows a simple TCP echo server using the frame package.
You can modify it freely or replace it entirely.
*/

func main() {
	// ------------------------------------------------------------
	// Example 1: TCP echo server with length‑prefixed frames
	// ------------------------------------------------------------
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port

	log.Printf("starting frame echo server on %s", addr)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		go handleFrameConn(conn)
	}
}

// handleFrameConn uses the frame package to read and write length‑prefixed messages.
// Replace with your actual import path: "yourmodule/frame"
func handleFrameConn(conn net.Conn) {
	defer conn.Close()
	log.Printf("client connected: %s", conn.RemoteAddr())

	// Uncomment and use the frame package as shown below.
	/*
		codec := frame.NewFrameCodec(conn)
		for {
			payload, err := codec.Receive()
			if err != nil {
				log.Printf("receive error: %v", err)
				return
			}
			log.Printf("received: %s", payload)
			// Echo back
			if err := codec.Send(payload); err != nil {
				log.Printf("send error: %v", err)
				return
			}
		}
	*/

	// Fallback line‑based echo if frame package is not imported.
	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			log.Printf("read error: %v", err)
			return
		}
		conn.Write(buf[:n])
	}
}

/*
// ------------------------------------------------------------
// Example 2: Using the lifecycle manager to run an HTTP server
// ------------------------------------------------------------
func httpServerExample() {
	mgr := lifecycle.New()

	srv := &http.Server{Addr: ":8080"}
	mgr.AddFunc("http",
		func(ctx context.Context) error {
			go func() {
				<-ctx.Done()
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				srv.Shutdown(shutdownCtx)
			}()
			return srv.ListenAndServe()
		},
		func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
	)

	if err := mgr.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

// ------------------------------------------------------------
// Example 3: Service discovery with etcd (requires etcd client)
// ------------------------------------------------------------
func discoveryExample() {
	reg, err := discovery.NewEtcdRegistry([]string{"localhost:2379"}, 5*time.Second)
	if err != nil {
		log.Fatal(err)
	}
	defer reg.Close()

	svc := discovery.ServiceInfo{
		Name:    "example-service",
		Addr:    "localhost:50051",
		Version: "v1",
	}
	reg.Register(context.Background(), svc, 10)
	time.Sleep(30 * time.Second)
}

// ------------------------------------------------------------
// Example 4: Using the keepalive package
// ------------------------------------------------------------
func keepaliveExample() {
	conn, err := net.Dial("tcp", "example.com:80")
	if err != nil {
		log.Fatal(err)
	}
	tcpConn := conn.(*net.TCPConn)
	keepalive.DefaultTCPConfig().SetKeepAlive(tcpConn)

	// Or use custom ping/pong
	kaConn := keepalive.NewKeepAliveConn(conn, nil)
	defer kaConn.Close()
}
*/

// Use this file as a playground – uncomment the examples you need,
// adjust import paths, and run with `go run scratch.go`.