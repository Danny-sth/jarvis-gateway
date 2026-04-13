package db

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// MobileSession represents a mobile app session
type MobileSession struct {
	ID         int64
	TelegramID int64
	Token      string
	DeviceName string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	LastUsedAt time.Time
}

// QRAuthCode represents a temporary QR auth code
type QRAuthCode struct {
	ID         int64
	Code       string
	TelegramID int64
	DeviceName string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	Used       bool
}

// GenerateQRCode creates a new QR auth code
func (c *Client) GenerateQRCode(telegramID int64, deviceName string, ttl time.Duration) (*QRAuthCode, error) {
	code := generateShortCode(6)
	expiresAt := time.Now().Add(ttl)

	query := `
		INSERT INTO qr_auth_codes (code, telegram_id, device_name, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`

	var qr QRAuthCode
	err := c.db.QueryRow(query, code, telegramID, deviceName, expiresAt).Scan(&qr.ID, &qr.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create qr code: %w", err)
	}

	qr.Code = code
	qr.TelegramID = telegramID
	qr.DeviceName = deviceName
	qr.ExpiresAt = expiresAt
	qr.Used = false

	return &qr, nil
}

// GetTelegramIDFromQRCode returns the telegram_id for a QR code without consuming it
// Used to check user existence before verification
func (c *Client) GetTelegramIDFromQRCode(code string) (int64, error) {
	query := `
		SELECT telegram_id, expires_at, used
		FROM qr_auth_codes
		WHERE code = $1
	`

	var telegramID int64
	var expiresAt time.Time
	var used bool

	err := c.db.QueryRow(query, strings.ToUpper(code)).Scan(&telegramID, &expiresAt, &used)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("invalid code")
	}
	if err != nil {
		return 0, fmt.Errorf("db error: %w", err)
	}

	if used {
		return 0, fmt.Errorf("code already used")
	}

	if time.Now().After(expiresAt) {
		return 0, fmt.Errorf("code expired")
	}

	return telegramID, nil
}

// VerifyQRCode verifies and consumes a QR code, creating a mobile session
func (c *Client) VerifyQRCode(code string, sessionTTL time.Duration) (*MobileSession, error) {
	// Find and validate QR code
	query := `
		SELECT id, telegram_id, device_name, expires_at, used
		FROM qr_auth_codes
		WHERE code = $1
	`

	var qr QRAuthCode
	err := c.db.QueryRow(query, strings.ToUpper(code)).Scan(
		&qr.ID, &qr.TelegramID, &qr.DeviceName, &qr.ExpiresAt, &qr.Used,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invalid code")
	}
	if err != nil {
		return nil, fmt.Errorf("db error: %w", err)
	}

	if qr.Used {
		return nil, fmt.Errorf("code already used")
	}

	if time.Now().After(qr.ExpiresAt) {
		return nil, fmt.Errorf("code expired")
	}

	// Mark QR code as used
	_, err = c.db.Exec("UPDATE qr_auth_codes SET used = TRUE WHERE id = $1", qr.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to mark code as used: %w", err)
	}

	// Create mobile session
	// Token is returned to client but stored as SHA256 hash in DB for security
	token := "mob_" + generateToken(32)
	tokenHash := hashToken(token)
	expiresAt := time.Now().Add(sessionTTL)

	insertQuery := `
		INSERT INTO mobile_sessions (telegram_id, token_hash, device_name, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, last_used_at
	`

	var session MobileSession
	err = c.db.QueryRow(insertQuery, qr.TelegramID, tokenHash, qr.DeviceName, expiresAt).Scan(
		&session.ID, &session.CreatedAt, &session.LastUsedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	session.TelegramID = qr.TelegramID
	session.Token = token
	session.DeviceName = qr.DeviceName
	session.ExpiresAt = expiresAt

	return &session, nil
}

// GetMobileSession retrieves a session by token (token is hashed for lookup)
func (c *Client) GetMobileSession(token string) (*MobileSession, error) {
	// Hash the input token to match against stored hash
	tokenHash := hashToken(token)

	query := `
		SELECT id, telegram_id, device_name, created_at, expires_at, last_used_at
		FROM mobile_sessions
		WHERE token_hash = $1
	`

	var session MobileSession
	var deviceName sql.NullString

	err := c.db.QueryRow(query, tokenHash).Scan(
		&session.ID, &session.TelegramID,
		&deviceName, &session.CreatedAt, &session.ExpiresAt, &session.LastUsedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("db error: %w", err)
	}

	if deviceName.Valid {
		session.DeviceName = deviceName.String
	}

	// Store the original token for the caller (not stored in DB)
	session.Token = token

	return &session, nil
}

// UpdateSessionActivity updates the last_used_at timestamp
func (c *Client) UpdateSessionActivity(token string) error {
	tokenHash := hashToken(token)
	_, err := c.db.Exec(
		"UPDATE mobile_sessions SET last_used_at = NOW() WHERE token_hash = $1",
		tokenHash,
	)
	return err
}

// DeleteExpiredSessions removes expired sessions and QR codes
func (c *Client) DeleteExpiredSessions() (int64, error) {
	result, err := c.db.Exec("DELETE FROM mobile_sessions WHERE expires_at < NOW()")
	if err != nil {
		return 0, err
	}
	count, _ := result.RowsAffected()

	// Also clean up old QR codes
	c.db.Exec("DELETE FROM qr_auth_codes WHERE expires_at < NOW()")

	return count, nil
}

// generateShortCode generates a short alphanumeric code
func generateShortCode(length int) string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // Excluded confusing chars: I, O, 0, 1
	b := make([]byte, length)
	rand.Read(b)
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}

// generateToken generates a hex token
func generateToken(length int) string {
	b := make([]byte, length/2)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// hashToken creates a SHA256 hash of the token for secure storage
// The plaintext token is returned to the client but never stored
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
