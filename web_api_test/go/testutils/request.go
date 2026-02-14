// Package request provides utilities for extracting data from HTTP requests.
// It includes helpers for JSON decoding, query parameters, form values,
// path parameters, and file uploads.
package testutils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// ----------------------------------------------------------------------
// JSON decoding
// ----------------------------------------------------------------------

// DecodeJSON reads the request body and decodes it as JSON into the provided
// value. It limits the body size to 1MB by default. Use DecodeJSONWithLimit
// to specify a custom limit.
func DecodeJSON(r *http.Request, v interface{}) error {
	return DecodeJSONWithLimit(r, v, 1024*1024) // 1 MB
}

// DecodeJSONWithLimit reads the request body with a maximum size limit and
// decodes it as JSON.
func DecodeJSONWithLimit(r *http.Request, v interface{}, maxBytes int64) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	// Check for extra data after JSON.
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

// ----------------------------------------------------------------------
// Query parameters
// ----------------------------------------------------------------------

// QueryInt returns the integer value of the query parameter key.
// If the parameter is missing or cannot be parsed, it returns the default value.
func QueryInt(r *http.Request, key string, defaultValue int) int {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultValue
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}
	return i
}

// QueryInt64 returns the int64 value of the query parameter key.
func QueryInt64(r *http.Request, key string, defaultValue int64) int64 {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultValue
	}
	i, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return defaultValue
	}
	return i
}

// QueryFloat returns the float64 value of the query parameter key.
func QueryFloat(r *http.Request, key string, defaultValue float64) float64 {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultValue
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return defaultValue
	}
	return f
}

// QueryBool returns the boolean value of the query parameter key.
// It accepts "true", "false", "1", "0", "t", "f" (case‑insensitive).
func QueryBool(r *http.Request, key string, defaultValue bool) bool {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultValue
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return defaultValue
	}
	return b
}

// QueryStrings returns a slice of values for the given query parameter key.
func QueryStrings(r *http.Request, key string) []string {
	return r.URL.Query()[key]
}

// ----------------------------------------------------------------------
// Path parameters (Go 1.22+)
// ----------------------------------------------------------------------

// PathValue returns the path parameter value for the given key.
// It uses r.PathValue, which is available in Go 1.22+.
func PathValue(r *http.Request, key string) string {
	return r.PathValue(key)
}

// PathInt returns the path parameter as an integer.
func PathInt(r *http.Request, key string, defaultValue int) int {
	val := PathValue(r, key)
	if val == "" {
		return defaultValue
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}
	return i
}

// PathInt64 returns the path parameter as an int64.
func PathInt64(r *http.Request, key string, defaultValue int64) int64 {
	val := PathValue(r, key)
	if val == "" {
		return defaultValue
	}
	i, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return defaultValue
	}
	return i
}

// ----------------------------------------------------------------------
// Form values
// ----------------------------------------------------------------------

// ParseForm is a convenience wrapper that calls r.ParseForm and returns an error if any.
func ParseForm(r *http.Request) error {
	return r.ParseForm()
}

// FormValue returns the first value for the named form field.
func FormValue(r *http.Request, key string) string {
	return r.FormValue(key)
}

// FormValues returns all values for the named form field.
func FormValues(r *http.Request, key string) []string {
	return r.Form[key]
}

// FormInt returns the form field as an integer.
func FormInt(r *http.Request, key string, defaultValue int) int {
	val := r.FormValue(key)
	if val == "" {
		return defaultValue
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}
	return i
}

// FormBool returns the form field as a boolean.
func FormBool(r *http.Request, key string, defaultValue bool) bool {
	val := r.FormValue(key)
	if val == "" {
		return defaultValue
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return defaultValue
	}
	return b
}

// ----------------------------------------------------------------------
// File uploads
// ----------------------------------------------------------------------

// FileInfo holds information about an uploaded file.
type FileInfo struct {
	File     io.ReadCloser
	Header   *multipart.FileHeader
	Size     int64
}

// FormFile retrieves the uploaded file for the given form field.
// It returns the file, its header, the file size, and an error.
func FormFile(r *http.Request, key string) (*FileInfo, error) {
	file, header, err := r.FormFile(key)
	if err != nil {
		return nil, err
	}
	return &FileInfo{
		File:   file,
		Header: header,
		Size:   header.Size,
	}, nil
}

// ----------------------------------------------------------------------
// Client IP
// ----------------------------------------------------------------------

// ClientIP extracts the client's IP address from the request, checking
// common headers like X-Forwarded-For and X-Real-IP before falling back
// to RemoteAddr. It returns the IP without port.
func ClientIP(r *http.Request) string {
	// Check X-Forwarded-For
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ips := strings.Split(fwd, ",")
		return strings.TrimSpace(ips[0])
	}
	// Check X-Real-IP
	if real := r.Header.Get("X-Real-IP"); real != "" {
		return strings.TrimSpace(real)
	}
	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// ----------------------------------------------------------------------
// Request ID
// ----------------------------------------------------------------------

// RequestID extracts the request ID from the context (set by middleware)
// or from the X-Request-ID header. If neither is present, it returns an empty string.
func RequestID(r *http.Request) string {
	// Try context first (if middleware stored it).
	if id, ok := r.Context().Value(requestIDKey).(string); ok && id != "" {
		return id
	}
	// Fall back to header.
	return r.Header.Get("X-Request-ID")
}

// requestIDKey is a private context key.
type requestIDKey struct{}

// WithRequestID returns a new context with the request ID stored.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func handler(w http.ResponseWriter, r *http.Request) {
//     // JSON
//     var data map[string]interface{}
//     if err := request.DecodeJSON(r, &data); err != nil {
//         http.Error(w, err.Error(), http.StatusBadRequest)
//         return
//     }
//
//     // Query params
//     page := request.QueryInt(r, "page", 1)
//
//     // Path param (Go 1.22)
//     id := request.PathInt(r, "id", 0)
//
//     // Form value
//     name := request.FormValue(r, "name")
//
//     // Client IP
//     ip := request.ClientIP(r)
// }