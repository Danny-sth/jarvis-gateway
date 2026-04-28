package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"duq-gateway/internal/config"
	"duq-gateway/internal/db"
	"duq-gateway/internal/queue"
)

// HistoryDeps - dependencies for history handlers
type HistoryDeps struct {
	Config      *config.Config
	DBClient    *db.Client
	QueueClient *queue.Client
}

// ConversationResponse - matches Android app expectations
type ConversationResponse struct {
	ID            string `json:"id"`
	UserID        int64  `json:"user_id"`
	Title         string `json:"title"`
	StartedAt     int64  `json:"started_at"`
	LastMessageAt int64  `json:"last_message_at"`
	IsActive      bool   `json:"is_active"`
}

// MessageResponse - matches Android app expectations
type MessageResponse struct {
	ID              string    `json:"id"`
	ConversationID  string    `json:"conversation_id"`
	Role            string    `json:"role"`
	Content         string    `json:"content"`
	HasAudio        bool      `json:"has_audio"`
	AudioDurationMs *int      `json:"audio_duration_ms"`
	Waveform        []float64 `json:"waveform"`
	CreatedAt       int64     `json:"created_at"`
}

// GetConversations returns list of conversations from PostgreSQL
// GET /api/conversations
func GetConversations(deps *HistoryDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get user from context (set by KeycloakAuth middleware)
		keycloakSub, ok := r.Context().Value("keycloak_sub").(string)
		if !ok || keycloakSub == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		// Get telegram_id for response (optional)
		telegramID, _ := r.Context().Value("telegram_id").(int64)
		log.Printf("[history] Getting conversations for keycloak_sub=%s", keycloakSub)

		// Get conversations from PostgreSQL
		conversations, err := deps.DBClient.GetConversationsByKeycloakSub(keycloakSub)
		if err != nil {
			log.Printf("[history] Error getting conversations: %v", err)
			http.Error(w, `{"error":"failed to get conversations"}`, http.StatusInternalServerError)
			return
		}

		// Convert to response format
		var response []ConversationResponse
		for _, conv := range conversations {
			title := formatDateTitle(conv.StartedAt)
			if conv.Title != nil && *conv.Title != "" {
				title = *conv.Title
			}

			resp := ConversationResponse{
				ID:            conv.ID.String(),
				UserID:        telegramID,
				Title:         title,
				StartedAt:     conv.StartedAt.Unix(),
				LastMessageAt: conv.LastMessageAt.Unix(),
				IsActive:      conv.IsActive,
			}
			response = append(response, resp)
		}

		// Return empty array if no conversations
		if response == nil {
			response = []ConversationResponse{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		log.Printf("[history] Returned %d conversations for keycloak_sub=%s", len(response), keycloakSub)
	}
}

// GetMessages returns messages for a conversation from PostgreSQL
// GET /api/conversations/{conversation_id}/messages
func GetMessages(deps *HistoryDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get user from context
		keycloakSub, ok := r.Context().Value("keycloak_sub").(string)
		if !ok || keycloakSub == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		conversationID := r.PathValue("conversation_id")
		if conversationID == "" {
			// Try legacy parameter name
			conversationID = r.PathValue("session_id")
		}
		if conversationID == "" {
			http.Error(w, `{"error":"conversation_id required"}`, http.StatusBadRequest)
			return
		}
		log.Printf("[history] Getting messages for conversation=%s, keycloak_sub=%s", conversationID, keycloakSub)

		// Verify conversation belongs to user
		conv, err := deps.DBClient.GetConversationByID(conversationID)
		if err != nil {
			log.Printf("[history] Error getting conversation: %v", err)
			http.Error(w, `{"error":"failed to get conversation"}`, http.StatusInternalServerError)
			return
		}
		if conv == nil || conv.KeycloakSub.String() != keycloakSub {
			http.Error(w, `{"error":"conversation not found"}`, http.StatusNotFound)
			return
		}

		// Get messages from PostgreSQL
		messages, err := deps.DBClient.GetMessagesByConversationID(conversationID, 100)
		if err != nil {
			log.Printf("[history] Error getting messages: %v", err)
			http.Error(w, `{"error":"failed to get messages"}`, http.StatusInternalServerError)
			return
		}

		// Convert to response format
		var response []MessageResponse
		for _, msg := range messages {
			resp := MessageResponse{
				ID:              msg.ID.String(),
				ConversationID:  msg.ConversationID.String(),
				Role:            msg.Role,
				Content:         msg.Content,
				HasAudio:        msg.HasAudio,
				AudioDurationMs: msg.AudioDurationMs,
				Waveform:        msg.Waveform,
				CreatedAt:       msg.CreatedAt.Unix(),
			}
			response = append(response, resp)
		}

		// Return empty array if no messages
		if response == nil {
			response = []MessageResponse{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		log.Printf("[history] Returned %d messages for conversation=%s", len(response), conversationID)
	}
}

// CreateConversation returns active conversation or placeholder for new one
// POST /api/conversations
// Note: Actual conversation creation happens in duq-core when first message is sent
func CreateConversation(deps *HistoryDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get user from context
		keycloakSub, ok := r.Context().Value("keycloak_sub").(string)
		if !ok || keycloakSub == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		// Get telegram_id for response (optional)
		telegramID, _ := r.Context().Value("telegram_id").(int64)

		// Try to get active conversation from PostgreSQL
		conv, err := deps.DBClient.GetActiveConversation(keycloakSub)
		if err != nil {
			log.Printf("[history] Error getting active conversation: %v", err)
		}

		if conv != nil {
			// Return existing active conversation
			title := formatDateTitle(conv.StartedAt)
			if conv.Title != nil && *conv.Title != "" {
				title = *conv.Title
			}

			resp := ConversationResponse{
				ID:            conv.ID.String(),
				UserID:        telegramID,
				Title:         title,
				StartedAt:     conv.StartedAt.Unix(),
				LastMessageAt: conv.LastMessageAt.Unix(),
				IsActive:      true,
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			log.Printf("[history] Returned existing active conversation %s for keycloak_sub=%s", conv.ID.String(), keycloakSub)
			return
		}

		// No active conversation - return placeholder
		// Actual creation happens in duq-core on first message
		now := time.Now().UTC()
		resp := ConversationResponse{
			ID:            "", // Will be set by duq-core
			UserID:        telegramID,
			Title:         formatDateTitle(now),
			StartedAt:     now.Unix(),
			LastMessageAt: now.Unix(),
			IsActive:      true,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		log.Printf("[history] Returned placeholder conversation for keycloak_sub=%s", keycloakSub)
	}
}

// formatDateTitle formats a date for display
func formatDateTitle(t time.Time) string {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	sessionDay := t.Truncate(24 * time.Hour)

	if sessionDay.Equal(today) {
		return "Today"
	}
	if sessionDay.Equal(today.AddDate(0, 0, -1)) {
		return "Yesterday"
	}
	return t.Format("January 2, 2006")
}
