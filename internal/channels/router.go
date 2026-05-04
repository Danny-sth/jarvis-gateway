package channels

import (
	"fmt"
	"log"
	"sync"
)

// SyncMessage represents a message to be synced between channels
type SyncMessage struct {
	MessageID      string `json:"message_id"`
	ConversationID string `json:"conversation_id"`
	UserID         string `json:"user_id"` // keycloak_sub
	Role           string `json:"role"`    // "user" or "assistant"
	Content        string `json:"content"`
	SourceChannel  string `json:"source_channel"` // "telegram", "android", etc.
}

// SyncChannel interface for channels that support history sync
type SyncChannel interface {
	Channel
	// SyncSend delivers a synced message from another channel
	SyncSend(msg *SyncMessage) error
	// IsUserConnected checks if user has connection on this channel
	IsUserConnected(userID string) bool
}

// PendingSync tracks pending Telegram messages for editMessage
type PendingSync struct {
	mu       sync.RWMutex
	pending  map[string]*PendingMessage // userID -> pending message
}

// PendingMessage holds state for Telegram editMessage
type PendingMessage struct {
	TelegramMessageID int64    // Message ID to edit
	ChatID            int64    // Telegram chat ID
	Quotes            []string // Accumulated user messages
}

// ResponseContext contains all info needed to route a response
type ResponseContext struct {
	ChatID    int64
	UserID    string // Keycloak sub or telegram_id
	UserEmail string
	Response  string
	IsVoice   bool
	TaskID    string // Task ID for correlation
	Source    string // Request source: "telegram", "android", "mcp"

	// Voice-aware fields (from Duq response)
	OutputType      string    // "text", "voice", or "both"
	VoicePriority   string    // "high", "normal", or "skip"
	VoiceData       []byte    // Audio bytes (MP3 from Duq)
	VoiceFormat     string    // Audio format (default: "mp3")
	Waveform        []float64 // Audio waveform for visualization
	AudioDurationMs int       // Audio duration in milliseconds

	// OAuth credentials for email channel
	GoogleAccessToken  string
	GoogleRefreshToken string
}

// Channel is the interface for output channels (Open/Closed, Dependency Inversion)
type Channel interface {
	// Name returns channel identifier
	Name() string
	// Send delivers response to this channel
	Send(ctx *ResponseContext) error
	// CanHandle returns true if this channel can handle the context
	CanHandle(ctx *ResponseContext) bool
}

// Router routes responses to appropriate channels
type Router struct {
	channels       map[string]Channel
	defaultChannel Channel
	fallback       Channel
	androidChannel Channel // For WebSocket broadcast to all connected clients
}

// NewRouter creates a router with registered channels
func NewRouter(channels []Channel, defaultName string) *Router {
	r := &Router{
		channels: make(map[string]Channel),
	}

	for _, ch := range channels {
		r.channels[ch.Name()] = ch
		if ch.Name() == defaultName {
			r.defaultChannel = ch
		}
		if ch.Name() == "telegram" {
			r.fallback = ch
		}
		if ch.Name() == "android" {
			r.androidChannel = ch
		}
	}

	// If no default set, use telegram
	if r.defaultChannel == nil && r.fallback != nil {
		r.defaultChannel = r.fallback
	}

	return r
}

// Route sends response to the specified channel
// Gateway is a dumb executor - agent decides everything via select_output_channel tool
// Gateway just delivers agent's response to the channel specified by agent
func (r *Router) Route(channelName string, ctx *ResponseContext) error {
	// Channel MUST be set by agent (via select_output_channel) or by worker fallback
	// Gateway does NOT decide routing - it just executes
	if channelName == "" {
		log.Printf("[router] ERROR: channel not specified, using default telegram")
		channelName = "telegram"
	}

	ch, ok := r.channels[channelName]
	if !ok {
		log.Printf("[router] Unknown channel '%s', using default", channelName)
		ch = r.defaultChannel
	}

	if ch == nil {
		return fmt.Errorf("no channel available")
	}

	// Check if channel can handle this context
	if !ch.CanHandle(ctx) {
		log.Printf("[router] Channel '%s' cannot handle context, falling back", ch.Name())
		if r.fallback != nil && r.fallback.CanHandle(ctx) {
			return r.routeWithFallbackNotice(ch, r.fallback, ctx)
		}
		return fmt.Errorf("channel '%s' cannot handle context and no fallback available", ch.Name())
	}

	// Send to primary channel only
	// NOTE: No broadcast to other channels. History sync is handled via:
	// - Messages saved to DB by Duq backend
	// - Android app fetches history via GET /api/messages
	// - WebSocket used only when android is the PRIMARY channel (user sends from app)
	return ch.Send(ctx)
}

// routeWithFallbackNotice sends error to fallback and then original response
func (r *Router) routeWithFallbackNotice(failed, fallback Channel, ctx *ResponseContext) error {
	// This is handled by individual channels now
	return fallback.Send(ctx)
}

// Broadcast sends a sync message to all channels except the source
// This implements fan-out pattern for multi-channel history sync
func (r *Router) Broadcast(msg *SyncMessage) {
	if msg.SourceChannel == "" {
		log.Printf("[router] WARNING: Broadcast called without source_channel")
		return
	}

	log.Printf("[router] Broadcasting: user=%s role=%s source=%s",
		truncateID(msg.UserID), msg.Role, msg.SourceChannel)

	for name, ch := range r.channels {
		// Skip source channel - don't echo back
		if name == msg.SourceChannel {
			continue
		}

		// Check if channel supports sync
		syncCh, ok := ch.(SyncChannel)
		if !ok {
			continue
		}

		// Check if user is connected to this channel
		if !syncCh.IsUserConnected(msg.UserID) {
			log.Printf("[router] User %s not connected to %s, skipping sync",
				truncateID(msg.UserID), name)
			continue
		}

		// Send sync message
		if err := syncCh.SyncSend(msg); err != nil {
			log.Printf("[router] Failed to sync to %s: %v", name, err)
		} else {
			log.Printf("[router] Synced to %s: user=%s role=%s",
				name, truncateID(msg.UserID), msg.Role)
		}
	}
}

// truncateID returns first 8 chars of ID for logging
func truncateID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

// NewPendingSync creates a new pending sync tracker
func NewPendingSync() *PendingSync {
	return &PendingSync{
		pending: make(map[string]*PendingMessage),
	}
}

// Get returns pending message for user
func (p *PendingSync) Get(userID string) *PendingMessage {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pending[userID]
}

// Set stores pending message for user
func (p *PendingSync) Set(userID string, msg *PendingMessage) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pending[userID] = msg
}

// Clear removes pending message for user
func (p *PendingSync) Clear(userID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.pending, userID)
}
