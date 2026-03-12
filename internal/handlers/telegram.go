package handlers

import (
	"bytes"
	"io"
	"log"
	"net/http"

	"jarvis-gateway/internal/config"
)

// OpenClaw's internal webhook endpoint
const openclawWebhookURL = "http://127.0.0.1:8787/telegram-webhook"

// Telegram creates a handler for Telegram webhook that proxies to OpenClaw
func Telegram(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Read the raw body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("[telegram] Failed to read body: %v", err)
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}
		r.Body.Close()

		log.Printf("[telegram] Proxying webhook to OpenClaw (%d bytes)", len(body))

		// Forward to OpenClaw's internal webhook endpoint
		req, err := http.NewRequest("POST", openclawWebhookURL, bytes.NewReader(body))
		if err != nil {
			log.Printf("[telegram] Failed to create proxy request: %v", err)
			http.Error(w, "Proxy error", http.StatusInternalServerError)
			return
		}

		// Copy headers
		req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
		if secret := r.Header.Get("X-Telegram-Bot-Api-Secret-Token"); secret != "" {
			req.Header.Set("X-Telegram-Bot-Api-Secret-Token", secret)
		}

		// Send to OpenClaw
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[telegram] Failed to proxy to OpenClaw: %v", err)
			// Return 200 to Telegram to avoid retries
			w.WriteHeader(http.StatusOK)
			return
		}
		defer resp.Body.Close()

		// Read OpenClaw response
		respBody, _ := io.ReadAll(resp.Body)

		log.Printf("[telegram] OpenClaw responded: %d (%d bytes)", resp.StatusCode, len(respBody))

		// Forward response back
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)
	}
}
