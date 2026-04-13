package channels

import (
	"log"
)

// SilentChannel suppresses output (for background tasks)
type SilentChannel struct{}

// NewSilentChannel creates a new silent channel
func NewSilentChannel() *SilentChannel {
	return &SilentChannel{}
}

func (c *SilentChannel) Name() string {
	return "silent"
}

func (c *SilentChannel) CanHandle(ctx *ResponseContext) bool {
	return true // Can always handle (by doing nothing)
}

func (c *SilentChannel) Send(ctx *ResponseContext) error {
	log.Printf("[silent] Response suppressed (len=%d chars)", len(ctx.Response))
	return nil
}
