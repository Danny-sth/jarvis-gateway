package keycloak

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"duq-gateway/internal/config"
)

// AdminService handles Keycloak admin operations
type AdminService struct {
	cfg         *config.Config
	httpClient  *http.Client
	token       string
	tokenExpiry time.Time
	mu          sync.RWMutex
}

// NewAdminService creates a new Keycloak admin service
func NewAdminService(cfg *config.Config) *AdminService {
	return &AdminService{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// IsEnabled returns true if Keycloak admin is configured
func (s *AdminService) IsEnabled() bool {
	return s.cfg.Keycloak.Enabled &&
		s.cfg.Keycloak.AdminClientID != "" &&
		s.cfg.Keycloak.AdminClientSecret != ""
}

// getAdminToken gets or refreshes the admin access token
func (s *AdminService) getAdminToken() (string, error) {
	s.mu.RLock()
	if s.token != "" && time.Now().Before(s.tokenExpiry) {
		token := s.token
		s.mu.RUnlock()
		return token, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double check after acquiring write lock
	if s.token != "" && time.Now().Before(s.tokenExpiry) {
		return s.token, nil
	}

	// Get token using client_credentials flow
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token",
		s.cfg.KeycloakInternalURL, s.cfg.Keycloak.Realm)

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", s.cfg.Keycloak.AdminClientID)
	data.Set("client_secret", s.cfg.Keycloak.AdminClientSecret)

	resp, err := s.httpClient.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to get admin token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to get admin token: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	s.token = tokenResp.AccessToken
	// Set expiry with some buffer (30 seconds before actual expiry)
	s.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-30) * time.Second)

	log.Printf("[keycloak-admin] Admin token obtained, expires in %d seconds", tokenResp.ExpiresIn)
	return s.token, nil
}

// KeycloakUser represents a user in Keycloak
type KeycloakUser struct {
	ID            string              `json:"id,omitempty"`
	Username      string              `json:"username"`
	Email         string              `json:"email,omitempty"`
	FirstName     string              `json:"firstName,omitempty"`
	LastName      string              `json:"lastName,omitempty"`
	Enabled       bool                `json:"enabled"`
	EmailVerified bool                `json:"emailVerified"`
	Attributes    map[string][]string `json:"attributes,omitempty"`
	Credentials   []KeycloakCredential `json:"credentials,omitempty"`
}

// KeycloakCredential represents user credentials in Keycloak
type KeycloakCredential struct {
	Type      string `json:"type"`
	Value     string `json:"value"`
	Temporary bool   `json:"temporary"`
}

// CreateUserFromTelegram creates a user in Keycloak from Telegram registration
func (s *AdminService) CreateUserFromTelegram(telegramID int64, username, firstName, lastName string) (*KeycloakUser, error) {
	if !s.IsEnabled() {
		log.Printf("[keycloak-admin] Admin service not enabled, skipping Keycloak user creation")
		return nil, nil
	}

	token, err := s.getAdminToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get admin token: %w", err)
	}

	// Generate username for Keycloak
	kcUsername := username
	if kcUsername == "" {
		kcUsername = fmt.Sprintf("tg_%d", telegramID)
	}

	// Generate a random password (user will use Telegram QR auth, not password)
	randomPassword := fmt.Sprintf("tg_%d_%d", telegramID, time.Now().UnixNano())

	user := &KeycloakUser{
		Username:      kcUsername,
		FirstName:     firstName,
		LastName:      lastName,
		Enabled:       true,
		EmailVerified: true, // Telegram users don't need email verification
		Attributes: map[string][]string{
			"telegram_id": {fmt.Sprintf("%d", telegramID)},
		},
		Credentials: []KeycloakCredential{
			{
				Type:      "password",
				Value:     randomPassword,
				Temporary: false,
			},
		},
	}

	// Create user in Keycloak
	usersURL := fmt.Sprintf("%s/admin/realms/%s/users",
		s.cfg.KeycloakInternalURL, s.cfg.Keycloak.Realm)

	body, err := json.Marshal(user)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, usersURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	defer resp.Body.Close()

	// 201 Created or 409 Conflict (user already exists)
	if resp.StatusCode == http.StatusCreated {
		// Get user ID from Location header
		location := resp.Header.Get("Location")
		if location != "" {
			parts := strings.Split(location, "/")
			if len(parts) > 0 {
				user.ID = parts[len(parts)-1]
			}
		}
		log.Printf("[keycloak-admin] Created Keycloak user: username=%s, telegram_id=%d, id=%s",
			kcUsername, telegramID, user.ID)
		return user, nil
	}

	if resp.StatusCode == http.StatusConflict {
		// User already exists, try to update telegram_id attribute
		log.Printf("[keycloak-admin] User already exists in Keycloak: username=%s", kcUsername)

		// Find user by username
		existingUser, err := s.FindUserByUsername(kcUsername)
		if err != nil {
			return nil, fmt.Errorf("failed to find existing user: %w", err)
		}
		if existingUser != nil {
			// Update telegram_id attribute if not set
			if existingUser.Attributes == nil {
				existingUser.Attributes = make(map[string][]string)
			}
			existingUser.Attributes["telegram_id"] = []string{fmt.Sprintf("%d", telegramID)}

			err = s.UpdateUser(existingUser)
			if err != nil {
				log.Printf("[keycloak-admin] Failed to update telegram_id: %v", err)
			} else {
				log.Printf("[keycloak-admin] Updated telegram_id for user: username=%s, telegram_id=%d",
					kcUsername, telegramID)
			}
			return existingUser, nil
		}
		return nil, nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	return nil, fmt.Errorf("failed to create user: status=%d, body=%s", resp.StatusCode, string(respBody))
}

// FindUserByUsername finds a user by username
func (s *AdminService) FindUserByUsername(username string) (*KeycloakUser, error) {
	token, err := s.getAdminToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get admin token: %w", err)
	}

	usersURL := fmt.Sprintf("%s/admin/realms/%s/users?username=%s&exact=true",
		s.cfg.KeycloakInternalURL, s.cfg.Keycloak.Realm, url.QueryEscape(username))

	req, err := http.NewRequest(http.MethodGet, usersURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to find user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to find user: status=%d", resp.StatusCode)
	}

	var users []KeycloakUser
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, fmt.Errorf("failed to decode users: %w", err)
	}

	if len(users) == 0 {
		return nil, nil
	}

	return &users[0], nil
}

// UpdateUser updates a user in Keycloak
func (s *AdminService) UpdateUser(user *KeycloakUser) error {
	if user.ID == "" {
		return fmt.Errorf("user ID is required for update")
	}

	token, err := s.getAdminToken()
	if err != nil {
		return fmt.Errorf("failed to get admin token: %w", err)
	}

	userURL := fmt.Sprintf("%s/admin/realms/%s/users/%s",
		s.cfg.KeycloakInternalURL, s.cfg.Keycloak.Realm, user.ID)

	body, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, userURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update user: status=%d, body=%s", resp.StatusCode, string(respBody))
	}

	return nil
}
