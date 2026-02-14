package testutils

import (
    "context"
    "encoding/json"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/gorilla/mux"
    "github.com/hashicorp/vent"
    "github.com/hashicorp/vent/pkg/sink"
)

// VentGateway handles incoming events and forwards them to Vent sinks.
type VentGateway struct {
    sinks []sink.Sink
    server *http.Server
}

func NewVentGateway(sinks ...sink.Sink) *VentGateway {
    return &VentGateway{sinks: sinks}
}

func (g *VentGateway) eventHandler(w http.ResponseWriter, r *http.Request) {
    var event map[string]interface{}
    if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    // Optionally add metadata (timestamp, source, etc.)
    event["received_at"] = time.Now().UTC()

    // Send to all configured Vent sinks
    for _, s := range g.sinks {
        if err := s.Send(event); err != nil {
            log.Printf("Sink error: %v", err)
        }
    }

    w.WriteHeader(http.StatusAccepted)
}

func (g *VentGateway) healthCheck(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (g *VentGateway) Start(addr string) error {
    r := mux.NewRouter()
    r.HandleFunc("/v1/events", g.eventHandler).Methods("POST")
    r.HandleFunc("/health", g.healthCheck).Methods("GET")

    g.server = &http.Server{Addr: addr, Handler: r}

    // Graceful shutdown on SIGINT/SIGTERM
    go func() {
        sig := make(chan os.Signal, 1)
        signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
        <-sig
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        g.server.Shutdown(ctx)
    }()

    log.Printf("Starting Vent gateway on %s", addr)
    return g.server.ListenAndServe()
}

func main() {
    // Example: configure sinks (e.g., Elasticsearch, file, stdout)
    sinks := []sink.Sink{
        sink.NewStdoutSink(),
        // sink.NewElasticsearchSink(...),
    }

    gateway := NewVentGateway(sinks...)
    if err := gateway.Start(":8080"); err != nil && err != http.ErrServerClosed {
        log.Fatal(err)
    }
}