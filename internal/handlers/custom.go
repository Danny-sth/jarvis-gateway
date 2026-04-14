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

type CustomWebhook struct {
	Message string `json:"message"`
	Source  string `json:"source,omitempty"`
}

// CustomDeps contains dependencies for custom handler
type CustomDeps struct {
	Config      *config.Config
	QueueClient *queue.Client
}

func Custom(deps *CustomDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var webhook CustomWebhook
		if err := json.NewDecoder(r.Body).Decode(&webhook); err != nil {
			http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		if webhook.Message == "" {
			http.Error(w, "Message is required", http.StatusBadRequest)
			return
		}

		log.Printf("Custom webhook from: %s", webhook.Source)

		// Push to Redis queue instead of direct HTTP
		callbackURL := fmt.Sprintf("http://%s/api/duq/callback", deps.Config.GatewayHost)
		task := &queue.Task{
			UserID:      deps.Config.TelegramChatID,
			Type:        "notification",
			Priority:    50,
			CallbackURL: callbackURL,
			Payload: map[string]interface{}{
				"message":        webhook.Message,
				"output_channel": "telegram",
				"source":         "custom_webhook",
			},
			RequestMetadata: map[string]interface{}{
				"chat_id": deps.Config.TelegramChatID,
				"source":  webhook.Source,
			},
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		taskID, err := deps.QueueClient.Push(ctx, task)
		if err != nil {
			log.Printf("Failed to queue custom notification: %v", err)
			http.Error(w, "Failed to queue notification", http.StatusInternalServerError)
			return
		}

		log.Printf("Custom notification queued: task_id=%s", taskID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "task_id": taskID})
	}
}
