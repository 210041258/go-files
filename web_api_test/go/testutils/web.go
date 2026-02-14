// Package web provides a lightweight, modular web server builder with
// middleware support, routing, and graceful shutdown. It builds on the
// standard net/http package and Go 1.22+ routing patterns.
package testutils

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"
)

// ----------------------------------------------------------------------
// Context keys for storing request-scoped values.
// ----------------------------------------------------------------------
type contextKey int

const (
	// RequestIDKey is the context key for the request ID.
	RequestIDKey contextKey = iota
)

// ----------------------------------------------------------------------
// Server is the main web server instance.
// ----------------------------------------------------------------------

// Server wraps an http.Server with middleware and routing.
type Server struct {
	*http.Server
	router     *http.ServeMux
	middleware []Middleware
	mu         sync.RWMutex
	routes     map[string]map[string]http.Handler // method -> pattern -> handler
	notFound   http.Handler
}

// Middleware defines a function that wraps an http.Handler.
type Middleware func(http.Handler) http.Handler

// New creates a new Server with default settings.
func New(addr string) *Server {
	s := &Server{
		Server: &http.Server{
			Addr:         addr,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		router:   http.NewServeMux(),
		routes:   make(map[string]map[string]http.Handler),
		notFound: http.NotFoundHandler(),
	}
	s.Server.Handler = s
	return s
}

// Use appends middleware to the chain. Middleware is executed in the order added.
func (s *Server) Use(mw ...Middleware) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.middleware = append(s.middleware, mw...)
}

// NotFound sets a custom handler for unmatched routes.
func (s *Server) NotFound(handler http.Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notFound = handler
}

// route registers a handler for a specific method and pattern.
func (s *Server) route(method, pattern string, handler http.Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.routes[method]; !ok {
		s.routes[method] = make(map[string]http.Handler)
	}
	s.routes[method][pattern] = handler
	// Also register with the underlying mux for ServeHTTP routing.
	s.router.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		handler.ServeHTTP(w, r)
	})
}

// Get registers a GET handler.
func (s *Server) Get(pattern string, handler http.HandlerFunc) {
	s.route(http.MethodGet, pattern, handler)
}

// Post registers a POST handler.
func (s *Server) Post(pattern string, handler http.HandlerFunc) {
	s.route(http.MethodPost, pattern, handler)
}

// Put registers a PUT handler.
func (s *Server) Put(pattern string, handler http.HandlerFunc) {
	s.route(http.MethodPut, pattern, handler)
}

// Delete registers a DELETE handler.
func (s *Server) Delete(pattern string, handler http.HandlerFunc) {
	s.route(http.MethodDelete, pattern, handler)
}

// Patch registers a PATCH handler.
func (s *Server) Patch(pattern string, handler http.HandlerFunc) {
	s.route(http.MethodPatch, pattern, handler)
}

// Head registers a HEAD handler.
func (s *Server) Head(pattern string, handler http.HandlerFunc) {
	s.route(http.MethodHead, pattern, handler)
}

// Options registers an OPTIONS handler.
func (s *Server) Options(pattern string, handler http.HandlerFunc) {
	s.route(http.MethodOptions, pattern, handler)
}

// Static serves static files from the given directory under the specified path prefix.
// For example, Static("/static", "./public") serves files in ./public at /static/...
func (s *Server) Static(prefix, dir string) {
	fs := http.FileServer(http.Dir(dir))
	if prefix == "/" {
		s.Get("/", fs.ServeHTTP)
	} else {
		s.Get(prefix+"/", http.StripPrefix(prefix, fs).ServeHTTP)
	}
}

// Group creates a new route group with an optional prefix.
func (s *Server) Group(prefix string) *Group {
	return &Group{
		server: s,
		prefix: prefix,
	}
}

// ----------------------------------------------------------------------
// Group for route prefixes.
// ----------------------------------------------------------------------

// Group represents a group of routes with a common prefix.
type Group struct {
	server *Server
	prefix string
}

// Get registers a GET handler under the group prefix.
func (g *Group) Get(pattern string, handler http.HandlerFunc) {
	g.server.Get(g.prefix+pattern, handler)
}

// Post registers a POST handler under the group prefix.
func (g *Group) Post(pattern string, handler http.HandlerFunc) {
	g.server.Post(g.prefix+pattern, handler)
}

// Put registers a PUT handler under the group prefix.
func (g *Group) Put(pattern string, handler http.HandlerFunc) {
	g.server.Put(g.prefix+pattern, handler)
}

// Delete registers a DELETE handler under the group prefix.
func (g *Group) Delete(pattern string, handler http.HandlerFunc) {
	g.server.Delete(g.prefix+pattern, handler)
}

// Patch registers a PATCH handler under the group prefix.
func (g *Group) Patch(pattern string, handler http.HandlerFunc) {
	g.server.Patch(g.prefix+pattern, handler)
}

// Head registers a HEAD handler under the group prefix.
func (g *Group) Head(pattern string, handler http.HandlerFunc) {
	g.server.Head(g.prefix+pattern, handler)
}

// Options registers an OPTIONS handler under the group prefix.
func (g *Group) Options(pattern string, handler http.HandlerFunc) {
	g.server.Options(g.prefix+pattern, handler)
}

// Use adds middleware to the group (applied only to routes in this group).
// This is a simplified approach – in production you might want per‑group
// middleware stacks; here we just provide a way to wrap handlers manually.
func (g *Group) Use(mw Middleware, pattern string, handler http.HandlerFunc) {
	g.server.Get(g.prefix+pattern, mw(handler).ServeHTTP)
}

// ----------------------------------------------------------------------
// ServeHTTP implements http.Handler, applying middleware chain.
// ----------------------------------------------------------------------

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Find the handler from the router.
	var h http.Handler
	s.mu.RLock()
	if methods, ok := s.routes[r.Method]; ok {
		if handler, ok := methods[r.URL.Path]; ok {
			h = handler
		}
	}
	s.mu.RUnlock()

	if h == nil {
		h = s.notFound
	}

	// Apply middleware chain.
	for i := len(s.middleware) - 1; i >= 0; i-- {
		h = s.middleware[i](h)
	}
	h.ServeHTTP(w, r)
}

// ----------------------------------------------------------------------
// Built-in middleware.
// ----------------------------------------------------------------------

// LoggerMiddleware logs each request with method, path, and duration.
func LoggerMiddleware(logger *log.Logger) Middleware {
	if logger == nil {
		logger = log.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			logger.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
		})
	}
}

// RecoveryMiddleware recovers from panics and returns a 500 error.
func RecoveryMiddleware(logger *log.Logger) Middleware {
	if logger == nil {
		logger = log.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Printf("panic: %v", err)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// RequestIDMiddleware adds a unique request ID to the context and response header.
// If X-Request-ID header is present, it is used; otherwise a random ID is generated.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			// In production, use a proper generator (e.g., uuid). For simplicity:
			id = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		ctx := context.WithValue(r.Context(), RequestIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// CORSMiddleware adds CORS headers to responses.
// It allows all origins and common methods by default; customize as needed.
func CORSMiddleware(allowedOrigins []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			// Simple check: allow if origin is in allowed list, or if list is empty allow all.
			allow := len(allowedOrigins) == 0
			for _, o := range allowedOrigins {
				if o == "*" || o == origin {
					allow = true
					break
				}
			}
			if allow {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ----------------------------------------------------------------------
// Helper response functions.
// ----------------------------------------------------------------------

// JSON sends a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(data)
}

// Error sends a JSON error response.
func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, map[string]string{"error": message})
}

// HTML sends an HTML response.
func HTML(w http.ResponseWriter, status int, html string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	w.Write([]byte(html))
}

// Redirect sends a redirect response.
func Redirect(w http.ResponseWriter, r *http.Request, url string, code int) {
	http.Redirect(w, r, url, code)
}

// ----------------------------------------------------------------------
// Graceful shutdown.
// ----------------------------------------------------------------------

// Start runs the server and blocks until shutdown. It handles SIGINT/SIGTERM.
func (s *Server) Start(ctx context.Context) error {
	// Listen for interrupt signals.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	go func() {
		<-stop
		log.Println("shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()

	log.Printf("server listening on %s", s.Addr)
	if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	<-ctx.Done()
	return nil
}

// ----------------------------------------------------------------------
// Example usage (commented)
// ----------------------------------------------------------------------
// func main() {
//     s := web.New(":8080")
//     s.Use(web.LoggerMiddleware(nil), web.RecoveryMiddleware(nil))
//
//     s.Get("/", func(w http.ResponseWriter, r *http.Request) {
//         web.HTML(w, http.StatusOK, "<h1>Hello</h1>")
//     })
//
//     api := s.Group("/api")
//     api.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
//         web.JSON(w, http.StatusOK, map[string]string{"pong": "ok"})
//     })
//
//     s.Static("/static", "./public")
//
//     if err := s.Start(context.Background()); err != nil {
//         log.Fatal(err)
//     }
// }