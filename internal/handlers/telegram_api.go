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
	"strings"

	"duq-gateway/internal/config"
)

// SendTelegramMessage sends a text message to Telegram (exported for use by other handlers)
func SendTelegramMessage(cfg *config.Config, chatID int64, text string) error {
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

// SendTelegramMessageWithKeyboard sends a message with inline keyboard
func SendTelegramMessageWithKeyboard(cfg *config.Config, chatID int64, text string, keyboard *InlineKeyboardMarkup) error {
	botToken := cfg.Telegram.BotToken
	if botToken == "" {
		return fmt.Errorf("telegram bot token not configured")
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	payload := map[string]interface{}{
		"chat_id":      chatID,
		"text":         text,
		"parse_mode":   "Markdown",
		"reply_markup": keyboard,
	}

	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(apiURL, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error: %s", string(body))
	}

	log.Printf("[telegram] Sent message with keyboard to %d", chatID)
	return nil
}

// SendTypingAction sends "typing" action to show the bot is processing
func SendTypingAction(cfg *config.Config, chatID int64) error {
	botToken := cfg.Telegram.BotToken
	if botToken == "" {
		return fmt.Errorf("telegram bot token not configured")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendChatAction", botToken)

	payload := map[string]interface{}{
		"chat_id": chatID,
		"action":  "typing",
	}

	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// AnswerCallbackQuery answers a callback query (removes loading state from button)
func AnswerCallbackQuery(cfg *config.Config, callbackID string, text string) error {
	botToken := cfg.Telegram.BotToken
	if botToken == "" {
		return fmt.Errorf("telegram bot token not configured")
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/answerCallbackQuery", botToken)

	payload := map[string]interface{}{
		"callback_query_id": callbackID,
	}
	if text != "" {
		payload["text"] = text
		payload["show_alert"] = false
	}

	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(apiURL, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// getMainMenuKeyboard returns the main menu inline keyboard (for callbacks)
func getMainMenuKeyboard() *InlineKeyboardMarkup {
	return &InlineKeyboardMarkup{
		InlineKeyboard: [][]InlineKeyboardButton{
			{
				{Text: "📜 История", CallbackData: "menu_history"},
				{Text: "⚙️ Настройки", CallbackData: "menu_settings"},
			},
			{
				{Text: "🛠 Инструменты", CallbackData: "menu_tools"},
				{Text: "❓ Помощь", CallbackData: "menu_help"},
			},
		},
	}
}

// getReplyKeyboard returns persistent keyboard at bottom of chat
func getReplyKeyboard() *ReplyKeyboardMarkup {
	return &ReplyKeyboardMarkup{
		Keyboard: [][]KeyboardButton{
			{
				{Text: "🛠 Инструменты"},
				{Text: "📜 История"},
			},
			{
				{Text: "⚙️ Настройки"},
				{Text: "❓ Помощь"},
			},
		},
		ResizeKeyboard: true,
		IsPersistent:   true,
	}
}

// SendTelegramMessageWithReplyKeyboard sends a message with persistent reply keyboard
func SendTelegramMessageWithReplyKeyboard(cfg *config.Config, chatID int64, text string, keyboard *ReplyKeyboardMarkup) error {
	botToken := cfg.Telegram.BotToken
	if botToken == "" {
		return fmt.Errorf("telegram bot token not configured")
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	payload := map[string]interface{}{
		"chat_id":      chatID,
		"text":         text,
		"parse_mode":   "Markdown",
		"reply_markup": keyboard,
	}

	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(apiURL, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error: %s", string(body))
	}

	log.Printf("[telegram] Sent message with reply keyboard to %d", chatID)
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

// formatTelegramUserID formats telegram chat ID to user ID string
func formatTelegramUserID(chatID int64) string {
	// Just the numeric ID to match duq's format
	return fmt.Sprintf("%d", chatID)
}

// formatUserName formats user display name
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

// truncateStr truncates string to maxLen chars
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// BotCommand represents a Telegram bot command for menu
type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// DeleteBotCommands removes all bot menu commands (use buttons instead)
func DeleteBotCommands(cfg *config.Config) error {
	botToken := cfg.Telegram.BotToken
	if botToken == "" {
		return fmt.Errorf("telegram bot token not configured")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/deleteMyCommands", botToken)

	resp, err := http.Post(url, "application/json", bytes.NewReader([]byte("{}")))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram API error: %s", string(body))
	}

	log.Printf("[telegram] Bot menu commands deleted (using buttons only)")
	return nil
}

// ============================================================================
// Message Reactions API (Bot API 7.0+)
// ============================================================================

// ReactionEmoji represents status reactions for message processing
// NOTE: Telegram only supports a specific set of emoji for reactions
// Supported: 👍 👎 ❤️ 🔥 🥰 👏 😁 🤔 🤯 😱 🤬 😢 🎉 🤩 🤮 💩 🙏 👌 🕊 🤡 🥱 🥴 😍 🐳 ❤️‍🔥 🌚 🌭 💯 🤣 ⚡️ 🍌 🏆 💔 🤨 😐 🍓 🍾 💋 🖕 😈 😴 😭 🤓 👻 👨‍💻 👀 🎃 🙈 😇 😨 🤝 ✍️ 🤗 🫡 🎅 🎄 ☃️ 💅 🤪 🗿 🆒 💘 🙉 🦄 😘 💊 🙊 😎 👾 🤷‍♂️ 🤷 🤷‍♀️ 😡
type ReactionEmoji string

const (
	ReactionQueued   ReactionEmoji = "👀" // Message received, queued (eyes = "seen")
	ReactionThinking ReactionEmoji = "🤔" // Processing started
	ReactionDone     ReactionEmoji = "👍" // Success
	ReactionError    ReactionEmoji = "💔" // Error occurred
)

// SetMessageReaction sets a reaction on a message (fire-and-forget)
// This is used for status indicators on user messages
func SetMessageReaction(cfg *config.Config, chatID int64, messageID int64, emoji ReactionEmoji) {
	go func() {
		if err := setMessageReactionSync(cfg, chatID, messageID, emoji); err != nil {
			// Fire-and-forget: log but don't fail
			log.Printf("[telegram-reaction] Failed to set %s on msg %d: %v", emoji, messageID, err)
		}
	}()
}

// setMessageReactionSync performs the actual API call
func setMessageReactionSync(cfg *config.Config, chatID int64, messageID int64, emoji ReactionEmoji) error {
	botToken := cfg.Telegram.BotToken
	if botToken == "" {
		return fmt.Errorf("telegram bot token not configured")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/setMessageReaction", botToken)

	// Telegram requires reaction as array of ReactionType objects
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
		"reaction": []map[string]string{
			{"type": "emoji", "emoji": string(emoji)},
		},
	}

	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %s", string(body))
	}

	log.Printf("[telegram-reaction] Set %s on chat=%d msg=%d", emoji, chatID, messageID)
	return nil
}

// SupportedReactions is the list of emoji Telegram supports for reactions
var SupportedReactions = map[string]bool{
	"👍": true, "👎": true, "❤️": true, "🔥": true, "🥰": true, "👏": true,
	"😁": true, "🤔": true, "🤯": true, "😱": true, "🤬": true, "😢": true,
	"🎉": true, "🤩": true, "🤮": true, "💩": true, "🙏": true, "👌": true,
	"🕊": true, "🤡": true, "🥱": true, "🥴": true, "😍": true, "🐳": true,
	"❤️‍🔥": true, "🌚": true, "🌭": true, "💯": true, "🤣": true, "⚡️": true,
	"🍌": true, "🏆": true, "💔": true, "🤨": true, "😐": true, "🍓": true,
	"🍾": true, "💋": true, "🖕": true, "😈": true, "😴": true, "😭": true,
	"🤓": true, "👻": true, "👨‍💻": true, "👀": true, "🎃": true, "🙈": true,
	"😇": true, "😨": true, "🤝": true, "✍️": true, "🤗": true, "🫡": true,
	"🎅": true, "🎄": true, "☃️": true, "💅": true, "🤪": true, "🗿": true,
	"🆒": true, "💘": true, "🙉": true, "🦄": true, "😘": true, "💊": true,
	"🙊": true, "😎": true, "👾": true, "🤷‍♂️": true, "🤷": true, "🤷‍♀️": true, "😡": true,
}

// SetReactionRequest is the request body for POST /api/telegram/reaction
type SetReactionRequest struct {
	ChatID    int64  `json:"chat_id"`
	MessageID int64  `json:"message_id"`
	Emoji     string `json:"emoji"`
}

// TelegramReactionHandler handles POST /api/telegram/reaction
// This endpoint is called by Duq to set reactions on user messages
func TelegramReactionHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req SetReactionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("[telegram-reaction] Invalid JSON: %v", err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if req.ChatID == 0 || req.MessageID == 0 || req.Emoji == "" {
			http.Error(w, "chat_id, message_id, and emoji required", http.StatusBadRequest)
			return
		}

		// Let Telegram API decide if emoji is valid - no pre-validation
		err := setMessageReactionSync(cfg, req.ChatID, req.MessageID, ReactionEmoji(req.Emoji))
		if err != nil {
			log.Printf("[telegram-reaction] Failed to set reaction: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}
}

// ClearMessageReaction removes all reactions from a message
func ClearMessageReaction(cfg *config.Config, chatID int64, messageID int64) {
	go func() {
		botToken := cfg.Telegram.BotToken
		if botToken == "" {
			return
		}

		url := fmt.Sprintf("https://api.telegram.org/bot%s/setMessageReaction", botToken)

		// Empty array clears reactions
		payload := map[string]interface{}{
			"chat_id":    chatID,
			"message_id": messageID,
			"reaction":   []map[string]string{},
		}

		jsonData, _ := json.Marshal(payload)
		resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
		if err != nil {
			log.Printf("[telegram-reaction] Failed to clear reaction: %v", err)
			return
		}
		resp.Body.Close()
	}()
}

// ============================================================================
// Feedback Keyboard
// ============================================================================

// GetFeedbackKeyboard returns inline keyboard with feedback buttons
func GetFeedbackKeyboard() *InlineKeyboardMarkup {
	return &InlineKeyboardMarkup{
		InlineKeyboard: [][]InlineKeyboardButton{
			{
				{Text: "👍", CallbackData: "feedback_positive"},
				{Text: "👎", CallbackData: "feedback_negative"},
			},
		},
	}
}

// SendTelegramMessageWithFeedback sends a message with feedback buttons
func SendTelegramMessageWithFeedback(cfg *config.Config, chatID int64, text string) (int64, error) {
	botToken := cfg.Telegram.BotToken
	if botToken == "" {
		return 0, fmt.Errorf("telegram bot token not configured")
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	payload := map[string]interface{}{
		"chat_id":      chatID,
		"text":         text,
		"reply_markup": GetFeedbackKeyboard(),
	}

	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(apiURL, "application/json", bytes.NewReader(jsonData))
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

	log.Printf("[telegram] Sent message with feedback buttons to %d, msg_id=%d", chatID, result.Result.MessageID)
	return result.Result.MessageID, nil
}

// UpdateMessageRemoveFeedback edits message to remove feedback buttons (after user voted)
func UpdateMessageRemoveFeedback(cfg *config.Config, chatID int64, messageID int64, text string, feedbackType string) error {
	botToken := cfg.Telegram.BotToken
	if botToken == "" {
		return fmt.Errorf("telegram bot token not configured")
	}

	// Add feedback indicator to text
	indicator := ""
	if feedbackType == "positive" {
		indicator = "\n\n✅ Спасибо за отзыв!"
	} else if feedbackType == "negative" {
		indicator = "\n\n📝 Спасибо, учтём!"
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/editMessageText", botToken)

	payload := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       text + indicator,
	}

	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(apiURL, "application/json", bytes.NewReader(jsonData))
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

// ============================================================================
// Group Chat API Functions
// ============================================================================

// SendMessageWithReply sends a message as a reply to another message
// Uses Telegram's reply_parameters to quote the original message
// baseURL is optional - if empty, uses Telegram's official API
func SendMessageWithReply(cfg *config.Config, chatID int64, text string, replyToMessageID int64, baseURL string) error {
	botToken := cfg.Telegram.BotToken
	if botToken == "" {
		return fmt.Errorf("telegram bot token not configured")
	}

	// Use provided baseURL or default to Telegram API
	apiBase := "https://api.telegram.org"
	if baseURL != "" {
		apiBase = baseURL
	}
	url := fmt.Sprintf("%s/bot%s/sendMessage", apiBase, botToken)

	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
		"reply_parameters": map[string]interface{}{
			"message_id": replyToMessageID,
		},
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

	log.Printf("[telegram] Sent reply message to %d (reply_to=%d)", chatID, replyToMessageID)
	return nil
}

// ReplyMessageHandler handles POST /api/telegram/reply
func ReplyMessageHandler(cfg *config.Config, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ReplyMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if req.ChatID == 0 || req.Text == "" {
			http.Error(w, "chat_id and text required", http.StatusBadRequest)
			return
		}

		err := SendMessageWithReply(cfg, req.ChatID, req.Text, req.ReplyToMessageID, baseURL)
		if err != nil {
			log.Printf("[telegram-reply] Failed: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}
}

// GetChatInfoHandler handles POST /api/telegram/chat/info
func GetChatInfoHandler(cfg *config.Config, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req GetChatInfoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if req.ChatID == 0 {
			http.Error(w, "chat_id required", http.StatusBadRequest)
			return
		}

		botToken := cfg.Telegram.BotToken
		if botToken == "" {
			http.Error(w, "telegram bot token not configured", http.StatusInternalServerError)
			return
		}

		apiBase := "https://api.telegram.org"
		if baseURL != "" {
			apiBase = baseURL
		}
		url := fmt.Sprintf("%s/bot%s/getChat", apiBase, botToken)

		payload := map[string]interface{}{
			"chat_id": req.ChatID,
		}

		jsonData, _ := json.Marshal(payload)
		resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			http.Error(w, string(body), http.StatusBadGateway)
			return
		}

		// Parse Telegram response and extract result
		var telegramResp struct {
			OK     bool         `json:"ok"`
			Result ChatFullInfo `json:"result"`
		}
		if err := json.Unmarshal(body, &telegramResp); err != nil {
			http.Error(w, "failed to parse telegram response", http.StatusInternalServerError)
			return
		}

		// Return just the chat info
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(telegramResp.Result)
	}
}

// GetChatMemberHandler handles POST /api/telegram/chat/member
func GetChatMemberHandler(cfg *config.Config, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req GetChatMemberRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if req.ChatID == 0 || req.UserID == 0 {
			http.Error(w, "chat_id and user_id required", http.StatusBadRequest)
			return
		}

		botToken := cfg.Telegram.BotToken
		if botToken == "" {
			http.Error(w, "telegram bot token not configured", http.StatusInternalServerError)
			return
		}

		apiBase := "https://api.telegram.org"
		if baseURL != "" {
			apiBase = baseURL
		}
		url := fmt.Sprintf("%s/bot%s/getChatMember", apiBase, botToken)

		payload := map[string]interface{}{
			"chat_id": req.ChatID,
			"user_id": req.UserID,
		}

		jsonData, _ := json.Marshal(payload)
		resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			http.Error(w, string(body), http.StatusBadGateway)
			return
		}

		// Parse Telegram response
		var telegramResp struct {
			OK     bool       `json:"ok"`
			Result ChatMember `json:"result"`
		}
		if err := json.Unmarshal(body, &telegramResp); err != nil {
			http.Error(w, "failed to parse telegram response", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(telegramResp.Result)
	}
}

// PinMessageHandler handles POST /api/telegram/pin
func PinMessageHandler(cfg *config.Config, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req PinMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if req.ChatID == 0 || req.MessageID == 0 {
			http.Error(w, "chat_id and message_id required", http.StatusBadRequest)
			return
		}

		botToken := cfg.Telegram.BotToken
		if botToken == "" {
			http.Error(w, "telegram bot token not configured", http.StatusInternalServerError)
			return
		}

		apiBase := "https://api.telegram.org"
		if baseURL != "" {
			apiBase = baseURL
		}
		url := fmt.Sprintf("%s/bot%s/pinChatMessage", apiBase, botToken)

		payload := map[string]interface{}{
			"chat_id":              req.ChatID,
			"message_id":           req.MessageID,
			"disable_notification": req.DisableNotification,
		}

		jsonData, _ := json.Marshal(payload)
		resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			http.Error(w, string(body), http.StatusBadGateway)
			return
		}

		log.Printf("[telegram] Pinned message %d in chat %d", req.MessageID, req.ChatID)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}
}

// UnpinMessageHandler handles POST /api/telegram/unpin
func UnpinMessageHandler(cfg *config.Config, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req UnpinMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if req.ChatID == 0 {
			http.Error(w, "chat_id required", http.StatusBadRequest)
			return
		}

		botToken := cfg.Telegram.BotToken
		if botToken == "" {
			http.Error(w, "telegram bot token not configured", http.StatusInternalServerError)
			return
		}

		apiBase := "https://api.telegram.org"
		if baseURL != "" {
			apiBase = baseURL
		}
		url := fmt.Sprintf("%s/bot%s/unpinChatMessage", apiBase, botToken)

		payload := map[string]interface{}{
			"chat_id": req.ChatID,
		}
		if req.MessageID != 0 {
			payload["message_id"] = req.MessageID
		}

		jsonData, _ := json.Marshal(payload)
		resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			http.Error(w, string(body), http.StatusBadGateway)
			return
		}

		log.Printf("[telegram] Unpinned message in chat %d", req.ChatID)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}
}

// EditMessageHandler handles POST /api/telegram/edit
func EditMessageHandler(cfg *config.Config, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req EditMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if req.ChatID == 0 || req.MessageID == 0 || req.Text == "" {
			http.Error(w, "chat_id, message_id and text required", http.StatusBadRequest)
			return
		}

		botToken := cfg.Telegram.BotToken
		if botToken == "" {
			http.Error(w, "telegram bot token not configured", http.StatusInternalServerError)
			return
		}

		apiBase := "https://api.telegram.org"
		if baseURL != "" {
			apiBase = baseURL
		}
		url := fmt.Sprintf("%s/bot%s/editMessageText", apiBase, botToken)

		payload := map[string]interface{}{
			"chat_id":    req.ChatID,
			"message_id": req.MessageID,
			"text":       req.Text,
		}

		jsonData, _ := json.Marshal(payload)
		resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			http.Error(w, string(body), http.StatusBadGateway)
			return
		}

		log.Printf("[telegram] Edited message %d in chat %d", req.MessageID, req.ChatID)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}
}
