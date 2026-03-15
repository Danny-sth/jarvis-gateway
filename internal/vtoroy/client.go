package vtoroy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"jarvis-gateway/internal/config"
)

// Client communicates with Vtoroy via HTTP API
type Client struct {
	baseURL     string
	httpClient  *http.Client
	defaultUser string
}

// ChatRequest represents the request body for /api/chat
type ChatRequest struct {
	Message string  `json:"message"`
	UserID  *string `json:"user_id,omitempty"`
	Role    string  `json:"role"`
}

// ChatResponse represents the response from /api/chat
type ChatResponse struct {
	Response string `json:"response"`
	UserID   string `json:"user_id"`
}

// NewClient creates a new Vtoroy HTTP client
func NewClient(cfg *config.Config) *Client {
	baseURL := cfg.VtoroyURL
	if baseURL == "" {
		baseURL = "http://localhost:8081"
	}

	return &Client{
		baseURL:     baseURL,
		defaultUser: cfg.TelegramChatID,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// SendMessage sends a message to the default user (with delivery via Telegram)
func (c *Client) SendMessage(message string) error {
	_, err := c.Send(message, c.defaultUser)
	return err
}

// Send sends a message to a specific user and returns the response
func (c *Client) Send(message, userID string) (string, error) {
	return c.chat(message, userID, true)
}

// SendWithoutDeliver sends a message without delivery (returns response only)
func (c *Client) SendWithoutDeliver(message, userID string) (string, error) {
	return c.chat(message, userID, false)
}

// chat is the internal method that calls vtoroy /api/chat
func (c *Client) chat(message, userID string, deliver bool) (string, error) {
	// For now, vtoroy doesn't have a "deliver" flag in API
	// When deliver=true, the response should be sent to Telegram
	// When deliver=false, we just return the response
	// TODO: Add deliver flag to vtoroy API if needed

	req := ChatRequest{
		Message: message,
		UserID:  &userID,
		Role:    "root",
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + "/api/chat"
	log.Printf("[vtoroy] POST %s user=%s message=%d chars", url, userID, len(message))

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("[vtoroy] Error response: %d %s", resp.StatusCode, string(respBody))
		return "", fmt.Errorf("vtoroy returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	log.Printf("[vtoroy] Success, response=%d chars", len(chatResp.Response))
	return chatResp.Response, nil
}
