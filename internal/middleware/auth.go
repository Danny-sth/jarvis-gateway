package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"jarvis-gateway/internal/config"
)

// BasicAuth middleware for protecting web pages with login/password
func BasicAuth(cfg *config.Config, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// If no credentials configured, skip auth
		if cfg.BasicAuth.Username == "" || cfg.BasicAuth.Password == "" {
			next(w, r)
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
			// No token configured = no auth required (for development)
			next(w, r)
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
