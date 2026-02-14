// Package testutils provides mock and simple in-memory tracing utilities
// for testing storage, memory, network, and database operations. It allows
// tests to record, inspect, and verify trace spans and their relationships.
package testutils

import (
    "context"
    "sync"
    "time"
)

// --------------------------------------------------------------------
// Span – represents a unit of work in a trace.
// --------------------------------------------------------------------

// SpanContext holds immutable trace identifiers.
type SpanContext struct {
    TraceID string
    SpanID  string
    ParentID string
}

// Span represents an individual operation within a trace.
type Span struct {
    Context    SpanContext
    Name       string
    StartTime  time.Time
    EndTime    time.Time
    Tags       map[string]interface{}
    Logs       []SpanLog
    Status     SpanStatus
}

// SpanLog is a timestamped log message within a span.
type SpanLog struct {
    Timestamp time.Time
    Fields    map[string]interface{}
}

// SpanStatus represents the outcome of a span.
type SpanStatus struct {
    Code    int    // 0 = OK, non‑zero = error
    Message string
}

const (
    StatusOK = 0
    StatusError = 1
)

// --------------------------------------------------------------------
// Tracer – interface for creating spans.
// --------------------------------------------------------------------

// Tracer defines methods for starting and managing spans.
type Tracer interface {
    // StartSpan starts a new span with the given name and optional parent context.
    // If parent is nil, the span becomes a new root.
    StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span)

    // EndSpan marks the span as finished. Typically called via defer.
    EndSpan(span Span, opts ...SpanOption)

    // Flush ensures all buffered spans are written (if needed).
    Flush() error

    // Close shuts down the tracer.
    Close() error
}

// SpanOption configures a span at creation or completion.
type SpanOption func(*spanConfig)

type spanConfig struct {
    tags map[string]interface{}
    logs []SpanLog
    status SpanStatus
    startTime time.Time
    endTime time.Time
}

func defaultSpanConfig() *spanConfig {
    return &spanConfig{
        tags: make(map[string]interface{}),
    }
}

// WithTags adds tags to the span.
func WithTags(tags map[string]interface{}) SpanOption {
    return func(c *spanConfig) {
        for k, v := range tags {
            c.tags[k] = v
        }
    }
}

// WithTag adds a single tag.
func WithTag(key string, value interface{}) SpanOption {
    return func(c *spanConfig) {
        c.tags[key] = value
    }
}

// WithLog adds a log entry.
func WithLog(fields map[string]interface{}) SpanOption {
    return func(c *spanConfig) {
        c.logs = append(c.logs, SpanLog{
            Timestamp: time.Now(),
            Fields:    fields,
        })
    }
}

// WithStatus sets the span status.
func WithStatus(code int, message string) SpanOption {
    return func(c *spanConfig) {
        c.status = SpanStatus{Code: code, Message: message}
    }
}

// WithStartTime overrides the span start time.
func WithStartTime(t time.Time) SpanOption {
    return func(c *spanConfig) {
        c.startTime = t
    }
}

// WithEndTime overrides the span end time.
func WithEndTime(t time.Time) SpanOption {
    return func(c *spanConfig) {
        c.endTime = t
    }
}

// --------------------------------------------------------------------
// MockTracer – a test double that records all spans.
// --------------------------------------------------------------------

// MockTracer implements Tracer for unit tests.
type MockTracer struct {
    mu          sync.Mutex
    spans       []Span
    startFunc   func(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span)
    endFunc     func(span Span, opts ...SpanOption)
    flushFunc   func() error
    closeFunc   func() error
    startCalls  int
    endCalls    int
    flushCalls  int
    closeCalls  int
}

// NewMockTracer creates a new mock tracer.
func NewMockTracer() *MockTracer {
    return &MockTracer{}
}

// SetStartFunc overrides the StartSpan method.
func (m *MockTracer) SetStartFunc(fn func(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span)) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.startFunc = fn
}

// SetEndFunc overrides the EndSpan method.
func (m *MockTracer) SetEndFunc(fn func(span Span, opts ...SpanOption)) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.endFunc = fn
}

// SetFlushFunc overrides the Flush method.
func (m *MockTracer) SetFlushFunc(fn func() error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.flushFunc = fn
}

// SetCloseFunc overrides the Close method.
func (m *MockTracer) SetCloseFunc(fn func() error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.closeFunc = fn
}

// StartSpan records the call and delegates to custom function or returns a new span.
func (m *MockTracer) StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span) {
    m.mu.Lock()
    m.startCalls++
    if m.startFunc != nil {
        fn := m.startFunc
        m.mu.Unlock()
        return fn(ctx, name, opts...)
    }
    // Default: create a simple span with generated IDs.
    span := Span{
        Context: SpanContext{
            TraceID: "mock-trace",
            SpanID:  "mock-span",
        },
        Name:      name,
        StartTime: time.Now(),
        Tags:      make(map[string]interface{}),
    }
    // Apply options
    cfg := defaultSpanConfig()
    for _, opt := range opts {
        opt(cfg)
    }
    for k, v := range cfg.tags {
        span.Tags[k] = v
    }
    if !cfg.startTime.IsZero() {
        span.StartTime = cfg.startTime
    }
    m.spans = append(m.spans, span)
    m.mu.Unlock()
    return ctx, span
}

// EndSpan records the call and delegates.
func (m *MockTracer) EndSpan(span Span, opts ...SpanOption) {
    m.mu.Lock()
    m.endCalls++
    if m.endFunc != nil {
        fn := m.endFunc
        m.mu.Unlock()
        fn(span, opts...)
        return
    }
    // Update the stored span with end time and options.
    for i, s := range m.spans {
        if s.Context.SpanID == span.Context.SpanID {
            cfg := defaultSpanConfig()
            for _, opt := range opts {
                opt(cfg)
            }
            if !cfg.endTime.IsZero() {
                span.EndTime = cfg.endTime
            } else if span.EndTime.IsZero() {
                span.EndTime = time.Now()
            }
            if cfg.status.Code != 0 || cfg.status.Message != "" {
                span.Status = cfg.status
            }
            if len(cfg.tags) > 0 {
                if span.Tags == nil {
                    span.Tags = make(map[string]interface{})
                }
                for k, v := range cfg.tags {
                    span.Tags[k] = v
                }
            }
            if len(cfg.logs) > 0 {
                span.Logs = append(span.Logs, cfg.logs...)
            }
            m.spans[i] = span
            break
        }
    }
    m.mu.Unlock()
}

// Flush records the call.
func (m *MockTracer) Flush() error {
    m.mu.Lock()
    m.flushCalls++
    if m.flushFunc != nil {
        fn := m.flushFunc
        m.mu.Unlock()
        return fn()
    }
    m.mu.Unlock()
    return nil
}

// Close records the call.
func (m *MockTracer) Close() error {
    m.mu.Lock()
    m.closeCalls++
    if m.closeFunc != nil {
        fn := m.closeFunc
        m.mu.Unlock()
        return fn()
    }
    m.mu.Unlock()
    return nil
}

// Spans returns a copy of all recorded spans.
func (m *MockTracer) Spans() []Span {
    m.mu.Lock()
    defer m.mu.Unlock()
    cp := make([]Span, len(m.spans))
    copy(cp, m.spans)
    return cp
}

// CallCounts returns the number of calls to each method.
func (m *MockTracer) CallCounts() (start, end, flush, close int) {
    m.mu.Lock()
    defer m.mu.Unlock()
    return m.startCalls, m.endCalls, m.flushCalls, m.closeCalls
}

// Reset clears recorded spans and call counts.
func (m *MockTracer) Reset() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.spans = nil
    m.startCalls = 0
    m.endCalls = 0
    m.flushCalls = 0
    m.closeCalls = 0
    m.startFunc = nil
    m.endFunc = nil
    m.flushFunc = nil
    m.closeFunc = nil
}

// --------------------------------------------------------------------
// InMemoryTracer – a simple tracer that stores spans in memory.
// --------------------------------------------------------------------

// InMemoryTracer implements Tracer with in‑memory span storage.
type InMemoryTracer struct {
    mu       sync.Mutex
    spans    []Span
    idGen    func() string // for generating trace/span IDs
}

// NewInMemoryTracer creates a new tracer with a simple ID generator.
func NewInMemoryTracer() *InMemoryTracer {
    return &InMemoryTracer{
        idGen: generateSimpleID,
    }
}

// SetIDGen allows overriding the ID generator (useful for deterministic tests).
func (t *InMemoryTracer) SetIDGen(fn func() string) {
    t.mu.Lock()
    defer t.mu.Unlock()
    t.idGen = fn
}

// StartSpan begins a new span.
func (t *InMemoryTracer) StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span) {
    t.mu.Lock()
    defer t.mu.Unlock()
    traceID := t.idGen()
    spanID := t.idGen()
    parentID := ""
    // Extract parent from context if present (simplified; real tracer would use context propagation)
    if parent := spanFromContext(ctx); parent != nil {
        traceID = parent.Context.TraceID
        parentID = parent.Context.SpanID
    }
    span := Span{
        Context: SpanContext{
            TraceID:  traceID,
            SpanID:   spanID,
            ParentID: parentID,
        },
        Name:      name,
        StartTime: time.Now(),
        Tags:      make(map[string]interface{}),
    }
    cfg := defaultSpanConfig()
    for _, opt := range opts {
        opt(cfg)
    }
    for k, v := range cfg.tags {
        span.Tags[k] = v
    }
    if !cfg.startTime.IsZero() {
        span.StartTime = cfg.startTime
    }
    if len(cfg.logs) > 0 {
        span.Logs = append(span.Logs, cfg.logs...)
    }
    t.spans = append(t.spans, span)
    // Return a new context containing the span.
    return context.WithValue(ctx, spanContextKey{}, &span), span
}

// EndSpan finishes a span.
func (t *InMemoryTracer) EndSpan(span Span, opts ...SpanOption) {
    t.mu.Lock()
    defer t.mu.Unlock()
    for i, s := range t.spans {
        if s.Context.SpanID == span.Context.SpanID {
            cfg := defaultSpanConfig()
            for _, opt := range opts {
                opt(cfg)
            }
            if !cfg.endTime.IsZero() {
                span.EndTime = cfg.endTime
            } else if span.EndTime.IsZero() {
                span.EndTime = time.Now()
            }
            if cfg.status.Code != 0 || cfg.status.Message != "" {
                span.Status = cfg.status
            }
            if len(cfg.tags) > 0 {
                if span.Tags == nil {
                    span.Tags = make(map[string]interface{})
                }
                for k, v := range cfg.tags {
                    span.Tags[k] = v
                }
            }
            if len(cfg.logs) > 0 {
                span.Logs = append(span.Logs, cfg.logs...)
            }
            t.spans[i] = span
            return
        }
    }
}

// Flush is a no‑op for in‑memory tracer.
func (t *InMemoryTracer) Flush() error { return nil }

// Close is a no‑op.
func (t *InMemoryTracer) Close() error { return nil }

// Spans returns a copy of all spans.
func (t *InMemoryTracer) Spans() []Span {
    t.mu.Lock()
    defer t.mu.Unlock()
    cp := make([]Span, len(t.spans))
    copy(cp, t.spans)
    return cp
}

// Clear removes all spans.
func (t *InMemoryTracer) Clear() {
    t.mu.Lock()
    defer t.mu.Unlock()
    t.spans = nil
}

// generateSimpleID returns a simple string ID (for tests).
var idCounter int64

func generateSimpleID() string {
    idCounter++
    return string(rune(idCounter)) // simplified; use strconv in real code
}

type spanContextKey struct{}

// spanFromContext retrieves the current span from context.
func spanFromContext(ctx context.Context) *Span {
    if ctx == nil {
        return nil
    }
    if span, ok := ctx.Value(spanContextKey{}).(*Span); ok {
        return span
    }
    return nil
}

// --------------------------------------------------------------------
// TracerConditioner – wraps a Tracer to inject delays and errors.
// --------------------------------------------------------------------

// TracerConditioner adds configurable delays and error injection to a Tracer.
type TracerConditioner struct {
    mu            sync.Mutex
    tracer        Tracer
    startDelay    time.Duration
    endDelay      time.Duration
    flushDelay    time.Duration
    startErrors   map[int]error
    endErrors     map[int]error
    flushErrors   map[int]error
    startCalls    int
    endCalls      int
    flushCalls    int
}

// NewTracerConditioner creates a conditioner around an existing Tracer.
func NewTracerConditioner(tracer Tracer) *TracerConditioner {
    return &TracerConditioner{
        tracer:      tracer,
        startErrors: make(map[int]error),
        endErrors:   make(map[int]error),
        flushErrors: make(map[int]error),
    }
}

// SetStartDelay adds a fixed delay before StartSpan.
func (c *TracerConditioner) SetStartDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.startDelay = d
}

// SetEndDelay adds a fixed delay before EndSpan.
func (c *TracerConditioner) SetEndDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.endDelay = d
}

// SetFlushDelay adds a fixed delay before Flush.
func (c *TracerConditioner) SetFlushDelay(d time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.flushDelay = d
}

// InjectStartError makes the nth call to StartSpan return the given error.
// Note: StartSpan doesn't return error normally; we'll simulate by returning
// a zero span and passing error via a panic or return value? Better to not do that.
// Instead, we'll make the conditioner return a sentinel error via a custom function.
// Simpler: we'll not inject errors on StartSpan; we'll just add delay.
func (c *TracerConditioner) InjectStartError(callNumber int, err error) {
    // Not implemented – StartSpan doesn't return error.
}

// InjectEndError makes the nth call to EndSpan return an error (but EndSpan doesn't return error).
// So we'll not implement either.

// StartSpan adds delay then delegates.
func (c *TracerConditioner) StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span) {
    c.mu.Lock()
    c.startCalls++
    delay := c.startDelay
    c.mu.Unlock()
    if delay > 0 {
        time.Sleep(delay)
    }
    return c.tracer.StartSpan(ctx, name, opts...)
}

// EndSpan adds delay then delegates.
func (c *TracerConditioner) EndSpan(span Span, opts ...SpanOption) {
    c.mu.Lock()
    c.endCalls++
    delay := c.endDelay
    c.mu.Unlock()
    if delay > 0 {
        time.Sleep(delay)
    }
    c.tracer.EndSpan(span, opts...)
}

// Flush adds delay then delegates.
func (c *TracerConditioner) Flush() error {
    c.mu.Lock()
    c.flushCalls++
    delay := c.flushDelay
    if err, ok := c.flushErrors[c.flushCalls]; ok {
        delete(c.flushErrors, c.flushCalls)
        c.mu.Unlock()
        return err
    }
    c.mu.Unlock()
    if delay > 0 {
        time.Sleep(delay)
    }
    return c.tracer.Flush()
}

// Close delegates.
func (c *TracerConditioner) Close() error {
    return c.tracer.Close()
}

// --------------------------------------------------------------------
// TraceAssertions – helper functions for testing with Tracer.
// --------------------------------------------------------------------

type testingT interface {
    Error(args ...interface{})
    Errorf(format string, args ...interface{})
}

// TraceAssertions provides convenience methods for verifying traces.
type TraceAssertions struct {
    t testingT
}

// NewTraceAssertions creates a new assertion helper.
func NewTraceAssertions(t testingT) *TraceAssertions {
    return &TraceAssertions{t: t}
}

// AssertSpanCount asserts the total number of spans.
func (a *TraceAssertions) AssertSpanCount(tracer interface{ Spans() []Span }, expected int) {
    spans := tracer.Spans()
    if len(spans) != expected {
        a.t.Errorf("expected %d spans, got %d", expected, len(spans))
    }
}

// AssertSpanExists asserts that a span with the given name exists.
func (a *TraceAssertions) AssertSpanExists(tracer interface{ Spans() []Span }, name string) {
    spans := tracer.Spans()
    for _, s := range spans {
        if s.Name == name {
            return
        }
    }
    a.t.Errorf("expected span with name %q not found", name)
}

// AssertSpanHasTag asserts that a span (by name) has a tag with the given key and value.
func (a *TraceAssertions) AssertSpanHasTag(tracer interface{ Spans() []Span }, spanName, tagKey string, tagValue interface{}) {
    spans := tracer.Spans()
    for _, s := range spans {
        if s.Name == spanName {
            val, ok := s.Tags[tagKey]
            if !ok {
                a.t.Errorf("span %q missing tag %q", spanName, tagKey)
                return
            }
            if val != tagValue {
                a.t.Errorf("span %q tag %q = %v, expected %v", spanName, tagKey, val, tagValue)
            }
            return
        }
    }
    a.t.Errorf("span with name %q not found", spanName)
}