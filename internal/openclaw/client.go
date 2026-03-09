package openclaw

import (
	"fmt"
	"os/exec"

	"jarvis-gateway/internal/config"
)

type Client struct {
	bin    string
	chatID string
}

func NewClient(cfg *config.Config) *Client {
	return &Client{
		bin:    cfg.OpenClawBin,
		chatID: cfg.TelegramChatID,
	}
}

// SendMessage sends a message to Telegram via OpenClaw
func (c *Client) SendMessage(message string) error {
	cmd := exec.Command(c.bin, "agent",
		"--to", "telegram:"+c.chatID,
		"--deliver",
		"-m", message,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("openclaw error: %v, output: %s", err, string(output))
	}

	return nil
}

// SendWithContext sends a message that JARVIS should process (not just deliver)
func (c *Client) SendWithContext(message string) error {
	cmd := exec.Command(c.bin, "agent",
		"--to", "telegram:"+c.chatID,
		"-m", message,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("openclaw error: %v, output: %s", err, string(output))
	}

	return nil
}
