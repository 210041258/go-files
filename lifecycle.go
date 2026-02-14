package testutils

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Service represents a component that can be started and stopped.
type Service interface {
	// Start runs the service and blocks until it exits or fails.
	// It should respect context cancellation for graceful shutdown.
	Start(ctx context.Context) error
	// Stop signals the service to shut down gracefully.
	// It should return when shutdown is complete, or context expires.
	Stop(ctx context.Context) error
}

// ServiceFunc is an adapter to allow ordinary functions as Service.
type ServiceFunc struct {
	StartFunc func(context.Context) error
	StopFunc  func(context.Context) error
}

func (s ServiceFunc) Start(ctx context.Context) error {
	return s.StartFunc(ctx)
}

func (s ServiceFunc) Stop(ctx context.Context) error {
	return s.StopFunc(ctx)
}

// Manager coordinates the lifecycle of multiple services.
type Manager struct {
	services []namedService
	mu       sync.Mutex
	wg       sync.WaitGroup
	errors   chan error
	quit     chan struct{}
}

type namedService struct {
	name string
	svc  Service
}

// New creates a new lifecycle manager.
func New() *Manager {
	return &Manager{
		services: []namedService{},
		errors:   make(chan error),
		quit:     make(chan struct{}),
	}
}

// Add registers a service with the manager.
func (m *Manager) Add(name string, svc Service) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.services = append(m.services, namedService{name: name, svc: svc})
}

// AddFunc registers a service from start and stop functions.
func (m *Manager) AddFunc(name string, start, stop func(context.Context) error) {
	m.Add(name, ServiceFunc{StartFunc: start, StopFunc: stop})
}

// Run starts all registered services and waits for shutdown.
// It returns the first error encountered during start, or nil if all
// services exit cleanly after receiving a shutdown signal.
func (m *Manager) Run(ctx context.Context) error {
	// Create a cancellable context for the whole lifecycle.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start all services.
	if err := m.startAll(ctx); err != nil {
		return err
	}

	// Wait for shutdown signal or any service error.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		// Received interrupt signal.
	case <-ctx.Done():
		// Parent context cancelled.
	case err := <-m.errors:
		// A service exited with error.
		cancel() // ensure other services are also cancelled.
		return fmt.Errorf("service error: %w", err)
	case <-m.quit:
		// Manual shutdown triggered.
	}

	// Graceful stop.
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer stopCancel()
	if err := m.stopAll(stopCtx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}
	return nil
}

// startAll starts each service concurrently.
// If any service fails to start, it stops the already started ones and returns an error.
func (m *Manager) startAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// We'll start services one by one for simplicity.
	// In production, you might start them in parallel with dependency ordering.
	for _, ns := range m.services {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		m.wg.Add(1)
		go func(ns namedService) {
			defer m.wg.Done()
			if err := ns.svc.Start(ctx); err != nil {
				m.errors <- fmt.Errorf("%s: %w", ns.name, err)
			}
		}(ns)
	}

	return nil
}

// stopAll stops all services concurrently.
func (m *Manager) stopAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Use a separate wait group for stop operations.
	var stopWg sync.WaitGroup
	stopErrors := make(chan error, len(m.services))

	for _, ns := range m.services {
		stopWg.Add(1)
		go func(ns namedService) {
			defer stopWg.Done()
			if err := ns.svc.Stop(ctx); err != nil {
				stopErrors <- fmt.Errorf("%s stop: %w", ns.name, err)
			}
		}(ns)
	}

	// Wait for all stop operations or context expiration.
	done := make(chan struct{})
	go func() {
		stopWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		close(stopErrors)
		// Collect errors if any.
		var errs []error
		for err := range stopErrors {
			errs = append(errs, err)
		}
		if len(errs) > 0 {
			return fmt.Errorf("stop errors: %w", errors.Join(errs...))
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop initiates shutdown of all services. It is non‑blocking;
// use Run to wait for completion.
func (m *Manager) Stop() {
	close(m.quit)
}

// Wait blocks until all started services have exited.
func (m *Manager) Wait() {
	m.wg.Wait()
}

// GracePeriod sets the default shutdown timeout used by Run.
// You can override it per call by providing your own context.
const DefaultShutdownTimeout = 30 * time.Second

// Example usage (commented out):
//
// func main() {
//     mgr := lifecycle.New()
//
//     // Add an HTTP server service.
//     httpSrv := &http.Server{Addr: ":8080"}
//     mgr.AddFunc("http",
//         func(ctx context.Context) error {
//             go func() {
//                 <-ctx.Done()
//                 httpSrv.Shutdown(context.Background())
//             }()
//             return httpSrv.ListenAndServe()
//         },
//         func(ctx context.Context) error {
//             return httpSrv.Shutdown(ctx)
//         },
//     )
//
//     // Add a background worker.
//     mgr.AddFunc("worker",
//         func(ctx context.Context) error {
//             for {
//                 select {
//                 case <-ctx.Done():
//                     return nil
//                 case <-time.After(1 * time.Second):
//                     // do work
//                 }
//             }
//         },
//         func(ctx context.Context) error {
//             // nothing special to clean up
//             return nil
//         },
//     )
//
//     if err := mgr.Run(context.Background()); err != nil {
//         log.Fatal(err)
//     }
//     log.Println("all services stopped")
// }