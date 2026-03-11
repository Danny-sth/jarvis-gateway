package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"jarvis-gateway/internal/config"
	"jarvis-gateway/internal/db"
)

// QRGenerateRequest represents a QR code generation request
type QRGenerateRequest struct {
	TelegramID int64  `json:"telegram_id"`
	DeviceName string `json:"device_name,omitempty"`
}

// QRGenerateResponse represents a QR code generation response
type QRGenerateResponse struct {
	Code      string `json:"code"`
	QRData    string `json:"qr_data"`
	ExpiresIn int    `json:"expires_in"`
}

// QRVerifyRequest represents a QR code verification request
type QRVerifyRequest struct {
	Code string `json:"code"`
}

// QRVerifyResponse represents a QR code verification response
type QRVerifyResponse struct {
	Token      string    `json:"token"`
	TelegramID int64     `json:"telegram_id"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// QRGenerate handles QR code generation for mobile auth
func QRGenerate(cfg *config.Config, dbClient *db.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req QRGenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		if req.TelegramID == 0 {
			http.Error(w, "telegram_id is required", http.StatusBadRequest)
			return
		}

		// Generate QR code with 5 minute TTL
		qrTTL := 5 * time.Minute
		qr, err := dbClient.GenerateQRCode(req.TelegramID, req.DeviceName, qrTTL)
		if err != nil {
			log.Printf("[auth] Failed to generate QR code: %v", err)
			http.Error(w, "Failed to generate code", http.StatusInternalServerError)
			return
		}

		log.Printf("[auth] QR code generated for telegram:%d, code=%s", req.TelegramID, qr.Code)

		response := QRGenerateResponse{
			Code:      qr.Code,
			QRData:    "jarvis://auth?code=" + qr.Code,
			ExpiresIn: int(qrTTL.Seconds()),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// QRVerify handles QR code verification from mobile app
func QRVerify(cfg *config.Config, dbClient *db.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req QRVerifyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		if req.Code == "" {
			http.Error(w, "code is required", http.StatusBadRequest)
			return
		}

		// Session TTL from config or default 30 days
		sessionTTL := time.Duration(cfg.Voice.SessionTTLDays) * 24 * time.Hour
		if sessionTTL == 0 {
			sessionTTL = 30 * 24 * time.Hour
		}

		session, err := dbClient.VerifyQRCode(req.Code, sessionTTL)
		if err != nil {
			log.Printf("[auth] QR verification failed: %v", err)
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		log.Printf("[auth] Mobile session created for telegram:%d, token=%s...",
			session.TelegramID, session.Token[:20])

		response := QRVerifyResponse{
			Token:      session.Token,
			TelegramID: session.TelegramID,
			ExpiresAt:  session.ExpiresAt,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}
