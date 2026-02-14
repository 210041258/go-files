package testutils

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

/*
sandbox.go – a clean slate for experimenting with Go code.

You can use this file to quickly test ideas, try out new packages,
or prototype features before integrating them into larger projects.

Run it directly:  go run sandbox.go
Or build and run: go build -o sandbox && ./sandbox

Environment variable PORT can be used to change the listening port
(default 8080).
*/

func main() {
	// Get port from environment, default to 8080.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port

	// Register a simple handler.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello from sandbox on %s!\n", addr)
	})

	// Optional: add a /health endpoint for readiness/liveness checks.
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Printf("Sandbox server listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}

/*
------------------------------------------------------------------------
Quick Experiments – uncomment and modify the sections below to test
specific packages or patterns.

// Example: TCP echo server
func tcpEchoServer() {
	ln, err := net.Listen("tcp", ":9000")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go io.Copy(conn, conn) // echo
	}
}

// Example: using the frame package (requires frame import)
// func frameEchoServer() {
// 	ln, _ := net.Listen("tcp", ":9001")
// 	for {
// 		conn, _ := ln.Accept()
// 		go handleFrameConn(conn)
// 	}
// }
// func handleFrameConn(conn net.Conn) {
// 	defer conn.Close()
// 	codec := frame.NewFrameCodec(conn)
// 	for {
// 		payload, err := codec.Receive()
// 		if err != nil {
// 			return
// 		}
// 		codec.Send(payload) // echo
// 	}
// }

// Example: simple WebSocket echo (requires gorilla/websocket)
// func wsEchoServer() {
// 	upgrader := websocket.Upgrader{}
// 	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
// 		conn, _ := upgrader.Upgrade(w, r, nil)
// 		defer conn.Close()
// 		for {
// 			mt, msg, _ := conn.ReadMessage()
// 			conn.WriteMessage(mt, msg)
// 		}
// 	})
// 	log.Fatal(http.ListenAndServe(":8082", nil))
// }

// Example: etcd service registration (requires etcd client)
// func etcdRegister() {
// 	cli, _ := clientv3.New(clientv3.Config{Endpoints: []string{"localhost:2379"}})
// 	defer cli.Close()
// 	ctx := context.Background()
// 	resp, _ := cli.Grant(ctx, 10)
// 	cli.Put(ctx, "/services/myapp/instance", "localhost:50051", clientv3.WithLease(resp.ID))
// 	keepAliveCh, _ := cli.KeepAlive(ctx, resp.ID)
// 	for range keepAliveCh {}
// }

// Add more experiments below...
*/