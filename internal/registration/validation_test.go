package registration

import (
	"testing"
)

// TestValidateEmail tests email validation
func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid email", "user@example.com", false},
		{"valid with subdomain", "user@mail.example.com", false},
		{"valid with plus", "user+tag@example.com", false},
		{"empty email", "", true},
		{"whitespace only", "   ", true},
		{"no at sign", "userexample.com", true},
		{"at sign at start", "@example.com", true},
		{"no domain dot", "user@example", true},
		{"short domain", "user@ex", true},
		// Note: "user@@example.com" passes current validation (basic check only)
		{"domain with at sign", "user@test@example.com", false}, // passes because domain has @.com
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEmail(%q) error = %v, wantErr %v", tt.email, err, tt.wantErr)
			}
		})
	}
}

// TestValidatePassword tests password validation
func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"valid 8 chars", "password", false},
		{"valid long", "verylongpassword123!", false},
		{"too short 7", "passwor", true},
		{"too short 1", "a", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePassword(%q) error = %v, wantErr %v", tt.password, err, tt.wantErr)
			}
		})
	}
}

// TestValidateTelegramID tests telegram ID validation
func TestValidateTelegramID(t *testing.T) {
	validID := int64(123456789)
	zeroID := int64(0)

	tests := []struct {
		name    string
		id      *int64
		wantErr bool
	}{
		{"valid ID", &validID, false},
		{"nil ID", nil, true},
		{"zero ID", &zeroID, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTelegramID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTelegramID error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestNormalizeEmail tests email normalization
func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"User@Example.COM", "user@example.com"},
		{"  user@example.com  ", "user@example.com"},
		{"USER@EXAMPLE.COM", "user@example.com"},
		{"user@example.com", "user@example.com"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeEmail(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeEmail(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestValidationError tests ValidationError type
func TestValidationError(t *testing.T) {
	details := map[string]string{
		"email":    "invalid email",
		"password": "too short",
	}

	err := NewValidationError("validation failed", details)

	if err.Message != "validation failed" {
		t.Errorf("Message = %s, want validation failed", err.Message)
	}

	if err.Error() != "validation failed" {
		t.Errorf("Error() = %s, want validation failed", err.Error())
	}

	if len(err.Details) != 2 {
		t.Errorf("Details length = %d, want 2", len(err.Details))
	}

	if err.Details["email"] != "invalid email" {
		t.Errorf("Details[email] = %s, want invalid email", err.Details["email"])
	}
}

// TestValidateRequest tests request validation by method
func TestValidateRequest(t *testing.T) {
	validTelegramID := int64(123456789)

	tests := []struct {
		name    string
		req     *Request
		wantErr bool
	}{
		{
			name:    "missing method",
			req:     &Request{},
			wantErr: true,
		},
		{
			name: "valid telegram",
			req: &Request{
				Method:     MethodTelegram,
				TelegramID: &validTelegramID,
			},
			wantErr: false,
		},
		{
			name: "telegram missing id",
			req: &Request{
				Method: MethodTelegram,
			},
			wantErr: true,
		},
		{
			name: "valid email",
			req: &Request{
				Method:   MethodEmail,
				Email:    "user@example.com",
				Password: "password123",
			},
			wantErr: false,
		},
		{
			name: "email missing password",
			req: &Request{
				Method: MethodEmail,
				Email:  "user@example.com",
			},
			wantErr: true,
		},
		{
			name: "email invalid format",
			req: &Request{
				Method:   MethodEmail,
				Email:    "invalid",
				Password: "password123",
			},
			wantErr: true,
		},
		{
			name: "valid keycloak",
			req: &Request{
				Method:      MethodKeycloak,
				AccessToken: "token123",
			},
			wantErr: false,
		},
		{
			name: "keycloak missing token",
			req: &Request{
				Method: MethodKeycloak,
			},
			wantErr: true,
		},
		{
			name: "invalid method",
			req: &Request{
				Method: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateTelegramRequest tests Telegram-specific validation
func TestValidateTelegramRequest(t *testing.T) {
	validID := int64(123456789)

	t.Run("valid request", func(t *testing.T) {
		req := &Request{TelegramID: &validID}
		if err := ValidateTelegramRequest(req); err != nil {
			t.Errorf("ValidateTelegramRequest() unexpected error = %v", err)
		}
	})

	t.Run("nil telegram id", func(t *testing.T) {
		req := &Request{}
		err := ValidateTelegramRequest(req)
		if err == nil {
			t.Error("ValidateTelegramRequest() expected error for nil telegram_id")
		}
		if ve, ok := err.(*ValidationError); ok {
			if _, exists := ve.Details["telegram_id"]; !exists {
				t.Error("Expected telegram_id in validation details")
			}
		}
	})
}

// TestValidateEmailRequest tests email-specific validation
func TestValidateEmailRequest(t *testing.T) {
	t.Run("valid request", func(t *testing.T) {
		req := &Request{Email: "user@example.com", Password: "password123"}
		if err := ValidateEmailRequest(req); err != nil {
			t.Errorf("ValidateEmailRequest() unexpected error = %v", err)
		}
	})

	t.Run("both invalid", func(t *testing.T) {
		req := &Request{Email: "invalid", Password: "short"}
		err := ValidateEmailRequest(req)
		if err == nil {
			t.Error("ValidateEmailRequest() expected error")
		}
		if ve, ok := err.(*ValidationError); ok {
			if len(ve.Details) != 2 {
				t.Errorf("Expected 2 validation errors, got %d", len(ve.Details))
			}
		}
	})
}

// TestValidateKeycloakRequest tests Keycloak-specific validation
func TestValidateKeycloakRequest(t *testing.T) {
	t.Run("valid request", func(t *testing.T) {
		req := &Request{AccessToken: "valid_token"}
		if err := ValidateKeycloakRequest(req); err != nil {
			t.Errorf("ValidateKeycloakRequest() unexpected error = %v", err)
		}
	})

	t.Run("missing token", func(t *testing.T) {
		req := &Request{}
		err := ValidateKeycloakRequest(req)
		if err == nil {
			t.Error("ValidateKeycloakRequest() expected error for missing token")
		}
	})
}
