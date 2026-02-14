package discovery

import (
    "context"
    "fmt"
    "time"

    "google.golang.org/grpc/resolver"
    clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdResolver implements gRPC resolver.Builder interface for etcd.
type EtcdResolver struct {
    client     *clientv3.Client
    serviceKey string
    timeout    time.Duration
}

// NewEtcdResolver creates a new EtcdResolver.
func NewEtcdResolver(endpoints []string, serviceKey string, timeout time.Duration) (*EtcdResolver, error) {
    cli, err := clientv3.New(clientv3.Config{
        Endpoints:   endpoints,
        DialTimeout: timeout,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to create etcd client: %w", err)
    }
    return &EtcdResolver{
        client:     cli,
        serviceKey: serviceKey,
        timeout:    timeout,
    }, nil
}

// Build implements resolver.Builder interface.
func (r *EtcdResolver) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
    res := &etcdWatcher{
        clientConn: cc,
        client:     r.client,
        serviceKey: r.serviceKey,
        timeout:    r.timeout,
    }
    go res.watch()
    return res, nil
}

// Scheme returns the scheme for this resolver.
func (r *EtcdResolver) Scheme() string {
    return "etcd"
}

// --- Watcher implementation ---
type etcdWatcher struct {
    clientConn resolver.ClientConn
    client     *clientv3.Client
    serviceKey string
    timeout    time.Duration
}

func (w *etcdWatcher) watch() {
    for {
        ctx, cancel := context.WithTimeout(context.Background(), w.timeout)
        resp, err := w.client.Get(ctx, w.serviceKey, clientv3.WithPrefix())
        cancel()
        if err != nil {
            fmt.Printf("etcd resolver: failed to fetch services: %v\n", err)
            time.Sleep(time.Second)
            continue
        }

        addrs := make([]resolver.Address, 0, len(resp.Kvs))
        for _, kv := range resp.Kvs {
            addrs = append(addrs, resolver.Address{Addr: string(kv.Value)})
        }

        w.clientConn.UpdateState(resolver.State{Addresses: addrs})

        // Watch for updates
        watchChan := w.client.Watch(context.Background(), w.serviceKey, clientv3.WithPrefix())
        for ev := range watchChan {
            for _, event := range ev.Events {
                fmt.Printf("etcd event: %s %q : %q\n", event.Type, event.Kv.Key, event.Kv.Value)
            }
            // Refresh addresses after events
            resp, err := w.client.Get(context.Background(), w.serviceKey, clientv3.WithPrefix())
            if err != nil {
                fmt.Printf("etcd resolver: failed to refresh services: %v\n", err)
                continue
            }
            addrs := make([]resolver.Address, 0, len(resp.Kvs))
            for _, kv := range resp.Kvs {
                addrs = append(addrs, resolver.Address{Addr: string(kv.Value)})
            }
            w.clientConn.UpdateState(resolver.State{Addresses: addrs})
        }
    }
}

func (w *etcdWatcher) ResolveNow(o resolver.ResolveNowOptions) {}
func (w *etcdWatcher) Close() {}
