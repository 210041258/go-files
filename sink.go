// Package sink defines interfaces and implementations for data egress.
package testutils

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "sync"
    "time"

)

// Common errors.
var (
    ErrSinkNotReady = errors.New("sink not ready")
    ErrSendFailed   = errors.New("send failed")
)

// Sink is a high-level abstraction for delivering messages.
// It may include batching, retries, and health checks.
type Sink interface {
    // Send delivers a single message.
    Send(ctx context.Context, msg plugs.Message) error
    // SendBatch delivers multiple messages efficiently.
    SendBatch(ctx context.Context, msgs []plugs.Message) error
    // Close gracefully shuts down the sink.
    Close() error
    // Health checks if the sink is operational.
    Health() bool
}

// Config holds common sink settings.
type Config struct {
    Type       string          `json:"type"`
    BatchSize  int             `json:"batch_size"`
    FlushInterval time.Duration `json:"flush_interval"`
    Retries    int             `json:"retries"`
    Timeout    time.Duration   `json:"timeout"`
    Options    json.RawMessage `json:"options"`
}

// Factory creates a Sink from a Config.
type Factory func(cfg Config) (Sink, error)

var (
    factories = make(map[string]Factory)
    mu        sync.RWMutex
)

// Register makes a sink type available.
func Register(typ string, factory Factory) {
    mu.Lock()
    defer mu.Unlock()
    if _, dup := factories[typ]; dup {
        panic(fmt.Sprintf("sink: type %q already registered", typ))
    }
    factories[typ] = factory
}

// Create instantiates a sink of the given type with the provided config.
func Create(typ string, cfg Config) (Sink, error) {
    mu.RLock()
    factory, ok := factories[typ]
    mu.RUnlock()
    if !ok {
        return nil, fmt.Errorf("sink: unknown type %q", typ)
    }
    return factory(cfg)
}

// StdoutSink is a simple sink that logs messages to stdout.
type StdoutSink struct {
    healthy bool
    mu      sync.Mutex
}

func NewStdoutSink(cfg Config) (Sink, error) {
    return &StdoutSink{healthy: true}, nil
}

func (s *StdoutSink) Send(ctx context.Context, msg plugs.Message) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    data, _ := json.Marshal(msg)
    fmt.Printf("STDOUT SINK: %s\n", data)
    return nil
}

func (s *StdoutSink) SendBatch(ctx context.Context, msgs []plugs.Message) error {
    for _, msg := range msgs {
        if err := s.Send(ctx, msg); err != nil {
            return err
        }
    }
    return nil
}

func (s *StdoutSink) Close() error {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.healthy = false
    return nil
}

func (s *StdoutSink) Health() bool {
    s.mu.Lock()
    defer s.mu.Unlock()
    return s.healthy
}

func init() {
    Register("stdout", NewStdoutSink)
}

// SinkPlugAdapter wraps a Sink to make it a plugs.SinkPlug.
// This allows the plug system to use any registered Sink.
type SinkPlugAdapter struct {
    id     string
    config json.RawMessage
    sink   Sink
    status plugs.Status
    mu     sync.RWMutex
}

func NewSinkPlugAdapter(id string, config json.RawMessage, sink Sink) *SinkPlugAdapter {
    return &SinkPlugAdapter{
        id:     id,
        config: config,
        sink:   sink,
        status: plugs.StatusIdle,
    }
}

func (a *SinkPlugAdapter) ID() string                { return a.id }
func (a *SinkPlugAdapter) Type() string              { return "sink-adapter" }
func (a *SinkPlugAdapter) Config() json.RawMessage   { return a.config }
func (a *SinkPlugAdapter) Status() plugs.Status {
    a.mu.RLock()
    defer a.mu.RUnlock()
    return a.status
}

func (a *SinkPlugAdapter) Start(ctx context.Context) error {
    a.mu.Lock()
    defer a.mu.Unlock()
    a.status = plugs.StatusRunning
    return nil
}

func (a *SinkPlugAdapter) Stop(ctx context.Context) error {
    a.mu.Lock()
    defer a.mu.Unlock()
    err := a.sink.Close()
    if err != nil {
        a.status = plugs.StatusError
        return err
    }
    a.status = plugs.StatusStopped
    return nil
}

func (a *SinkPlugAdapter) Send(ctx context.Context, msg plugs.Message) error {
    return a.sink.Send(ctx, msg)
}

func (a *SinkPlugAdapter) BatchSend(ctx context.Context, msgs []plugs.Message) error {
    return a.sink.SendBatch(ctx, msgs)
}