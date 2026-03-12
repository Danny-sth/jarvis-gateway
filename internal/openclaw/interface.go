package openclaw

import "jarvis-gateway/internal/config"

// Client interface for OpenClaw communication
type Client interface {
	// SendMessage sends a message to the default user via agent
	SendMessage(message string) error
	// Send sends a message to a specific user via agent with --deliver
	Send(message, userID string) (string, error)
	// SendWithoutDeliver sends a message without --deliver flag (for custom delivery)
	SendWithoutDeliver(message, userID string) (string, error)
}

// NewClient creates a new OpenClaw client (CLI-based for reliable delivery)
func NewClient(cfg *config.Config) Client {
	return NewCLIClient(cfg)
}
