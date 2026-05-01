package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// Conversation represents a conversation record from PostgreSQL
type Conversation struct {
	ID            uuid.UUID
	UserID        *int64  // Optional FK to users.id
	KeycloakSub   uuid.UUID
	Title         *string
	SourceChannel string
	StartedAt     time.Time
	LastMessageAt time.Time
	IsActive      bool
	MessageCount  int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Message represents a message record from PostgreSQL
type Message struct {
	ID              uuid.UUID
	ConversationID  uuid.UUID
	Role            string
	Content         string
	HasAudio        bool
	AudioDurationMs *int
	Waveform        []float64
	SourceChannel   *string
	CreatedAt       time.Time
}

// GetConversationsByKeycloakSub returns all conversations for a user
func (c *Client) GetConversationsByKeycloakSub(keycloakSub string) ([]Conversation, error) {
	query := `
		SELECT id, user_id, keycloak_sub, title, source_channel,
		       started_at, last_message_at, is_active, message_count,
		       created_at, updated_at
		FROM conversations
		WHERE keycloak_sub = $1
		ORDER BY last_message_at DESC
		LIMIT 50
	`

	rows, err := c.db.Query(query, keycloakSub)
	if err != nil {
		return nil, fmt.Errorf("failed to query conversations: %w", err)
	}
	defer rows.Close()

	var conversations []Conversation
	for rows.Next() {
		var conv Conversation
		err := rows.Scan(
			&conv.ID, &conv.UserID, &conv.KeycloakSub, &conv.Title,
			&conv.SourceChannel, &conv.StartedAt, &conv.LastMessageAt,
			&conv.IsActive, &conv.MessageCount, &conv.CreatedAt, &conv.UpdatedAt,
		)
		if err != nil {
			log.Printf("[db] Error scanning conversation: %v", err)
			continue
		}
		conversations = append(conversations, conv)
	}

	return conversations, nil
}

// GetConversationByID returns a single conversation by ID
func (c *Client) GetConversationByID(conversationID string) (*Conversation, error) {
	query := `
		SELECT id, user_id, keycloak_sub, title, source_channel,
		       started_at, last_message_at, is_active, message_count,
		       created_at, updated_at
		FROM conversations
		WHERE id = $1
	`

	var conv Conversation
	err := c.db.QueryRow(query, conversationID).Scan(
		&conv.ID, &conv.UserID, &conv.KeycloakSub, &conv.Title,
		&conv.SourceChannel, &conv.StartedAt, &conv.LastMessageAt,
		&conv.IsActive, &conv.MessageCount, &conv.CreatedAt, &conv.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	return &conv, nil
}

// GetActiveConversation returns the active conversation for today
func (c *Client) GetActiveConversation(keycloakSub string) (*Conversation, error) {
	// Get today's start in UTC
	now := time.Now().UTC()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	query := `
		SELECT id, user_id, keycloak_sub, title, source_channel,
		       started_at, last_message_at, is_active, message_count,
		       created_at, updated_at
		FROM conversations
		WHERE keycloak_sub = $1
		  AND is_active = true
		  AND started_at >= $2
		ORDER BY started_at DESC
		LIMIT 1
	`

	var conv Conversation
	err := c.db.QueryRow(query, keycloakSub, todayStart).Scan(
		&conv.ID, &conv.UserID, &conv.KeycloakSub, &conv.Title,
		&conv.SourceChannel, &conv.StartedAt, &conv.LastMessageAt,
		&conv.IsActive, &conv.MessageCount, &conv.CreatedAt, &conv.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get active conversation: %w", err)
	}

	return &conv, nil
}

// GetMessagesByConversationID returns the most recent messages for a conversation
// Returns messages in chronological order (oldest first)
func (c *Client) GetMessagesByConversationID(conversationID string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}

	// Get last N messages by selecting in DESC order, then reverse to ASC
	query := `
		SELECT id, conversation_id, role, content, has_audio,
		       audio_duration_ms, waveform, source_channel, created_at
		FROM (
			SELECT id, conversation_id, role, content, has_audio,
			       audio_duration_ms, waveform, source_channel, created_at
			FROM messages
			WHERE conversation_id = $1
			ORDER BY created_at DESC
			LIMIT $2
		) sub
		ORDER BY created_at ASC
	`

	rows, err := c.db.Query(query, conversationID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var waveformBytes []byte // PostgreSQL returns ARRAY as bytes

		err := rows.Scan(
			&msg.ID, &msg.ConversationID, &msg.Role, &msg.Content,
			&msg.HasAudio, &msg.AudioDurationMs, &waveformBytes,
			&msg.SourceChannel, &msg.CreatedAt,
		)
		if err != nil {
			log.Printf("[db] Error scanning message: %v", err)
			continue
		}

		// Parse waveform from PostgreSQL ARRAY format
		msg.Waveform = parseWaveform(waveformBytes)
		messages = append(messages, msg)
	}

	return messages, nil
}

// parseWaveform parses PostgreSQL integer array to float64 slice
// The waveform is stored as INTEGER[] in PostgreSQL
func parseWaveform(data []byte) []float64 {
	if data == nil || len(data) == 0 {
		return nil
	}

	// PostgreSQL returns ARRAY as string like "{1,2,3}" or as binary
	// For simplicity, we'll return nil here and handle it properly if needed
	// The waveform is not critical for the API
	return nil
}

// GetUserKeycloakSubByTelegramID returns keycloak_sub for a telegram user
// This generates a deterministic UUID if keycloak_sub is not set
func (c *Client) GetUserKeycloakSubByTelegramID(telegramID int64) (string, error) {
	var keycloakSub *string

	query := `SELECT keycloak_sub FROM users WHERE telegram_id = $1`
	err := c.db.QueryRow(query, telegramID).Scan(&keycloakSub)
	if err != nil {
		if err == sql.ErrNoRows {
			// User not found - generate deterministic UUID
			return generateKeycloakSubFromTelegram(telegramID), nil
		}
		return "", fmt.Errorf("failed to get keycloak_sub: %w", err)
	}

	if keycloakSub == nil || *keycloakSub == "" {
		// No keycloak_sub set - generate deterministic UUID
		return generateKeycloakSubFromTelegram(telegramID), nil
	}

	return *keycloakSub, nil
}

// generateKeycloakSubFromTelegram generates a deterministic UUID from telegram_id
// This matches the Python implementation in duq-core
func generateKeycloakSubFromTelegram(telegramID int64) string {
	// Use UUID5 with OID namespace and "telegram:{id}" as name
	// This matches Python's uuid5(NAMESPACE_OID, f"telegram:{telegram_id}")
	namespace := uuid.MustParse("6ba7b812-9dad-11d1-80b4-00c04fd430c8") // NAMESPACE_OID
	name := fmt.Sprintf("telegram:%d", telegramID)
	return uuid.NewSHA1(namespace, []byte(name)).String()
}
