package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"duq-gateway/internal/config"
)

// VoiceFileInfo contains information about a voice/audio file from Telegram
type VoiceFileInfo struct {
	FileID   string `json:"file_id"`
	FileSize int    `json:"file_size,omitempty"`
	Duration int    `json:"duration,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
}

// getVoiceFileURL gets the download URL for a Telegram file
// This is used by the backend worker to download and transcribe
func getVoiceFileURL(cfg *config.Config, fileID string) (string, error) {
	// Get file path from Telegram
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getFile?file_id=%s",
		cfg.Telegram.BotToken, fileID)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to get file info: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			FilePath string `json:"file_path"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.OK || result.Result.FilePath == "" {
		return "", fmt.Errorf("telegram API error: %s", string(body))
	}

	// Build download URL
	downloadURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s",
		cfg.Telegram.BotToken, result.Result.FilePath)

	return downloadURL, nil
}

// downloadVoiceFile downloads voice/audio file from Telegram
// Returns the raw audio bytes
func downloadVoiceFile(cfg *config.Config, fileID string) ([]byte, error) {
	downloadURL, err := getVoiceFileURL(cfg, fileID)
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(downloadURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return data, nil
}

// transcribeVoice is now a pass-through - actual transcription happens in backend worker
// Gateway just downloads and base64 encodes the audio for the queue
// Returns empty string - the text will come from backend after processing
func transcribeVoice(cfg *config.Config, fileID string) (string, error) {
	// In queue-based architecture, we don't transcribe in gateway
	// We pass the file_id through the queue and backend handles transcription
	// Return placeholder - the actual message will use voice_file_id in payload
	return "", nil
}
