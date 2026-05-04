package channels

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
	"strings"
)

// TelegramConfig holds telegram-specific configuration
type TelegramConfig struct {
	BotToken string
}

// UserLookup is a function to get telegram_id from keycloak_sub
type UserLookup func(keycloakSub string) (telegramID int64, err error)

// TelegramChannel sends responses via Telegram
type TelegramChannel struct {
	config      TelegramConfig
	userLookup  UserLookup
	pendingSync *PendingSync
}

// NewTelegramChannel creates a new Telegram channel
// Note: TTS is done by Duq, no local TTS config needed
func NewTelegramChannel(botToken string) *TelegramChannel {
	return &TelegramChannel{
		config: TelegramConfig{
			BotToken: botToken,
		},
		pendingSync: NewPendingSync(),
	}
}

// SetUserLookup sets the function to resolve keycloak_sub -> telegram_id
func (c *TelegramChannel) SetUserLookup(lookup UserLookup) {
	c.userLookup = lookup
}

func (c *TelegramChannel) Name() string {
	return "telegram"
}

func (c *TelegramChannel) CanHandle(ctx *ResponseContext) bool {
	return c.config.BotToken != "" && ctx.ChatID != 0
}

func (c *TelegramChannel) Send(ctx *ResponseContext) error {
	// Check voice-aware fields from Duq
	shouldVoice := ctx.IsVoice ||
		ctx.OutputType == "voice" ||
		ctx.OutputType == "both"

	if shouldVoice && ctx.VoicePriority != "skip" {
		return c.sendVoiceResponse(ctx)
	}
	return c.sendTextMessage(ctx.ChatID, ctx.Response)
}

func (c *TelegramChannel) sendVoiceResponse(ctx *ResponseContext) error {
	// TTS is done by Duq - we only handle pre-synthesized audio
	if len(ctx.VoiceData) > 0 {
		log.Printf("[telegram] Using pre-synthesized audio from Duq (%d bytes, format=%s)",
			len(ctx.VoiceData), ctx.VoiceFormat)
		return c.sendVoiceFromData(ctx)
	}

	// No voice data from Duq - fallback to text
	log.Printf("[telegram] No voice data from Duq, falling back to text")
	return c.sendTextMessage(ctx.ChatID, ctx.Response)
}

// sendVoiceFromData sends pre-synthesized audio from Duq
// If format is OGG - sends directly, otherwise converts MP3 to OGG Opus
func (c *TelegramChannel) sendVoiceFromData(ctx *ResponseContext) error {
	const maxCaptionLen = 1024

	// Determine caption
	caption := ""
	if len(ctx.Response) <= maxCaptionLen {
		caption = ctx.Response
	}

	// If already OGG format (from Duq TTS), send directly without conversion
	if ctx.VoiceFormat == "ogg" {
		log.Printf("[telegram] Voice already in OGG format, sending directly")
		return c.sendOggVoice(ctx.ChatID, ctx.VoiceData, caption)
	}

	// Legacy path: Convert MP3 to OGG Opus for Telegram
	log.Printf("[telegram] Converting MP3 to OGG (format=%s)", ctx.VoiceFormat)

	// Save MP3 to temp file
	tmpMP3, err := os.CreateTemp("", "duq_tts_*.mp3")
	if err != nil {
		return fmt.Errorf("failed to create temp mp3: %w", err)
	}
	defer os.Remove(tmpMP3.Name())

	if _, err := tmpMP3.Write(ctx.VoiceData); err != nil {
		tmpMP3.Close()
		return fmt.Errorf("failed to write mp3: %w", err)
	}
	tmpMP3.Close()

	// Convert MP3 to OGG Opus for Telegram
	tmpOGG := strings.TrimSuffix(tmpMP3.Name(), ".mp3") + ".ogg"
	defer os.Remove(tmpOGG)

	cmd := exec.Command("ffmpeg", "-y", "-i", tmpMP3.Name(), "-c:a", "libopus", "-b:a", "64k", tmpOGG)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg conversion failed: %w", err)
	}

	// Read OGG file
	oggData, err := os.ReadFile(tmpOGG)
	if err != nil {
		return fmt.Errorf("failed to read ogg: %w", err)
	}

	// Send voice note
	return c.sendOggVoice(ctx.ChatID, oggData, caption)
}

// sendOggVoice sends OGG audio to Telegram
func (c *TelegramChannel) sendOggVoice(chatID int64, oggData []byte, caption string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendVoice", c.config.BotToken)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("chat_id", fmt.Sprintf("%d", chatID))

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

	log.Printf("[telegram] Sent voice note to %d (%d bytes)", chatID, len(oggData))
	return nil
}

// SendTextMessage sends a plain text message (exported for fallback use)
func (c *TelegramChannel) SendTextMessage(chatID int64, text string) error {
	return c.sendTextMessage(chatID, text)
}

func (c *TelegramChannel) sendTextMessage(chatID int64, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.config.BotToken)

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

// ============================================================================
// SyncChannel implementation for multi-channel history sync
// ============================================================================

// IsUserConnected checks if user has telegram_id (can receive messages)
func (c *TelegramChannel) IsUserConnected(userID string) bool {
	if c.userLookup == nil {
		return false
	}
	telegramID, err := c.userLookup(userID)
	return err == nil && telegramID != 0
}

// SyncSend delivers a synced message from another channel (e.g., Android)
// Uses editMessage pattern: quote user messages, append Duq response
func (c *TelegramChannel) SyncSend(msg *SyncMessage) error {
	if c.userLookup == nil {
		return fmt.Errorf("user lookup not configured")
	}

	telegramID, err := c.userLookup(msg.UserID)
	if err != nil {
		return fmt.Errorf("failed to lookup telegram_id: %w", err)
	}
	if telegramID == 0 {
		return fmt.Errorf("user has no telegram_id")
	}

	if msg.Role == "user" {
		return c.syncUserMessage(telegramID, msg)
	} else if msg.Role == "assistant" {
		return c.syncAssistantMessage(telegramID, msg)
	}

	return nil
}

// syncUserMessage handles syncing user messages from other channels
// Creates new message with quote or edits existing to add more quotes
func (c *TelegramChannel) syncUserMessage(chatID int64, msg *SyncMessage) error {
	pending := c.pendingSync.Get(msg.UserID)

	if pending == nil {
		// First message - create new with quote
		text := c.formatQuote([]string{msg.Content}, false)
		msgID, err := c.sendMessageWithID(chatID, text)
		if err != nil {
			return err
		}

		// Store pending for later edit
		c.pendingSync.Set(msg.UserID, &PendingMessage{
			TelegramMessageID: msgID,
			ChatID:            chatID,
			Quotes:            []string{msg.Content},
		})

		log.Printf("[telegram-sync] Created quote message: chat=%d msg_id=%d", chatID, msgID)
	} else {
		// Additional message - edit to add quote
		pending.Quotes = append(pending.Quotes, msg.Content)
		text := c.formatQuote(pending.Quotes, false)

		if err := c.editMessage(pending.ChatID, pending.TelegramMessageID, text); err != nil {
			return err
		}

		log.Printf("[telegram-sync] Added quote: chat=%d msg_id=%d quotes=%d",
			chatID, pending.TelegramMessageID, len(pending.Quotes))
	}

	return nil
}

// syncAssistantMessage handles syncing Duq responses from other channels
// Edits pending message to add response after quotes
func (c *TelegramChannel) syncAssistantMessage(chatID int64, msg *SyncMessage) error {
	pending := c.pendingSync.Get(msg.UserID)

	if pending == nil {
		// No pending quotes - just send response as new message
		return c.sendTextMessage(chatID, "🦆 "+msg.Content)
	}

	// Edit message to add response after quotes
	text := c.formatQuote(pending.Quotes, true) + "\n\n🦆 " + msg.Content

	if err := c.editMessage(pending.ChatID, pending.TelegramMessageID, text); err != nil {
		return err
	}

	// Clear pending - conversation turn complete
	c.pendingSync.Clear(msg.UserID)

	log.Printf("[telegram-sync] Added response: chat=%d msg_id=%d", chatID, pending.TelegramMessageID)
	return nil
}

// formatQuote formats user messages as a quote block
func (c *TelegramChannel) formatQuote(quotes []string, complete bool) string {
	var sb strings.Builder

	sb.WriteString("┌─ 📱 Android ─────────────\n")
	for _, q := range quotes {
		sb.WriteString("│ ")
		sb.WriteString(q)
		sb.WriteString("\n")
	}
	sb.WriteString("└──────────────────────────")

	if !complete {
		sb.WriteString("\n⏳ Обработка...")
	}

	return sb.String()
}

// sendMessageWithID sends a message and returns the message_id
func (c *TelegramChannel) sendMessageWithID(chatID int64, text string) (int64, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.config.BotToken)

	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}

	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("telegram API error: %s", string(body))
	}

	// Parse response to get message_id
	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			MessageID int64 `json:"message_id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Result.MessageID, nil
}

// editMessage edits an existing message
func (c *TelegramChannel) editMessage(chatID int64, messageID int64, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/editMessageText", c.config.BotToken)

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       text,
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

	return nil
}
