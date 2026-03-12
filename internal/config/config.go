package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
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

type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type VoiceConfig struct {
	STTCommand     string `json:"stt_command"`      // Path to whisper-stt
	TTSVoice       string `json:"tts_voice"`        // e.g., ru-RU-DmitryNeural
	SessionTTLDays int    `json:"session_ttl_days"` // Mobile session TTL in days
}

type TelegramConfig struct {
	BotToken string `json:"bot_token"` // Telegram bot token for downloading files
}

type Config struct {
	Port           string            `json:"port"`
	TelegramChatID string            `json:"telegram_chat_id"`
	Tokens         map[string]string `json:"tokens"` // source -> token
	OpenClaw       OpenClawConfig    `json:"openclaw"`
	DocsPath       string            `json:"docs_path"`
	BasicAuth      BasicAuthConfig   `json:"basic_auth"`
	Database       DatabaseConfig    `json:"database"`
	Voice          VoiceConfig       `json:"voice"`
	Telegram       TelegramConfig    `json:"telegram"`
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
		Database: DatabaseConfig{
			Host: "localhost",
			Port: 5433,
			User: "jarvis",
			Name: "jarvis",
		},
		Voice: VoiceConfig{
			STTCommand:     "/usr/local/bin/whisper-stt",
			TTSVoice:       "ru-RU-DmitryNeural",
			SessionTTLDays: 30,
		},
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

	// Database env overrides
	if host := os.Getenv("JARVIS_DB_HOST"); host != "" {
		cfg.Database.Host = host
	}
	if port := os.Getenv("JARVIS_DB_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Database.Port = p
		}
	}
	if user := os.Getenv("JARVIS_DB_USER"); user != "" {
		cfg.Database.User = user
	}
	if pass := os.Getenv("JARVIS_DB_PASSWORD"); pass != "" {
		cfg.Database.Password = pass
	}
	if name := os.Getenv("JARVIS_DB_NAME"); name != "" {
		cfg.Database.Name = name
	}

	// Voice env overrides
	if stt := os.Getenv("JARVIS_STT_COMMAND"); stt != "" {
		cfg.Voice.STTCommand = stt
	}
	if tts := os.Getenv("JARVIS_TTS_VOICE"); tts != "" {
		cfg.Voice.TTSVoice = tts
	}

	// Telegram env overrides
	if botToken := os.Getenv("TELEGRAM_BOT_TOKEN"); botToken != "" {
		cfg.Telegram.BotToken = botToken
	}

	// Token overrides: JARVIS_TOKEN_CALENDAR, JARVIS_TOKEN_GMAIL, etc.
	tokenSources := []string{"calendar", "gmail", "github", "custom", "qr", "voice"}
	for _, src := range tokenSources {
		envKey := "JARVIS_TOKEN_" + src
		if token := os.Getenv(envKey); token != "" {
			cfg.Tokens[src] = token
		}
	}

	return cfg, nil
}
