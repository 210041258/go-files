// Package main implements the bridge gateway, the central data routing service.
package testutils

import (
    "context"
    "encoding/json"
    "flag"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "yourproject/bridge"
    "yourproject/plugs"
    "yourproject/sink"
    "yourproject/batch"
    "yourproject/flash"
     "yourproject/sources/http"
     "yourproject/sources/kafka"
     "yourproject/sinks/elasticsearch"
    "yourproject/sinks/stdout"
)

// Config holds the entire gateway configuration.
type Config struct {
    Bridge bridge.Config         `json:"bridge"`
    Sources []SourceConfig       `json:"sources"`
    Sinks   []SinkConfig         `json:"sinks"`
    Batch   batch.Config         `json:"batch"`
    Flash   flash.Config         `json:"flash"`
    HTTP    struct {
        ListenAddr string `json:"listen_addr"`
    } `json:"http"`
}

// SourceConfig defines a source plug instance.
type SourceConfig struct {
    ID      string          `json:"id"`
    Type    string          `json:"type"`
    Config  json.RawMessage `json:"config"`
}

// SinkConfig defines a sink plug instance.
type SinkConfig struct {
    ID      string          `json:"id"`
    Type    string          `json:"type"`
    Config  json.RawMessage `json:"config"`
}

func main() {
    var configPath string
    flag.StringVar(&configPath, "config", "config.json", "path to config file")
    flag.Parse()

    // Load configuration
    cfg, err := loadConfig(configPath)
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // Set up logging and metrics (simplified)
    log.Println("Starting bridge gateway...")

    // Initialize flash storage (fast, ephemeral)
    flashStore, err := flash.NewStore(cfg.Flash)
    if err != nil {
        log.Fatalf("Failed to create flash store: %v", err)
    }
    defer flashStore.Close()

    // Initialize batch accumulator and processor
    batchCh := make(chan *batch.Batch, 100)
    accumulator := batch.NewAccumulator(cfg.Batch, batchCh)
    processor := batch.NewProcessor(cfg.Batch, batchCh, 0, func(ctx context.Context, b *batch.Batch) error {
        // This function is called when a batch is ready.
        // For now, we send it to all sinks (simplified routing).
        // In a real system, you'd route based on metadata or configuration.
        for _, sink := range sinks {
            if err := sink.SendBatch(ctx, b.Messages); err != nil {
                log.Printf("Sink error: %v", err)
                // Optionally store failed batch in flash for retry
                storeFailedBatch(b, flashStore)
            }
        }
        return nil
    })

    // Initialize bridge network manager
    b := bridge.NewBridge(
        func(conn *bridge.Connection) {
            log.Printf("New connection: %s", conn.ID)
            // Optionally set tags based on connection metadata
        },
        func(conn *bridge.Connection, data []byte) {
            // This is called when data arrives on a connection.
            // We convert raw data to a plug.Message and feed it to the accumulator.
            msg := plugs.Message{
                ID:        generateID(),
                Timestamp: time.Now().UnixNano(),
                Payload:   data,
                Metadata: map[string]string{
                    "conn_id": conn.ID,
                },
            }
            if err := accumulator.Add(context.Background(), msg); err != nil {
                log.Printf("Failed to add to batch: %v", err)
                // Could store in flash or dead-letter queue
            }
        },
    )

    // Start all source listeners (via bridge)
    for _, srcCfg := range cfg.Sources {
        plug, err := plugs.Create(srcCfg.Type, srcCfg.Config)
        if err != nil {
            log.Fatalf("Failed to create source %s: %v", srcCfg.ID, err)
        }
        source, ok := plug.(plugs.SourcePlug)
        if !ok {
            log.Fatalf("Plug %s is not a source", srcCfg.ID)
        }
        // Start the source; it will use bridge.AddListener internally.
        if err := source.Start(context.Background()); err != nil {
            log.Fatalf("Failed to start source %s: %v", srcCfg.ID, err)
        }
        defer source.Stop(context.Background())
    }

    // Instantiate all sinks (as sink.Sink, not plugs.SinkPlug)
    sinks := make([]sink.Sink, 0, len(cfg.Sinks))
    for _, snkCfg := range cfg.Sinks {
        s, err := sink.Create(snkCfg.Type, sink.Config{
            Type:          snkCfg.Type,
            BatchSize:     cfg.Batch.MaxSize,
            FlushInterval: cfg.Batch.FlushInterval,
            Options:       snkCfg.Config,
        })
        if err != nil {
            log.Fatalf("Failed to create sink %s: %v", snkCfg.ID, err)
        }
        sinks = append(sinks, s)
    }

    // Start batch processor
    processor.Start(context.Background())
    defer processor.Stop()

    // Start HTTP server for health and metrics
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
    })
    http.Handle("/metrics", promhttp.Handler()) // if using Prometheus
    httpServer := &http.Server{Addr: cfg.HTTP.ListenAddr, Handler: nil}
    go func() {
        if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Printf("HTTP server error: %v", err)
        }
    }()

    // Wait for shutdown signal
    sig := make(chan os.Signal, 1)
    signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
    <-sig
    log.Println("Shutting down...")

    // Graceful shutdown
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    httpServer.Shutdown(ctx)
    b.Close()
    accumulator.Close()
    processor.Stop()
    for _, s := range sinks {
        s.Close()
    }
    log.Println("Shutdown complete")
}

func loadConfig(path string) (*Config, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()
    var cfg Config
    if err := json.NewDecoder(f).Decode(&cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}

func generateID() string {
    return fmt.Sprintf("%d", time.Now().UnixNano())
}

func storeFailedBatch(b *batch.Batch, store flash.Store) {
    // Store batch in flash for later retry or manual inspection
    data, _ := json.Marshal(b)
    store.Set("failed:"+b.Created.String(), data, 24*time.Hour)
}