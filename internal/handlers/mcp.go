package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"duq-gateway/internal/config"
	"duq-gateway/internal/queue"
)

// MCPRequest is the request body for MCP endpoint
type MCPRequest struct {
	Request string `json:"request"`
	Context string `json:"context,omitempty"`
}

// MCPResponse is the response body for MCP endpoint
type MCPResponse struct {
	Response string `json:"response,omitempty"`
	Error    string `json:"error,omitempty"`
}

// MCPDeps contains dependencies for MCP handler
type MCPDeps struct {
	Config      *config.Config
	QueueClient *queue.Client
	CredService CredentialServiceInterface
}

// MCP creates a synchronous MCP handler
// This endpoint pushes to Redis queue and waits for response
// Authentication is handled by Keycloak middleware in main.go
func MCP(deps *MCPDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req MCPRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"Invalid JSON"}`, http.StatusBadRequest)
			return
		}

		if req.Request == "" {
			http.Error(w, `{"error":"request field is required"}`, http.StatusBadRequest)
			return
		}

		log.Printf("[mcp] Request: %s", truncateStr(req.Request, 100))

		// Build message with context
		message := req.Request
		if req.Context != "" {
			message = fmt.Sprintf("%s\n\nContext: %s", req.Request, req.Context)
		}

		// Use owner's user ID for MCP requests
		userID := deps.Config.TelegramChatID

		// Parse telegram_id for credentials lookup
		var telegramID int64
		fmt.Sscanf(userID, "%d", &telegramID)

		// Get user email from credentials (for email channel)
		var userEmail string
		if deps.CredService != nil {
			if creds, err := deps.CredService.GetCredentials(telegramID, "google"); err == nil && creds != nil {
				userEmail = creds.Email
				log.Printf("[mcp] User %d has email: %s", telegramID, userEmail)
			}
		}

		// Build task - reusing same structure as Telegram handler
		callbackURL := fmt.Sprintf("http://%s/api/duq/callback", deps.Config.GatewayHost)

		task := &queue.Task{
			UserID:      userID,
			Type:        "message",
			Priority:    50,
			CallbackURL: callbackURL,
			Payload: map[string]interface{}{
				"message":        message,
				"output_channel": "mcp", // Different channel for logging
			},
			RequestMetadata: map[string]interface{}{
				"source":     "mcp",
				"chat_id":    telegramID, // For fallback to Telegram
				"user_email": userEmail,  // For email channel
			},
		}

		// Push to queue
		ctx := r.Context()
		taskID, err := deps.QueueClient.Push(ctx, task)
		if err != nil {
			log.Printf("[mcp] Failed to push to queue: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(MCPResponse{Error: "Failed to queue request"})
			return
		}

		log.Printf("[mcp] Task queued: %s", taskID)

		// Wait for response (2 minute timeout)
		waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()

		response, err := deps.QueueClient.WaitForResponse(waitCtx, taskID, 2*time.Minute)
		if err != nil {
			log.Printf("[mcp] Wait failed: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusGatewayTimeout)
			json.NewEncoder(w).Encode(MCPResponse{Error: fmt.Sprintf("Timeout: %v", err)})
			return
		}

		// Extract response text
		w.Header().Set("Content-Type", "application/json")

		if success, ok := response["success"].(bool); ok && success {
			if result, ok := response["result"].(map[string]interface{}); ok {
				if respText, ok := result["response"].(string); ok {
					log.Printf("[mcp] Response: %s", truncateStr(respText, 100))
					json.NewEncoder(w).Encode(MCPResponse{Response: respText})
					return
				}
			}
		}

		// Error case
		if errMsg, ok := response["error"].(string); ok {
			json.NewEncoder(w).Encode(MCPResponse{Error: errMsg})
			return
		}

		json.NewEncoder(w).Encode(MCPResponse{Error: "Unknown response format"})
	}
}
