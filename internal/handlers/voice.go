package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"jarvis-gateway/internal/config"
	"jarvis-gateway/internal/db"
	"jarvis-gateway/internal/openclaw"
	"jarvis-gateway/internal/voice"
)

// VoiceResponse represents the voice endpoint response
type VoiceResponse struct {
	Text  string `json:"text"`
	Audio string `json:"audio"` // base64 encoded OGG
}

// VoiceErrorResponse represents an error response
type VoiceErrorResponse struct {
	Error string `json:"error"`
}

// Voice handles voice message processing: WAV -> STT -> OpenClaw -> TTS -> OGG
func Voice(cfg *config.Config, dbClient *db.Client) http.HandlerFunc {
	client := openclaw.NewClient(cfg)

	sttCfg := voice.STTConfig{
		Command: cfg.Voice.STTCommand,
		Timeout: voice.DefaultSTTConfig().Timeout,
	}
	if sttCfg.Command == "" {
		sttCfg = voice.DefaultSTTConfig()
	}

	ttsCfg := voice.TTSConfig{
		Voice:   cfg.Voice.TTSVoice,
		Timeout: voice.DefaultTTSConfig().Timeout,
	}
	if ttsCfg.Voice == "" {
		ttsCfg = voice.DefaultTTSConfig()
	}

	return func(w http.ResponseWriter, r *http.Request) {
		// Get telegram_id from context (set by MobileAuth middleware)
		telegramID, ok := r.Context().Value("telegram_id").(int64)
		if !ok {
			sendVoiceError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		userID := fmt.Sprintf("telegram:%d", telegramID)
		log.Printf("[voice] Request from %s", userID)

		// Parse multipart form (max 20MB)
		if err := r.ParseMultipartForm(20 << 20); err != nil {
			sendVoiceError(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Get audio file
		file, header, err := r.FormFile("audio")
		if err != nil {
			sendVoiceError(w, "Audio file required", http.StatusBadRequest)
			return
		}
		defer file.Close()

		log.Printf("[voice] Received audio: %s, size=%d", header.Filename, header.Size)

		// Save to temp file
		tempDir := os.TempDir()
		wavPath := filepath.Join(tempDir, uuid.New().String()+".wav")

		tempFile, err := os.Create(wavPath)
		if err != nil {
			sendVoiceError(w, "Failed to save audio", http.StatusInternalServerError)
			return
		}

		_, err = io.Copy(tempFile, file)
		tempFile.Close()
		if err != nil {
			os.Remove(wavPath)
			sendVoiceError(w, "Failed to save audio", http.StatusInternalServerError)
			return
		}

		defer os.Remove(wavPath)

		// Step 1: STT - transcribe audio to text
		text, err := voice.Transcribe(sttCfg, wavPath)
		if err != nil {
			log.Printf("[voice] STT failed: %v", err)
			sendVoiceError(w, "Speech recognition failed", http.StatusInternalServerError)
			return
		}

		if text == "" {
			sendVoiceError(w, "No speech detected", http.StatusBadRequest)
			return
		}

		log.Printf("[voice] STT result: %q", truncate(text, 100))

		// Step 2: OpenClaw - process message
		response, err := client.Send(text, userID)
		if err != nil {
			log.Printf("[voice] OpenClaw failed: %v", err)
			sendVoiceError(w, "Agent processing failed", http.StatusInternalServerError)
			return
		}

		log.Printf("[voice] Agent response: %q", truncate(response, 100))

		// Step 3: TTS - synthesize response to audio
		oggBytes, err := voice.SynthesizeToOGG(ttsCfg, response)
		if err != nil {
			log.Printf("[voice] TTS failed: %v", err)
			sendVoiceError(w, "Speech synthesis failed", http.StatusInternalServerError)
			return
		}

		// Return JSON response with base64 audio
		resp := VoiceResponse{
			Text:  response,
			Audio: base64.StdEncoding.EncodeToString(oggBytes),
		}

		log.Printf("[voice] Success: text=%d chars, audio=%d bytes",
			len(response), len(oggBytes))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func sendVoiceError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(VoiceErrorResponse{Error: message})
}

// truncate truncates string to max length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
