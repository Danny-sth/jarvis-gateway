package channels

import (
	"fmt"
	"log"
)

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

	// Send to primary channel
	err := ch.Send(ctx)

	// Broadcast to WebSocket for real-time sync (if not already sent via android channel)
	// This ensures all connected Android clients receive updates regardless of source
	if ch.Name() != "android" && r.androidChannel != nil && r.androidChannel.CanHandle(ctx) {
		// Fire-and-forget broadcast - don't fail primary delivery if WS fails
		go func() {
			if wsErr := r.androidChannel.Send(ctx); wsErr != nil {
				log.Printf("[router] WebSocket broadcast failed: %v", wsErr)
			}
		}()
	}

	return err
}

// routeWithFallbackNotice sends error to fallback and then original response
func (r *Router) routeWithFallbackNotice(failed, fallback Channel, ctx *ResponseContext) error {
	// This is handled by individual channels now
	return fallback.Send(ctx)
}
