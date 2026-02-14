// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestApplication is a test helper that wraps an HTTP server with common
// testing utilities: session store, clock, cleanup, and request helpers.
type TestApplication struct {
	// Server is the underlying test server.
	Server *httptest.Server
	// Client is an HTTP client configured to talk to the server (cookies enabled).
	Client *http.Client
	// Mux is the HTTP handler used by the server; you can register routes on it.
	Mux *http.ServeMux
	// SessionStore is the session store used by the application (if enabled).
	SessionStore *SessionStore
	// Clock is the clock used by the application (mockable).
	Clock Clock
	// Cleanup holds cleanup functions to run when the application stops.
	Cleanup *Cleanup
	// testing.TB for logging and failing tests.
	t testing.TB
}

// TestApplicationOption configures a TestApplication.
type TestApplicationOption func(*TestApplication)

// WithSessionStore enables session support using the provided store.
// If store is nil, a new SessionStore is created.
func WithSessionStore(store *SessionStore) TestApplicationOption {
	return func(app *TestApplication) {
		if store == nil {
			store = NewSessionStore()
		}
		app.SessionStore = store
		// Add session middleware to the mux.
		originalMux := app.Mux
		app.Mux = http.NewServeMux()
		// Wrap the original mux with session middleware.
		app.Mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Attach session to request context? We'll just add a helper.
			// For simplicity, we'll leave session handling to the test.
			// In practice, the application code would use the session store.
			originalMux.ServeHTTP(w, r)
		})
	}
}

// WithClock sets a custom clock for the application.
func WithClock(clock Clock) TestApplicationOption {
	return func(app *TestApplication) {
		app.Clock = clock
	}
}

// NewTestApplication creates a new test application with the given options.
// It automatically starts the server and registers a cleanup to close it.
func NewTestApplication(t testing.TB, opts ...TestApplicationOption) *TestApplication {
	t.Helper()

	app := &TestApplication{
		Mux:     http.NewServeMux(),
		Clock:   RealClock{},
		Cleanup: &Cleanup{},
		t:       t,
	}

	// Apply options.
	for _, opt := range opts {
		opt(app)
	}

	// Create the test server.
	app.Server = httptest.NewServer(app.Mux)
	app.Client = app.Server.Client()
	app.Client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse // don't follow redirects automatically
	}

	// Register cleanup to close the server.
	app.Cleanup.Add(func() {
		app.Server.Close()
	})
	app.Cleanup.Defer(t)

	return app
}

// ------------------------------------------------------------------------
// Request helpers
// ------------------------------------------------------------------------

// RequestBuilder helps build HTTP requests with common test utilities.
type RequestBuilder struct {
	app    *TestApplication
	method string
	path   string
	body   io.Reader
	header http.Header
	cookies []*http.Cookie
}

// GET starts building a GET request.
func (app *TestApplication) GET(path string) *RequestBuilder {
	return &RequestBuilder{
		app:    app,
		method: "GET",
		path:   path,
		header: make(http.Header),
	}
}

// POST starts building a POST request.
func (app *TestApplication) POST(path string, body io.Reader) *RequestBuilder {
	return &RequestBuilder{
		app:    app,
		method: "POST",
		path:   path,
		body:   body,
		header: make(http.Header),
	}
}

// PUT starts building a PUT request.
func (app *TestApplication) PUT(path string, body io.Reader) *RequestBuilder {
	return &RequestBuilder{
		app:    app,
		method: "PUT",
		path:   path,
		body:   body,
		header: make(http.Header),
	}
}

// DELETE starts building a DELETE request.
func (app *TestApplication) DELETE(path string) *RequestBuilder {
	return &RequestBuilder{
		app:    app,
		method: "DELETE",
		path:   path,
		header: make(http.Header),
	}
}

// WithHeader adds a header to the request.
func (b *RequestBuilder) WithHeader(key, value string) *RequestBuilder {
	b.header.Set(key, value)
	return b
}

// WithJSONBody sets the request body to the JSON encoding of v and sets
// Content-Type to application/json.
func (b *RequestBuilder) WithJSONBody(v interface{}) *RequestBuilder {
	data, err := json.Marshal(v)
	if err != nil {
		b.app.t.Fatalf("WithJSONBody: failed to marshal: %v", err)
	}
	b.body = bytes.NewReader(data)
	b.header.Set("Content-Type", "application/json")
	return b
}

// WithCookie adds a cookie to the request.
func (b *RequestBuilder) WithCookie(cookie *http.Cookie) *RequestBuilder {
	b.cookies = append(b.cookies, cookie)
	return b
}

// WithSessionCookie adds a session cookie with the given session ID.
func (b *RequestBuilder) WithSessionCookie(sessionID string) *RequestBuilder {
	return b.WithCookie(&http.Cookie{
		Name:  "session_id", // default session cookie name
		Value: sessionID,
		Path:  "/",
	})
}

// Do executes the request and returns the response.
func (b *RequestBuilder) Do() *http.Response {
	req, err := http.NewRequest(b.method, b.app.Server.URL+b.path, b.body)
	if err != nil {
		b.app.t.Fatalf("Do: failed to create request: %v", err)
	}
	for key, values := range b.header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	for _, cookie := range b.cookies {
		req.AddCookie(cookie)
	}
	resp, err := b.app.Client.Do(req)
	if err != nil {
		b.app.t.Fatalf("Do: request failed: %v", err)
	}
	return resp
}

// ------------------------------------------------------------------------
// Response helpers
// ------------------------------------------------------------------------

// DecodeJSON reads the response body and decodes it into v.
func DecodeJSON(t testing.TB, resp *http.Response, v interface{}) {
	t.Helper()
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("DecodeJSON: failed to read body: %v", err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("DecodeJSON: failed to unmarshal %q: %v", string(data), err)
	}
}

// RequireStatus fails the test if the response status code does not match.
func RequireStatus(t testing.TB, resp *http.Response, status int) {
	t.Helper()
	if resp.StatusCode != status {
		t.Fatalf("expected status %d, got %d", status, resp.StatusCode)
	}
}

// ------------------------------------------------------------------------
// Session helpers
// ------------------------------------------------------------------------

// CreateSession creates a new session with the given data and returns its ID.
// It stores the session in the application's session store.
func (app *TestApplication) CreateSession(data map[string]interface{}, expiresIn time.Duration) string {
	if app.SessionStore == nil {
		app.t.Fatal("CreateSession: session store not enabled (use WithSessionStore)")
	}
	sess, err := app.SessionStore.NewSession(expiresIn)
	if err != nil {
		app.t.Fatalf("CreateSession: failed to create session: %v", err)
	}
	sess.Data = data
	app.SessionStore.Set(sess)
	return sess.ID
}