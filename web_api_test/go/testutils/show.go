package testutils

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

func main() {
	// Command line flags
	port := flag.Int("port", 8080, "port to listen on")
	message := flag.String("message", "Hello, World!", "message to display")
	flag.Parse()

	// Handler function that writes the message
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s\n", *message)
	})

	// Start server
	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Serving on http://localhost%s with message: %q\n", addr, *message)
	log.Fatal(http.ListenAndServe(addr, nil))
}