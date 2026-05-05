package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"duq-gateway/internal/config"
	"duq-gateway/internal/oauth"
	"github.com/google/uuid"
	"duq-gateway/internal/queue"
)


// TelegramWithDeps creates a handler with full dependencies
func TelegramWithDeps(deps *TelegramDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var update TelegramUpdateFull
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			log.Printf("[telegram] Failed to decode update: %v", err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Handle callback queries (button clicks)
		if update.CallbackQuery != nil {
			handleCallbackQuery(w, update.CallbackQuery, deps)
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

		// Handle /start for registration only, then continue to LLM
		isStartCommand := strings.HasPrefix(text, "/start")
		if isStartCommand {
			handleStartRegistration(w, msg, deps)
			// Don't return - let the message go to LLM for greeting
		}

		// Handle /tools command or button "🛠 Инструменты"
		if text == "/tools" || text == "🛠 Инструменты" {
			handleToolsCommand(w, msg.Chat.ID, deps)
			return
		}

		// Handle /settings command or button "⚙️ Настройки"
		if text == "/settings" || text == "⚙️ Настройки" {
			handleMenuSettings(w, msg.Chat.ID, deps)
			return
		}

		// Handle /help command or button "❓ Помощь"
		if text == "/help" || text == "❓ Помощь" {
			handleMenuHelp(w, msg.Chat.ID, deps)
			return
		}

		// Handle button "📜 История"
		if text == "📜 История" {
			handleMenuHistory(w, msg.Chat.ID, deps)
			return
		}

		// All other commands go to LLM - no hardcoded responses

		// Voice/audio data for queue (backend will transcribe)
		var voiceData []byte
		var voiceFileID string

		if text == "" {
			if msg.Voice != nil {
				voiceFileID = msg.Voice.FileID
				// Download voice file for backend to transcribe
				audioData, err := downloadVoiceFile(deps.Config, voiceFileID)
				if err != nil {
					log.Printf("[telegram] Failed to download voice: %v", err)
					text = "[Voice message - download failed]"
				} else {
					voiceData = audioData
					text = "[Voice message - pending transcription]"
					log.Printf("[telegram] Downloaded voice: %d bytes", len(audioData))
				}
			} else if msg.Audio != nil {
				voiceFileID = msg.Audio.FileID
				// Download audio file for backend to transcribe
				audioData, err := downloadVoiceFile(deps.Config, voiceFileID)
				if err != nil {
					log.Printf("[telegram] Failed to download audio: %v", err)
					text = "[Audio file - download failed]"
				} else {
					voiceData = audioData
					text = "[Audio file - pending transcription]"
					log.Printf("[telegram] Downloaded audio: %d bytes", len(audioData))
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
		var opts chatOptions

		// Get allowed tools from RBAC if available
		if deps.RBACService != nil {
			// Ensure user info is updated
			username := ""
			firstName := ""
			lastName := ""
			if msg.From != nil {
				username = msg.From.Username
				firstName = msg.From.FirstName
				lastName = msg.From.LastName
			}
			deps.RBACService.EnsureUser(telegramID, username, firstName, lastName)

			// Get internal users.id for RBAC (not telegram_id!)
			internalUserID, err := deps.RBACService.GetUserIDByTelegramID(telegramID)
			if err != nil {
				log.Printf("[telegram] User not registered: telegram_id=%d, error=%v", telegramID, err)
				// User not in DB yet - they need to register via /start
			} else {
				// Ensure user has default 'user' role
				if err := deps.RBACService.EnsureUserRole(internalUserID); err != nil {
					log.Printf("[telegram] Failed to ensure user role: %v", err)
				}

				// Get allowed tools using internal user ID
				tools, err := deps.RBACService.GetAllowedTools(internalUserID)
				if err != nil {
					log.Printf("[telegram] RBAC error: %v", err)
				} else {
					opts.AllowedTools = tools
					log.Printf("[telegram] User %d (internal_id=%d) has %d allowed tools",
						telegramID, internalUserID, len(tools))
				}
			}
		}

		// NOTE: Conversation history is now managed by Duq backend.
		// Gateway is pass-through — no session management here.
		// Duq loads history from DB and saves messages automatically.

		// Get user from database (for db_user_id, keycloak_sub and preferences)
		var dbUserID int64
		var keycloakSub string
		if deps.DBClient != nil {
			user, err := deps.DBClient.GetUserByTelegramID(telegramID)
			if err != nil {
				log.Printf("[telegram] Error getting user: %v", err)
			} else if user != nil {
				dbUserID = user.ID
				keycloakSub = user.KeycloakSub
				opts.UserPreferences = &UserPreferences{
					Timezone:          user.Timezone,
					PreferredLanguage: user.PreferredLanguage,
				}
				log.Printf("[telegram] User %d (db_id=%d) preferences: tz=%s, lang=%s",
					telegramID, dbUserID, user.Timezone, user.PreferredLanguage)
			} else {
				// User not found - use defaults
				prefs := deps.DBClient.GetUserPreferencesByTelegramID(telegramID)
				opts.UserPreferences = &UserPreferences{
					Timezone:          prefs.Timezone,
					PreferredLanguage: prefs.PreferredLanguage,
				}
				log.Printf("[telegram] User %d not found in DB, using defaults", telegramID)
			}
		}

		// Get GWS credentials if available
		var userEmail string
		if deps.CredService != nil {
			creds, err := deps.CredService.GetCredentialsByTelegramID(telegramID, "google")
			if err != nil {
				log.Printf("[telegram] Error fetching GWS credentials: %v", err)
			} else if creds != nil {
				// Auto-refresh if token expired
				oauthCfg := oauth.GoogleOAuthConfig{
					ClientID:     deps.Config.GoogleOAuth.ClientID,
					ClientSecret: deps.Config.GoogleOAuth.ClientSecret,
				}
				if err := oauth.RefreshGoogleTokenIfNeeded(oauthCfg, deps.CredService, creds); err != nil {
					log.Printf("[telegram] Failed to refresh token: %v", err)
				}
				opts.GWSCredentials = map[string]string{
					"access_token":  creds.AccessToken,
					"refresh_token": creds.RefreshToken,
					"token_type":    creds.TokenType,
				}
				userEmail = creds.Email
				log.Printf("[telegram] User %d has GWS credentials (email=%s)", telegramID, userEmail)
			}
		}

		// Push directly to Redis queue - Duq worker will pick it up
		// Always use HTTP for internal Docker callbacks (TLS is for external traffic only)
		callbackURL := fmt.Sprintf("http://%s/api/duq/callback", deps.Config.GatewayHost)

		inputType := "text"
		if isVoice {
			inputType = "voice"
		}

		// NOTE: History is no longer sent to Duq.
		// Duq manages conversation history in its own database.

		// Build user preferences for payload
		var userPrefs map[string]string
		if opts.UserPreferences != nil {
			userPrefs = map[string]string{
				"timezone":           opts.UserPreferences.Timezone,
				"preferred_language": opts.UserPreferences.PreferredLanguage,
			}
		}

		// Build payload
		payload := map[string]interface{}{
			"message":          text,
			"output_channel":   "telegram",
			"allowed_tools":    opts.AllowedTools,
			"user_preferences": userPrefs,
			"gws_credentials":  opts.GWSCredentials,
		}

		// Include voice data as base64 for backend transcription
		if len(voiceData) > 0 {
			payload["voice_data"] = base64.StdEncoding.EncodeToString(voiceData)
			payload["voice_format"] = "ogg" // Telegram voice messages are OGG
		}

		// Build request metadata
		requestMetadata := map[string]interface{}{
			"chat_id":         msg.Chat.ID,
			"user_message_id": msg.MessageID, // For status reaction on callback
			"user_email":      userEmail,
			"is_voice":        isVoice,
			"input_type":      inputType,
			"source":          "telegram",
			"is_start":        isStartCommand,
			// Group chat fields
			"chat_type":  msg.Chat.Type, // "private", "group", "supergroup", "channel"
			"chat_title": msg.Chat.Title,
		}

		// Add sender info (in groups, from_user_id != chat_id)
		if msg.From != nil {
			requestMetadata["from_user_id"] = msg.From.ID
			requestMetadata["from_username"] = msg.From.Username
		}

		// Add reply info if this message is a reply to another
		if msg.ReplyToMessage != nil {
			requestMetadata["reply_to_message_id"] = msg.ReplyToMessage.MessageID
			requestMetadata["reply_to_message_text"] = msg.ReplyToMessage.Text
			if msg.ReplyToMessage.From != nil {
				requestMetadata["reply_to_from_username"] = msg.ReplyToMessage.From.Username
			}
		}

		// Add thread ID for forum topics
		if msg.MessageThreadID != 0 {
			requestMetadata["message_thread_id"] = msg.MessageThreadID
		}
		// Generate trace_id for request tracing
		traceID := r.Header.Get("X-Trace-Id")
		if traceID == "" {
			traceID = uuid.New().String()
		}
		requestMetadata["trace_id"] = traceID
		log.Printf("[telegram] Request trace_id: %s", traceID)
		// Continue with metadata
		// Include db_user_id for memory operations (critical for Hindsight)
		if dbUserID > 0 {
			requestMetadata["db_user_id"] = dbUserID
		}
		// Include keycloak_sub for conversation history
		if keycloakSub != "" {
			requestMetadata["keycloak_sub"] = keycloakSub
		}

		task := &queue.Task{
			UserID:          userID,
			Type:            "message",
			Priority:        50,
			CallbackURL:     callbackURL,
			Payload:         payload,
			RequestMetadata: requestMetadata,
		}

		// Send typing indicator before pushing to queue
		SendTypingAction(deps.Config, msg.Chat.ID)

		_, err := deps.QueueClient.Push(r.Context(), task)
		if err != nil {
			log.Printf("[telegram] Failed to push to Redis queue: %v", err)
			SendTelegramMessage(deps.Config, msg.Chat.ID, "⚠️ Сервис временно недоступен. Попробуй позже.")
			// Set error reaction on user's message
			SetMessageReaction(deps.Config, msg.Chat.ID, int64(msg.MessageID), ReactionError)
		} else {
			log.Printf("[telegram] Message pushed to Redis queue for user %s", userID)
			// Note: Reactions now set by Duq agent via set_reaction tool
		}

		w.WriteHeader(http.StatusOK)
	}
}

// TelegramSend creates a handler for sending Telegram messages
// Used by duq scheduler for morning messages
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
				if err := SendTelegramMessage(cfg, req.ChatID, req.Text); err != nil {
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
			if err := SendTelegramMessage(cfg, req.ChatID, req.Text); err != nil {
				log.Printf("[telegram/send] Failed to send text: %v", err)
				http.Error(w, "Failed to send text", http.StatusInternalServerError)
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}
}
