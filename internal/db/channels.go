package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// ChannelConfig represents channel configuration
type ChannelConfig struct {
	ResponseMode        string   `json:"response_mode"`
	ObserveAll          bool     `json:"observe_all"`
	AllowedTools        []string `json:"allowed_tools,omitempty"`
	PersonalityOverride string   `json:"personality_override,omitempty"`
}

// Channel represents a group/channel entity
type Channel struct {
	ID          int64
	TelegramID  int64
	ChannelType string
	Title       string
	Username    string
	Config      ChannelConfig
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// UpsertChannel creates or updates a channel record
func (c *Client) UpsertChannel(telegramID int64, channelType, title, username string) (int64, error) {
	var id int64
	err := c.db.QueryRow(`
		INSERT INTO channels (telegram_id, channel_type, title, username)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (telegram_id) DO UPDATE SET
			channel_type = EXCLUDED.channel_type,
			title = COALESCE(EXCLUDED.title, channels.title),
			username = COALESCE(EXCLUDED.username, channels.username),
			updated_at = NOW()
		RETURNING id
	`, telegramID, channelType, title, username).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("upsert channel failed: %w", err)
	}

	log.Printf("[db] Upserted channel: telegram_id=%d type=%s id=%d", telegramID, channelType, id)
	return id, nil
}

// GetChannelConfig returns channel configuration
func (c *Client) GetChannelConfig(telegramID int64) (*ChannelConfig, error) {
	var configJSON []byte
	err := c.db.QueryRow(`
		SELECT config FROM channels WHERE telegram_id = $1
	`, telegramID).Scan(&configJSON)

	if err == sql.ErrNoRows {
		// Return default config if channel not found
		return &ChannelConfig{
			ResponseMode: "full_llm",
			ObserveAll:   true,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get channel config failed: %w", err)
	}

	var config ChannelConfig
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return nil, fmt.Errorf("parse channel config failed: %w", err)
	}

	return &config, nil
}

// UpdateChannelMembership tracks user membership in channel
// Note: channel_id in channel_memberships table is the telegram_id (BIGINT), not internal id
func (c *Client) UpdateChannelMembership(userID int64, channelTelegramID int64, isActive bool) error {
	if isActive {
		_, err := c.db.Exec(`
			INSERT INTO channel_memberships (user_id, channel_id, is_active)
			VALUES ($1, $2, TRUE)
			ON CONFLICT (user_id, channel_id)
			DO UPDATE SET is_active = TRUE, left_at = NULL
		`, userID, channelTelegramID)
		if err != nil {
			return fmt.Errorf("update channel membership (join) failed: %w", err)
		}
		return nil
	}

	_, err := c.db.Exec(`
		UPDATE channel_memberships
		SET is_active = FALSE, left_at = NOW()
		WHERE user_id = $1 AND channel_id = $2
	`, userID, channelTelegramID)
	if err != nil {
		return fmt.Errorf("update channel membership (leave) failed: %w", err)
	}
	return nil
}
