package handlers

import (
	"duq-gateway/internal/channels"
	"duq-gateway/internal/config"
	"duq-gateway/internal/credentials"
	"duq-gateway/internal/db"
	"duq-gateway/internal/queue"
	"duq-gateway/internal/registration"
)

// TelegramDeps contains dependencies for the Telegram handler
type TelegramDeps struct {
	Config              *config.Config
	QueueClient         *queue.Client
	RBACService         RBACServiceInterface
	CredService         CredentialServiceInterface
	ChannelRouter       *channels.Router
	DBClient            *db.Client
	RegistrationService *registration.Service
}

// RBACServiceInterface for RBAC operations
type RBACServiceInterface interface {
	GetAllowedTools(userID int64) ([]string, error)
	EnsureUser(userID int64, username, firstName, lastName string) error
	GetUserIDByTelegramID(telegramID int64) (int64, error)
	EnsureUserRole(userID int64) error
}

// CredentialServiceInterface for user credentials operations
type CredentialServiceInterface interface {
	GetCredentials(userID int64, provider string) (*credentials.UserCredentials, error)
	GetCredentialsByTelegramID(telegramID int64, provider string) (*credentials.UserCredentials, error)
	SaveCredentials(creds *credentials.UserCredentials) error
}
