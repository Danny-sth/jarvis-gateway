package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"jarvis-gateway/internal/config"
	"jarvis-gateway/internal/openclaw"
)

// Telegram Update structures
type TelegramUpdate struct {
	UpdateID int              `json:"update_id"`
	Message  *TelegramMessage `json:"message,omitempty"`
}

type TelegramMessage struct {
	MessageID int               `json:"message_id"`
	From      *TelegramUser     `json:"from,omitempty"`
	Chat      *TelegramChat     `json:"chat"`
	Text      string            `json:"text,omitempty"`
	Voice     *TelegramVoice    `json:"voice,omitempty"`
	Audio     *TelegramAudio    `json:"audio,omitempty"`
	Photo     []TelegramPhoto   `json:"photo,omitempty"`
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
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Duration     int    `json:"duration"`
	MimeType     string `json:"mime_type,omitempty"`
	FileSize     int    `json:"file_size,omitempty"`
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

// TelegramFileResponse is the response from getFile API
type TelegramFileResponse struct {
	OK     bool `json:"ok"`
	Result struct {
		FileID   string `json:"file_id"`
		FilePath string `json:"file_path"`
	} `json:"result"`
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
			// Handle voice message with STT
			if msg.Voice != nil {
				transcribed, err := transcribeVoice(cfg, msg.Voice.FileID)
				if err != nil {
					log.Printf("[telegram] STT failed: %v", err)
					text = "[Voice message - STT failed]"
				} else {
					text = transcribed
					log.Printf("[telegram] STT result: %s", truncate(text, 100))
				}
			} else if msg.Audio != nil {
				transcribed, err := transcribeVoice(cfg, msg.Audio.FileID)
				if err != nil {
					log.Printf("[telegram] STT failed for audio: %v", err)
					text = "[Audio file - STT failed]"
				} else {
					text = transcribed
				}
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

// transcribeVoice downloads a voice file from Telegram and runs STT
func transcribeVoice(cfg *config.Config, fileID string) (string, error) {
	botToken := cfg.Telegram.BotToken
	if botToken == "" {
		return "", fmt.Errorf("telegram bot token not configured")
	}

	// Step 1: Get file path from Telegram
	getFileURL := fmt.Sprintf("https://api.telegram.org/bot%s/getFile?file_id=%s", botToken, fileID)
	resp, err := http.Get(getFileURL)
	if err != nil {
		return "", fmt.Errorf("failed to get file info: %w", err)
	}
	defer resp.Body.Close()

	var fileResp TelegramFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&fileResp); err != nil {
		return "", fmt.Errorf("failed to decode file response: %w", err)
	}

	if !fileResp.OK || fileResp.Result.FilePath == "" {
		return "", fmt.Errorf("telegram API returned error or empty path")
	}

	// Step 2: Download the file
	downloadURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", botToken, fileResp.Result.FilePath)
	fileResp2, err := http.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}
	defer fileResp2.Body.Close()

	// Save to temp file
	ext := filepath.Ext(fileResp.Result.FilePath)
	if ext == "" {
		ext = ".ogg" // Telegram voice messages are OGG by default
	}
	tmpFile, err := os.CreateTemp("", "telegram_voice_*"+ext)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, fileResp2.Body); err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}
	tmpFile.Close()

	// Step 3: Run STT
	sttCommand := cfg.Voice.STTCommand
	if sttCommand == "" {
		sttCommand = "/usr/local/bin/whisper-stt"
	}

	log.Printf("[telegram] Running STT: %s %s", sttCommand, tmpFile.Name())
	cmd := exec.Command(sttCommand, tmpFile.Name())
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("STT failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("STT command failed: %w", err)
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return "", fmt.Errorf("STT returned empty result")
	}

	return result, nil
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
