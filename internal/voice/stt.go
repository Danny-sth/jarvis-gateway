package voice

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

// STTConfig holds STT configuration
type STTConfig struct {
	Command string // Path to whisper-stt command
	Timeout time.Duration
}

// DefaultSTTConfig returns default STT configuration
func DefaultSTTConfig() STTConfig {
	return STTConfig{
		Command: "/usr/local/bin/whisper-stt",
		Timeout: 120 * time.Second,
	}
}

// Transcribe converts audio file to text using whisper-stt
func Transcribe(cfg STTConfig, wavPath string) (string, error) {
	log.Printf("[stt] Transcribing: %s", wavPath)
	start := time.Now()

	cmd := exec.Command(cfg.Command, wavPath)

	// Run with timeout
	done := make(chan error, 1)
	var output []byte
	var cmdErr error

	go func() {
		output, cmdErr = cmd.CombinedOutput()
		done <- cmdErr
	}()

	select {
	case <-time.After(cfg.Timeout):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return "", fmt.Errorf("STT timeout after %v", cfg.Timeout)
	case err := <-done:
		if err != nil {
			log.Printf("[stt] Command failed: %v, output: %s", err, string(output))
			return "", fmt.Errorf("STT failed: %w", err)
		}
	}

	text := strings.TrimSpace(string(output))
	log.Printf("[stt] Transcription complete in %v: %q", time.Since(start), truncate(text, 50))

	return text, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
