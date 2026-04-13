package registration

import (
	"errors"
	"strings"
)

// ValidationError represents a validation error with field-level details
type ValidationError struct {
	Message string
	Details map[string]string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// NewValidationError creates a new validation error
func NewValidationError(message string, details map[string]string) *ValidationError {
	return &ValidationError{
		Message: message,
		Details: details,
	}
}

// ValidateEmail performs basic email validation
func ValidateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return errors.New("email is required")
	}

	// Basic check: must contain @ and a dot after @
	atIdx := strings.Index(email, "@")
	if atIdx < 1 {
		return errors.New("invalid email format")
	}

	domain := email[atIdx+1:]
	if len(domain) < 3 || !strings.Contains(domain, ".") {
		return errors.New("invalid email domain")
	}

	return nil
}

// ValidatePassword validates password strength
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	return nil
}

// ValidateTelegramID validates telegram_id
func ValidateTelegramID(telegramID *int64) error {
	if telegramID == nil || *telegramID == 0 {
		return errors.New("telegram_id is required")
	}
	return nil
}

// ValidateTelegramRequest validates a Telegram registration request
func ValidateTelegramRequest(req *Request) error {
	details := make(map[string]string)

	if err := ValidateTelegramID(req.TelegramID); err != nil {
		details["telegram_id"] = err.Error()
	}

	if len(details) > 0 {
		return NewValidationError("validation failed", details)
	}

	return nil
}

// ValidateEmailRequest validates an email registration request
func ValidateEmailRequest(req *Request) error {
	details := make(map[string]string)

	if err := ValidateEmail(req.Email); err != nil {
		details["email"] = err.Error()
	}

	if err := ValidatePassword(req.Password); err != nil {
		details["password"] = err.Error()
	}

	if len(details) > 0 {
		return NewValidationError("validation failed", details)
	}

	return nil
}

// ValidateKeycloakRequest validates a Keycloak registration request
func ValidateKeycloakRequest(req *Request) error {
	if req.AccessToken == "" {
		return NewValidationError("validation failed", map[string]string{
			"access_token": "access_token is required",
		})
	}
	return nil
}

// ValidateRequest validates a registration request based on method
func ValidateRequest(req *Request) error {
	if req.Method == "" {
		return NewValidationError("validation failed", map[string]string{
			"method": "method is required",
		})
	}

	switch req.Method {
	case MethodTelegram:
		return ValidateTelegramRequest(req)
	case MethodEmail:
		return ValidateEmailRequest(req)
	case MethodKeycloak:
		return ValidateKeycloakRequest(req)
	default:
		return NewValidationError("validation failed", map[string]string{
			"method": "invalid method: must be 'telegram', 'email', or 'keycloak'",
		})
	}
}

// NormalizeEmail normalizes email address
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
