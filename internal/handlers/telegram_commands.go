package handlers

import (
	"context"
	"log"
	"net/http"

	"duq-gateway/internal/registration"
)

// handleStartRegistration handles user registration on /start
// Only registers new users, does NOT send any response - LLM will handle greeting
func handleStartRegistration(w http.ResponseWriter, msg *TelegramMessage, deps *TelegramDeps) {
	telegramID := msg.Chat.ID

	// Check if user exists using RegistrationService (unified API)
	if deps.RegistrationService != nil {
		userExists := deps.RegistrationService.CheckUserExists(telegramID)

		if !userExists {
			// NEW USER: Auto-register via unified Registration API
			username := ""
			firstName := ""
			lastName := ""
			if msg.From != nil {
				username = msg.From.Username
				firstName = msg.From.FirstName
				lastName = msg.From.LastName
			}

			// Use unified registration service
			regReq := &registration.Request{
				Method:     registration.MethodTelegram,
				TelegramID: &telegramID,
				Username:   username,
				FirstName:  firstName,
				LastName:   lastName,
			}

			_, err := deps.RegistrationService.Register(context.Background(), regReq)
			if err != nil {
				log.Printf("[telegram] Registration service error: %v", err)
				SendTelegramMessage(deps.Config, telegramID, "❌ Ошибка регистрации. Попробуй позже.")
				return
			}

			log.Printf("[telegram] New user registered: telegram_id=%d, username=%s", telegramID, username)
		}
	}

	// No response here - LLM will handle the greeting
	// The /start command continues to be processed as a regular message
}
