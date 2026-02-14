package testutils

import (
    "context"
    "encoding/json"
    "log"
    "net/http"
    "os"
    "os/signal"
    "sync"
    "syscall"
    "time"

    "github.com/gorilla/mux"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

// Outlet represents a destination for data.
type Outlet struct {
    ID         string                 `json:"id"`
    Type       string                 `json:"type"` // e.g., "kafka", "http", "s3"
    Config     map[string]interface{} `json:"config"`
    Status     string                 `json:"status"`
}

// OutletGateway manages outlets and routes data to them.
type OutletGateway struct {
    mu       sync.RWMutex
    outlets  map[string]*Outlet
    clients  map[string]interface{} // e.g., Kafka producers, HTTP clients
    server   *http.Server
}

func NewOutletGateway() *OutletGateway {
    return &OutletGateway{
        outlets: make(map[string]*Outlet),
        clients: make(map[string]interface{}),
    }
}

// RegisterOutlet creates a new outlet.
func (g *OutletGateway) registerOutlet(w http.ResponseWriter, r *http.Request) {
    var outlet Outlet
    if err := json.NewDecoder(r.Body).Decode(&outlet); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    g.mu.Lock()
    defer g.mu.Unlock()

    if _, exists := g.outlets[outlet.ID]; exists {
        http.Error(w, "Outlet already exists", http.StatusConflict)
        return
    }

    // Initialize client for the outlet type (e.g., Kafka producer)
    // This is simplified; in practice, you'd validate config and create client.
    g.outlets[outlet.ID] = &outlet
    outlet.Status = "active"
    g.clients[outlet.ID] = nil // placeholder

    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(outlet)
}

// SendData forwards data to a specific outlet.
func (g *OutletGateway) sendData(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    outletID := vars["id"]

    g.mu.RLock()
    outlet, ok := g.outlets[outletID]
    g.mu.RUnlock()

    if !ok {
        http.Error(w, "Outlet not found", http.StatusNotFound)
        return
    }

    var payload interface{}
    if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    // Send data using the appropriate client (e.g., Kafka producer)
    // For simplicity, we just log.
    log.Printf("Sending data to outlet %s (%s): %v", outlet.ID, outlet.Type, payload)

    // In real code: use g.clients[outletID] to send.
    // Handle errors, retries, etc.

    w.WriteHeader(http.StatusAccepted)
}

// ListOutlets returns all registered outlets.
func (g *OutletGateway) listOutlets(w http.ResponseWriter, r *http.Request) {
    g.mu.RLock()
    list := make([]*Outlet, 0, len(g.outlets))
    for _, o := range g.outlets {
        list = append(list, o)
    }
    g.mu.RUnlock()

    json.NewEncoder(w).Encode(list)
}

// Health check.
func (g *OutletGateway) healthCheck(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (g *OutletGateway) Start(addr string) error {
    r := mux.NewRouter()
    r.HandleFunc("/outlets", g.registerOutlet).Methods("POST")
    r.HandleFunc("/outlets", g.listOutlets).Methods("GET")
    r.HandleFunc("/outlets/{id}/data", g.sendData).Methods("POST")
    r.Handle("/metrics", promhttp.Handler())
    r.HandleFunc("/health", g.healthCheck).Methods("GET")

    g.server = &http.Server{Addr: addr, Handler: r}

    go func() {
        sig := make(chan os.Signal, 1)
        signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
        <-sig
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        g.server.Shutdown(ctx)
    }()

    log.Printf("Starting Outlet gateway on %s", addr)
    return g.server.ListenAndServe()
}

func main() {
    gateway := NewOutletGateway()
    if err := gateway.Start(":8081"); err != nil && err != http.ErrServerClosed {
        log.Fatal(err)
    }
}