package oauth

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"duq-gateway/internal/credentials"
)

// GoogleOAuthConfig holds Google OAuth configuration
type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
}

// RefreshGoogleTokenIfNeeded checks if token is expired and refreshes it
func RefreshGoogleTokenIfNeeded(cfg GoogleOAuthConfig, credService CredentialService, creds *credentials.UserCredentials) error {
	if creds == nil || !creds.IsExpired() {
		return nil
	}

	if creds.RefreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	log.Printf("[oauth] Token expired for user %d, refreshing...", creds.UserID)

	data := url.Values{
		"client_id":     {cfg.ClientID},
		"client_secret": {cfg.ClientSecret},
		"refresh_token": {creds.RefreshToken},
		"grant_type":    {"refresh_token"},
	}

	resp, err := http.Post(
		"https://oauth2.googleapis.com/token",
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("refresh failed: %s", string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("failed to parse token response: %w", err)
	}

	// Update credentials
	creds.AccessToken = tokenResp.AccessToken
	creds.TokenType = tokenResp.TokenType
	if tokenResp.ExpiresIn > 0 {
		creds.ExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	}

	// Save updated credentials
	if err := credService.SaveCredentials(creds); err != nil {
		return fmt.Errorf("failed to save refreshed credentials: %w", err)
	}

	log.Printf("[oauth] Token refreshed successfully for user %d", creds.UserID)
	return nil
}

// CredentialService interface for credential storage
type CredentialService interface {
	SaveCredentials(creds *credentials.UserCredentials) error
	GetCredentials(telegramID int64, provider string) (*credentials.UserCredentials, error)
}
