// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

// FullTest is a complete test fixture that includes an HTTP server,
// session management, a controllable clock, request recording, and assertions.
type FullTest struct {
	*TestApplication
	Clock        *MockClock
	SessionStore *SessionStore
	Recorder     *Recorder
	T            *testing.T
	cleanup      *Cleanup
}

// NewFullTest creates a new FullTest fixture with sensible defaults.
// It starts an HTTP server, initializes a mock clock at the current time,
// creates a session store, and sets up a request recorder.
func NewFullTest(t *testing.T, opts ...TestApplicationOption) *FullTest {
	t.Helper()

	mockClock := NewMockClock(time.Now())
	sessionStore := NewSessionStore()

	allOpts := append([]TestApplicationOption{
		WithClock(mockClock),
		WithSessionStore(sessionStore),
	}, opts...)

	app := NewTestApplication(t, allOpts...)
	recorder := NewRecorder(app.Client)

	ft := &FullTest{
		TestApplication: app,
		Clock:           mockClock,
		SessionStore:    sessionStore,
		Recorder:        recorder,
		T:               t,
		cleanup:         &Cleanup{},
	}
	ft.cleanup.Defer(t)
	return ft
}

// Request starts building a new HTTP request that will be recorded.
func (ft *FullTest) Request(method, path string) *FullRequestBuilder {
	return &FullRequestBuilder{
		ft:     ft,
		method: method,
		path:   path,
		header: make(http.Header),
	}
}

// GET is a shortcut for Request("GET", path).
func (ft *FullTest) GET(path string) *FullRequestBuilder {
	return ft.Request("GET", path)
}

// POST is a shortcut for Request("POST", path) with optional JSON body.
func (ft *FullTest) POST(path string, body ...interface{}) *FullRequestBuilder {
	rb := ft.Request("POST", path)
	if len(body) > 0 {
		rb = rb.WithJSONBody(body[0])
	}
	return rb
}

// PUT is a shortcut for Request("PUT", path) with optional JSON body.
func (ft *FullTest) PUT(path string, body ...interface{}) *FullRequestBuilder {
	rb := ft.Request("PUT", path)
	if len(body) > 0 {
		rb = rb.WithJSONBody(body[0])
	}
	return rb
}

// DELETE is a shortcut for Request("DELETE", path).
func (ft *FullTest) DELETE(path string) *FullRequestBuilder {
	return ft.Request("DELETE", path)
}

// AdvanceTime moves the mock clock forward by d and triggers any timers/tickers.
func (ft *FullTest) AdvanceTime(d time.Duration) {
	ft.Clock.Advance(d)
}

// CreateUserSession creates a new session with the given user ID and returns the session ID.
// This is a convenience for tests that need an authenticated user.
func (ft *FullTest) CreateUserSession(userID interface{}, expiresIn time.Duration) string {
	sess, err := ft.SessionStore.NewSession(expiresIn)
	if err != nil {
		ft.T.Fatal(err)
	}
	sess.Data["user_id"] = userID
	ft.SessionStore.Set(sess)
	return sess.ID
}

// ------------------------------------------------------------------------
// Assertion helpers
// ------------------------------------------------------------------------

// AssertNoError checks that err is nil and logs a fatal error if not.
func (ft *FullTest) AssertNoError(err error, msgAndArgs ...interface{}) {
	ft.T.Helper()
	NoError(ft.T, err, msgAndArgs...)
}

// AssertEqual checks that expected and actual are deeply equal.
func (ft *FullTest) AssertEqual(expected, actual interface{}, msgAndArgs ...interface{}) {
	ft.T.Helper()
	Equal(ft.T, expected, actual, msgAndArgs...)
}

// AssertStatus checks that the response has the expected HTTP status code.
func (ft *FullTest) AssertStatus(resp *http.Response, status int) {
	ft.T.Helper()
	if resp.StatusCode != status {
		ft.T.Fatalf("expected status %d, got %d", status, resp.StatusCode)
	}
}

// ------------------------------------------------------------------------
// Request builder with recording
// ------------------------------------------------------------------------

// FullRequestBuilder extends RequestBuilder to record interactions and provide assertions.
type FullRequestBuilder struct {
	ft      *FullTest
	method  string
	path    string
	body    interface{}
	header  http.Header
	cookies []*http.Cookie
}

// WithHeader adds a header to the request.
func (rb *FullRequestBuilder) WithHeader(key, value string) *FullRequestBuilder {
	rb.header.Set(key, value)
	return rb
}

// WithCookie adds a cookie to the request.
func (rb *FullRequestBuilder) WithCookie(cookie *http.Cookie) *FullRequestBuilder {
	rb.cookies = append(rb.cookies, cookie)
	return rb
}

// WithSessionCookie adds a session cookie with the given session ID.
func (rb *FullRequestBuilder) WithSessionCookie(sessionID string) *FullRequestBuilder {
	return rb.WithCookie(&http.Cookie{
		Name:  "session_id",
		Value: sessionID,
		Path:  "/",
	})
}

// WithJSONBody sets the request body to the JSON encoding of v and sets
// Content-Type to application/json.
func (rb *FullRequestBuilder) WithJSONBody(v interface{}) *FullRequestBuilder {
	rb.body = v
	rb.header.Set("Content-Type", "application/json")
	return rb
}

// Do executes the request through the recorder and returns the response.
func (rb *FullRequestBuilder) Do() *http.Response {
	rb.ft.T.Helper()

	// Build the request.
	var bodyReader io.Reader
	if rb.body != nil {
		data, err := json.Marshal(rb.body)
		if err != nil {
			rb.ft.T.Fatal(err)
		}
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(rb.method, rb.ft.Server.URL+rb.path, bodyReader)
	if err != nil {
		rb.ft.T.Fatal(err)
	}
	for key, values := range rb.header {
		for _, v := range values {
			req.Header.Add(key, v)
		}
	}
	for _, cookie := range rb.cookies {
		req.AddCookie(cookie)
	}

	resp, err := rb.ft.Recorder.Do(req)
	if err != nil {
		rb.ft.T.Fatal(err)
	}
	return resp
}

// DoAndExpect executes the request and asserts that the response has the given status.
func (rb *FullRequestBuilder) DoAndExpect(status int) *http.Response {
	resp := rb.Do()
	rb.ft.AssertStatus(resp, status)
	return resp
}