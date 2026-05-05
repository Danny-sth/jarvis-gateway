package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"duq-gateway/internal/config"
)

const (
	// PollingTimeout is the long polling timeout in seconds
	PollingTimeout = 30
	// RetryDelay is the delay between polling errors
	RetryDelay = 5 * time.Second
)

// Poller handles Telegram long polling as alternative to webhooks
type Poller struct {
	config      *config.Config
	webhookURL  string
	client      *http.Client
	lastOffset  int64
	running     bool
	mu          sync.Mutex
	stopCh      chan struct{}
}

// TelegramUpdate represents an incoming update from Telegram
type TelegramUpdate struct {
	UpdateID      int64       `json:"update_id"`
	Message       interface{} `json:"message,omitempty"`
	ChannelPost   interface{} `json:"channel_post,omitempty"` // Posts in channels
	CallbackQuery interface{} `json:"callback_query,omitempty"`
}

// TelegramResponse represents the API response
type TelegramResponse struct {
	OK     bool             `json:"ok"`
	Result []TelegramUpdate `json:"result,omitempty"`
	Error  string           `json:"description,omitempty"`
}

// NewPoller creates a new Telegram poller
func NewPoller(cfg *config.Config, webhookURL string) *Poller {
	return &Poller{
		config:     cfg,
		webhookURL: webhookURL,
		client: &http.Client{
			Timeout: time.Duration(PollingTimeout+10) * time.Second,
		},
		stopCh: make(chan struct{}),
	}
}

// Start begins long polling in a goroutine
func (p *Poller) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return fmt.Errorf("poller already running")
	}
	p.running = true
	p.mu.Unlock()

	// First, delete any existing webhook
	if err := p.deleteWebhook(); err != nil {
		log.Printf("[telegram-poller] Warning: failed to delete webhook: %v", err)
	}

	log.Printf("[telegram-poller] Starting long polling (timeout=%ds)", PollingTimeout)

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Printf("[telegram-poller] Context cancelled, stopping")
				return
			case <-p.stopCh:
				log.Printf("[telegram-poller] Stop signal received")
				return
			default:
				if err := p.poll(); err != nil {
					log.Printf("[telegram-poller] Polling error: %v", err)
					time.Sleep(RetryDelay)
				}
			}
		}
	}()

	return nil
}

// Stop stops the poller
func (p *Poller) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		close(p.stopCh)
		p.running = false
	}
}

// IsRunning returns true if the poller is running
func (p *Poller) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// deleteWebhook removes any existing webhook to enable getUpdates
func (p *Poller) deleteWebhook() error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/deleteWebhook", p.config.Telegram.BotToken)
	resp, err := p.client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deleteWebhook failed: %s", string(body))
	}

	log.Printf("[telegram-poller] Webhook deleted, switching to long polling")
	return nil
}

// poll performs one long polling request
func (p *Poller) poll() error {
	url := fmt.Sprintf(
		"https://api.telegram.org/bot%s/getUpdates?timeout=%d&offset=%d&allowed_updates=[\"message\",\"channel_post\",\"callback_query\"]",
		p.config.Telegram.BotToken,
		PollingTimeout,
		p.lastOffset,
	)

	resp, err := p.client.Get(url)
	if err != nil {
		return fmt.Errorf("getUpdates failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var result TelegramResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.OK {
		return fmt.Errorf("API error: %s", result.Error)
	}

	// Process each update
	for _, update := range result.Result {
		// Update offset for next poll (add 1 to acknowledge this update)
		if update.UpdateID >= p.lastOffset {
			p.lastOffset = update.UpdateID + 1
		}

		// Forward to webhook handler
		if err := p.forwardToWebhook(update); err != nil {
			log.Printf("[telegram-poller] Failed to forward update %d: %v", update.UpdateID, err)
		}
	}

	return nil
}

// forwardToWebhook sends the update to the internal webhook handler
func (p *Poller) forwardToWebhook(update TelegramUpdate) error {
	// Re-serialize the full update as JSON
	data, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("failed to marshal update: %w", err)
	}

	// Post to internal webhook endpoint
	req, err := http.NewRequest("POST", p.webhookURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to forward update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook returned %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("[telegram-poller] Forwarded update %d to webhook", update.UpdateID)
	return nil
}
