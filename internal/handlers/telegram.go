package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"jarvis-gateway/internal/config"
	"jarvis-gateway/internal/openclaw"
)

// Telegram Update structures
type TelegramUpdate struct {
	UpdateID int              `json:"update_id"`
	Message  *TelegramMessage `json:"message,omitempty"`
}

type TelegramMessage struct {
	MessageID int           `json:"message_id"`
	From      *TelegramUser `json:"from,omitempty"`
	Chat      *TelegramChat `json:"chat"`
	Text      string        `json:"text,omitempty"`
	Voice     *TelegramVoice `json:"voice,omitempty"`
	Audio     *TelegramAudio `json:"audio,omitempty"`
	Photo     []TelegramPhoto `json:"photo,omitempty"`
	Document  *TelegramDocument `json:"document,omitempty"`
}

type TelegramUser struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

type TelegramChat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"` // private, group, supergroup, channel
	Title     string `json:"title,omitempty"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

type TelegramVoice struct {
	FileID   string `json:"file_id"`
	Duration int    `json:"duration"`
	MimeType string `json:"mime_type,omitempty"`
	FileSize int    `json:"file_size,omitempty"`
}

type TelegramAudio struct {
	FileID   string `json:"file_id"`
	Duration int    `json:"duration"`
	MimeType string `json:"mime_type,omitempty"`
	FileSize int    `json:"file_size,omitempty"`
	Title    string `json:"title,omitempty"`
}

type TelegramPhoto struct {
	FileID   string `json:"file_id"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	FileSize int    `json:"file_size,omitempty"`
}

type TelegramDocument struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	FileSize int    `json:"file_size,omitempty"`
}

// Telegram creates a handler for Telegram webhook
func Telegram(cfg *config.Config) http.HandlerFunc {
	client := openclaw.NewClient(cfg)

	return func(w http.ResponseWriter, r *http.Request) {
		var update TelegramUpdate
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			log.Printf("[telegram] Failed to decode update: %v", err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Skip if no message
		if update.Message == nil {
			w.WriteHeader(http.StatusOK)
			return
		}

		msg := update.Message

		// Skip bot messages
		if msg.From != nil && msg.From.IsBot {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Get message text
		text := msg.Text
		if text == "" {
			// Handle voice/audio/photo with placeholder
			if msg.Voice != nil {
				text = "[Voice message]"
			} else if msg.Audio != nil {
				text = "[Audio file]"
			} else if len(msg.Photo) > 0 {
				text = "[Photo]"
			} else if msg.Document != nil {
				text = "[Document]"
			} else {
				// Unknown message type, skip
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		// Build user ID for session routing
		userID := formatTelegramUserID(msg.Chat.ID)

		log.Printf("[telegram] Message from %s (chat %d): %s",
			formatUserName(msg.From), msg.Chat.ID, truncate(text, 50))

		// Send to OpenClaw agent
		response, err := client.Send(text, userID)
		if err != nil {
			log.Printf("[telegram] Failed to send to agent: %v", err)
			// Return 200 to avoid Telegram retries, but log error
			w.WriteHeader(http.StatusOK)
			return
		}

		log.Printf("[telegram] Agent response: %s", truncate(response, 100))

		// OpenClaw agent handles delivery via --deliver flag
		// We just acknowledge the webhook
		w.WriteHeader(http.StatusOK)
	}
}

// formatTelegramUserID formats Telegram chat ID for OpenClaw session
func formatTelegramUserID(chatID int64) string {
	return "telegram:" + formatInt64(chatID)
}

// formatInt64 converts int64 to string
func formatInt64(n int64) string {
	if n == 0 {
		return "0"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}

	if negative {
		digits = append([]byte{'-'}, digits...)
	}

	return string(digits)
}

// formatUserName formats user name for logging
func formatUserName(user *TelegramUser) string {
	if user == nil {
		return "unknown"
	}
	if user.Username != "" {
		return "@" + user.Username
	}
	name := user.FirstName
	if user.LastName != "" {
		name += " " + user.LastName
	}
	return name
}

// truncate truncates string to max length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
