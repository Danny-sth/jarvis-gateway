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

type CalendarEvent struct {
	Title     string `json:"title"`
	StartTime string `json:"start_time"` // RFC3339
	EndTime   string `json:"end_time"`   // RFC3339
	Location  string `json:"location,omitempty"`
	MeetLink  string `json:"meet_link,omitempty"`
	EventID   string `json:"event_id,omitempty"`
}

type CalendarWebhook struct {
	Type    string        `json:"type"` // "reminder", "created", "updated", "cancelled"
	Event   CalendarEvent `json:"event"`
	Minutes int           `json:"minutes_before,omitempty"` // for reminders
}

// CalendarDeps contains dependencies for calendar handler
type CalendarDeps struct {
	Config      *config.Config
	QueueClient *queue.Client
}

func Calendar(deps *CalendarDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var webhook CalendarWebhook
		if err := json.NewDecoder(r.Body).Decode(&webhook); err != nil {
			http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		message := formatCalendarMessage(webhook)
		if message == "" {
			http.Error(w, "Unknown webhook type", http.StatusBadRequest)
			return
		}

		log.Printf("Calendar webhook: %s - %s", webhook.Type, webhook.Event.Title)

		// Push to Redis queue instead of direct HTTP
		callbackURL := fmt.Sprintf("http://%s/api/duq/callback", deps.Config.GatewayHost)
		task := &queue.Task{
			UserID:      deps.Config.TelegramChatID,
			Type:        "notification",
			Priority:    70,
			CallbackURL: callbackURL,
			Payload: map[string]interface{}{
				"message":        message,
				"output_channel": "telegram",
				"source":         "calendar_webhook",
			},
			RequestMetadata: map[string]interface{}{
				"chat_id": deps.Config.TelegramChatID, // string, will be parsed by callback handler
				"source":  "calendar",
			},
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		taskID, err := deps.QueueClient.Push(ctx, task)
		if err != nil {
			log.Printf("Failed to queue calendar notification: %v", err)
			http.Error(w, "Failed to queue notification", http.StatusInternalServerError)
			return
		}

		log.Printf("Calendar notification queued: task_id=%s", taskID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "task_id": taskID})
	}
}

func formatCalendarMessage(webhook CalendarWebhook) string {
	event := webhook.Event

	// Parse start time
	startTime, err := time.Parse(time.RFC3339, event.StartTime)
	if err != nil {
		startTime = time.Now()
	}
	timeStr := startTime.Format("15:04")

	switch webhook.Type {
	case "reminder":
		msg := fmt.Sprintf("Напоминание: %s в %s", event.Title, timeStr)
		if event.MeetLink != "" {
			msg += fmt.Sprintf("\nСсылка: %s", event.MeetLink)
		}
		if event.Location != "" {
			msg += fmt.Sprintf("\nМесто: %s", event.Location)
		}
		return msg

	case "created":
		return fmt.Sprintf("Новое событие: %s в %s", event.Title, timeStr)

	case "updated":
		return fmt.Sprintf("Событие изменено: %s, новое время: %s", event.Title, timeStr)

	case "cancelled":
		return fmt.Sprintf("Событие отменено: %s", event.Title)

	default:
		return ""
	}
}
