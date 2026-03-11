package voice

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// TTSConfig holds TTS configuration
type TTSConfig struct {
	Voice   string // e.g., "ru-RU-DmitryNeural"
	Timeout time.Duration
}

// DefaultTTSConfig returns default TTS configuration
func DefaultTTSConfig() TTSConfig {
	return TTSConfig{
		Voice:   "ru-RU-DmitryNeural",
		Timeout: 60 * time.Second,
	}
}

// Synthesize converts text to speech using edge-tts
// Returns path to generated MP3 file
func Synthesize(cfg TTSConfig, text string) (string, error) {
	log.Printf("[tts] Synthesizing: %q", truncate(text, 50))
	start := time.Now()

	// Create temp file for output
	tmpFile := filepath.Join(os.TempDir(), uuid.New().String()+".mp3")

	cmd := exec.Command("edge-tts",
		"--text", text,
		"--voice", cfg.Voice,
		"--write-media", tmpFile,
	)

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
		os.Remove(tmpFile)
		return "", fmt.Errorf("TTS timeout after %v", cfg.Timeout)
	case err := <-done:
		if err != nil {
			log.Printf("[tts] Command failed: %v, output: %s", err, string(output))
			os.Remove(tmpFile)
			return "", fmt.Errorf("TTS failed: %w", err)
		}
	}

	// Verify file exists
	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		return "", fmt.Errorf("TTS output file not created")
	}

	log.Printf("[tts] Synthesis complete in %v: %s", time.Since(start), tmpFile)
	return tmpFile, nil
}

// ConvertToOGG converts MP3 to OGG Vorbis using ffmpeg
func ConvertToOGG(mp3Path string) ([]byte, error) {
	log.Printf("[tts] Converting to OGG: %s", mp3Path)

	oggPath := mp3Path[:len(mp3Path)-4] + ".ogg"

	cmd := exec.Command("ffmpeg",
		"-i", mp3Path,
		"-acodec", "libvorbis",
		"-aq", "5",
		"-y", // Overwrite
		oggPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[tts] FFmpeg failed: %v, output: %s", err, string(output))
		return nil, fmt.Errorf("conversion failed: %w", err)
	}

	// Read OGG file
	oggBytes, err := os.ReadFile(oggPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read OGG: %w", err)
	}

	// Cleanup temp files
	os.Remove(mp3Path)
	os.Remove(oggPath)

	log.Printf("[tts] Conversion complete: %d bytes", len(oggBytes))
	return oggBytes, nil
}

// SynthesizeToOGG combines synthesis and conversion
func SynthesizeToOGG(cfg TTSConfig, text string) ([]byte, error) {
	mp3Path, err := Synthesize(cfg, text)
	if err != nil {
		return nil, err
	}

	return ConvertToOGG(mp3Path)
}
