// Package bridge provides network connectivity management for the data gateway.
package testutils

import (
    "context"
    "crypto/tls"
    "errors"
    "fmt"
    "io"
    "net"
    "sync"
    "time"
)

// Common errors.
var (
    ErrBridgeClosed = errors.New("bridge is closed")
    ErrConnNotFound = errors.New("connection not found")
)

// ListenerConfig holds configuration for a network listener.
type ListenerConfig struct {
    Network string        // "tcp", "tcp4", "tcp6", "unix"
    Address string        // e.g., ":8080", "/tmp/sock"
    TLS     *tls.Config   // optional TLS
}

// DialerConfig holds configuration for outbound connections.
type DialerConfig struct {
    Network   string
    Address   string
    Timeout   time.Duration
    KeepAlive time.Duration
    TLS       *tls.Config
}

// Connection represents a managed network connection.
type Connection struct {
    ID        string
    Conn      net.Conn
    Created   time.Time
    LastUsed  time.Time
    tags      map[string]string
    mu        sync.RWMutex
    onClose   func()
}

// SetTag adds metadata to the connection.
func (c *Connection) SetTag(key, value string) {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.tags == nil {
        c.tags = make(map[string]string)
    }
    c.tags[key] = value
}

// GetTag retrieves metadata.
func (c *Connection) GetTag(key string) (string, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    val, ok := c.tags[key]
    return val, ok
}

// Close terminates the connection and runs the onClose hook.
func (c *Connection) Close() error {
    if c.onClose != nil {
        c.onClose()
    }
    return c.Conn.Close()
}

// Bridge manages listeners and connections.
type Bridge struct {
    listeners    map[string]*listenerWrapper
    conns        map[string]*Connection
    mu           sync.RWMutex
    wg           sync.WaitGroup
    ctx          context.Context
    cancel       context.CancelFunc
    connHandler  func(*Connection)          // called on new connection
    msgHandler   func(*Connection, []byte) // called on incoming data
}

type listenerWrapper struct {
    net.Listener
    cfg    ListenerConfig
    bridge *Bridge
}

// NewBridge creates a bridge with optional handlers.
func NewBridge(connHandler func(*Connection), msgHandler func(*Connection, []byte)) *Bridge {
    ctx, cancel := context.WithCancel(context.Background())
    return &Bridge{
        listeners:   make(map[string]*listenerWrapper),
        conns:       make(map[string]*Connection),
        ctx:         ctx,
        cancel:      cancel,
        connHandler: connHandler,
        msgHandler:  msgHandler,
    }
}

// AddListener starts a new network listener.
func (b *Bridge) AddListener(cfg ListenerConfig) error {
    b.mu.Lock()
    defer b.mu.Unlock()
    key := cfg.Network + "://" + cfg.Address
    if _, exists := b.listeners[key]; exists {
        return fmt.Errorf("listener already exists for %s", key)
    }

    var l net.Listener
    var err error
    if cfg.TLS != nil {
        l, err = tls.Listen(cfg.Network, cfg.Address, cfg.TLS)
    } else {
        l, err = net.Listen(cfg.Network, cfg.Address)
    }
    if err != nil {
        return err
    }

    w := &listenerWrapper{Listener: l, cfg: cfg, bridge: b}
    b.listeners[key] = w
    b.wg.Add(1)
    go w.acceptLoop()
    return nil
}

// acceptLoop runs per listener.
func (lw *listenerWrapper) acceptLoop() {
    defer lw.bridge.wg.Done()
    for {
        conn, err := lw.Accept()
        if err != nil {
            select {
            case <-lw.bridge.ctx.Done():
                return
            default:
                // Temporary error? Sleep and continue.
                if ne, ok := err.(net.Error); ok && ne.Temporary() {
                    time.Sleep(100 * time.Millisecond)
                    continue
                }
                return
            }
        }
        // Wrap connection
        id := generateID()
        c := &Connection{
            ID:       id,
            Conn:     conn,
            Created:  time.Now(),
            LastUsed: time.Now(),
            onClose: func() {
                lw.bridge.removeConnection(id)
            },
        }
        lw.bridge.addConnection(c)

        // Start reading from this connection
        lw.bridge.wg.Add(1)
        go lw.bridge.readLoop(c)

        // Notify handler
        if lw.bridge.connHandler != nil {
            lw.bridge.connHandler(c)
        }
    }
}

// readLoop reads data from a connection and passes it to msgHandler.
func (b *Bridge) readLoop(c *Connection) {
    defer b.wg.Done()
    defer c.Close()
    buf := make([]byte, 4096)
    for {
        select {
        case <-b.ctx.Done():
            return
        default:
            // Set a read deadline to avoid hanging forever
            c.Conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
            n, err := c.Conn.Read(buf)
            if err != nil {
                if err != io.EOF && !errors.Is(err, net.ErrClosed) {
                    // log error
                }
                return
            }
            // Update last used
            c.mu.Lock()
            c.LastUsed = time.Now()
            c.mu.Unlock()
            // Call message handler with a copy of the data
            if b.msgHandler != nil {
                data := append([]byte(nil), buf[:n]...)
                b.msgHandler(c, data)
            }
        }
    }
}

// Dial creates an outbound connection and adds it to the bridge.
func (b *Bridge) Dial(ctx context.Context, cfg DialerConfig) (*Connection, error) {
    var d net.Dialer
    d.Timeout = cfg.Timeout
    d.KeepAlive = cfg.KeepAlive

    var conn net.Conn
    var err error
    if cfg.TLS != nil {
        conn, err = tls.DialWithDialer(&d, cfg.Network, cfg.Address, cfg.TLS)
    } else {
        conn, err = d.DialContext(ctx, cfg.Network, cfg.Address)
    }
    if err != nil {
        return nil, err
    }

    id := generateID()
    c := &Connection{
        ID:       id,
        Conn:     conn,
        Created:  time.Now(),
        LastUsed: time.Now(),
        onClose: func() {
            b.removeConnection(id)
        },
    }
    b.addConnection(c)

    b.wg.Add(1)
    go b.readLoop(c)

    if b.connHandler != nil {
        b.connHandler(c)
    }
    return c, nil
}

// Send writes data to a specific connection.
func (b *Bridge) Send(connID string, data []byte) error {
    b.mu.RLock()
    c, ok := b.conns[connID]
    b.mu.RUnlock()
    if !ok {
        return ErrConnNotFound
    }
    c.mu.Lock()
    defer c.mu.Unlock()
    c.LastUsed = time.Now()
    _, err := c.Conn.Write(data)
    return err
}

// Broadcast sends data to all connections with a given tag.
func (b *Bridge) Broadcast(tagKey, tagValue string, data []byte) error {
    b.mu.RLock()
    defer b.mu.RUnlock()
    var lastErr error
    for _, c := range b.conns {
        if val, ok := c.GetTag(tagKey); ok && val == tagValue {
            if err := b.Send(c.ID, data); err != nil {
                lastErr = err
            }
        }
    }
    return lastErr
}

// Close shuts down the bridge.
func (b *Bridge) Close() error {
    b.cancel()
    b.mu.Lock()
    // Close all listeners
    for _, l := range b.listeners {
        l.Close()
    }
    // Close all connections
    for _, c := range b.conns {
        c.Close()
    }
    b.mu.Unlock()
    b.wg.Wait()
    return nil
}

func (b *Bridge) addConnection(c *Connection) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.conns[c.ID] = c
}

func (b *Bridge) removeConnection(id string) {
    b.mu.Lock()
    defer b.mu.Unlock()
    delete(b.conns, id)
}

func generateID() string {
    return fmt.Sprintf("%d", time.Now().UnixNano())
}