package channels

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// EmailSender is an abstraction for sending emails (Dependency Inversion)
type EmailSender interface {
	Send(to, subject, body, accessToken string) error
}

// GmailAPISender sends emails via Gmail API using OAuth tokens
type GmailAPISender struct{}

func (s *GmailAPISender) Send(to, subject, body, accessToken string) error {
	// Build RFC 2822 formatted email
	emailContent := fmt.Sprintf(
		"To: %s\r\n"+
			"Subject: %s\r\n"+
			"Content-Type: text/plain; charset=utf-8\r\n"+
			"\r\n"+
			"%s",
		to, subject, body,
	)

	// Base64url encode the email
	encoded := base64.URLEncoding.EncodeToString([]byte(emailContent))
	// Remove padding for Gmail API
	encoded = strings.TrimRight(encoded, "=")

	// Build request body
	reqBody := map[string]string{"raw": encoded}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send via Gmail API
	req, err := http.NewRequest(
		"POST",
		"https://gmail.googleapis.com/gmail/v1/users/me/messages/send",
		bytes.NewBuffer(jsonBody),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[email] Gmail API error: %s", string(respBody))
		return fmt.Errorf("Gmail API error: %d", resp.StatusCode)
	}

	log.Printf("[email] Sent email to %s via Gmail API", to)
	return nil
}

// EmailChannel sends responses via email
type EmailChannel struct {
	sender   EmailSender
	fallback *TelegramChannel
}

// NewEmailChannel creates a new email channel with fallback
func NewEmailChannel(sender EmailSender, fallback *TelegramChannel) *EmailChannel {
	return &EmailChannel{
		sender:   sender,
		fallback: fallback,
	}
}

func (c *EmailChannel) Name() string {
	return "email"
}

func (c *EmailChannel) CanHandle(ctx *ResponseContext) bool {
	// Нужен email И OAuth токен
	return ctx.UserEmail != "" && ctx.GoogleAccessToken != ""
}

func (c *EmailChannel) Send(ctx *ResponseContext) error {
	if ctx.UserEmail == "" {
		// Fallback to telegram with error message
		if c.fallback != nil {
			c.fallback.SendTextMessage(ctx.ChatID, "❌ Google аккаунт не подключён. Подключи через /connect_google")
			return c.fallback.Send(ctx)
		}
		return fmt.Errorf("no email configured and no fallback available")
	}

	if ctx.GoogleAccessToken == "" {
		if c.fallback != nil {
			c.fallback.SendTextMessage(ctx.ChatID, "❌ OAuth токен не найден. Переподключи Google через /connect_google")
			return c.fallback.Send(ctx)
		}
		return fmt.Errorf("no OAuth token available")
	}

	err := c.sender.Send(ctx.UserEmail, "Duq Report", ctx.Response, ctx.GoogleAccessToken)
	if err != nil {
		// Fallback to telegram with error + response
		if c.fallback != nil {
			c.fallback.SendTextMessage(ctx.ChatID, "❌ Не удалось отправить на email. Ответ:\n\n"+ctx.Response)
		}
		return err
	}

	// Notify user via telegram that email was sent
	if c.fallback != nil {
		c.fallback.SendTextMessage(ctx.ChatID, fmt.Sprintf("📧 Ответ отправлен на %s", ctx.UserEmail))
	}

	return nil
}
