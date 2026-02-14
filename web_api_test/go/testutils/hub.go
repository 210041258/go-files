package testutils

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Message represents a generic message exchanged through the hub.
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
	Room    string          `json:"room,omitempty"`
	Sender  string          `json:"sender,omitempty"`
	Time    time.Time       `json:"time,omitempty"`
}

// Client holds a WebSocket connection and its metadata.
type Client struct {
	Hub  *Hub
	Conn *websocket.Conn
	Send chan Message

	ID   string
	Room string
}

// Hub manages all active clients and handles message routing.
type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from clients.
	Broadcast chan Message

	// Register requests from clients.
	Register chan *Client

	// Unregister requests from clients.
	Unregister chan *Client

	// Room management.
	rooms map[string]map[*Client]bool

	mu   sync.RWMutex
	done chan struct{}
}

// NewHub creates a new Hub instance.
func NewHub() *Hub {
	return &Hub{
		Broadcast:  make(chan Message),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		rooms:      make(map[string]map[*Client]bool),
		done:       make(chan struct{}),
	}
}

// Run starts the hub's event loop. It should be run as a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case <-h.done:
			return
		case client := <-h.Register:
			h.registerClient(client)
		case client := <-h.Unregister:
			h.unregisterClient(client)
		case message := <-h.Broadcast:
			h.routeMessage(message)
		}
	}
}

// Stop gracefully shuts down the hub.
func (h *Hub) Stop() {
	close(h.done)
}

// registerClient adds a new client to the hub and optionally to a room.
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[client] = true
	if client.Room != "" {
		if _, ok := h.rooms[client.Room]; !ok {
			h.rooms[client.Room] = make(map[*Client]bool)
		}
		h.rooms[client.Room][client] = true
	}
}

// unregisterClient removes a client and closes its send channel.
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.Send)
		if client.Room != "" {
			delete(h.rooms[client.Room], client)
			if len(h.rooms[client.Room]) == 0 {
				delete(h.rooms, client.Room)
			}
		}
	}
}

// routeMessage sends a message to all clients, optionally filtered by room.
func (h *Hub) routeMessage(message Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if message.Room == "" {
		// Broadcast to all clients.
		for client := range h.clients {
			select {
			case client.Send <- message:
			default:
				// Client's send buffer is full; assume dead and unregister.
				go h.unregisterClient(client)
			}
		}
	} else {
		// Broadcast to a specific room.
		for client := range h.rooms[message.Room] {
			select {
			case client.Send <- message:
			default:
				go h.unregisterClient(client)
			}
		}
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// RoomCount returns the number of active rooms.
func (h *Hub) RoomCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms)
}

// ClientsInRoom returns the number of clients in a specific room.
func (h *Hub) ClientsInRoom(room string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms[room])
}

// WritePump pumps messages from the hub to the WebSocket connection.
func (c *Client) WritePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Hub closed the channel.
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteJSON(message); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ReadPump pumps messages from the WebSocket connection to the hub.
// It also handles pong responses and custom join room logic.
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(512 * 1024) // 512KB max message size
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg Message
		err := c.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				// Log error if needed.
			}
			break
		}
		// Set message metadata.
		msg.Time = time.Now()
		msg.Sender = c.ID

		// Handle join room commands.
		if msg.Type == "join" && msg.Room != "" {
			c.Hub.mu.Lock()
			// Leave current room.
			if c.Room != "" {
				delete(c.Hub.rooms[c.Room], c)
			}
			c.Room = msg.Room
			// Join new room.
			if _, ok := c.Hub.rooms[c.Room]; !ok {
				c.Hub.rooms[c.Room] = make(map[*Client]bool)
			}
			c.Hub.rooms[c.Room][c] = true
			c.Hub.mu.Unlock()
			continue
		}

		// Forward other messages to the hub for broadcasting.
		c.Hub.Broadcast <- msg
	}
}

// WebSocketHandler is a helper to upgrade HTTP connections and register the client.
// It is intended to be used as an http.HandlerFunc.
func (h *Hub) WebSocketHandler(upgrader websocket.Upgrader) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		// Extract client ID and initial room from query parameters (or set defaults).
		clientID := r.URL.Query().Get("client_id")
		if clientID == "" {
			clientID = conn.RemoteAddr().String()
		}
		room := r.URL.Query().Get("room")

		client := &Client{
			Hub:  h,
			Conn: conn,
			Send: make(chan Message, 256),
			ID:   clientID,
			Room: room,
		}

		h.Register <- client

		// Start the pumps.
		go client.WritePump()
		go client.ReadPump()
	}
}