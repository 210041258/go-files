package testutils

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ErrorSeverity represents the severity level of an error
type ErrorSeverity int

const (
	SeverityLow ErrorSeverity = iota
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

var severityNames = []string{"LOW", "MEDIUM", "HIGH", "CRITICAL"}

// ErrorMetadata holds additional information about an error
type ErrorMetadata struct {
	Timestamp  time.Time              `json:"timestamp"`
	Severity   ErrorSeverity          `json:"severity"`
	Code       string                 `json:"code,omitempty"`
	Component  string                 `json:"component,omitempty"`
	Operation  string                 `json:"operation,omitempty"`
	StackTrace string                 `json:"stack_trace,omitempty"`
	Context    map[string]interface{} `json:"context,omitempty"`
}

// WrappedError represents an error with metadata
type WrappedError struct {
	error
	Metadata ErrorMetadata `json:"metadata"`
}

// WrapError wraps an error with metadata
func WrapError(err error, opts ...ErrorOption) error {
	if err == nil {
		return nil
	}

	metadata := ErrorMetadata{
		Timestamp: time.Now().UTC(),
		Severity:  SeverityMedium,
		Context:   make(map[string]interface{}),
	}

	for _, opt := range opts {
		opt(&metadata)
	}

	// Capture stack trace if not already provided
	if metadata.StackTrace == "" {
		var buf [4096]byte
		n := runtime.Stack(buf[:], false)
		metadata.StackTrace = string(buf[:n])
	}

	return &WrappedError{
		error:    err,
		Metadata: metadata,
	}
}

// ErrorOption configures error metadata
type ErrorOption func(*ErrorMetadata)

// WithSeverity sets the error severity
func WithSeverity(severity ErrorSeverity) ErrorOption {
	return func(m *ErrorMetadata) {
		m.Severity = severity
	}
}

// WithErrorCode sets an error code
func WithErrorCode(code string) ErrorOption {
	return func(m *ErrorMetadata) {
		m.Code = code
	}
}

// WithComponent sets the component where the error occurred
func WithComponent(component string) ErrorOption {
	return func(m *ErrorMetadata) {
		m.Component = component
	}
}

// WithOperation sets the operation being performed
func WithOperation(operation string) ErrorOption {
	return func(m *ErrorMetadata) {
		m.Operation = operation
	}
}

// WithStackTrace sets a custom stack trace
func WithStackTrace(trace string) ErrorOption {
	return func(m *ErrorMetadata) {
		m.StackTrace = trace
	}
}

// WithContext adds context data to the error
func WithContext(key string, value interface{}) ErrorOption {
	return func(m *ErrorMetadata) {
		if m.Context == nil {
			m.Context = make(map[string]interface{})
		}
		m.Context[key] = value
	}
}

// WithContextMap adds multiple context entries
func WithContextMap(context map[string]interface{}) ErrorOption {
	return func(m *ErrorMetadata) {
		if m.Context == nil {
			m.Context = make(map[string]interface{})
		}
		for k, v := range context {
			m.Context[k] = v
		}
	}
}

// CompositeError combines multiple errors with improved formatting and metadata
type CompositeError struct {
	mu        sync.RWMutex
	Errors    []*WrappedError `json:"errors"`
	Prefix    string          `json:"prefix,omitempty"`
	Metadata  ErrorMetadata   `json:"metadata"`
	createdAt time.Time
}

// NewCompositeError creates a new CompositeError
func NewCompositeError(prefix string, opts ...ErrorOption) *CompositeError {
	metadata := ErrorMetadata{
		Timestamp: time.Now().UTC(),
		Severity:  SeverityMedium,
		Context:   make(map[string]interface{}),
	}

	for _, opt := range opts {
		opt(&metadata)
	}

	return &CompositeError{
		Errors:    make([]*WrappedError, 0),
		Prefix:    prefix,
		Metadata:  metadata,
		createdAt: time.Now().UTC(),
	}
}

// Error returns the formatted error string
func (ce *CompositeError) Error() string {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	if len(ce.Errors) == 0 {
		return "no errors"
	}

	var builder strings.Builder
	if ce.Prefix != "" {
		builder.WriteString(ce.Prefix)
		builder.WriteString(": ")
	}

	builder.WriteString(fmt.Sprintf("%d error(s) occurred", len(ce.Errors)))

	// Add metadata summary
	if ce.Metadata.Component != "" || ce.Metadata.Operation != "" {
		builder.WriteString(" [")
		if ce.Metadata.Component != "" {
			builder.WriteString(ce.Metadata.Component)
		}
		if ce.Metadata.Operation != "" {
			if ce.Metadata.Component != "" {
				builder.WriteString(".")
			}
			builder.WriteString(ce.Metadata.Operation)
		}
		builder.WriteString("]")
	}

	builder.WriteString(":\n")

	for i, wrappedErr := range ce.Errors {
		builder.WriteString(fmt.Sprintf("  %d. ", i+1))

		// Add error severity if available
		severity := severityNames[wrappedErr.Metadata.Severity]
		builder.WriteString(fmt.Sprintf("[%s] ", severity))

		// Add error code if available
		if wrappedErr.Metadata.Code != "" {
			builder.WriteString(fmt.Sprintf("(%s) ", wrappedErr.Metadata.Code))
		}

		builder.WriteString(wrappedErr.error.Error())

		// Add timestamp if it's recent (within last hour)
		if time.Since(wrappedErr.Metadata.Timestamp) < time.Hour {
			builder.WriteString(fmt.Sprintf(" (at %s)", wrappedErr.Metadata.Timestamp.Format("15:04:05")))
		}

		builder.WriteString("\n")

		// Add context if available
		if len(wrappedErr.Metadata.Context) > 0 {
			builder.WriteString("     Context: ")
			first := true
			for k, v := range wrappedErr.Metadata.Context {
				if !first {
					builder.WriteString(", ")
				}
				builder.WriteString(fmt.Sprintf("%s=%v", k, v))
				first = false
			}
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

// Add adds an error to the composite error with optional metadata
func (ce *CompositeError) Add(err error, opts ...ErrorOption) {
	if err == nil {
		return
	}

	ce.mu.Lock()
	defer ce.mu.Unlock()

	// Handle other CompositeError instances
	if other, ok := err.(*CompositeError); ok {
		other.mu.RLock()
		ce.Errors = append(ce.Errors, other.Errors...)
		other.mu.RUnlock()
		return
	}

	// Handle WrappedError instances
	var wrappedErr *WrappedError
	if we, ok := err.(*WrappedError); ok {
		wrappedErr = we
	} else {
		// Wrap the error with metadata
		metadata := ErrorMetadata{
			Timestamp: time.Now().UTC(),
			Severity:  ce.Metadata.Severity, // Inherit from composite
			Context:   make(map[string]interface{}),
		}

		// Apply options
		for _, opt := range opts {
			opt(&metadata)
		}

		// Capture stack trace if not provided
		if metadata.StackTrace == "" {
			var buf [4096]byte
			n := runtime.Stack(buf[:], false)
			metadata.StackTrace = string(buf[:n])
		}

		// Merge with composite context
		for k, v := range ce.Metadata.Context {
			if _, exists := metadata.Context[k]; !exists {
				metadata.Context[k] = v
			}
		}

		wrappedErr = &WrappedError{
			error:    err,
			Metadata: metadata,
		}
	}

	ce.Errors = append(ce.Errors, wrappedErr)
}

// AddWithMetadata adds an error with explicit metadata
func (ce *CompositeError) AddWithMetadata(err error, metadata ErrorMetadata) {
	if err == nil {
		return
	}

	ce.mu.Lock()
	defer ce.mu.Unlock()

	// Ensure timestamp
	if metadata.Timestamp.IsZero() {
		metadata.Timestamp = time.Now().UTC()
	}

	// Ensure context map
	if metadata.Context == nil {
		metadata.Context = make(map[string]interface{})
	}

	// Merge with composite context
	for k, v := range ce.Metadata.Context {
		if _, exists := metadata.Context[k]; !exists {
			metadata.Context[k] = v
		}
	}

	wrappedErr := &WrappedError{
		error:    err,
		Metadata: metadata,
	}

	ce.Errors = append(ce.Errors, wrappedErr)
}

// HasErrors returns true if there are any errors
func (ce *CompositeError) HasErrors() bool {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	return len(ce.Errors) > 0
}

// ErrorCount returns the number of errors
func (ce *CompositeError) ErrorCount() int {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	return len(ce.Errors)
}

// First returns the first error, if any
func (ce *CompositeError) First() error {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	if len(ce.Errors) == 0 {
		return nil
	}
	return ce.Errors[0].error
}

// Last returns the last error, if any
func (ce *CompositeError) Last() error {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	if len(ce.Errors) == 0 {
		return nil
	}
	return ce.Errors[len(ce.Errors)-1].error
}

// All returns all underlying errors
func (ce *CompositeError) All() []error {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	errors := make([]error, len(ce.Errors))
	for i, wrappedErr := range ce.Errors {
		errors[i] = wrappedErr.error
	}
	return errors
}

// AllWrapped returns all wrapped errors with metadata
func (ce *CompositeError) AllWrapped() []*WrappedError {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	wrapped := make([]*WrappedError, len(ce.Errors))
	copy(wrapped, ce.Errors)
	return wrapped
}

// FilterBySeverity returns errors with the specified severity or higher
func (ce *CompositeError) FilterBySeverity(minSeverity ErrorSeverity) *CompositeError {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	filtered := NewCompositeError(ce.Prefix + " (filtered)")
	filtered.Metadata = ce.Metadata

	for _, wrappedErr := range ce.Errors {
		if wrappedErr.Metadata.Severity >= minSeverity {
			filtered.Errors = append(filtered.Errors, wrappedErr)
		}
	}

	return filtered
}

// FilterByComponent returns errors from the specified component
func (ce *CompositeError) FilterByComponent(component string) *CompositeError {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	filtered := NewCompositeError(ce.Prefix + " (filtered)")
	filtered.Metadata = ce.Metadata

	for _, wrappedErr := range ce.Errors {
		if wrappedErr.Metadata.Component == component {
			filtered.Errors = append(filtered.Errors, wrappedErr)
		}
	}

	return filtered
}

// Contains checks if the composite contains an error matching the target
func (ce *CompositeError) Contains(target error) bool {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	for _, wrappedErr := range ce.Errors {
		if errors.Is(wrappedErr.error, target) {
			return true
		}
	}
	return false
}

// ContainsCode checks if any error has the specified error code
func (ce *CompositeError) ContainsCode(code string) bool {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	for _, wrappedErr := range ce.Errors {
		if wrappedErr.Metadata.Code == code {
			return true
		}
	}
	return false
}

// Unwrap returns the underlying errors (compatible with errors.Unwrap)
func (ce *CompositeError) Unwrap() []error {
	return ce.All()
}

// As implements errors.As
func (ce *CompositeError) As(target interface{}) bool {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	for _, wrappedErr := range ce.Errors {
		if errors.As(wrappedErr.error, target) {
			return true
		}
	}
	return false
}

// Is implements errors.Is
func (ce *CompositeError) Is(target error) bool {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	for _, wrappedErr := range ce.Errors {
		if errors.Is(wrappedErr.error, target) {
			return true
		}
	}
	return false
}

// JSON returns the composite error as JSON
func (ce *CompositeError) JSON() ([]byte, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	type jsonError struct {
		Message  string        `json:"message"`
		Metadata ErrorMetadata `json:"metadata"`
	}

	type jsonComposite struct {
		Prefix    string        `json:"prefix,omitempty"`
		Count     int           `json:"count"`
		CreatedAt time.Time     `json:"created_at"`
		Metadata  ErrorMetadata `json:"metadata"`
		Errors    []jsonError   `json:"errors"`
	}

	data := jsonComposite{
		Prefix:    ce.Prefix,
		Count:     len(ce.Errors),
		CreatedAt: ce.createdAt,
		Metadata:  ce.Metadata,
		Errors:    make([]jsonError, len(ce.Errors)),
	}

	for i, wrappedErr := range ce.Errors {
		data.Errors[i] = jsonError{
			Message:  wrappedErr.error.Error(),
			Metadata: wrappedErr.Metadata,
		}
	}

	return json.MarshalIndent(data, "", "  ")
}

// Summary returns a summary of errors by severity
func (ce *CompositeError) Summary() map[string]int {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	summary := make(map[string]int)
	summary["total"] = len(ce.Errors)

	for _, severity := range severityNames {
		summary[severity] = 0
	}

	for _, wrappedErr := range ce.Errors {
		severity := severityNames[wrappedErr.Metadata.Severity]
		summary[severity]++
	}

	return summary
}

// Clear removes all errors
func (ce *CompositeError) Clear() {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	ce.Errors = make([]*WrappedError, 0)
}

// Age returns the age of the composite error
func (ce *CompositeError) Age() time.Duration {
	return time.Since(ce.createdAt)
}

// Merge merges another composite error into this one
func (ce *CompositeError) Merge(other *CompositeError) {
	if other == nil {
		return
	}

	other.mu.RLock()
	ce.mu.Lock()
	defer ce.mu.Unlock()
	defer other.mu.RUnlock()

	ce.Errors = append(ce.Errors, other.Errors...)
}

// ErrorChain represents a chain of errors (for compatibility with Go's error chain)
func (ce *CompositeError) ErrorChain() []error {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	chain := make([]error, 0, len(ce.Errors))
	for _, wrappedErr := range ce.Errors {
		chain = append(chain, wrappedErr.error)
	}
	return chain
}

// PrettyPrint returns a formatted string for human-readable output
func (ce *CompositeError) PrettyPrint() string {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	if len(ce.Errors) == 0 {
		return "✓ No errors"
	}

	var builder strings.Builder
	builder.WriteString("╔═══════════════════════════════════════════════════════════╗\n")
	builder.WriteString(fmt.Sprintf("║ %-55s ║\n",
		fmt.Sprintf("Composite Error: %s", ce.Prefix)))
	builder.WriteString("╠═══════════════════════════════════════════════════════════╣\n")
	builder.WriteString(fmt.Sprintf("║ %-55s ║\n",
		fmt.Sprintf("Total Errors: %d", len(ce.Errors))))
	builder.WriteString(fmt.Sprintf("║ %-55s ║\n",
		fmt.Sprintf("Created: %s", ce.createdAt.Format("2006-01-02 15:04:05"))))
	builder.WriteString("╠═══════════════════════════════════════════════════════════╣\n")

	for i, wrappedErr := range ce.Errors {
		if i > 0 {
			builder.WriteString("╟───────────────────────────────────────────────────────────────────────╢\n")
		}

		severity := severityNames[wrappedErr.Metadata.Severity]
		builder.WriteString(fmt.Sprintf("║ [%5s] %-50s ║\n",
			severity,
			truncate(wrappedErr.error.Error(), 50)))

		if wrappedErr.Metadata.Code != "" {
			builder.WriteString(fmt.Sprintf("║       Code: %-48s ║\n",
				wrappedErr.Metadata.Code))
		}

		if wrappedErr.Metadata.Component != "" || wrappedErr.Metadata.Operation != "" {
			location := fmt.Sprintf("%s.%s",
				wrappedErr.Metadata.Component,
				wrappedErr.Metadata.Operation)
			builder.WriteString(fmt.Sprintf("║       Location: %-45s ║\n",
				truncate(location, 45)))
		}

		if len(wrappedErr.Metadata.Context) > 0 {
			builder.WriteString("║       Context:                                           ║\n")
			for k, v := range wrappedErr.Metadata.Context {
				builder.WriteString(fmt.Sprintf("║         • %-20s: %-30s ║\n",
					truncate(k, 20),
					truncate(fmt.Sprintf("%v", v), 30)))
			}
		}
	}

	builder.WriteString("╚═══════════════════════════════════════════════════════════╝\n")
	return builder.String()
}

// Helper function to truncate strings
func truncate(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length-3] + "..."
}

// Example usage
func ExampleCompositeError() {
	// Create a composite error
	composite := NewCompositeError("database operations",
		WithComponent("database"),
		WithOperation("batch_update"),
		WithContext("transaction_id", "txn-12345"),
	)

	// Add errors with metadata
	composite.Add(errors.New("connection timeout"),
		WithSeverity(SeverityHigh),
		WithErrorCode("DB_CONN_001"),
		WithContext("timeout_ms", 5000),
	)

	composite.Add(errors.New("invalid query syntax"),
		WithSeverity(SeverityMedium),
		WithErrorCode("DB_QUERY_002"),
		WithContext("query", "SELECT * FROM users WHERE"),
	)

	// Check if there are errors
	if composite.HasErrors() {
		fmt.Println(composite.Error())
		fmt.Println()

		// Get summary
		fmt.Println("Summary:", composite.Summary())

		// Filter by severity
		criticalErrors := composite.FilterBySeverity(SeverityHigh)
		if criticalErrors.HasErrors() {
			fmt.Println("Critical errors:", criticalErrors.ErrorCount())
		}

		// Output as JSON
		if jsonData, err := composite.JSON(); err == nil {
			fmt.Println("JSON:", string(jsonData))
		}

		// Pretty print
		fmt.Println(composite.PrettyPrint())
	}
}
