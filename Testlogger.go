package testutils

import (
	"bytes"
	"io"
	"log"
)

// TestLogger is a logger used for testing
type TestLogger struct {
	testID     string                 // optional: test identifier
	logLevel   int                    // optional: log level
	output     io.Writer              // actual output writer
	jsonOutput bool                   // whether to log in JSON format
	fields     map[string]interface{} // extra fields for structured logging
	callerSkip int                    // optional caller skip for stack traces
	intUtils   interface{}            // optional utils placeholder
	Buffer     *bytes.Buffer          // buffer to capture logs for testing
	logger     *log.Logger            // underlying Go logger
}

// NewTestLogger creates a new TestLogger
func NewTestLogger() *TestLogger {
	buf := new(bytes.Buffer)
	tl := &TestLogger{
		output: buf,
		Buffer: buf,
		fields: make(map[string]interface{}),
		logger: log.New(buf, "", log.LstdFlags),
	}
	return tl
}

// Info logs an info message
func (l *TestLogger) Info(msg string) {
	l.logger.Println("[INFO]", msg)
}

// Error logs an error message
func (l *TestLogger) Error(msg string) {
	l.logger.Println("[ERROR]", msg)
}

// WithField adds a key-value field for structured logging
func (l *TestLogger) WithField(key string, value interface{}) *TestLogger {
	newFields := make(map[string]interface{})
	for k, v := range l.fields {
		newFields[k] = v
	}
	newFields[key] = value

	return &TestLogger{
		testID:     l.testID,
		logLevel:   l.logLevel,
		output:     l.output,
		jsonOutput: l.jsonOutput,
		fields:     newFields,
		callerSkip: l.callerSkip,
		intUtils:   l.intUtils,
		Buffer:     l.Buffer,
		logger:     l.logger,
	}
}

// ResetBuffer clears the internal buffer (useful for testing)
func (l *TestLogger) ResetBuffer() {
	if l.Buffer != nil {
		l.Buffer.Reset()
	}
}
