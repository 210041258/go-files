// Package testutils provides utilities for testing web APIs.
package testutils

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ------------------------------------------------------------------------
// Network helpers
// ------------------------------------------------------------------------

// WaitForPort attempts to connect to a TCP port until it succeeds or times out.
// It is useful for waiting for a service to start.
func WaitForPort(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", addr)
}

// FreePort asks the kernel for a free, open port that is ready to use.
// It returns the port number and a listener that must be closed when done.
func FreePort() (int, net.Listener, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, nil, err
	}
	addr := l.Addr().(*net.TCPAddr)
	return addr.Port, l, nil
}

// ------------------------------------------------------------------------
// In‑memory storage (key‑value)
// ------------------------------------------------------------------------

// MemStore is a simple thread‑safe in‑memory key‑value store for testing.
type MemStore struct {
	mu   sync.RWMutex
	data map[string]interface{}
}

// NewMemStore creates an empty MemStore.
func NewMemStore() *MemStore {
	return &MemStore{
		data: make(map[string]interface{}),
	}
}

// Get retrieves a value by key. Returns nil if not found.
func (s *MemStore) Get(key string) interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[key]
}

// Set stores a value under the given key.
func (s *MemStore) Set(key string, val interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = val
}

// Delete removes a key.
func (s *MemStore) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}

// Clear removes all keys.
func (s *MemStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make(map[string]interface{})
}

// ------------------------------------------------------------------------
// Memory usage introspection
// ------------------------------------------------------------------------

// MemStats captures a snapshot of memory statistics.
type MemStats struct {
	Alloc      uint64 // bytes allocated and not yet freed
	TotalAlloc uint64 // total bytes allocated (even if freed)
	Sys        uint64 // total bytes obtained from system
	NumGC      uint32 // number of completed GC cycles
}

// ReadMemStats returns a snapshot of the current memory statistics.
func ReadMemStats() MemStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return MemStats{
		Alloc:      m.Alloc,
		TotalAlloc: m.TotalAlloc,
		Sys:        m.Sys,
		NumGC:      m.NumGC,
	}
}

// ForceGC runs a garbage collection and returns the updated MemStats.
func ForceGC() MemStats {
	runtime.GC()
	return ReadMemStats()
}

// ------------------------------------------------------------------------
// Database test helpers (simplified)
// ------------------------------------------------------------------------

// DBConnString represents a database connection configuration.
type DBConnString struct {
	Driver   string // e.g., "postgres", "mysql", "sqlite3"
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	Params   map[string]string // additional connection parameters
}

// String returns the connection string in the format expected by the driver.
// This is a simplistic implementation; real drivers have different formats.
func (dc DBConnString) String() string {
	switch dc.Driver {
	case "postgres":
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s %s",
			dc.Host, dc.Port, dc.User, dc.Password, dc.DBName, dc.formatParams("=", " "))
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?%s",
			dc.User, dc.Password, dc.Host, dc.Port, dc.DBName, dc.formatParams("=", "&"))
	case "sqlite3":
		return dc.DBName // file path
	default:
		return ""
	}
}

func (dc DBConnString) formatParams(sep, pairSep string) string {
	if len(dc.Params) == 0 {
		return ""
	}
	var pairs []string
	for k, v := range dc.Params {
		pairs = append(pairs, k+sep+v)
	}
	return strings.Join(pairs, pairSep)
}

// WaitForDB attempts to connect to a database until it succeeds or times out.
// It uses the provided driver name and connection string.
func WaitForDB(driver, connStr string, timeout time.Duration) error {
	// This is a placeholder; real implementation would use database/sql with
	// a suitable driver. To avoid forcing database drivers as dependencies,
	// we skip actual connection attempts here. In a real test, you would
	// import the driver and call sql.Open.
	deadline := time.Now().Add(timeout)
	var err error
	for time.Now().Before(deadline) {
		// Simulate a connection check; in real code you would do db.Ping().
		// For this placeholder, we just assume success after a short wait.
		time.Sleep(100 * time.Millisecond)
		return nil // replace with real check
	}
	return fmt.Errorf("timeout waiting for database %s: %v", driver, err)
}

// ------------------------------------------------------------------------
// Container helpers (Docker)
// ------------------------------------------------------------------------

// RunContainer starts a Docker container for a service (e.g., database, redis).
// It returns a cleanup function that stops and removes the container.
// This is a placeholder; real implementation would use a Docker client.
func RunContainer(image string, portMap map[int]int, env []string) (host string, cleanup func(), err error) {
	// Placeholder: just return dummy values and a no‑op cleanup.
	host = "localhost"
	cleanup = func() {}
	err = nil
	return
}

// ------------------------------------------------------------------------
// Must variants – panic on error
// ------------------------------------------------------------------------

// MustWaitForPort calls WaitForPort and panics on error.
func MustWaitForPort(addr string, timeout time.Duration) {
	if err := WaitForPort(addr, timeout); err != nil {
		panic("testutils: WaitForPort failed: " + err.Error())
	}
}

// MustFreePort calls FreePort and panics on error.
func MustFreePort() (int, net.Listener) {
	port, l, err := FreePort()
	if err != nil {
		panic("testutils: FreePort failed: " + err.Error())
	}
	return port, l
}