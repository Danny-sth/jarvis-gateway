package channels

import (
	"encoding/base64"
	"fmt"
	"log"

	"duq-gateway/internal/websocket"
)

// AndroidChannel sends responses via WebSocket to Android clients
type AndroidChannel struct {
	hub      *websocket.Hub
	fallback *TelegramChannel
}

// NewAndroidChannel creates a new Android WebSocket channel
func NewAndroidChannel(hub *websocket.Hub, fallback *TelegramChannel) *AndroidChannel {
	return &AndroidChannel{
		hub:      hub,
		fallback: fallback,
	}
}

func (c *AndroidChannel) Name() string {
	return "android"
}

func (c *AndroidChannel) CanHandle(ctx *ResponseContext) bool {
	// Can handle if hub exists and there's a user to send to
	// Note: We don't check for active connections here - if no connection,
	// the message will be dropped (user offline) or fallback to telegram
	return c.hub != nil
}

func (c *AndroidChannel) Send(ctx *ResponseContext) error {
	// Build WebSocket message
	msg := &websocket.Message{
		Type:   "response",
		TaskID: ctx.TaskID,
		Text:   ctx.Response,
	}

	// Add voice data if present (base64 encoded)
	if len(ctx.VoiceData) > 0 {
		msg.VoiceData = base64.StdEncoding.EncodeToString(ctx.VoiceData)
		msg.Waveform = ctx.Waveform
		msg.AudioDurationMs = ctx.AudioDurationMs
	}

	// Get userID from ChatID (for Telegram users, ChatID == UserID)
	userID := ctx.UserID
	if userID == "" {
		// Fallback: format ChatID as string
		userID = formatUserID(ctx.ChatID)
	}

	// Check if user has active WebSocket connections
	connCount := c.hub.GetConnectionCount(userID)
	if connCount == 0 {
		log.Printf("[android] No active connections for user=%s, falling back to telegram", userID)
		// Fallback to Telegram if available
		if c.fallback != nil && c.fallback.CanHandle(ctx) {
			return c.fallback.Send(ctx)
		}
		log.Printf("[android] No fallback available, message dropped for user=%s", userID)
		return nil
	}

	// Send via WebSocket
	c.hub.SendToUser(userID, msg)

	log.Printf("[android] Sent to user=%s (%d connections), text=%d bytes, voice=%d bytes",
		userID, connCount, len(ctx.Response), len(ctx.VoiceData))

	return nil
}

func formatUserID(chatID int64) string {
	return fmt.Sprintf("%d", chatID)
}
