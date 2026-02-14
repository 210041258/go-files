package discovery

import (
    "context"
    "fmt"
    "time"

    clientv3 "go.etcd.io/etcd/client/v3"
    "go.etcd.io/etcd/client/v3/concurrency"
)

// EtcdRegistry handles registering services in etcd with TTL/lease.
type EtcdRegistry struct {
    client   *clientv3.Client
    leaseID  clientv3.LeaseID
    key      string
    value    string
    ttl      int64
    cancel   context.CancelFunc
}

// NewEtcdRegistry creates a new registry instance.
// endpoints: etcd servers, ttlSeconds: lease TTL, key: service key, value: service address
func NewEtcdRegistry(endpoints []string, key, value string, ttlSeconds int64) (*EtcdRegistry, error) {
    cli, err := clientv3.New(clientv3.Config{
        Endpoints:   endpoints,
        DialTimeout: 5 * time.Second,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to connect to etcd: %w", err)
    }

    return &EtcdRegistry{
        client: cli,
        key:    key,
        value:  value,
        ttl:    ttlSeconds,
    }, nil
}

// Register registers the service with a TTL and keeps it alive.
func (r *EtcdRegistry) Register() error {
    ctx, cancel := context.WithCancel(context.Background())
    r.cancel = cancel

    // Create a lease
    leaseResp, err := r.client.Grant(ctx, r.ttl)
    if err != nil {
        return fmt.Errorf("failed to create lease: %w", err)
    }
    r.leaseID = leaseResp.ID

    // Put key with lease
    _, err = r.client.Put(ctx, r.key, r.value, clientv3.WithLease(r.leaseID))
    if err != nil {
        return fmt.Errorf("failed to register service: %w", err)
    }

    // Keep lease alive in background
    ch, err := r.client.KeepAlive(ctx, r.leaseID)
    if err != nil {
        return fmt.Errorf("failed to set up keepalive: %w", err)
    }

    go func() {
        for {
            select {
            case _, ok := <-ch:
                if !ok {
                    fmt.Println("etcd keepalive channel closed")
                    return
                }
            case <-ctx.Done():
                return
            }
        }
    }()

    return nil
}

// Deregister removes the service entry from etcd.
func (r *EtcdRegistry) Deregister() error {
    if r.cancel != nil {
        r.cancel()
    }
    _, err := r.client.Delete(context.Background(), r.key)
    return err
}

// Close closes the etcd client connection.
func (r *EtcdRegistry) Close() error {
    if r.cancel != nil {
        r.cancel()
    }
    return r.client.Close()
}
