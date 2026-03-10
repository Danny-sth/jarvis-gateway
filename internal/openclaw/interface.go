package openclaw

import "jarvis-gateway/internal/config"

// Client interface for OpenClaw communication
type Client interface {
	// SendMessage sends a message to the default user via agent
	SendMessage(message string) error
	// Send sends a message to a specific user via agent
	Send(message, userID string) (string, error)
}

// NewClient creates a new OpenClaw client (WebSocket-based)
func NewClient(cfg *config.Config) Client {
	return NewWSClient(cfg)
}
