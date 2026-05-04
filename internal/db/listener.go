package db

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/lib/pq"
)

// MessageNotification represents a new message notification from PostgreSQL
type MessageNotification struct {
	KeycloakSub    string `json:"keycloak_sub"`
	ConversationID string `json:"conversation_id"`
	MessageID      string `json:"message_id"`
	Role           string `json:"role"`
	Content        string `json:"content"`
	SourceChannel  string `json:"source_channel"`
	CreatedAt      string `json:"created_at"`
}

// MessageHandler is called when a new message notification is received
type MessageHandler func(notification *MessageNotification)

// Listener listens for PostgreSQL NOTIFY events
type Listener struct {
	connStr string
	handler MessageHandler
	pqListener *pq.Listener
}

// NewListener creates a new PostgreSQL listener
func NewListener(cfg Config, handler MessageHandler) *Listener {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name,
	)
	return &Listener{
		connStr: connStr,
		handler: handler,
	}
}

// Start begins listening for notifications
func (l *Listener) Start(ctx context.Context) error {
	// Create listener with reconnect logic
	l.pqListener = pq.NewListener(
		l.connStr,
		10*time.Second, // min reconnect interval
		time.Minute,    // max reconnect interval
		func(event pq.ListenerEventType, err error) {
			switch event {
			case pq.ListenerEventConnected:
				log.Printf("[db-listener] Connected to PostgreSQL")
			case pq.ListenerEventDisconnected:
				log.Printf("[db-listener] Disconnected from PostgreSQL: %v", err)
			case pq.ListenerEventReconnected:
				log.Printf("[db-listener] Reconnected to PostgreSQL")
			case pq.ListenerEventConnectionAttemptFailed:
				log.Printf("[db-listener] Connection attempt failed: %v", err)
			}
		},
	)

	// Subscribe to channel
	if err := l.pqListener.Listen("messages_changed"); err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	log.Printf("[db-listener] Listening for 'messages_changed' notifications")

	// Process notifications in goroutine
	go l.processNotifications(ctx)

	return nil
}

// processNotifications handles incoming notifications
func (l *Listener) processNotifications(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Printf("[db-listener] Shutting down")
			return

		case notification := <-l.pqListener.Notify:
			if notification == nil {
				// Connection lost, listener will reconnect
				continue
			}

			// Parse notification payload
			var msg MessageNotification
			if err := json.Unmarshal([]byte(notification.Extra), &msg); err != nil {
				log.Printf("[db-listener] Failed to parse notification: %v", err)
				continue
			}

			log.Printf("[db-listener] New message: user=%s, role=%s, source=%s, conv=%s",
				msg.KeycloakSub, msg.Role, msg.SourceChannel, msg.ConversationID[:8])

			// Call handler
			if l.handler != nil {
				l.handler(&msg)
			}

		case <-time.After(90 * time.Second):
			// Ping to keep connection alive
			if err := l.pqListener.Ping(); err != nil {
				log.Printf("[db-listener] Ping failed: %v", err)
			}
		}
	}
}

// Close closes the listener
func (l *Listener) Close() error {
	if l.pqListener != nil {
		return l.pqListener.Close()
	}
	return nil
}
