package middleware

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"jarvis-gateway/internal/config"
	"jarvis-gateway/internal/db"
)

// BasicAuth middleware for protecting web pages with login/password
func BasicAuth(cfg *config.Config, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Credentials must be configured
		if cfg.BasicAuth.Username == "" || cfg.BasicAuth.Password == "" {
			http.Error(w, "BasicAuth not configured", http.StatusInternalServerError)
			return
		}

		user, pass, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="JARVIS Gateway"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		userMatch := subtle.ConstantTimeCompare([]byte(user), []byte(cfg.BasicAuth.Username)) == 1
		passMatch := subtle.ConstantTimeCompare([]byte(pass), []byte(cfg.BasicAuth.Password)) == 1

		if !userMatch || !passMatch {
			w.Header().Set("WWW-Authenticate", `Basic realm="JARVIS Gateway"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

// Auth middleware for webhook token authentication
func Auth(cfg *config.Config, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract source from path: /api/calendar -> calendar
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 3 {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}
		source := parts[len(parts)-1]

		// Get expected token for this source
		expectedToken, exists := cfg.Tokens[source]
		if !exists || expectedToken == "" {
			http.Error(w, "Token not configured for source: "+source, http.StatusInternalServerError)
			return
		}

		// Check Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			// Also check X-Webhook-Token for compatibility
			authHeader = r.Header.Get("X-Webhook-Token")
		}

		// Remove "Bearer " prefix if present
		token := strings.TrimPrefix(authHeader, "Bearer ")

		if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

// MobileAuth middleware for mobile app token authentication
func MobileAuth(dbClient *db.Client, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get token from Authorization header
		authHeader := r.Header.Get("Authorization")
		token := strings.TrimPrefix(authHeader, "Bearer ")

		if token == "" || !strings.HasPrefix(token, "mob_") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Lookup session in database
		session, err := dbClient.GetMobileSession(token)
		if err != nil || session == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Check if token is expired
		if time.Now().After(session.ExpiresAt) {
			http.Error(w, "Token expired", http.StatusUnauthorized)
			return
		}

		// Update last activity (async, don't block request)
		go dbClient.UpdateSessionActivity(token)

		// Add telegram_id to request context
		ctx := context.WithValue(r.Context(), "telegram_id", session.TelegramID)
		next(w, r.WithContext(ctx))
	}
}
