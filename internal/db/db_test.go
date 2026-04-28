package db

import (
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// Helper function to create a mock Client
func newMockClient(t *testing.T) (*Client, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	return &Client{db: db}, mock
}

func TestClient_Close(t *testing.T) {
	client, mock := newMockClient(t)
	mock.ExpectClose()

	err := client.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestClient_DB(t *testing.T) {
	client, _ := newMockClient(t)
	if client.DB() == nil {
		t.Error("DB() returned nil")
	}
}

func TestGetUserPreferencesByTelegramID_Success(t *testing.T) {
	client, mock := newMockClient(t)
	defer client.db.Close()

	telegramID := int64(123456)
	rows := sqlmock.NewRows([]string{"timezone", "preferred_language"}).
		AddRow("Europe/Moscow", "en")

	// Query now uses parameterized defaults ($2, $3)
	mock.ExpectQuery(`SELECT COALESCE\(timezone, \$2\), COALESCE\(preferred_language, \$3\)`).
		WithArgs(telegramID, "UTC", "ru").
		WillReturnRows(rows)

	prefs := client.GetUserPreferencesByTelegramID(telegramID)

	if prefs.Timezone != "Europe/Moscow" {
		t.Errorf("expected timezone Europe/Moscow, got %s", prefs.Timezone)
	}
	if prefs.PreferredLanguage != "en" {
		t.Errorf("expected language en, got %s", prefs.PreferredLanguage)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetUserPreferencesByTelegramID_NotFound(t *testing.T) {
	client, mock := newMockClient(t)
	defer client.db.Close()

	telegramID := int64(999999)

	// Query now uses parameterized defaults ($2, $3)
	mock.ExpectQuery(`SELECT COALESCE\(timezone, \$2\), COALESCE\(preferred_language, \$3\)`).
		WithArgs(telegramID, "UTC", "ru").
		WillReturnError(sql.ErrNoRows)

	prefs := client.GetUserPreferencesByTelegramID(telegramID)

	// Should return defaults
	if prefs.Timezone != "UTC" {
		t.Errorf("expected default timezone UTC, got %s", prefs.Timezone)
	}
	if prefs.PreferredLanguage != "ru" {
		t.Errorf("expected default language ru, got %s", prefs.PreferredLanguage)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestCheckUserExistsByTelegramID_Exists(t *testing.T) {
	client, mock := newMockClient(t)
	defer client.db.Close()

	telegramID := int64(123456)
	rows := sqlmock.NewRows([]string{"exists"}).AddRow(true)

	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM users WHERE telegram_id = \$1\)`).
		WithArgs(telegramID).
		WillReturnRows(rows)

	exists := client.CheckUserExistsByTelegramID(telegramID)

	if !exists {
		t.Error("expected user to exist")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestCheckUserExistsByTelegramID_NotExists(t *testing.T) {
	client, mock := newMockClient(t)
	defer client.db.Close()

	telegramID := int64(999999)
	rows := sqlmock.NewRows([]string{"exists"}).AddRow(false)

	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM users WHERE telegram_id = \$1\)`).
		WithArgs(telegramID).
		WillReturnRows(rows)

	exists := client.CheckUserExistsByTelegramID(telegramID)

	if exists {
		t.Error("expected user to not exist")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestCheckUserExistsByTelegramID_Error(t *testing.T) {
	client, mock := newMockClient(t)
	defer client.db.Close()

	telegramID := int64(123456)

	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM users WHERE telegram_id = \$1\)`).
		WithArgs(telegramID).
		WillReturnError(sql.ErrConnDone)

	exists := client.CheckUserExistsByTelegramID(telegramID)

	if exists {
		t.Error("expected false on error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetUserByTelegramID_Success(t *testing.T) {
	client, mock := newMockClient(t)
	defer client.db.Close()

	telegramID := int64(123456)
	rows := sqlmock.NewRows([]string{
		"id", "telegram_id", "username", "first_name", "last_name",
		"role", "is_active", "timezone", "preferred_language",
	}).AddRow(1, telegramID, "testuser", "Test", "User", "user", true, "UTC", "ru")

	mock.ExpectQuery(`SELECT id, telegram_id, COALESCE\(username, ''\)`).
		WithArgs(telegramID).
		WillReturnRows(rows)

	user, err := client.GetUserByTelegramID(telegramID)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if user.ID != 1 {
		t.Errorf("expected ID 1, got %d", user.ID)
	}
	if user.Username != "testuser" {
		t.Errorf("expected username testuser, got %s", user.Username)
	}
	if user.Role != "user" {
		t.Errorf("expected role user, got %s", user.Role)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetUserByTelegramID_NotFound(t *testing.T) {
	client, mock := newMockClient(t)
	defer client.db.Close()

	telegramID := int64(999999)

	mock.ExpectQuery(`SELECT id, telegram_id, COALESCE\(username, ''\)`).
		WithArgs(telegramID).
		WillReturnError(sql.ErrNoRows)

	user, err := client.GetUserByTelegramID(telegramID)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if user != nil {
		t.Errorf("expected nil user, got %v", user)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestCreateUserFromTelegram_Success(t *testing.T) {
	client, mock := newMockClient(t)
	defer client.db.Close()

	telegramID := int64(123456)
	keycloakSub := "kc-uuid-12345"
	rows := sqlmock.NewRows([]string{
		"id", "keycloak_sub", "telegram_id", "username", "first_name", "last_name",
		"role", "is_active", "timezone", "preferred_language",
	}).AddRow(1, keycloakSub, telegramID, "newuser", "New", "User", "user", true, "UTC", "ru")

	mock.ExpectQuery(`INSERT INTO users`).
		WithArgs(keycloakSub, telegramID, "newuser", "New", "User").
		WillReturnRows(rows)

	user, err := client.CreateUserFromTelegram(telegramID, "newuser", "New", "User", keycloakSub)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if user.ID != 1 {
		t.Errorf("expected ID 1, got %d", user.ID)
	}
	if user.KeycloakSub != keycloakSub {
		t.Errorf("expected keycloak_sub %s, got %s", keycloakSub, user.KeycloakSub)
	}
	if user.Username != "newuser" {
		t.Errorf("expected username newuser, got %s", user.Username)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestCreateUserFromTelegram_MissingKeycloakSub(t *testing.T) {
	client, _ := newMockClient(t)
	defer client.db.Close()

	telegramID := int64(123456)

	_, err := client.CreateUserFromTelegram(telegramID, "newuser", "New", "User", "")

	if err == nil {
		t.Error("expected error for missing keycloak_sub")
	}
}

func TestCheckUserExistsByEmail_Exists(t *testing.T) {
	client, mock := newMockClient(t)
	defer client.db.Close()

	email := "test@example.com"
	rows := sqlmock.NewRows([]string{"exists"}).AddRow(true)

	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM users WHERE email = \$1\)`).
		WithArgs(email).
		WillReturnRows(rows)

	exists, err := client.CheckUserExistsByEmail(email)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected email to exist")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestCheckUserExistsByEmail_NotExists(t *testing.T) {
	client, mock := newMockClient(t)
	defer client.db.Close()

	email := "nonexistent@example.com"
	rows := sqlmock.NewRows([]string{"exists"}).AddRow(false)

	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM users WHERE email = \$1\)`).
		WithArgs(email).
		WillReturnRows(rows)

	exists, err := client.CheckUserExistsByEmail(email)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected email to not exist")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetUserByEmail_Success(t *testing.T) {
	client, mock := newMockClient(t)
	defer client.db.Close()

	email := "test@example.com"
	var telegramID *int64
	rows := sqlmock.NewRows([]string{
		"id", "email", "telegram_id", "first_name", "last_name",
		"role", "is_active", "timezone", "preferred_language",
	}).AddRow(1, email, telegramID, "Test", "User", "user", true, "UTC", "ru")

	mock.ExpectQuery(`SELECT id, COALESCE\(email, ''\)`).
		WithArgs(email).
		WillReturnRows(rows)

	user, err := client.GetUserByEmail(email)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if user.Email != email {
		t.Errorf("expected email %s, got %s", email, user.Email)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetUserByEmail_NotFound(t *testing.T) {
	client, mock := newMockClient(t)
	defer client.db.Close()

	email := "nonexistent@example.com"

	mock.ExpectQuery(`SELECT id, COALESCE\(email, ''\)`).
		WithArgs(email).
		WillReturnError(sql.ErrNoRows)

	user, err := client.GetUserByEmail(email)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if user != nil {
		t.Errorf("expected nil user, got %v", user)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestCreateUserWithEmail_Success(t *testing.T) {
	client, mock := newMockClient(t)
	defer client.db.Close()

	email := "new@example.com"
	passwordHash := "hashedpassword"
	var telegramID *int64

	rows := sqlmock.NewRows([]string{
		"id", "email", "telegram_id", "first_name", "last_name",
		"role", "is_active", "timezone", "preferred_language",
	}).AddRow(1, email, telegramID, "New", "User", "user", false, "UTC", "ru")

	mock.ExpectQuery(`INSERT INTO users`).
		WithArgs(email, passwordHash, telegramID, "New", "User").
		WillReturnRows(rows)

	user, err := client.CreateUserWithEmail(email, passwordHash, telegramID, "New", "User")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if user.Email != email {
		t.Errorf("expected email %s, got %s", email, user.Email)
	}
	if user.IsActive {
		t.Error("expected user to be inactive initially")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestActivateUser_Success(t *testing.T) {
	client, mock := newMockClient(t)
	defer client.db.Close()

	userID := int64(1)

	mock.ExpectExec(`UPDATE users SET is_active = true`).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := client.ActivateUser(userID)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestActivateUser_NotFound(t *testing.T) {
	client, mock := newMockClient(t)
	defer client.db.Close()

	userID := int64(999)

	mock.ExpectExec(`UPDATE users SET is_active = true`).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := client.ActivateUser(userID)

	if err == nil {
		t.Error("expected error for non-existent user")
	}
	if err.Error() != "user not found" {
		t.Errorf("expected 'user not found' error, got: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestCreateEmailVerificationToken_Success(t *testing.T) {
	client, mock := newMockClient(t)
	defer client.db.Close()

	userID := int64(1)
	token := "verification-token-123"
	ttl := 24 * time.Hour

	// Table is now created via migration, only expect the insert
	mock.ExpectExec(`INSERT INTO email_verification_tokens`).
		WithArgs(userID, token, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := client.CreateEmailVerificationToken(userID, token, ttl)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestValidateEmailVerificationToken_Valid(t *testing.T) {
	client, mock := newMockClient(t)
	defer client.db.Close()

	token := "valid-token"
	userID := int64(1)
	expiresAt := time.Now().Add(1 * time.Hour) // Not expired

	rows := sqlmock.NewRows([]string{"user_id", "expires_at"}).
		AddRow(userID, expiresAt)

	mock.ExpectQuery(`SELECT user_id, expires_at FROM email_verification_tokens`).
		WithArgs(token).
		WillReturnRows(rows)

	resultID, err := client.ValidateEmailVerificationToken(token)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if resultID != userID {
		t.Errorf("expected user ID %d, got %d", userID, resultID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestValidateEmailVerificationToken_Expired(t *testing.T) {
	client, mock := newMockClient(t)
	defer client.db.Close()

	token := "expired-token"
	userID := int64(1)
	expiresAt := time.Now().Add(-1 * time.Hour) // Expired

	rows := sqlmock.NewRows([]string{"user_id", "expires_at"}).
		AddRow(userID, expiresAt)

	mock.ExpectQuery(`SELECT user_id, expires_at FROM email_verification_tokens`).
		WithArgs(token).
		WillReturnRows(rows)

	_, err := client.ValidateEmailVerificationToken(token)

	if err == nil {
		t.Error("expected error for expired token")
	}
	if err.Error() != "token expired" {
		t.Errorf("expected 'token expired' error, got: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestValidateEmailVerificationToken_NotFound(t *testing.T) {
	client, mock := newMockClient(t)
	defer client.db.Close()

	token := "nonexistent-token"

	mock.ExpectQuery(`SELECT user_id, expires_at FROM email_verification_tokens`).
		WithArgs(token).
		WillReturnError(sql.ErrNoRows)

	_, err := client.ValidateEmailVerificationToken(token)

	if err == nil {
		t.Error("expected error for non-existent token")
	}
	if err.Error() != "token not found" {
		t.Errorf("expected 'token not found' error, got: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestDeleteEmailVerificationToken_Success(t *testing.T) {
	client, mock := newMockClient(t)
	defer client.db.Close()

	token := "token-to-delete"

	mock.ExpectExec(`DELETE FROM email_verification_tokens WHERE token`).
		WithArgs(token).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := client.DeleteEmailVerificationToken(token)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestDeleteEmailVerificationTokenByUserID_Success(t *testing.T) {
	client, mock := newMockClient(t)
	defer client.db.Close()

	userID := int64(1)

	mock.ExpectExec(`DELETE FROM email_verification_tokens WHERE user_id`).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := client.DeleteEmailVerificationTokenByUserID(userID)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

// Test Config struct
func TestConfig_Fields(t *testing.T) {
	cfg := Config{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "secret",
		Name:     "testdb",
	}

	if cfg.Host != "localhost" {
		t.Errorf("expected host localhost, got %s", cfg.Host)
	}
	if cfg.Port != 5432 {
		t.Errorf("expected port 5432, got %d", cfg.Port)
	}
	if cfg.User != "postgres" {
		t.Errorf("expected user postgres, got %s", cfg.User)
	}
	if cfg.Password != "secret" {
		t.Errorf("expected password secret, got %s", cfg.Password)
	}
	if cfg.Name != "testdb" {
		t.Errorf("expected name testdb, got %s", cfg.Name)
	}
}

// Test User struct
func TestUser_Fields(t *testing.T) {
	user := User{
		ID:                1,
		TelegramID:        123456,
		Username:          "testuser",
		FirstName:         "Test",
		LastName:          "User",
		Role:              "admin",
		IsActive:          true,
		Timezone:          "Europe/Moscow",
		PreferredLanguage: "en",
	}

	if user.ID != 1 {
		t.Errorf("expected ID 1, got %d", user.ID)
	}
	if user.TelegramID != 123456 {
		t.Errorf("expected TelegramID 123456, got %d", user.TelegramID)
	}
	if user.Role != "admin" {
		t.Errorf("expected role admin, got %s", user.Role)
	}
	if !user.IsActive {
		t.Error("expected user to be active")
	}
}

// Test UserPreferences struct
func TestUserPreferences_Fields(t *testing.T) {
	prefs := UserPreferences{
		Timezone:          "America/New_York",
		PreferredLanguage: "en",
	}

	if prefs.Timezone != "America/New_York" {
		t.Errorf("expected timezone America/New_York, got %s", prefs.Timezone)
	}
	if prefs.PreferredLanguage != "en" {
		t.Errorf("expected language en, got %s", prefs.PreferredLanguage)
	}
}

// Test UserWithEmail struct
func TestUserWithEmail_Fields(t *testing.T) {
	telegramID := int64(123456)
	user := UserWithEmail{
		ID:                1,
		Email:             "test@example.com",
		TelegramID:        &telegramID,
		FirstName:         "Test",
		LastName:          "User",
		Role:              "user",
		IsActive:          true,
		Timezone:          "UTC",
		PreferredLanguage: "ru",
	}

	if user.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %s", user.Email)
	}
	if user.TelegramID == nil || *user.TelegramID != 123456 {
		t.Error("expected telegram ID 123456")
	}
}

func TestUserWithEmail_NilTelegramID(t *testing.T) {
	user := UserWithEmail{
		ID:         1,
		Email:      "test@example.com",
		TelegramID: nil,
	}

	if user.TelegramID != nil {
		t.Error("expected nil telegram ID")
	}
}
