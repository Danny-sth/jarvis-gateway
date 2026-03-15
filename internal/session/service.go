package session

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Message represents a chat message
type Message struct {
	Role      string          `json:"role"`      // "user" or "assistant"
	Content   string          `json:"content"`
	ToolCalls json.RawMessage `json:"tool_calls,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

// Conversation represents a chat conversation
type Conversation struct {
	ID            string    `json:"id"`
	UserID        int64     `json:"user_id"`
	Title         string    `json:"title,omitempty"`
	StartedAt     time.Time `json:"started_at"`
	LastMessageAt time.Time `json:"last_message_at"`
	IsActive      bool      `json:"is_active"`
}

// Service handles conversation and message operations
type Service struct {
	db *sql.DB
}

// NewService creates a new session service
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// GetOrCreateConversationID gets or creates conversation and returns just the ID
// This is a convenience method for the handler interface
func (s *Service) GetOrCreateConversationID(userID int64) (string, error) {
	conv, err := s.GetOrCreateConversation(userID)
	if err != nil {
		return "", err
	}
	return conv.ID, nil
}

// GetOrCreateConversation gets the active conversation or creates a new one
// A new conversation is created if:
// - No active conversation exists
// - Last message was more than 24 hours ago
func (s *Service) GetOrCreateConversation(userID int64) (*Conversation, error) {
	// Try to get active conversation from last 24h
	query := `
		SELECT id, user_id, title, started_at, last_message_at, is_active
		FROM conversations
		WHERE user_id = $1
		  AND is_active = TRUE
		  AND last_message_at > NOW() - INTERVAL '24 hours'
		ORDER BY last_message_at DESC
		LIMIT 1
	`

	var conv Conversation
	var idStr string
	err := s.db.QueryRow(query, userID).Scan(
		&idStr, &conv.UserID, &conv.Title,
		&conv.StartedAt, &conv.LastMessageAt, &conv.IsActive,
	)

	if err == nil {
		conv.ID = idStr
		return &conv, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to query conversation: %w", err)
	}

	// Create new conversation
	newID := uuid.New().String()
	insertQuery := `
		INSERT INTO conversations (id, user_id, is_active)
		VALUES ($1, $2, TRUE)
		RETURNING started_at, last_message_at
	`

	err = s.db.QueryRow(insertQuery, newID, userID).Scan(
		&conv.StartedAt, &conv.LastMessageAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	conv.ID = newID
	conv.UserID = userID
	conv.IsActive = true

	return &conv, nil
}

// SaveMessageSimple saves a message without tool calls (for handler interface)
func (s *Service) SaveMessageSimple(conversationID string, role, content string) error {
	return s.SaveMessage(conversationID, role, content, nil)
}

// SaveMessage saves a message to the conversation
func (s *Service) SaveMessage(conversationID string, role, content string, toolCalls json.RawMessage) error {
	query := `
		INSERT INTO messages (conversation_id, role, content, tool_calls)
		VALUES ($1, $2, $3, $4)
	`

	var tc interface{}
	if len(toolCalls) > 0 {
		tc = toolCalls
	}

	_, err := s.db.Exec(query, conversationID, role, content, tc)
	if err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}

	// Update conversation last_message_at
	_, err = s.db.Exec(
		"UPDATE conversations SET last_message_at = NOW() WHERE id = $1",
		conversationID,
	)
	return err
}

// GetRecentMessages returns the last N messages from a conversation
func (s *Service) GetRecentMessages(conversationID string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT role, content, tool_calls, created_at
		FROM messages
		WHERE conversation_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := s.db.Query(query, conversationID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var toolCalls sql.NullString
		if err := rows.Scan(&msg.Role, &msg.Content, &toolCalls, &msg.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		if toolCalls.Valid {
			msg.ToolCalls = json.RawMessage(toolCalls.String)
		}
		messages = append(messages, msg)
	}

	// Reverse to get chronological order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// GetConversationHistory returns messages in a format suitable for the LLM
func (s *Service) GetConversationHistory(userID int64, limit int) (string, []Message, error) {
	conv, err := s.GetOrCreateConversation(userID)
	if err != nil {
		return "", nil, err
	}

	messages, err := s.GetRecentMessages(conv.ID, limit)
	if err != nil {
		return "", nil, err
	}

	return conv.ID, messages, nil
}

// EndConversation marks a conversation as inactive
func (s *Service) EndConversation(conversationID string) error {
	_, err := s.db.Exec(
		"UPDATE conversations SET is_active = FALSE WHERE id = $1",
		conversationID,
	)
	return err
}

// SetConversationTitle updates the conversation title
func (s *Service) SetConversationTitle(conversationID, title string) error {
	_, err := s.db.Exec(
		"UPDATE conversations SET title = $1 WHERE id = $2",
		title, conversationID,
	)
	return err
}

// GetUserConversations returns all conversations for a user
func (s *Service) GetUserConversations(userID int64, limit int) ([]Conversation, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT id, user_id, title, started_at, last_message_at, is_active
		FROM conversations
		WHERE user_id = $1
		ORDER BY last_message_at DESC
		LIMIT $2
	`

	rows, err := s.db.Query(query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query conversations: %w", err)
	}
	defer rows.Close()

	var conversations []Conversation
	for rows.Next() {
		var conv Conversation
		var title sql.NullString
		if err := rows.Scan(
			&conv.ID, &conv.UserID, &title,
			&conv.StartedAt, &conv.LastMessageAt, &conv.IsActive,
		); err != nil {
			return nil, fmt.Errorf("failed to scan conversation: %w", err)
		}
		if title.Valid {
			conv.Title = title.String
		}
		conversations = append(conversations, conv)
	}

	return conversations, nil
}
