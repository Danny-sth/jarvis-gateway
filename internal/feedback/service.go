package feedback

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// FeedbackType represents the type of feedback
type FeedbackType string

const (
	FeedbackPositive FeedbackType = "positive"
	FeedbackNegative FeedbackType = "negative"
)

// Feedback represents user feedback on a bot response
type Feedback struct {
	ID              int64
	UserID          int64  // telegram_id
	KeycloakSub     string // for multi-channel correlation
	MessageID       int64  // telegram message_id of bot's response
	ConversationID  string // conversation UUID
	FeedbackType    FeedbackType
	ResponsePreview string // first 200 chars of response
	CreatedAt       time.Time
}

// Service handles feedback storage and retrieval
type Service struct {
	db *sql.DB
}

// NewService creates a new feedback service
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// EnsureTable creates the feedback table if it doesn't exist
func (s *Service) EnsureTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS message_feedback (
		id SERIAL PRIMARY KEY,
		user_id BIGINT NOT NULL,
		keycloak_sub UUID,
		message_id BIGINT NOT NULL,
		conversation_id UUID,
		feedback_type VARCHAR(20) NOT NULL,
		response_preview TEXT,
		created_at TIMESTAMP DEFAULT NOW(),

		UNIQUE(user_id, message_id)
	);

	CREATE INDEX IF NOT EXISTS idx_feedback_user ON message_feedback(user_id);
	CREATE INDEX IF NOT EXISTS idx_feedback_keycloak ON message_feedback(keycloak_sub);
	CREATE INDEX IF NOT EXISTS idx_feedback_type ON message_feedback(feedback_type);
	`

	_, err := s.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create feedback table: %w", err)
	}

	log.Printf("[feedback] Table ensured")
	return nil
}

// Save stores feedback, updating if already exists (user changed their mind)
func (s *Service) Save(f *Feedback) error {
	query := `
	INSERT INTO message_feedback (user_id, keycloak_sub, message_id, conversation_id, feedback_type, response_preview)
	VALUES ($1, NULLIF($2, '')::UUID, $3, NULLIF($4, '')::UUID, $5, $6)
	ON CONFLICT (user_id, message_id) DO UPDATE SET
		feedback_type = EXCLUDED.feedback_type,
		created_at = NOW()
	RETURNING id
	`

	err := s.db.QueryRow(
		query,
		f.UserID,
		f.KeycloakSub,
		f.MessageID,
		f.ConversationID,
		f.FeedbackType,
		truncatePreview(f.ResponsePreview, 200),
	).Scan(&f.ID)

	if err != nil {
		return fmt.Errorf("failed to save feedback: %w", err)
	}

	log.Printf("[feedback] Saved: user=%d msg=%d type=%s", f.UserID, f.MessageID, f.FeedbackType)
	return nil
}

// SaveData saves feedback from FeedbackData (for interface compatibility with handlers)
// This method allows handlers to pass their FeedbackData struct without circular imports
func (s *Service) SaveData(userID int64, keycloakSub string, messageID int64, conversationID string, feedbackType string, responsePreview string) error {
	f := &Feedback{
		UserID:          userID,
		KeycloakSub:     keycloakSub,
		MessageID:       messageID,
		ConversationID:  conversationID,
		FeedbackType:    FeedbackType(feedbackType),
		ResponsePreview: responsePreview,
	}
	return s.Save(f)
}

// GetStats returns feedback statistics for a user
func (s *Service) GetStats(userID int64) (positive, negative int, err error) {
	query := `
	SELECT
		COALESCE(SUM(CASE WHEN feedback_type = 'positive' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN feedback_type = 'negative' THEN 1 ELSE 0 END), 0)
	FROM message_feedback
	WHERE user_id = $1
	`

	err = s.db.QueryRow(query, userID).Scan(&positive, &negative)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get stats: %w", err)
	}

	return positive, negative, nil
}

// GetGlobalStats returns overall feedback statistics
func (s *Service) GetGlobalStats() (positive, negative int, err error) {
	query := `
	SELECT
		COALESCE(SUM(CASE WHEN feedback_type = 'positive' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN feedback_type = 'negative' THEN 1 ELSE 0 END), 0)
	FROM message_feedback
	`

	err = s.db.QueryRow(query).Scan(&positive, &negative)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get global stats: %w", err)
	}

	return positive, negative, nil
}

func truncatePreview(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
