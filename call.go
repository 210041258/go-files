// Package call provides a robust HTTP client with retries, JSON encoding,
// context support, and pluggable logging. It is ideal for building API clients
// and service‑to‑service communication.
//
// Example:
//
//	client := call.NewClient().
//		WithBaseURL("https://api.example.com").
//		WithHeader("Authorization", "Bearer token")
//
//	var resp call.Response
//	err := client.Get(ctx, "/users/123", &resp)
package testutils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Defaults
const (
	DefaultTimeout      = 30 * time.Second
	DefaultRetryWaitMin = 100 * time.Millisecond
	DefaultRetryWaitMax = 2 * time.Second
	DefaultRetryMax     = 3
)

// Client is an HTTP client with configurable base URL, headers, and retry policy.
type Client struct {
	httpClient   *http.Client
	baseURL      string
	headers      http.Header
	retryMax     int
	retryWaitMin time.Duration
	retryWaitMax time.Duration
	logger       func(level int, format string, args ...interface{})
}

// NewClient creates a new Client with default settings.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		headers:      make(http.Header),
		retryMax:     DefaultRetryMax,
		retryWaitMin: DefaultRetryWaitMin,
		retryWaitMax: DefaultRetryWaitMax,
		logger: func(level int, format string, args ...interface{}) {
			verbose.Printf(level, format, args...) // if verbose package is used
		},
	}
}

// WithHTTPClient replaces the underlying http.Client.
func (c *Client) WithHTTPClient(httpClient *http.Client) *Client {
	c.httpClient = httpClient
	return c
}

// WithBaseURL sets the base URL for all requests.
// The base URL should have a scheme (http:// or https://).
func (c *Client) WithBaseURL(baseURL string) *Client {
	c.baseURL = strings.TrimRight(baseURL, "/")
	return c
}

// WithHeader adds a header to all requests.
func (c *Client) WithHeader(key, value string) *Client {
	c.headers.Set(key, value)
	return c
}

// WithHeaders adds multiple headers.
func (c *Client) WithHeaders(h http.Header) *Client {
	for k, v := range h {
		for _, vv := range v {
			c.headers.Add(k, vv)
		}
	}
	return c
}

// WithRetry sets the retry policy.
func (c *Client) WithRetry(max int, waitMin, waitMax time.Duration) *Client {
	c.retryMax = max
	c.retryWaitMin = waitMin
	c.retryWaitMax = waitMax
	return c
}

// WithLogger sets a custom logger function.
// If nil, logging is disabled.
func (c *Client) WithLogger(logger func(level int, format string, args ...interface{})) *Client {
	c.logger = logger
	return c
}

// --------------------------------------------------------------------
// Core request execution
// --------------------------------------------------------------------

// Do sends an HTTP request and returns the response.
// It handles JSON marshaling/unmarshaling, retries, and context cancellation.
func (c *Client) Do(ctx context.Context, method, path string, body, result interface{}) (*http.Response, error) {
	// Build URL
	urlStr := path
	if c.baseURL != "" {
		if strings.HasPrefix(path, "/") {
			urlStr = c.baseURL + path
		} else {
			urlStr = c.baseURL + "/" + path
		}
	}

	// Prepare request body
	var bodyReader io.Reader
	if body != nil {
		switch v := body.(type) {
		case []byte:
			bodyReader = bytes.NewReader(v)
		case string:
			bodyReader = strings.NewReader(v)
		case io.Reader:
			bodyReader = v
		default:
			// Assume JSON
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("call: marshal request body: %w", err)
			}
			bodyReader = bytes.NewReader(jsonBytes)
		}
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, urlStr, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("call: create request: %w", err)
	}

	// Apply client headers
	for k, v := range c.headers {
		req.Header[k] = v
	}

	// Set default content type for JSON if not set and body is JSON
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Execute with retries
	var resp *http.Response
	var doErr error
	for attempt := 0; attempt <= c.retryMax; attempt++ {
		// If context is already done, abort
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Clone request body for each retry if it's rewindable
		if bodyReader != nil {
			if r, ok := bodyReader.(io.Seeker); ok {
				r.Seek(0, io.SeekStart)
			}
		}

		resp, doErr = c.httpClient.Do(req)
		if doErr == nil {
			// Success – break retry loop
			break
		}

		// Log retryable error
		if c.logger != nil {
			c.logger(2, "call: request failed (attempt %d/%d): %v", attempt+1, c.retryMax+1, doErr)
		}

		// If this was the last attempt, break and return error
		if attempt == c.retryMax {
			break
		}

		// Wait before retrying with exponential backoff
		wait := c.backoff(attempt)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if doErr != nil {
		return nil, fmt.Errorf("call: request failed after %d attempts: %w", c.retryMax+1, doErr)
	}
	defer resp.Body.Close()

	// Decode response if result is provided
	if result != nil {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("call: read response body: %w", err)
		}

		// If result is a *[]byte or *string, assign directly
		switch v := result.(type) {
		case *[]byte:
			*v = bodyBytes
		case *string:
			*v = string(bodyBytes)
		default:
			// Assume JSON
			if err := json.Unmarshal(bodyBytes, v); err != nil {
				return nil, fmt.Errorf("call: unmarshal response: %w (body: %s)", err, truncate(bodyBytes, 100))
			}
		}
	}

	return resp, nil
}

// backoff calculates the wait duration for a given retry attempt.
func (c *Client) backoff(attempt int) time.Duration {
	// Exponential backoff: 2^attempt * waitMin, capped at waitMax
	wait := c.retryWaitMin * (1 << attempt)
	if wait > c.retryWaitMax {
		wait = c.retryWaitMax
	}
	return wait
}

// truncate returns a truncated string for error messages.
func truncate(b []byte, limit int) string {
	if len(b) <= limit {
		return string(b)
	}
	return string(b[:limit]) + "..."
}

// --------------------------------------------------------------------
// Convenience methods
// --------------------------------------------------------------------

// Get sends a GET request.
func (c *Client) Get(ctx context.Context, path string, result interface{}) (*http.Response, error) {
	return c.Do(ctx, http.MethodGet, path, nil, result)
}

// Post sends a POST request.
func (c *Client) Post(ctx context.Context, path string, body, result interface{}) (*http.Response, error) {
	return c.Do(ctx, http.MethodPost, path, body, result)
}

// Put sends a PUT request.
func (c *Client) Put(ctx context.Context, path string, body, result interface{}) (*http.Response, error) {
	return c.Do(ctx, http.MethodPut, path, body, result)
}

// Patch sends a PATCH request.
func (c *Client) Patch(ctx context.Context, path string, body, result interface{}) (*http.Response, error) {
	return c.Do(ctx, http.MethodPatch, path, body, result)
}

// Delete sends a DELETE request.
func (c *Client) Delete(ctx context.Context, path string, result interface{}) (*http.Response, error) {
	return c.Do(ctx, http.MethodDelete, path, nil, result)
}

// --------------------------------------------------------------------
// Response helper
// --------------------------------------------------------------------

// IsSuccess checks if the HTTP status code is 2xx.
func IsSuccess(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}

// --------------------------------------------------------------------
// Mocking utilities for tests
// --------------------------------------------------------------------

// MockRoundTripper is a http.RoundTripper that returns preconfigured responses.
// Use it to mock HTTP calls in tests.
type MockRoundTripper struct {
	responses map[string]*http.Response
	err       error
}

// NewMockRoundTripper creates a MockRoundTripper.
func NewMockRoundTripper() *MockRoundTripper {
	return &MockRoundTripper{
		responses: make(map[string]*http.Response),
	}
}

// RegisterResponse sets the response for a given method+URL.
func (m *MockRoundTripper) RegisterResponse(method, url string, statusCode int, body []byte) {
	key := method + ":" + url
	m.responses[key] = &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     make(http.Header),
	}
}

// RoundTrip implements http.RoundTripper.
func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	key := req.Method + ":" + req.URL.String()
	if resp, ok := m.responses[key]; ok {
		return resp, nil
	}
	// Default 404
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(bytes.NewReader([]byte("not found"))),
		Header:     make(http.Header),
	}, nil
}

// WithError forces the round tripper to return an error.
func (m *MockRoundTripper) WithError(err error) {
	m.err = err
}
