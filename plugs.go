// Package plugs defines a plugin framework for data gateways.
package testutils

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
)

// Status represents the operational state of a plug.
type Status string

const (
    StatusIdle         Status = "IDLE"
    StatusStarting     Status = "STARTING"
    StatusRunning      Status = "RUNNING"
    StatusStopping     Status = "STOPPING"
    StatusStopped      Status = "STOPPED"
    StatusError        Status = "ERROR"
)

// Plug is the fundamental interface for all pluggable components.
type Plug interface {
    // ID returns the unique identifier of this plug instance.
    ID() string
    // Type returns the plug type (e.g., "kafka-source", "http-sink").
    Type() string
    // Start initializes the plug and begins processing.
    Start(ctx context.Context) error
    // Stop gracefully shuts down the plug.
    Stop(ctx context.Context) error
    // Status returns the current status.
    Status() Status
    // Config returns the configuration used to create this plug.
    Config() json.RawMessage
}

// SourcePlug is a plug that ingests data from an external source.
type SourcePlug interface {
    Plug
    // Consume returns a read-only channel for incoming messages.
    Consume() <-chan Message
    // Errors returns a channel for asynchronous errors.
    Errors() <-chan error
}

// SinkPlug is a plug that sends data to an external destination.
type SinkPlug interface {
    Plug
    // Send delivers a message to the sink. It may block or return an error.
    Send(ctx context.Context, msg Message) error
    // BatchSend delivers multiple messages efficiently.
    BatchSend(ctx context.Context, msgs []Message) error
}

// Message is a generic container for data passing through the system.
type Message struct {
    ID        string            `json:"id"`
    Timestamp int64             `json:"timestamp"`
    Payload   json.RawMessage   `json:"payload"`
    Metadata  map[string]string `json:"metadata"`
}

// Factory is a function that instantiates a plug from raw configuration.
type Factory func(config json.RawMessage) (Plug, error)

// Registry holds all registered plug types.
var (
    registry   = make(map[string]Factory)
    registryMu sync.RWMutex
)

// Register makes a plug type available. It panics if the type is already registered.
func Register(typ string, factory Factory) {
    registryMu.Lock()
    defer registryMu.Unlock()
    if _, dup := registry[typ]; dup {
        panic(fmt.Sprintf("plugs: type %q already registered", typ))
    }
    registry[typ] = factory
}

// Create instantiates a plug of the given type with the provided configuration.
func Create(typ string, config json.RawMessage) (Plug, error) {
    registryMu.RLock()
    factory, ok := registry[typ]
    registryMu.RUnlock()
    if !ok {
        return nil, fmt.Errorf("plugs: unknown type %q", typ)
    }
    return factory(config)
}

// ListTypes returns all registered plug type names.
func ListTypes() []string {
    registryMu.RLock()
    defer registryMu.RUnlock()
    names := make([]string, 0, len(registry))
    for n := range registry {
        names = append(names, n)
    }
    return names
}