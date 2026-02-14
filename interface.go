// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"io"
	"net/http"
	"time"
)

// TestApplication defines the interface for a test application fixture.
// It wraps an HTTP server and provides helpers for making requests and
// managing sessions.
type TestApplication interface {
	// GET starts building a GET request to the application.
	GET(path string) RequestBuilder

	// POST starts building a POST request with the given body.
	POST(path string, body io.Reader) RequestBuilder

	// PUT starts building a PUT request with the given body.
	PUT(path string, body io.Reader) RequestBuilder

	// DELETE starts building a DELETE request.
	DELETE(path string) RequestBuilder

	// CreateSession creates a new session with the given data and optional
	// expiration, and returns its session ID. The session is stored in the
	// application's session store.
	CreateSession(data map[string]interface{}, expiresIn time.Duration) string

	// ServerURL returns the base URL of the test server (e.g., "http://127.0.0.1:12345").
	ServerURL() string

	// Client returns the HTTP client used by the application (with cookies enabled).
	Client() *http.Client

	// Clock returns the clock used by the application (mockable).
	Clock() Clock

	// Cleanup returns the cleanup registry for the application.
	Cleanup() *Cleanup
}

// RequestBuilder defines the interface for building and executing HTTP requests
// against a TestApplication.
type RequestBuilder interface {
	// WithHeader adds a header to the request.
	WithHeader(key, value string) RequestBuilder

	// WithJSONBody sets the request body to the JSON encoding of v and sets
	// Content-Type to application/json.
	WithJSONBody(v interface{}) RequestBuilder

	// WithCookie adds a cookie to the request.
	WithCookie(cookie *http.Cookie) RequestBuilder

	// WithSessionCookie adds a session cookie with the given session ID,
	// using the default session cookie name ("session_id").
	WithSessionCookie(sessionID string) RequestBuilder

	// Do executes the request and returns the response.
	Do() *http.Response
}

// HTTPClient is an interface for making HTTP requests, matching the Do method
// of http.Client. Useful for mocking HTTP calls in tests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// JSONCodec defines the interface for JSON marshaling/unmarshaling.
type JSONCodec interface {
	Marshal(v interface{}) ([]byte, error)
	Unmarshal(data []byte, v interface{}) error
}

// Logger defines a minimal logging interface, compatible with testing.TB.
type Logger interface {
	Log(args ...interface{})
	Logf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
}