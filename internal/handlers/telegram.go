package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"jarvis-gateway/internal/config"
	"jarvis-gateway/internal/session"
	"jarvis-gateway/internal/vtoroy"
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
	Type      string `json:"type"`
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

type TelegramFileResponse struct {
	OK     bool `json:"ok"`
	Result struct {
		FileID   string `json:"file_id"`
		FilePath string `json:"file_path"`
	} `json:"result"`
}

// TelegramDeps contains dependencies for the Telegram handler
type TelegramDeps struct {
	Config         *config.Config
	VtoroyClient   *vtoroy.Client
	RBACService    RBACServiceInterface
	SessionService SessionServiceInterface
}

// RBACServiceInterface for RBAC operations
type RBACServiceInterface interface {
	GetAllowedTools(userID int64) ([]string, error)
	EnsureUser(userID int64, username, firstName, lastName string) error
}

// SessionServiceInterface for conversation operations
type SessionServiceInterface interface {
	GetOrCreateConversationID(userID int64) (string, error)
	GetRecentMessagesSimple(conversationID string, limit int) ([]session.HistoryMessage, error)
	SaveMessageSimple(conversationID string, role, content string) error
}

// Telegram creates a handler for Telegram webhook (legacy, without RBAC)
func Telegram(cfg *config.Config) http.HandlerFunc {
	client := vtoroy.NewClient(cfg)
	return TelegramWithDeps(&TelegramDeps{
		Config:       cfg,
		VtoroyClient: client,
	})
}

// TelegramWithDeps creates a handler with full dependencies
func TelegramWithDeps(deps *TelegramDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var update TelegramUpdate
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			log.Printf("[telegram] Failed to decode update: %v", err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if update.Message == nil {
			w.WriteHeader(http.StatusOK)
			return
		}

		msg := update.Message

		if msg.From != nil && msg.From.IsBot {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Check if this is a voice message
		isVoice := msg.Voice != nil || msg.Audio != nil

		// Get message text
		text := msg.Text
		if text == "" {
			if msg.Voice != nil {
				transcribed, err := transcribeVoice(deps.Config, msg.Voice.FileID)
				if err != nil {
					log.Printf("[telegram] STT failed: %v", err)
					text = "[Voice message - STT failed]"
				} else {
					text = transcribed
					log.Printf("[telegram] STT result: %s", truncateStr(text, 100))
				}
			} else if msg.Audio != nil {
				transcribed, err := transcribeVoice(deps.Config, msg.Audio.FileID)
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
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		userID := formatTelegramUserID(msg.Chat.ID)
		telegramID := msg.Chat.ID

		log.Printf("[telegram] Message from %s (chat %d): %s",
			formatUserName(msg.From), msg.Chat.ID, truncateStr(text, 50))

		// Build chat options
		var opts vtoroy.ChatOptions

		// Get allowed tools from RBAC if available
		if deps.RBACService != nil {
			// Ensure user exists
			username := ""
			firstName := ""
			lastName := ""
			if msg.From != nil {
				username = msg.From.Username
				firstName = msg.From.FirstName
				lastName = msg.From.LastName
			}
			deps.RBACService.EnsureUser(telegramID, username, firstName, lastName)

			tools, err := deps.RBACService.GetAllowedTools(telegramID)
			if err != nil {
				log.Printf("[telegram] RBAC error: %v", err)
			} else {
				opts.AllowedTools = tools
				log.Printf("[telegram] User %d has %d allowed tools", telegramID, len(tools))
			}
		}

		// Get conversation history if available
		if deps.SessionService != nil {
			convID, err := deps.SessionService.GetOrCreateConversationID(telegramID)
			if err != nil {
				log.Printf("[telegram] Session error: %v", err)
			} else {
				opts.ConversationID = convID

				// Get recent messages for context
				messages, err := deps.SessionService.GetRecentMessagesSimple(convID, 20)
				if err != nil {
					log.Printf("[telegram] History error: %v", err)
				} else {
					for _, m := range messages {
						opts.History = append(opts.History, vtoroy.HistoryMessage{
							Role:    m.Role,
							Content: m.Content,
						})
					}
				}

				// Save user message
				if err := deps.SessionService.SaveMessageSimple(convID, "user", text); err != nil {
					log.Printf("[telegram] Failed to save user message: %v", err)
				}
			}
		}

		// Send to Vtoroy agent
		response, err := deps.VtoroyClient.SendWithOptions(text, userID, opts)
		if err != nil {
			log.Printf("[telegram] Failed to send to agent: %v", err)
			w.WriteHeader(http.StatusOK)
			return
		}

		log.Printf("[telegram] Agent response: %s", truncateStr(response, 100))

		// Save assistant response
		if deps.SessionService != nil && opts.ConversationID != "" && response != "" {
			if err := deps.SessionService.SaveMessageSimple(opts.ConversationID, "assistant", response); err != nil {
				log.Printf("[telegram] Failed to save assistant message: %v", err)
			}
		}

		if response == "" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Send response back to Telegram
		go func() {
			if isVoice {
				// Voice input -> voice + text response
				const maxCaptionLen = 1024

				if len(response) <= maxCaptionLen {
					// Short response: send voice with caption (single message)
					if err := sendTelegramVoiceWithCaption(deps.Config, msg.Chat.ID, response, response); err != nil {
						log.Printf("[telegram] Failed to send voice with caption: %v", err)
					}
				} else {
					// Long response: send text first, then voice without caption
					if err := sendTelegramMessage(deps.Config, msg.Chat.ID, response); err != nil {
						log.Printf("[telegram] Failed to send text: %v", err)
					}
					if err := sendTelegramVoiceWithCaption(deps.Config, msg.Chat.ID, response, ""); err != nil {
						log.Printf("[telegram] Failed to send voice: %v", err)
					}
				}
			} else {
				// Text input -> text response
				if err := sendTelegramMessage(deps.Config, msg.Chat.ID, response); err != nil {
					log.Printf("[telegram] Failed to send text response: %v", err)
				}
			}
		}()

		w.WriteHeader(http.StatusOK)
	}
}

// transcribeVoice downloads a voice file from Telegram and runs STT
func transcribeVoice(cfg *config.Config, fileID string) (string, error) {
	botToken := cfg.Telegram.BotToken
	if botToken == "" {
		return "", fmt.Errorf("telegram bot token not configured")
	}

	// Get file path from Telegram
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

	// Download the file
	downloadURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", botToken, fileResp.Result.FilePath)
	fileResp2, err := http.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}
	defer fileResp2.Body.Close()

	// Save to temp file (original format)
	ext := filepath.Ext(fileResp.Result.FilePath)
	if ext == "" {
		ext = ".ogg"
	}
	tmpOrig, err := os.CreateTemp("", "telegram_voice_*"+ext)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpOrig.Name())
	defer tmpOrig.Close()

	if _, err := io.Copy(tmpOrig, fileResp2.Body); err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}
	tmpOrig.Close()

	// Convert to WAV using ffmpeg for better compatibility
	tmpWav := strings.TrimSuffix(tmpOrig.Name(), ext) + ".wav"
	defer os.Remove(tmpWav)

	log.Printf("[telegram] Converting %s to WAV", tmpOrig.Name())
	convertCmd := exec.Command("ffmpeg", "-y", "-i", tmpOrig.Name(), "-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", tmpWav)
	if convertOutput, err := convertCmd.CombinedOutput(); err != nil {
		log.Printf("[telegram] ffmpeg error: %s", string(convertOutput))
		return "", fmt.Errorf("failed to convert audio: %w", err)
	}

	// Run STT on WAV file
	sttCommand := cfg.Voice.STTCommand
	if sttCommand == "" {
		sttCommand = "/usr/local/bin/whisper-stt"
	}

	log.Printf("[telegram] Running STT: %s %s", sttCommand, tmpWav)
	cmd := exec.Command(sttCommand, tmpWav)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[telegram] STT error output: %s", string(output))
		return "", fmt.Errorf("STT command failed: %w", err)
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return "", fmt.Errorf("STT returned empty result")
	}

	return result, nil
}

// sendTelegramMessage sends a text message to Telegram
func sendTelegramMessage(cfg *config.Config, chatID int64, text string) error {
	botToken := cfg.Telegram.BotToken
	if botToken == "" {
		return fmt.Errorf("telegram bot token not configured")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}

	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error: %s", string(body))
	}

	log.Printf("[telegram] Sent text message to %d", chatID)
	return nil
}

// sendTelegramVoiceWithCaption generates TTS and sends voice note to Telegram
// If caption is provided and <= 1024 chars, it will be attached to the voice message
func sendTelegramVoiceWithCaption(cfg *config.Config, chatID int64, text string, caption string) error {
	botToken := cfg.Telegram.BotToken
	if botToken == "" {
		return fmt.Errorf("telegram bot token not configured")
	}

	voice := cfg.Voice.TTSVoice
	if voice == "" {
		voice = "ru-RU-DmitryNeural"
	}

	// Generate OGG voice file using edge-tts + ffmpeg
	tmpMP3, err := os.CreateTemp("", "tts_*.mp3")
	if err != nil {
		return fmt.Errorf("failed to create temp mp3: %w", err)
	}
	tmpMP3.Close()
	defer os.Remove(tmpMP3.Name())

	tmpOGG := strings.TrimSuffix(tmpMP3.Name(), ".mp3") + ".ogg"
	defer os.Remove(tmpOGG)

	// Run edge-tts
	log.Printf("[telegram] Running TTS: edge-tts --voice %s", voice)
	cmd := exec.Command("edge-tts", "--voice", voice, "--text", text, "--write-media", tmpMP3.Name())
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("edge-tts failed: %w", err)
	}

	// Convert to OGG opus (Telegram voice note format)
	cmd = exec.Command("ffmpeg", "-y", "-i", tmpMP3.Name(), "-c:a", "libopus", "-b:a", "64k", tmpOGG)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg conversion failed: %w", err)
	}

	// Read OGG file
	oggData, err := os.ReadFile(tmpOGG)
	if err != nil {
		return fmt.Errorf("failed to read ogg: %w", err)
	}

	// Send voice note via Telegram API
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendVoice", botToken)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("chat_id", fmt.Sprintf("%d", chatID))

	// Add caption if provided
	if caption != "" {
		writer.WriteField("caption", caption)
	}

	part, err := writer.CreateFormFile("voice", "voice.ogg")
	if err != nil {
		return err
	}
	part.Write(oggData)
	writer.Close()

	resp, err := http.Post(url, writer.FormDataContentType(), &buf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error: %s", string(body))
	}

	if caption != "" {
		log.Printf("[telegram] Sent voice note with caption to %d (%d bytes)", chatID, len(oggData))
	} else {
		log.Printf("[telegram] Sent voice note to %d (%d bytes)", chatID, len(oggData))
	}
	return nil
}

func formatTelegramUserID(chatID int64) string {
	// Just the numeric ID to match vtoroy's format
	return fmt.Sprintf("%d", chatID)
}

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

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// TelegramSendRequest is the request body for /api/telegram/send
type TelegramSendRequest struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
	Voice  bool   `json:"voice"` // If true, also send TTS voice note
}

// TelegramSend creates a handler for sending Telegram messages
// Used by vtoroy scheduler for morning messages
func TelegramSend(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req TelegramSendRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("[telegram/send] Failed to decode request: %v", err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if req.ChatID == 0 || req.Text == "" {
			http.Error(w, "chat_id and text required", http.StatusBadRequest)
			return
		}

		log.Printf("[telegram/send] Sending to %d: %s", req.ChatID, truncateStr(req.Text, 50))

		if req.Voice {
			// Voice + text response (same logic as voice input response)
			const maxCaptionLen = 1024

			if len(req.Text) <= maxCaptionLen {
				// Short: voice with caption
				if err := sendTelegramVoiceWithCaption(cfg, req.ChatID, req.Text, req.Text); err != nil {
					log.Printf("[telegram/send] Failed to send voice with caption: %v", err)
					http.Error(w, "Failed to send voice", http.StatusInternalServerError)
					return
				}
			} else {
				// Long: text first, then voice without caption
				if err := sendTelegramMessage(cfg, req.ChatID, req.Text); err != nil {
					log.Printf("[telegram/send] Failed to send text: %v", err)
					http.Error(w, "Failed to send text", http.StatusInternalServerError)
					return
				}
				if err := sendTelegramVoiceWithCaption(cfg, req.ChatID, req.Text, ""); err != nil {
					log.Printf("[telegram/send] Failed to send voice: %v", err)
					http.Error(w, "Failed to send voice", http.StatusInternalServerError)
					return
				}
			}
		} else {
			// Text only
			if err := sendTelegramMessage(cfg, req.ChatID, req.Text); err != nil {
				log.Printf("[telegram/send] Failed to send text: %v", err)
				http.Error(w, "Failed to send text", http.StatusInternalServerError)
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}
}
