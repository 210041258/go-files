package testutils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"google.golang.org/grpc/resolver"
)

// ServiceInfo holds information about a registered service instance.
type ServiceInfo struct {
	Name    string            `json:"name"`
	Addr    string            `json:"addr"` // host:port
	Meta    map[string]string `json:"meta,omitempty"`
	Weight  int               `json:"weight,omitempty"`
	Version string            `json:"version,omitempty"`
}

// Registry defines the interface for service registration and discovery.
type Registry interface {
	// Register registers a service instance with a lease TTL.
	Register(ctx context.Context, service ServiceInfo, ttl int64) error
	// Deregister removes a service instance.
	Deregister(ctx context.Context, service ServiceInfo) error
	// GetService returns all healthy instances of a service.
	GetService(ctx context.Context, name string) ([]ServiceInfo, error)
	// Watch creates a watcher for service updates.
	Watch(ctx context.Context, name string) (<-chan []ServiceInfo, error)
	// Close closes the registry client.
	Close() error
}

// EtcdRegistry implements Registry using etcd as the backend.
type EtcdRegistry struct {
	client   *clientv3.Client
	leaseID  clientv3.LeaseID
	keepAliveCh <-chan *clientv3.LeaseKeepAliveResponse
	mu       sync.Mutex
	services map[string]struct{} // track registered services for auto-deregister
}

// NewEtcdRegistry creates a new etcd-based registry.
func NewEtcdRegistry(endpoints []string, dialTimeout time.Duration) (*EtcdRegistry, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: dialTimeout,
	})
	if err != nil {
		return nil, err
	}
	return &EtcdRegistry{
		client:   cli,
		services: make(map[string]struct{}),
	}, nil
}

// key builds the etcd key for a service instance.
func (r *EtcdRegistry) key(service ServiceInfo) string {
	return fmt.Sprintf("/services/%s/%s", service.Name, service.Addr)
}

// Register registers a service with etcd using a lease.
func (r *EtcdRegistry) Register(ctx context.Context, service ServiceInfo, ttl int64) error {
	// Create a new lease for this instance
	resp, err := r.client.Grant(ctx, ttl)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.leaseID = resp.ID
	r.mu.Unlock()

	// Keep the lease alive in background
	keepAliveCh, err := r.client.KeepAlive(ctx, r.leaseID)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.keepAliveCh = keepAliveCh
	r.mu.Unlock()

	// Handle keep-alive responses
	go func() {
		for range keepAliveCh {
			// lease extended successfully
		}
		log.Println("Service lease keep-alive stopped")
	}()

	// Marshal service info to JSON
	data, err := json.Marshal(service)
	if err != nil {
		return err
	}

	// Put key with lease
	key := r.key(service)
	_, err = r.client.Put(ctx, key, string(data), clientv3.WithLease(r.leaseID))
	if err != nil {
		return err
	}

	r.mu.Lock()
	r.services[key] = struct{}{}
	r.mu.Unlock()

	log.Printf("Service registered: %s at %s", service.Name, service.Addr)
	return nil
}

// Deregister removes a service instance.
func (r *EtcdRegistry) Deregister(ctx context.Context, service ServiceInfo) error {
	key := r.key(service)
	_, err := r.client.Delete(ctx, key)
	if err == nil {
		r.mu.Lock()
		delete(r.services, key)
		r.mu.Unlock()
		log.Printf("Service deregistered: %s at %s", service.Name, service.Addr)
	}
	return err
}

// GetService returns all instances of a service.
func (r *EtcdRegistry) GetService(ctx context.Context, name string) ([]ServiceInfo, error) {
	prefix := fmt.Sprintf("/services/%s/", name)
	resp, err := r.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	services := make([]ServiceInfo, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var s ServiceInfo
		if err := json.Unmarshal(kv.Value, &s); err != nil {
			log.Printf("Skipping invalid service data for key %s: %v", kv.Key, err)
			continue
		}
		services = append(services, s)
	}
	return services, nil
}

// Watch monitors service changes and streams updates.
func (r *EtcdRegistry) Watch(ctx context.Context, name string) (<-chan []ServiceInfo, error) {
	prefix := fmt.Sprintf("/services/%s/", name)
	ch := make(chan []ServiceInfo, 1)

	// Send initial snapshot
	initial, err := r.GetService(ctx, name)
	if err != nil {
		return nil, err
	}
	ch <- initial

	// Start watching for changes
	watchCh := r.client.Watch(ctx, prefix, clientv3.WithPrefix())
	go func() {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			case wresp := <-watchCh:
				if wresp.Err() != nil {
					log.Printf("Watch error: %v", wresp.Err())
					return
				}
				// On any change, fetch full list again (simplistic approach)
				// For efficiency, could incrementally update cache, but for simplicity:
				services, err := r.GetService(ctx, name)
				if err != nil {
					log.Printf("Failed to fetch service list after watch event: %v", err)
					continue
				}
				ch <- services
			}
		}
	}()
	return ch, nil
}

// Close closes the etcd client and revokes lease.
func (r *EtcdRegistry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Revoke lease if exists
	if r.leaseID != 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err := r.client.Revoke(ctx, r.leaseID)
		cancel()
		if err != nil {
			log.Printf("Failed to revoke lease: %v", err)
		}
	}
	return r.client.Close()
}

// etcdResolver implements gRPC resolver for etcd-based service discovery.
type etcdResolver struct {
	registry     *EtcdRegistry
	serviceName  string
	cc           resolver.ClientConn
	watchCh      <-chan []ServiceInfo
	cancel       context.CancelFunc
}

// Build creates a new resolver for the given target.
func (e *EtcdRegistry) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	serviceName := target.Endpoint
	ctx, cancel := context.WithCancel(context.Background())
	watchCh, err := e.Watch(ctx, serviceName)
	if err != nil {
		cancel()
		return nil, err
	}
	r := &etcdResolver{
		registry:    e,
		serviceName: serviceName,
		cc:          cc,
		watchCh:     watchCh,
		cancel:      cancel,
	}
	go r.watchUpdates()
	return r, nil
}

// Scheme returns the resolver scheme.
func (e *EtcdRegistry) Scheme() string {
	return "etcd"
}

// watchUpdates listens for service updates and updates the ClientConn.
func (r *etcdResolver) watchUpdates() {
	for services := range r.watchCh {
		addrs := make([]resolver.Address, 0, len(services))
		for _, s := range services {
			addr := resolver.Address{
				Addr:       s.Addr,
				ServerName: s.Name,
				Metadata:   s.Meta,
			}
			// If weight is present, encode as attribute (optional)
			if s.Weight > 0 {
				addr.BalancerAttributes = nil // can set weight via attributes or metadata
			}
			addrs = append(addrs, addr)
		}
		state := resolver.State{
			Addresses: addrs,
		}
		r.cc.UpdateState(state)
	}
}

// ResolveNow is a no-op because we watch continuously.
func (r *etcdResolver) ResolveNow(o resolver.ResolveNowOptions) {}

// Close stops the resolver.
func (r *etcdResolver) Close() {
	r.cancel()
}

// init registers the etcd resolver builder if etcd endpoints are provided via environment.
// For production, you'd typically inject the builder explicitly.
func init() {
	// Example: auto-register if endpoints set in env
	// This is optional and not recommended for all cases; better to register explicitly.
}

// RegisterResolver registers the etcd resolver with the given endpoints.
func RegisterResolver(endpoints []string, dialTimeout time.Duration) error {
	reg, err := NewEtcdRegistry(endpoints, dialTimeout)
	if err != nil {
		return err
	}
	resolver.Register(reg)
	return nil
}

// Example usage:
// func main() {
//     // Registry side
//     registry, _ := NewEtcdRegistry([]string{"localhost:2379"}, 5*time.Second)
//     service := ServiceInfo{
//         Name: "my-service",
//         Addr: "localhost:50051",
//         Meta: map[string]string{"version": "v1"},
//     }
//     registry.Register(context.Background(), service, 10)
//     defer registry.Close()
//
//     // Client side
//     resolver.SetDefaultScheme("etcd")
//     conn, _ := grpc.Dial("etcd:///my-service", grpc.WithInsecure())
//     ...
// }