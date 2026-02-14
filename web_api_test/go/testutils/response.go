// Package response provides utilities for writing HTTP responses.
// It offers functions for JSON, HTML, errors, redirects, and file serving,
// reducing boilerplate in HTTP handlers.
package testutils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
)

// ----------------------------------------------------------------------
// JSON responses
// ----------------------------------------------------------------------

// JSON writes the data as JSON with the given status code.
// It sets the Content-Type header to application/json.
func JSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// If encoding fails, we can't write a JSON error because headers already sent.
		// Just log it? In a real app, use a logger. Here we fallback to plain text.
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// JSONBytes writes a raw JSON byte slice with the given status code.
// It does not validate that the bytes are valid JSON.
func JSONBytes(w http.ResponseWriter, status int, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(data)
}

// ----------------------------------------------------------------------
// Error responses
// ----------------------------------------------------------------------

// Error sends a JSON error response with the given status code and message.
// The response body is {"error": message}.
func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, map[string]string{"error": message})
}

// Errorf is like Error but with formatted message.
func Errorf(w http.ResponseWriter, status int, format string, args ...interface{}) {
	Error(w, status, fmt.Sprintf(format, args...))
}

// ----------------------------------------------------------------------
// HTML responses
// ----------------------------------------------------------------------

// HTML writes a string as HTML with the given status code.
// It sets Content-Type to text/html.
func HTML(w http.ResponseWriter, status int, html string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	w.Write([]byte(html))
}

// HTMLBytes writes a byte slice as HTML.
func HTMLBytes(w http.ResponseWriter, status int, data []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	w.Write(data)
}

// ----------------------------------------------------------------------
// Text responses
// ----------------------------------------------------------------------

// Text writes a plain text response with the given status code.
func Text(w http.ResponseWriter, status int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	w.Write([]byte(text))
}

// TextBytes writes a byte slice as plain text.
func TextBytes(w http.ResponseWriter, status int, data []byte) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	w.Write(data)
}

// ----------------------------------------------------------------------
// Redirects
// ----------------------------------------------------------------------

// Redirect sends an HTTP redirect to the specified URL with the given status code.
// The status code should be one of 301, 302, 303, 307, or 308.
func Redirect(w http.ResponseWriter, r *http.Request, url string, code int) {
	http.Redirect(w, r, url, code)
}

// RedirectPermanent sends a 301 Moved Permanently redirect.
func RedirectPermanent(w http.ResponseWriter, r *http.Request, url string) {
	http.Redirect(w, r, url, http.StatusMovedPermanently)
}

// RedirectTemporary sends a 302 Found redirect.
func RedirectTemporary(w http.ResponseWriter, r *http.Request, url string) {
	http.Redirect(w, r, url, http.StatusFound)
}

// ----------------------------------------------------------------------
// No Content
// ----------------------------------------------------------------------

// NoContent sends a 204 No Content response.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// ----------------------------------------------------------------------
// File serving
// ----------------------------------------------------------------------

// File serves a single file. It sets the proper Content-Type based on extension.
// It uses http.ServeFile, which handles range requests and caches.
func File(w http.ResponseWriter, r *http.Request, filePath string) {
	http.ServeFile(w, r, filePath)
}

// Attachment serves a file as an attachment (forcing download).
// The filename parameter is the name the browser should save the file as.
func Attachment(w http.ResponseWriter, r *http.Request, filePath, filename string) {
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	http.ServeFile(w, r, filePath)
}

// ----------------------------------------------------------------------
// Status code capture (for middleware)
// ----------------------------------------------------------------------

// StatusRecorder is an http.ResponseWriter that records the status code.
// It can be used in middleware to capture the response status for logging.
type StatusRecorder struct {
	http.ResponseWriter
	Status int
}

// WriteHeader captures the status code and calls the underlying WriteHeader.
func (r *StatusRecorder) WriteHeader(status int) {
	r.Status = status
	r.ResponseWriter.WriteHeader(status)
}

// Write captures the status code if not already set (defaults to 200) and writes data.
func (r *StatusRecorder) Write(b []byte) (int, error) {
	if r.Status == 0 {
		r.Status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func exampleHandler(w http.ResponseWriter, r *http.Request) {
//     // JSON response
//     response.JSON(w, http.StatusOK, map[string]string{"message": "ok"})
//
//     // Error response
//     response.Error(w, http.StatusBadRequest, "invalid input")
//
//     // HTML response
//     response.HTML(w, http.StatusOK, "<h1>Hello</h1>")
//
//     // Redirect
//     response.RedirectTemporary(w, r, "/new-location")
//
//     // No content
//     response.NoContent(w)
// }