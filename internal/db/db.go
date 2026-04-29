package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"duq-gateway/internal/config"

	_ "github.com/lib/pq"
)

// Client wraps PostgreSQL connection
type Client struct {
	db *sql.DB
}

// Config for database connection
type Config struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

// New creates a new DB client
func New(cfg Config) (*Client, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping db: %w", err)
	}

	log.Printf("[db] Connected to PostgreSQL: %s:%d/%s", cfg.Host, cfg.Port, cfg.Name)
	return &Client{db: db}, nil
}

// Close closes the database connection
func (c *Client) Close() error {
	return c.db.Close()
}

// DB returns the underlying sql.DB
func (c *Client) DB() *sql.DB {
	return c.db
}

// UserPreferences holds user-specific settings
type UserPreferences struct {
	Timezone          string
	PreferredLanguage string
}

// GetUserPreferencesByTelegramID returns user preferences by telegram_id
// Returns default values from config if user not found
func (c *Client) GetUserPreferencesByTelegramID(telegramID int64) *UserPreferences {
	var timezone, preferredLanguage string
	query := `SELECT COALESCE(timezone, $2), COALESCE(preferred_language, $3)
	          FROM users WHERE telegram_id = $1`
	err := c.db.QueryRow(query, telegramID, config.DefaultTimezone, config.DefaultPreferredLanguage).Scan(&timezone, &preferredLanguage)
	if err != nil {
		// User not found or error - return defaults
		return &UserPreferences{
			Timezone:          config.DefaultTimezone,
			PreferredLanguage: config.DefaultPreferredLanguage,
		}
	}
	return &UserPreferences{
		Timezone:          timezone,
		PreferredLanguage: preferredLanguage,
	}
}

// User represents a user record
type User struct {
	ID                int64
	KeycloakSub       string // Keycloak subject ID - PRIMARY source of truth
	TelegramID        *int64 // nullable - user may register via email/Keycloak only
	Username          string
	FirstName         string
	LastName          string
	Role              string
	IsActive          bool
	Timezone          string
	PreferredLanguage string
}

// CheckUserExistsByTelegramID checks if a user with the given telegram_id exists
func (c *Client) CheckUserExistsByTelegramID(telegramID int64) bool {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE telegram_id = $1)`
	err := c.db.QueryRow(query, telegramID).Scan(&exists)
	if err != nil {
		log.Printf("[db] Error checking user existence: %v", err)
		return false
	}
	return exists
}

// GetUserByTelegramID returns full user info by telegram_id
func (c *Client) GetUserByTelegramID(telegramID int64) (*User, error) {
	query := `SELECT id, COALESCE(keycloak_sub, ''), telegram_id, COALESCE(username, ''),
	          COALESCE(first_name, ''), COALESCE(last_name, ''), role, is_active,
	          COALESCE(timezone, 'UTC'), COALESCE(preferred_language, 'ru')
	          FROM users WHERE telegram_id = $1`

	var user User
	err := c.db.QueryRow(query, telegramID).Scan(
		&user.ID, &user.KeycloakSub, &user.TelegramID, &user.Username,
		&user.FirstName, &user.LastName, &user.Role, &user.IsActive,
		&user.Timezone, &user.PreferredLanguage,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

// GetUserByID returns full user info by id
func (c *Client) GetUserByID(userID int64) (*User, error) {
	query := `SELECT id, COALESCE(keycloak_sub, ''), telegram_id, COALESCE(username, ''),
	          COALESCE(first_name, ''), COALESCE(last_name, ''), role, is_active,
	          COALESCE(timezone, 'UTC'), COALESCE(preferred_language, 'ru')
	          FROM users WHERE id = $1`

	var user User
	err := c.db.QueryRow(query, userID).Scan(
		&user.ID, &user.KeycloakSub, &user.TelegramID, &user.Username,
		&user.FirstName, &user.LastName, &user.Role, &user.IsActive,
		&user.Timezone, &user.PreferredLanguage,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

// GetUserByKeycloakSub returns full user info by keycloak_sub UUID
func (c *Client) GetUserByKeycloakSub(keycloakSub string) (*User, error) {
	query := `SELECT id, COALESCE(keycloak_sub, ''), telegram_id, COALESCE(username, ''),
	          COALESCE(first_name, ''), COALESCE(last_name, ''), role, is_active,
	          COALESCE(timezone, 'UTC'), COALESCE(preferred_language, 'ru')
	          FROM users WHERE keycloak_sub = $1`

	var user User
	err := c.db.QueryRow(query, keycloakSub).Scan(
		&user.ID, &user.KeycloakSub, &user.TelegramID, &user.Username,
		&user.FirstName, &user.LastName, &user.Role, &user.IsActive,
		&user.Timezone, &user.PreferredLanguage,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by keycloak_sub: %w", err)
	}
	return &user, nil
}

// CreateUserFromTelegram creates a new user from Telegram registration
// keycloakSub is REQUIRED - Keycloak is the primary source of truth
// Returns the created user or error
func (c *Client) CreateUserFromTelegram(telegramID int64, username, firstName, lastName, keycloakSub string) (*User, error) {
	if keycloakSub == "" {
		return nil, fmt.Errorf("keycloak_sub is required")
	}

	query := `INSERT INTO users (keycloak_sub, telegram_id, username, first_name, last_name, role, is_active, timezone, preferred_language, created_at, updated_at)
	          VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), 'user', true, 'UTC', 'ru', NOW(), NOW())
	          ON CONFLICT (telegram_id) DO UPDATE SET
	            keycloak_sub = COALESCE(NULLIF(EXCLUDED.keycloak_sub, ''), users.keycloak_sub),
	            username = COALESCE(NULLIF(EXCLUDED.username, ''), users.username),
	            first_name = COALESCE(NULLIF(EXCLUDED.first_name, ''), users.first_name),
	            last_name = COALESCE(NULLIF(EXCLUDED.last_name, ''), users.last_name),
	            updated_at = NOW()
	          RETURNING id, keycloak_sub, telegram_id, COALESCE(username, ''), COALESCE(first_name, ''),
	                    COALESCE(last_name, ''), role, is_active, timezone, preferred_language`

	var user User
	err := c.db.QueryRow(query, keycloakSub, telegramID, username, firstName, lastName).Scan(
		&user.ID, &user.KeycloakSub, &user.TelegramID, &user.Username,
		&user.FirstName, &user.LastName, &user.Role, &user.IsActive,
		&user.Timezone, &user.PreferredLanguage,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	log.Printf("[db] Created/updated user from Telegram: id=%d, keycloak_sub=%s, telegram_id=%v, username=%s",
		user.ID, user.KeycloakSub, user.TelegramID, user.Username)
	return &user, nil
}

// ===== EMAIL REGISTRATION METHODS =====

// UserWithEmail extends User with email field
type UserWithEmail struct {
	ID                int64
	KeycloakSub       string // Keycloak subject ID - PRIMARY source of truth
	Email             string
	TelegramID        *int64
	FirstName         string
	LastName          string
	Role              string
	IsActive          bool
	Timezone          string
	PreferredLanguage string
}

// CheckUserExistsByEmail checks if a user with the given email exists
func (c *Client) CheckUserExistsByEmail(email string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`
	err := c.db.QueryRow(query, email).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check email existence: %w", err)
	}
	return exists, nil
}

// GetUserByEmail returns user by email
func (c *Client) GetUserByEmail(email string) (*UserWithEmail, error) {
	query := `SELECT id, COALESCE(keycloak_sub, ''), COALESCE(email, ''), telegram_id, COALESCE(first_name, ''),
	          COALESCE(last_name, ''), role, is_active,
	          COALESCE(timezone, 'UTC'), COALESCE(preferred_language, 'ru')
	          FROM users WHERE email = $1`

	var user UserWithEmail
	err := c.db.QueryRow(query, email).Scan(
		&user.ID, &user.KeycloakSub, &user.Email, &user.TelegramID, &user.FirstName,
		&user.LastName, &user.Role, &user.IsActive,
		&user.Timezone, &user.PreferredLanguage,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return &user, nil
}

// CreateUserWithEmail creates a new user with email registration
// keycloakSub is REQUIRED - Keycloak is the primary source of truth
// is_active=false until email is verified
func (c *Client) CreateUserWithEmail(email, passwordHash string, telegramID *int64, firstName, lastName, keycloakSub string) (*UserWithEmail, error) {
	if keycloakSub == "" {
		return nil, fmt.Errorf("keycloak_sub is required")
	}

	query := `INSERT INTO users (keycloak_sub, email, password_hash, telegram_id, first_name, last_name, role, is_active, timezone, preferred_language, created_at, updated_at)
	          VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), 'user', false, 'UTC', 'ru', NOW(), NOW())
	          ON CONFLICT (email) DO UPDATE SET
	            keycloak_sub = COALESCE(NULLIF(EXCLUDED.keycloak_sub, ''), users.keycloak_sub),
	            updated_at = NOW()
	          RETURNING id, keycloak_sub, email, telegram_id, COALESCE(first_name, ''), COALESCE(last_name, ''),
	                    role, is_active, timezone, preferred_language`

	var user UserWithEmail
	err := c.db.QueryRow(query, keycloakSub, email, passwordHash, telegramID, firstName, lastName).Scan(
		&user.ID, &user.KeycloakSub, &user.Email, &user.TelegramID, &user.FirstName,
		&user.LastName, &user.Role, &user.IsActive,
		&user.Timezone, &user.PreferredLanguage,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	log.Printf("[db] Created/updated user with email: id=%d, keycloak_sub=%s, email=%s", user.ID, user.KeycloakSub, user.Email)
	return &user, nil
}

// ActivateUser sets is_active=true for a user
func (c *Client) ActivateUser(userID int64) error {
	query := `UPDATE users SET is_active = true, updated_at = NOW() WHERE id = $1`
	result, err := c.db.Exec(query, userID)
	if err != nil {
		return fmt.Errorf("failed to activate user: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	log.Printf("[db] Activated user: id=%d", userID)
	return nil
}

// ===== EMAIL VERIFICATION TOKEN METHODS =====

// CreateEmailVerificationToken creates a verification token for a user
// Note: email_verification_tokens table must exist (see migration 007)
func (c *Client) CreateEmailVerificationToken(userID int64, token string, ttl time.Duration) error {
	expiresAt := time.Now().Add(ttl)
	query := `INSERT INTO email_verification_tokens (user_id, token, expires_at, created_at)
	          VALUES ($1, $2, $3, NOW())
	          ON CONFLICT (user_id) DO UPDATE SET token = $2, expires_at = $3, created_at = NOW()`

	_, err := c.db.Exec(query, userID, token, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to create verification token: %w", err)
	}

	log.Printf("[db] Created verification token for user %d, expires at %s", userID, expiresAt.Format(time.RFC3339))
	return nil
}

// ValidateEmailVerificationToken validates a token and returns the user ID
func (c *Client) ValidateEmailVerificationToken(token string) (int64, error) {
	var userID int64
	var expiresAt time.Time

	query := `SELECT user_id, expires_at FROM email_verification_tokens WHERE token = $1`
	err := c.db.QueryRow(query, token).Scan(&userID, &expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("token not found")
		}
		return 0, fmt.Errorf("failed to validate token: %w", err)
	}

	if time.Now().After(expiresAt) {
		return 0, fmt.Errorf("token expired")
	}

	return userID, nil
}

// DeleteEmailVerificationToken deletes a token after use
func (c *Client) DeleteEmailVerificationToken(token string) error {
	query := `DELETE FROM email_verification_tokens WHERE token = $1`
	_, err := c.db.Exec(query, token)
	if err != nil {
		return fmt.Errorf("failed to delete token: %w", err)
	}
	return nil
}

// DeleteEmailVerificationTokenByUserID deletes all tokens for a user
func (c *Client) DeleteEmailVerificationTokenByUserID(userID int64) error {
	query := `DELETE FROM email_verification_tokens WHERE user_id = $1`
	_, err := c.db.Exec(query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete tokens: %w", err)
	}
	return nil
}

// UpdateUserKeycloakSub updates the keycloak_sub field for a user
func (c *Client) UpdateUserKeycloakSub(userID int64, keycloakSub string) error {
	query := `UPDATE users SET keycloak_sub = $1, updated_at = NOW() WHERE id = $2`
	result, err := c.db.Exec(query, keycloakSub, userID)
	if err != nil {
		return fmt.Errorf("failed to update keycloak_sub: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	log.Printf("[db] Updated keycloak_sub for user %d: %s", userID, keycloakSub)
	return nil
}

