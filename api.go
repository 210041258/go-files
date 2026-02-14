// Package api provides a lightweight, composable framework for building HTTP
// servers with context‑aware handlers, middleware chaining, structured responses,
// and graceful shutdown. It is designed to be zero‑dependency and integrates
// with your other packages (verbose, safe, value, unique, signalutil, call).
//
// Example:
//
//	app := api.NewApp()
//	app.Get("/health", func(ctx context.Context, req *api.Request) (*api.Response, error) {
//		return api.JSON(http.StatusOK, map[string]string{"status": "ok"})
//	})
//	app.Group("/api/v1", func(g *api.Group) {
//		g.Use(api.Logger())
//		g.Get("/users", listUsers)
//	})
//	app.Run(":8080")
package testutils

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"yourmodule/safe"
	"yourmodule/signalutil"
	"yourmodule/unique"
	"yourmodule/value"
	"yourmodule/verbose"
)

// --------------------------------------------------------------------
// Core types
// --------------------------------------------------------------------

// Request wraps an http.Request with additional context and utilities.
type Request struct {
	*http.Request
	PathParams map[string]string
	RequestID  string
}

// Response is the structured return value of a handler.
type Response struct {
	Status  int
	Headers http.Header
	Body    any    // will be encoded according to Content-Type
	RawBody []byte // if set, Body is ignored and raw bytes are sent
}

// Handler is the primary function signature for endpoint logic.
// It receives a context (which may be cancelled on client disconnect)
// and a parsed request, and returns a response or an error.
type Handler func(ctx context.Context, req *Request) (*Response, error)

// Middleware wraps a Handler, returning a new Handler that may perform
// pre‑processing, post‑processing, or short‑circuiting.
type Middleware func(Handler) Handler

// App is the top‑level server container.
type App struct {
	mux         *http.ServeMux
	middlewares []Middleware
	groups      []*Group
	server      *http.Server
	addr        string
	mu          sync.RWMutex
}

// Group represents a route prefix with its own middleware chain.
type Group struct {
	prefix      string
	parent      *App
	middlewares []Middleware
}

// --------------------------------------------------------------------
// App constructor
// --------------------------------------------------------------------

// NewApp creates a new App with default settings.
func NewApp() *App {
	return &App{
		mux:    http.NewServeMux(),
		server: &http.Server{},
	}
}

// Use appends a global middleware that applies to all routes.
func (a *App) Use(mw Middleware) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.middlewares = append(a.middlewares, mw)
}

// Group creates a new route group with the given prefix.
// All routes added inside the group will have the prefix and inherit
// the app's global middlewares plus any group‑specific ones.
func (a *App) Group(prefix string, fn func(*Group)) *Group {
	g := &Group{
		prefix:      prefix,
		parent:      a,
		middlewares: make([]Middleware, len(a.middlewares)),
	}
	copy(g.middlewares, a.middlewares)
	a.mu.Lock()
	a.groups = append(a.groups, g)
	a.mu.Unlock()
	fn(g)
	return g
}

// Run starts the HTTP server and blocks until shutdown.
// It listens on addr (e.g., ":8080") and handles SIGINT/SIGTERM gracefully.
func (a *App) Run(addr string) error {
	a.addr = addr
	a.server.Addr = addr
	a.server.Handler = a

	// Graceful shutdown on interrupt
	ctx, stop := signalutil.NotifyContext(context.Background(), signalutil.CommonSignals()...)
	defer stop()

	verbose.Println(1, "API server listening on", addr)
	err := a.server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("api: server error: %w", err)
	}

	<-ctx.Done()
	verbose.Println(1, "Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return a.server.Shutdown(shutdownCtx)
}

// ServeHTTP implements http.Handler, routing requests to registered handlers.
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Generate request ID
	reqID, _ := unique.NewNanoID(12)
	r = r.WithContext(context.WithValue(r.Context(), requestIDKey, reqID))

	// Wrap http.ResponseWriter with optional status capture
	rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

	// Find and execute handler
	handler, params := a.lookup(r.Method, r.URL.Path)
	if handler == nil {
		http.NotFound(rw, r)
		return
	}

	req := &Request{
		Request:    r,
		PathParams: params,
		RequestID:  reqID,
	}

	ctx := req.Context()
	ctx = context.WithValue(ctx, requestIDKey, reqID)
	req.Request = req.WithContext(ctx)

	// Execute handler
	resp, err := handler(ctx, req)
	if err != nil {
		// Convert error to response using default error handler
		resp = a.errorHandler(err)
	}

	// Write response
	a.writeResponse(rw, r, resp)
}

// --------------------------------------------------------------------
// Route registration
// --------------------------------------------------------------------

// Handle registers a handler for the given pattern and method.
func (a *App) Handle(method, pattern string, handler Handler) {
	fullPattern := pattern
	// Apply all middlewares (global + route‑specific) to the handler
	final := a.applyMiddlewares(handler, a.middlewares)
	a.mux.HandleFunc(fullPattern, func(w http.ResponseWriter, r *http.Request) {
		// This is a bridge: we already route via a.ServeHTTP,
		// so this should rarely be called directly.
		a.ServeHTTP(w, r)
	})
	// Actually, we need to register the pattern with method matching.
	// We'll use a custom router implementation below.
	// For now, we store routes in a map for lookup.
	a.registerRoute(method, pattern, final)
}

// Get is a shortcut for Handle with http.MethodGet.
func (a *App) Get(pattern string, handler Handler) { a.Handle(http.MethodGet, pattern, handler) }

// Post is a shortcut for Handle with http.MethodPost.
func (a *App) Post(pattern string, handler Handler) { a.Handle(http.MethodPost, pattern, handler) }

// Put is a shortcut for Handle with http.MethodPut.
func (a *App) Put(pattern string, handler Handler) { a.Handle(http.MethodPut, pattern, handler) }

// Patch is a shortcut for Handle with http.MethodPatch.
func (a *App) Patch(pattern string, handler Handler) { a.Handle(http.MethodPatch, pattern, handler) }

// Delete is a shortcut for Handle with http.MethodDelete.
func (a *App) Delete(pattern string, handler Handler) { a.Handle(http.MethodDelete, pattern, handler) }

// --------------------------------------------------------------------
// Group methods
// --------------------------------------------------------------------

// Use appends a middleware to the group's chain.
func (g *Group) Use(mw Middleware) {
	g.middlewares = append(g.middlewares, mw)
}

// Handle registers a route under the group's prefix.
func (g *Group) Handle(method, pattern string, handler Handler) {
	fullPattern := strings.TrimRight(g.prefix, "/") + "/" + strings.TrimLeft(pattern, "/")
	// Apply group middlewares (which already include app's)
	final := g.parent.applyMiddlewares(handler, g.middlewares)
	g.parent.registerRoute(method, fullPattern, final)
}

// Get is a shortcut for Handle with http.MethodGet.
func (g *Group) Get(pattern string, handler Handler)   { g.Handle(http.MethodGet, pattern, handler) }
func (g *Group) Post(pattern string, handler Handler)  { g.Handle(http.MethodPost, pattern, handler) }
func (g *Group) Put(pattern string, handler Handler)   { g.Handle(http.MethodPut, pattern, handler) }
func (g *Group) Patch(pattern string, handler Handler) { g.Handle(http.MethodPatch, pattern, handler) }
func (g *Group) Delete(pattern string, handler Handler) {
	g.Handle(http.MethodDelete, pattern, handler)
}

// --------------------------------------------------------------------
// Router implementation (simple trie‑based)
// --------------------------------------------------------------------

type routeEntry struct {
	method  string
	pattern string
	handler Handler
	params  []string // parameter names extracted from pattern
}

type node struct {
	children      map[string]*node
	paramChild    *node
	wildcardChild *node
	handler       map[string]Handler // method -> handler
	paramName     string
}

var pathParamKey = struct{}{}

func (a *App) initRouter() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.root == nil {
		a.root = &node{children: make(map[string]*node)}
	}
}

// root node for routing (stored in App)
func (a *App) registerRoute(method, pattern string, handler Handler) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.initRouter()

	parts := splitPath(pattern)
	current := a.root
	paramNames := []string{}

	for _, part := range parts {
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, ":") {
			// Parameter, e.g., :id
			name := part[1:]
			paramNames = append(paramNames, name)
			if current.paramChild == nil {
				current.paramChild = &node{
					children:  make(map[string]*node),
					paramName: name,
				}
			}
			current = current.paramChild
		} else if part == "*" {
			// Wildcard (catch‑all)
			if current.wildcardChild == nil {
				current.wildcardChild = &node{
					children: make(map[string]*node),
				}
			}
			current = current.wildcardChild
			break // wildcard consumes rest
		} else {
			// Static segment
			if _, ok := current.children[part]; !ok {
				current.children[part] = &node{children: make(map[string]*node)}
			}
			current = current.children[part]
		}
	}
	if current.handler == nil {
		current.handler = make(map[string]Handler)
	}
	current.handler[method] = handler
}

func (a *App) lookup(method, path string) (Handler, map[string]string) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.root == nil {
		return nil, nil
	}
	parts := splitPath(path)
	current := a.root
	params := make(map[string]string)
	var paramIdx int

	for i, part := range parts {
		if part == "" {
			continue
		}
		// 1. Try static match
		if next, ok := current.children[part]; ok {
			current = next
			continue
		}
		// 2. Try param match
		if current.paramChild != nil {
			params[current.paramChild.paramName] = part
			current = current.paramChild
			paramIdx++
			continue
		}
		// 3. Try wildcard (catch‑all)
		if current.wildcardChild != nil {
			// Collect remaining parts
			remain := strings.Join(parts[i:], "/")
			params["*"] = remain
			current = current.wildcardChild
			break
		}
		// No match
		return nil, nil
	}
	handler, ok := current.handler[method]
	if !ok {
		return nil, nil
	}
	return handler, params
}

// splitPath splits a URL path into segments, ignoring empty ones.
func splitPath(path string) []string {
	return strings.FieldsFunc(path, func(r rune) bool { return r == '/' })
}

// --------------------------------------------------------------------
// Middleware helpers
// --------------------------------------------------------------------

// applyMiddlewares chains middlewares to a base handler.
func (a *App) applyMiddlewares(h Handler, mws []Middleware) Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// Recovery middleware catches panics and returns a 500 error.
func Recovery() Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, req *Request) (resp *Response, err error) {
			defer func() {
				if r := recover(); r != nil {
					err = &Error{
						Code:    http.StatusInternalServerError,
						Message: "internal server error",
						Cause:   safe.PanicError(r),
						Stack:   debug.Stack(),
					}
				}
			}()
			return next(ctx, req)
		}
	}
}

// Logger middleware logs requests and responses.
func Logger() Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, req *Request) (*Response, error) {
			start := time.Now()
			resp, err := next(ctx, req)
			duration := time.Since(start)
			if verbose.V(1) {
				status := http.StatusOK
				if err != nil {
					status = http.StatusInternalServerError
				} else if resp != nil {
					status = resp.Status
				}
				verbose.Printf(1, "%s %s %d %v %s",
					req.Method, req.URL.Path, status, duration, req.RequestID)
			}
			return resp, err
		}
	}
}

// RequestID ensures each request has a unique ID (already set in App.ServeHTTP).
func RequestID() Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, req *Request) (*Response, error) {
			if req.RequestID == "" {
				req.RequestID = unique.MustRandomString(12)
			}
			return next(ctx, req)
		}
	}
}

// --------------------------------------------------------------------
// Response helpers
// --------------------------------------------------------------------

// JSON returns a JSON response with the given status code and body.
func JSON(status int, body any) (*Response, error) {
	return &Response{
		Status: status,
		Headers: http.Header{
			"Content-Type": []string{"application/json; charset=utf-8"},
		},
		Body: body,
	}, nil
}

// Text returns a plain text response.
func Text(status int, text string) (*Response, error) {
	return &Response{
		Status:  status,
		Headers: http.Header{"Content-Type": []string{"text/plain; charset=utf-8"}},
		RawBody: []byte(text),
	}, nil
}

// NoContent returns a 204 No Content response.
func NoContent() (*Response, error) {
	return &Response{Status: http.StatusNoContent}, nil
}

// writeResponse encodes and sends the response.
func (a *App) writeResponse(w http.ResponseWriter, r *http.Request, resp *Response) {
	// Set headers
	for k, v := range resp.Headers {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.Status)

	if resp.RawBody != nil {
		w.Write(resp.RawBody)
		return
	}
	if resp.Body != nil {
		// Default to JSON if no Content-Type set
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
		}
		if err := json.NewEncoder(w).Encode(resp.Body); err != nil {
			verbose.Printf(0, "api: failed to encode response: %v", err)
		}
	}
}

// --------------------------------------------------------------------
// Error handling
// --------------------------------------------------------------------

// Error represents a structured HTTP error.
type Error struct {
	Code    int    `json:"-"`
	Message string `json:"message"`
	Cause   error  `json:"-"`
	Stack   []byte `json:"-"`
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// ErrorResponse returns a JSON error response.
func ErrorResponse(code int, message string) *Response {
	return &Response{
		Status: code,
		Headers: http.Header{
			"Content-Type": []string{"application/json; charset=utf-8"},
		},
		Body: map[string]string{"error": message},
	}
}

// default error handler converts any error to a Response.
func (a *App) errorHandler(err error) *Response {
	if e, ok := err.(*Error); ok {
		return ErrorResponse(e.Code, e.Message)
	}
	return ErrorResponse(http.StatusInternalServerError, "internal server error")
}

// --------------------------------------------------------------------
// Request helpers
// --------------------------------------------------------------------

// PathParam returns the value of a named path parameter as a string.
// If the parameter does not exist, returns None.
func (r *Request) PathParam(name string) value.Option[string] {
	val, ok := r.PathParams[name]
	if !ok {
		return value.None[string]()
	}
	return value.Some(val)
}

// QueryParam returns the first value of the query parameter as a string Option.
func (r *Request) QueryParam(name string) value.Option[string] {
	vals := r.URL.Query()[name]
	if len(vals) == 0 {
		return value.None[string]()
	}
	return value.Some(vals[0])
}

// QueryParams returns all values of the query parameter.
func (r *Request) QueryParams(name string) []string {
	return r.URL.Query()[name]
}

// Header returns the first value of the header as a string Option.
func (r *Request) Header(name string) value.Option[string] {
	vals := r.Request.Header[name]
	if len(vals) == 0 {
		return value.None[string]()
	}
	return value.Some(vals[0])
}

// BindJSON decodes the request body into the provided struct.
func (r *Request) BindJSON(v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// RequestIDKey is the context key for the request ID.
type requestIDKey struct{}

var requestIDKey = requestIDKey{}

// GetRequestID returns the request ID from the context, or empty string.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// --------------------------------------------------------------------
// Internal response writer wrapper
// --------------------------------------------------------------------

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
