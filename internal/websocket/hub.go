package websocket

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Connection represents a single WebSocket connection
type Connection struct {
	Conn      *websocket.Conn
	UserID    string
	DeviceID  string
	CreatedAt time.Time
	ctx       context.Context
	cancel    context.CancelFunc
}

// Message represents a message to send via WebSocket
type Message struct {
	Type            string    `json:"type"`                        // "response", "notification", "ping", "new_message"
	TaskID          string    `json:"task_id,omitempty"`           // Task ID this message relates to
	Text            string    `json:"text,omitempty"`              // Text response
	VoiceData       string    `json:"voice_data,omitempty"`        // Base64 encoded audio
	Waveform        []float64 `json:"waveform,omitempty"`          // Audio waveform for visualization
	AudioDurationMs int       `json:"audio_duration_ms,omitempty"` // Audio duration in milliseconds
	Error           string    `json:"error,omitempty"`             // Error message if any
	Timestamp       int64     `json:"timestamp"`                   // Unix timestamp

	// History sync fields (type: "new_message")
	MessageID      string `json:"message_id,omitempty"`      // Message UUID
	ConversationID string `json:"conversation_id,omitempty"` // Conversation UUID
	Role           string `json:"role,omitempty"`            // "user" or "assistant"
	Content        string `json:"content,omitempty"`         // Message content
}

// Hub manages all WebSocket connections
type Hub struct {
	mu          sync.RWMutex
	connections map[string][]*Connection // userID -> connections

	register   chan *Connection
	unregister chan *Connection
	broadcast  chan *broadcastMessage
}

type broadcastMessage struct {
	userID  string
	message *Message
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		connections: make(map[string][]*Connection),
		register:    make(chan *Connection, 256),
		unregister:  make(chan *Connection, 256),
		broadcast:   make(chan *broadcastMessage, 1024),
	}
}

// Run starts the hub's event loop
func (h *Hub) Run(ctx context.Context) {
	log.Println("[ws-hub] Starting WebSocket hub")

	for {
		select {
		case conn := <-h.register:
			h.addConnection(conn)

		case conn := <-h.unregister:
			h.removeConnection(conn)

		case msg := <-h.broadcast:
			h.sendToUser(msg.userID, msg.message)

		case <-ctx.Done():
			log.Println("[ws-hub] Shutting down")
			h.closeAllConnections()
			return
		}
	}
}

// Register adds a new connection to the hub
func (h *Hub) Register(conn *Connection) {
	h.register <- conn
}

// Unregister removes a connection from the hub
func (h *Hub) Unregister(conn *Connection) {
	h.unregister <- conn
}

// SendToUser sends a message to all connections of a user
func (h *Hub) SendToUser(userID string, msg *Message) {
	if msg.Timestamp == 0 {
		msg.Timestamp = time.Now().Unix()
	}
	h.broadcast <- &broadcastMessage{userID: userID, message: msg}
}

// GetConnectionCount returns the number of active connections for a user
func (h *Hub) GetConnectionCount(userID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections[userID])
}

// GetTotalConnections returns total number of all connections
func (h *Hub) GetTotalConnections() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	total := 0
	for _, conns := range h.connections {
		total += len(conns)
	}
	return total
}

func (h *Hub) addConnection(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.connections[conn.UserID] = append(h.connections[conn.UserID], conn)
	log.Printf("[ws-hub] Connection added: user=%s device=%s total=%d",
		conn.UserID, conn.DeviceID, len(h.connections[conn.UserID]))
}

func (h *Hub) removeConnection(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	conns := h.connections[conn.UserID]
	for i, c := range conns {
		if c == conn {
			// Remove connection from slice
			h.connections[conn.UserID] = append(conns[:i], conns[i+1:]...)
			break
		}
	}

	// Clean up empty user entry
	if len(h.connections[conn.UserID]) == 0 {
		delete(h.connections, conn.UserID)
	}

	// Cancel connection context
	if conn.cancel != nil {
		conn.cancel()
	}

	log.Printf("[ws-hub] Connection removed: user=%s device=%s",
		conn.UserID, conn.DeviceID)
}

func (h *Hub) sendToUser(userID string, msg *Message) {
	h.mu.RLock()
	conns := h.connections[userID]
	h.mu.RUnlock()

	if len(conns) == 0 {
		log.Printf("[ws-hub] No connections for user=%s, message dropped", userID)
		return
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[ws-hub] Failed to marshal message: %v", err)
		return
	}

	// Send to all user's connections
	for _, conn := range conns {
		go func(c *Connection) {
			ctx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
			defer cancel()

			if err := c.Conn.Write(ctx, websocket.MessageText, data); err != nil {
				log.Printf("[ws-hub] Failed to send to user=%s device=%s: %v",
					c.UserID, c.DeviceID, err)
				h.Unregister(c)
			}
		}(conn)
	}

	log.Printf("[ws-hub] Message sent to user=%s (%d connections)", userID, len(conns))
}

func (h *Hub) closeAllConnections() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for userID, conns := range h.connections {
		for _, conn := range conns {
			conn.Conn.Close(websocket.StatusGoingAway, "server shutdown")
			if conn.cancel != nil {
				conn.cancel()
			}
		}
		delete(h.connections, userID)
	}
}
