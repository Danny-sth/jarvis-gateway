package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"duq-gateway/internal/channels"
	"duq-gateway/internal/config"
	"duq-gateway/internal/oauth"
)

const (
	// CallbackDeliveryTimeout is the max time to deliver response to channel
	CallbackDeliveryTimeout = 30 * time.Second
)

// CallbackPayload from Duq worker (defines locally, no import from duq package)
type CallbackPayload struct {
	TaskID          string                 `json:"task_id"`
	UserID          string                 `json:"user_id"`
	Success         bool                   `json:"success"`
	Error           string                 `json:"error,omitempty"`
	Result          map[string]interface{} `json:"result,omitempty"`
	RequestMetadata map[string]interface{} `json:"request_metadata,omitempty"`
	ExecutionTimeMs *int64                 `json:"execution_time_ms,omitempty"`
}

// CallbackDeps contains dependencies for the callback handler
type CallbackDeps struct {
	Config        *config.Config
	ChannelRouter *channels.Router
	CredService   CredentialServiceInterface // Для OAuth токенов email канала
}

// DuqCallback handles callbacks from Duq worker.
// When a queued task completes, Duq POSTs the result here.
func DuqCallback(deps *CallbackDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("[callback] Failed to read body: %v", err)
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var payload CallbackPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			log.Printf("[callback] Failed to decode payload: %v", err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		log.Printf("[callback] Received: task_id=%s, user_id=%s, success=%v",
			payload.TaskID, payload.UserID, payload.Success)

		// Extract response from result
		var response string
		var outputChannel string = "telegram"
		var voiceData []byte
		var voiceFormat string

		if payload.Success && payload.Result != nil {
			if resp, ok := payload.Result["response"].(string); ok {
				response = resp
			}
			if ch, ok := payload.Result["channel"].(string); ok {
				outputChannel = ch
			}
			// Extract voice data from Duq worker (base64 encoded)
			if vd, ok := payload.Result["voice_data"].(string); ok && vd != "" {
				decoded, err := base64.StdEncoding.DecodeString(vd)
				if err != nil {
					log.Printf("[callback] Failed to decode voice_data: %v", err)
				} else {
					voiceData = decoded
					log.Printf("[callback] Voice data extracted: %d bytes", len(voiceData))
				}
			}
			if vf, ok := payload.Result["voice_format"].(string); ok {
				voiceFormat = vf
			}
		} else if !payload.Success {
			response = "Error processing request: " + payload.Error
		}

		if response == "" {
			log.Printf("[callback] Empty response, skipping delivery")
			w.WriteHeader(http.StatusOK)
			return
		}

		// Extract chat_id from request_metadata
		var chatID int64
		if payload.RequestMetadata != nil {
			if cid, ok := payload.RequestMetadata["chat_id"].(float64); ok {
				chatID = int64(cid)
			}
		}

		// Fallback: parse user_id as chat_id (for Telegram, user_id IS chat_id)
		if chatID == 0 {
			var uid int64
			if err := json.Unmarshal([]byte(payload.UserID), &uid); err == nil {
				chatID = uid
			}
		}

		if chatID == 0 {
			log.Printf("[callback] Cannot determine chat_id, skipping delivery")
			w.WriteHeader(http.StatusOK)
			return
		}

		// Get user email from metadata if available
		var userEmail string
		var googleAccessToken string
		if payload.RequestMetadata != nil {
			if email, ok := payload.RequestMetadata["user_email"].(string); ok {
				userEmail = email
			}
		}

		// Для email канала получаем OAuth токены из БД
		if outputChannel == "email" && deps.CredService != nil {
			creds, err := deps.CredService.GetCredentials(chatID, "google")
			if err == nil && creds != nil {
				// Автообновление токена если истёк
				oauthCfg := oauth.GoogleOAuthConfig{
					ClientID:     deps.Config.GoogleOAuth.ClientID,
					ClientSecret: deps.Config.GoogleOAuth.ClientSecret,
				}
				if err := oauth.RefreshGoogleTokenIfNeeded(oauthCfg, deps.CredService, creds); err != nil {
					log.Printf("[callback] Failed to refresh token: %v", err)
				}
				googleAccessToken = creds.AccessToken
				if userEmail == "" {
					userEmail = creds.Email
				}
				log.Printf("[callback] Got OAuth credentials for email channel: email=%s", userEmail)
			} else {
				log.Printf("[callback] No OAuth credentials for user %d", chatID)
			}
		}

		// Check if this was a voice message and extract source
		isVoice := false
		source := "telegram" // Default source
		keycloakSub := ""    // For WebSocket routing (Android uses keycloak_sub)
		if payload.RequestMetadata != nil {
			if v, ok := payload.RequestMetadata["is_voice"].(bool); ok {
				isVoice = v
			}
			if s, ok := payload.RequestMetadata["source"].(string); ok && s != "" {
				source = s
			}
			if ks, ok := payload.RequestMetadata["keycloak_sub"].(string); ok && ks != "" {
				keycloakSub = ks
			}
		}

		// Route response via channel router with timeout
		// Use goroutine with context to prevent hanging on slow channels
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), CallbackDeliveryTimeout)
			defer cancel()

			// For WebSocket routing, prefer keycloak_sub (Android auth) over telegram_id
			wsUserID := payload.UserID
			if keycloakSub != "" {
				wsUserID = keycloakSub
			}

			respCtx := &channels.ResponseContext{
				ChatID:            chatID,
				UserID:            wsUserID, // keycloak_sub for WebSocket, telegram_id for fallback
				UserEmail:         userEmail,
				Response:          response,
				IsVoice:           isVoice,
				TaskID:            payload.TaskID, // For task correlation
				Source:            source,         // For channel routing
				VoiceData:         voiceData,
				VoiceFormat:       voiceFormat,
				GoogleAccessToken: googleAccessToken,
			}

			// Channel for delivery result
			done := make(chan error, 1)

			go func() {
				if deps.ChannelRouter != nil {
					done <- deps.ChannelRouter.Route(outputChannel, respCtx)
				} else {
					// Fallback: send to Telegram directly
					done <- SendTelegramMessage(deps.Config, chatID, response)
				}
			}()

			// Wait for delivery or timeout
			select {
			case err := <-done:
				if err != nil {
					log.Printf("[callback] Channel routing failed for task %s: %v", payload.TaskID, err)
				}
			case <-ctx.Done():
				log.Printf("[callback] Channel delivery timeout for task %s (channel=%s)", payload.TaskID, outputChannel)
			}
		}()

		executionTime := "unknown"
		if payload.ExecutionTimeMs != nil {
			executionTime = fmt.Sprintf("%dms", *payload.ExecutionTimeMs)
		}
		// Log with actual channel (use source if outputChannel empty)
		actualChannel := outputChannel
		if actualChannel == "" {
			actualChannel = source
		}
		log.Printf("[callback] Delivered task_id=%s to channel=%s source=%s (exec_time=%s)",
			payload.TaskID, actualChannel, source, executionTime)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}
}

// NewCallbackDeps creates CallbackDeps from existing services
func NewCallbackDeps(cfg *config.Config, channelRouter *channels.Router, credService CredentialServiceInterface) *CallbackDeps {
	return &CallbackDeps{
		Config:        cfg,
		ChannelRouter: channelRouter,
		CredService:   credService,
	}
}
