package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type BasicAuthConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type OpenClawConfig struct {
	GatewayURL string `json:"gateway_url"` // http://127.0.0.1:18789
	Token      string `json:"token"`       // Gateway auth token
	AgentID    string `json:"agent_id"`    // default: "main"
}

type Config struct {
	Port           string            `json:"port"`
	TelegramChatID string            `json:"telegram_chat_id"`
	Tokens         map[string]string `json:"tokens"` // source -> token
	OpenClaw       OpenClawConfig    `json:"openclaw"`
	DocsPath       string            `json:"docs_path"`
	BasicAuth      BasicAuthConfig   `json:"basic_auth"`
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:           "8082",
		TelegramChatID: "764733417",
		OpenClaw: OpenClawConfig{
			GatewayURL: "http://127.0.0.1:18789",
			AgentID:    "main",
		},
		Tokens: make(map[string]string),
	}

	// Try to load from config file
	configPaths := []string{
		"/etc/jarvis-gateway/config.json",
		filepath.Join(os.Getenv("HOME"), ".config/jarvis-gateway/config.json"),
		"config.json",
	}

	for _, path := range configPaths {
		if data, err := os.ReadFile(path); err == nil {
			if err := json.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
			break
		}
	}

	// Override from environment
	if port := os.Getenv("JARVIS_PORT"); port != "" {
		cfg.Port = port
	}
	if chatID := os.Getenv("JARVIS_TELEGRAM_CHAT_ID"); chatID != "" {
		cfg.TelegramChatID = chatID
	}
	if url := os.Getenv("OPENCLAW_GATEWAY_URL"); url != "" {
		cfg.OpenClaw.GatewayURL = url
	}
	if token := os.Getenv("OPENCLAW_GATEWAY_TOKEN"); token != "" {
		cfg.OpenClaw.Token = token
	}
	if agentID := os.Getenv("OPENCLAW_AGENT_ID"); agentID != "" {
		cfg.OpenClaw.AgentID = agentID
	}

	// Token overrides: JARVIS_TOKEN_CALENDAR, JARVIS_TOKEN_GMAIL, etc.
	tokenSources := []string{"calendar", "gmail", "github", "custom"}
	for _, src := range tokenSources {
		envKey := "JARVIS_TOKEN_" + src
		if token := os.Getenv(envKey); token != "" {
			cfg.Tokens[src] = token
		}
	}

	return cfg, nil
}
