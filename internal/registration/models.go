package registration

import "time"

// Method represents the registration method
type Method string

const (
	MethodTelegram Method = "telegram"
	MethodEmail    Method = "email"
	MethodKeycloak Method = "keycloak"
)

// Request is the unified registration request
type Request struct {
	Method Method `json:"method"`

	// Telegram registration
	TelegramID *int64 `json:"telegram_id,omitempty"`
	Username   string `json:"username,omitempty"`

	// Email registration
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`

	// Keycloak registration
	AccessToken string `json:"access_token,omitempty"`

	// Common fields
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

// User represents a registered user
type User struct {
	ID                int64  `json:"id"`
	Email             string `json:"email,omitempty"`
	TelegramID        *int64 `json:"telegram_id,omitempty"`
	Username          string `json:"username,omitempty"`
	FirstName         string `json:"first_name,omitempty"`
	LastName          string `json:"last_name,omitempty"`
	Role              string `json:"role"`
	IsActive          bool   `json:"is_active"`
	Timezone          string `json:"timezone,omitempty"`
	PreferredLanguage string `json:"preferred_language,omitempty"`
}

// Response is the unified registration response
type Response struct {
	Success              bool   `json:"success"`
	Message              string `json:"message"`
	UserID               int64  `json:"user_id,omitempty"`
	VerificationRequired bool   `json:"verification_required,omitempty"`
	User                 *User  `json:"user,omitempty"`
}

// ErrorResponse is the error response for registration
type ErrorResponse struct {
	Success bool              `json:"success"`
	Error   string            `json:"error"`
	Code    ErrorCode         `json:"code"`
	Details map[string]string `json:"details,omitempty"`
}

// ErrorCode represents error codes
type ErrorCode string

const (
	ErrInvalidMethod   ErrorCode = "INVALID_METHOD"
	ErrValidationError ErrorCode = "VALIDATION_ERROR"
	ErrEmailExists     ErrorCode = "EMAIL_EXISTS"
	ErrTelegramExists  ErrorCode = "TELEGRAM_EXISTS"
	ErrInvalidToken    ErrorCode = "INVALID_TOKEN"
	ErrInternalError   ErrorCode = "INTERNAL_ERROR"
)

// ActivationPolicy determines how a user is activated after registration
type ActivationPolicy int

const (
	// ActivateImmediately - user is active right away (Telegram, Keycloak)
	ActivateImmediately ActivationPolicy = iota
	// RequireEmailVerification - user must verify email before activation
	RequireEmailVerification
)

// CreateUserParams contains parameters for creating a user
type CreateUserParams struct {
	Email        string
	PasswordHash string
	TelegramID   *int64
	Username     string
	FirstName    string
	LastName     string
	KeycloakSub  string
	IsActive     bool
	Role         string
}

// EmailVerificationToken represents a verification token
type EmailVerificationToken struct {
	ID        int64
	UserID    int64
	Token     string
	ExpiresAt time.Time
	CreatedAt time.Time
}
