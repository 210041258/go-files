// Package testutils provides helpers for testing the data gateway.
package testutils

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
    "time"

)

// MockSource is a controllable source plug for testing.
type MockSource struct {
    id          string
    typ         string
    config      json.RawMessage
    status      plugs.Status
    mu          sync.RWMutex
    out         chan plugs.Message
    errCh       chan error
    cancel      context.CancelFunc
    messages    []plugs.Message // optional preloaded messages
}

func NewMockSource(id string, messages ...plugs.Message) *MockSource {
    return &MockSource{
        id:       id,
        typ:      "mock-source",
        status:   plugs.StatusIdle,
        out:      make(chan plugs.Message, 10),
        errCh:    make(chan error, 1),
        messages: messages,
    }
}

func (m *MockSource) ID() string                     { return m.id }
func (m *MockSource) Type() string                    { return m.typ }
func (m *MockSource) Config() json.RawMessage         { return m.config }
func (m *MockSource) Status() plugs.Status {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.status
}

func (m *MockSource) Start(ctx context.Context) error {
    m.mu.Lock()
    m.status = plugs.StatusRunning
    ctx, cancel := context.WithCancel(ctx)
    m.cancel = cancel
    m.mu.Unlock()

    go func() {
        for _, msg := range m.messages {
            select {
            case m.out <- msg:
            case <-ctx.Done():
                return
            }
        }
        // Keep running, waiting for injection via PushMessage
        <-ctx.Done()
    }()
    return nil
}

func (m *MockSource) Stop(ctx context.Context) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    if m.cancel != nil {
        m.cancel()
    }
    m.status = plugs.StatusStopped
    close(m.out)
    close(m.errCh)
    return nil
}

func (m *MockSource) Consume() <-chan plugs.Message {
    return m.out
}

func (m *MockSource) Errors() <-chan error {
    return m.errCh
}

// PushMessage injects a message into the source's output channel (for tests).
func (m *MockSource) PushMessage(msg plugs.Message) error {
    select {
    case m.out <- msg:
        return nil
    default:
        return fmt.Errorf("message channel full")
    }
}

// MockSink is a controllable sink plug for testing.
type MockSink struct {
    id          string
    typ         string
    config      json.RawMessage
    status      plugs.Status
    mu          sync.RWMutex
    received    []plugs.Message
    failOnSend  bool // simulate errors
}

func NewMockSink(id string) *MockSink {
    return &MockSink{
        id:       id,
        typ:      "mock-sink",
        status:   plugs.StatusIdle,
        received: []plugs.Message{},
    }
}

func (m *MockSink) ID() string                     { return m.id }
func (m *MockSink) Type() string                    { return m.typ }
func (m *MockSink) Config() json.RawMessage         { return m.config }
func (m *MockSink) Status() plugs.Status {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.status
}

func (m *MockSink) Start(ctx context.Context) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.status = plugs.StatusRunning
    return nil
}

func (m *MockSink) Stop(ctx context.Context) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.status = plugs.StatusStopped
    return nil
}

func (m *MockSink) Send(ctx context.Context, msg plugs.Message) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    if m.failOnSend {
        return fmt.Errorf("simulated send failure")
    }
    m.received = append(m.received, msg)
    return nil
}

func (m *MockSink) BatchSend(ctx context.Context, msgs []plugs.Message) error {
    for _, msg := range msgs {
        if err := m.Send(ctx, msg); err != nil {
            return err
        }
    }
    return nil
}

// ReceivedMessages returns all messages that were sent to this sink.
func (m *MockSink) ReceivedMessages() []plugs.Message {
    m.mu.RLock()
    defer m.mu.RUnlock()
    copy := make([]plugs.Message, len(m.received))
    copy = append(copy, m.received...)
    return copy
}

// SetFailOnSend controls whether Send returns an error.
func (m *MockSink) SetFailOnSend(fail bool) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.failOnSend = fail
}

// TestMessage creates a standard test message.
func TestMessage(id string, payload interface{}) plugs.Message {
    data, _ := json.Marshal(payload)
    return plugs.Message{
        ID:        id,
        Timestamp: time.Now().UnixNano(),
        Payload:   data,
        Metadata:  map[string]string{"test": "true"},
    }
}

// RunPipeline connects a source, a processor (optional), and a sink, and runs them.
// This is a simplified example; real code would manage goroutines and cancellation.
func RunPipeline(ctx context.Context, source plugs.SourcePlug, sink plugs.SinkPlug) error {
    if err := source.Start(ctx); err != nil {
        return err
    }
    if err := sink.Start(ctx); err != nil {
        source.Stop(ctx)
        return err
    }

    go func() {
        for {
            select {
            case msg, ok := <-source.Consume():
                if !ok {
                    return
                }
                // Optionally run through a processor here
                if err := sink.Send(ctx, msg); err != nil {
                    // handle error, maybe send to an error channel
                }
            case err := <-source.Errors():
                // log error
                _ = err
            case <-ctx.Done():
                return
            }
        }
    }()
    return nil
}