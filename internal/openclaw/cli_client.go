package openclaw

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"jarvis-gateway/internal/config"
)

// CLIClient communicates with OpenClaw via CLI
type CLIClient struct {
	defaultUser string
	timeout     time.Duration
}

func NewCLIClient(cfg *config.Config) *CLIClient {
	return &CLIClient{
		defaultUser: "telegram:" + cfg.TelegramChatID,
		timeout:     120 * time.Second,
	}
}

// SendMessage sends a message to the default user via agent
func (c *CLIClient) SendMessage(message string) error {
	_, err := c.Send(message, c.defaultUser)
	return err
}

// Send sends a message to a specific user via agent with delivery
func (c *CLIClient) Send(message, userID string) (string, error) {
	// Parse userID format: "telegram:764733417" -> channel=telegram, target=764733417
	channel := "telegram"
	target := userID
	if idx := indexOf(userID, ":"); idx != -1 {
		channel = userID[:idx]
		target = userID[idx+1:]
	}

	// Build command: openclaw agent --channel telegram --to 764733417 --message "..." --deliver
	args := []string{
		"agent",
		"--channel", channel,
		"--to", target,
		"--message", message,
		"--deliver",
	}

	log.Printf("[openclaw-cli] Executing: openclaw %s", strings.Join(args, " "))

	cmd := exec.Command("openclaw", args...)

	// Run with timeout
	done := make(chan error, 1)
	var output []byte
	var cmdErr error

	go func() {
		output, cmdErr = cmd.CombinedOutput()
		done <- cmdErr
	}()

	select {
	case <-time.After(c.timeout):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return "", fmt.Errorf("command timeout after %v", c.timeout)
	case err := <-done:
		if err != nil {
			log.Printf("[openclaw-cli] Command error: %v, output: %s", err, string(output))
			return "", fmt.Errorf("openclaw command failed: %w", err)
		}
	}

	result := strings.TrimSpace(string(output))
	log.Printf("[openclaw-cli] Success, output length: %d", len(result))

	return result, nil
}

// indexOf returns the index of sep in s, or -1 if not found
func indexOf(s, sep string) int {
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			return i
		}
	}
	return -1
}
