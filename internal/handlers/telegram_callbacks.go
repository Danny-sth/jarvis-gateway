package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// handleCallbackQuery processes inline keyboard button clicks
func handleCallbackQuery(w http.ResponseWriter, callback *TelegramCallbackQuery, deps *TelegramDeps) {
	if callback.Message == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	chatID := callback.Message.Chat.ID
	data := callback.Data

	// Answer callback to remove loading state
	AnswerCallbackQuery(deps.Config, callback.ID, "")

	log.Printf("[telegram] Callback from %d: %s", chatID, data)

	switch data {
	case "menu_history":
		handleMenuHistory(w, chatID, deps)
	case "menu_settings":
		handleMenuSettings(w, chatID, deps)
	case "menu_tools":
		handleMenuTools(w, chatID, deps)
	case "menu_help":
		handleMenuHelp(w, chatID, deps)
	case "menu_back":
		handleMenuBack(w, chatID, deps)
	default:
		log.Printf("[telegram] Unknown callback data: %s", data)
		w.WriteHeader(http.StatusOK)
	}
}

// handleMenuHistory shows last 10 messages
// NOTE: History is managed by Duq backend, not gateway
func handleMenuHistory(w http.ResponseWriter, chatID int64, deps *TelegramDeps) {
	SendTelegramMessage(deps.Config, chatID, "📜 История хранится на сервере. Спроси меня: \"покажи историю\"")
	w.WriteHeader(http.StatusOK)
}

// handleMenuSettings shows user settings
func handleMenuSettings(w http.ResponseWriter, chatID int64, deps *TelegramDeps) {
	if deps.DBClient != nil {
		user, err := deps.DBClient.GetUserByTelegramID(chatID)
		if err != nil || user == nil {
			SendTelegramMessage(deps.Config, chatID, "❌ Настройки недоступны. Отправь /start для регистрации.")
			w.WriteHeader(http.StatusOK)
			return
		}

		settingsText := fmt.Sprintf(`⚙️ *Твои настройки*

👤 *Аккаунт:*
• Имя: %s %s
• Username: %s
• Роль: %s

🌍 *Предпочтения:*
• Часовой пояс: %s
• Язык: %s

Для изменения настроек обратись к администратору.`, user.FirstName, user.LastName, user.Username, user.Role, user.Timezone, user.PreferredLanguage)

		// Settings keyboard with back button
		keyboard := &InlineKeyboardMarkup{
			InlineKeyboard: [][]InlineKeyboardButton{
				{{Text: "« Назад", CallbackData: "menu_back"}},
			},
		}
		SendTelegramMessageWithKeyboard(deps.Config, chatID, settingsText, keyboard)
	} else {
		SendTelegramMessage(deps.Config, chatID, "❌ Настройки недоступны")
	}
	w.WriteHeader(http.StatusOK)
}

// handleMenuTools delegates to handleToolsCommand for real-time API status
func handleMenuTools(w http.ResponseWriter, chatID int64, deps *TelegramDeps) {
	handleToolsCommand(w, chatID, deps)
}

// handleMenuHelp shows help message
func handleMenuHelp(w http.ResponseWriter, chatID int64, deps *TelegramDeps) {
	helpText := `❓ *Помощь*

🤖 Я — *Duq*, твой AI-ассистент.

*Что я умею:*
• Отвечать на вопросы
• Управлять календарём и задачами
• Работать с почтой Gmail
• Искать в интернете
• И многое другое!

*Как общаться:*
• Просто напиши текстовое сообщение
• Или отправь голосовое — я пойму!

*Команды:*
• /start — главное меню
• /history — история сообщений
• /settings — настройки`

	keyboard := &InlineKeyboardMarkup{
		InlineKeyboard: [][]InlineKeyboardButton{
			{{Text: "« Назад", CallbackData: "menu_back"}},
		},
	}
	SendTelegramMessageWithKeyboard(deps.Config, chatID, helpText, keyboard)
	w.WriteHeader(http.StatusOK)
}

// handleMenuBack returns to main menu
func handleMenuBack(w http.ResponseWriter, chatID int64, deps *TelegramDeps) {
	welcomeText := `🏠 *Главное меню*

Выбери действие или просто отправь мне сообщение!`

	SendTelegramMessageWithKeyboard(deps.Config, chatID, welcomeText, getMainMenuKeyboard())
	w.WriteHeader(http.StatusOK)
}

// ToolStatusResponse represents the API response from Duq /api/tools/status
type ToolStatusResponse struct {
	Total       int          `json:"total"`
	Available   int          `json:"available"`
	Unavailable int          `json:"unavailable"`
	Tools       []ToolStatus `json:"tools"`
}

// ToolStatus represents a single tool's status
type ToolStatus struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Category         string   `json:"category"`
	Available        bool     `json:"available"`
	RequiredServices []string `json:"required_services"`
	MissingServices  []string `json:"missing_services"`
	OwnerOnly        bool     `json:"owner_only"`
}

// handleToolsCommand fetches real-time tool status from Duq API
func handleToolsCommand(w http.ResponseWriter, chatID int64, deps *TelegramDeps) {
	// Fetch from Duq API
	duqURL := deps.Config.DuqURL
	if duqURL == "" {
		duqURL = "http://duq-core:8081"
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(duqURL + "/api/tools/status")
	if err != nil {
		log.Printf("[telegram] Failed to fetch tools status: %v", err)
		SendTelegramMessage(deps.Config, chatID, "⚠️ Не удалось получить статус инструментов")
		w.WriteHeader(http.StatusOK)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[telegram] Failed to read tools status: %v", err)
		SendTelegramMessage(deps.Config, chatID, "⚠️ Ошибка при чтении ответа")
		w.WriteHeader(http.StatusOK)
		return
	}

	var status ToolStatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		log.Printf("[telegram] Failed to parse tools status: %v, body: %s", err, string(body))
		SendTelegramMessage(deps.Config, chatID, "⚠️ Ошибка при разборе ответа")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Group tools by category
	categories := make(map[string][]ToolStatus)
	for _, tool := range status.Tools {
		categories[tool.Category] = append(categories[tool.Category], tool)
	}

	// Build message with top categories
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🛠 *Статус инструментов*\n\n"))
	sb.WriteString(fmt.Sprintf("Всего: %d | ✅ %d | ❌ %d\n\n", status.Total, status.Available, status.Unavailable))

	// Show top categories with counts
	for category, tools := range categories {
		availCount := 0
		for _, t := range tools {
			if t.Available {
				availCount++
			}
		}

		categoryIcon := "📦"
		switch category {
		case "Google Mail", "Google Calendar", "Google Drive", "Google Tasks":
			categoryIcon = "📧"
		case "Weather":
			categoryIcon = "🌤"
		case "Web":
			categoryIcon = "🌐"
		case "Task Management":
			categoryIcon = "📋"
		case "File System":
			categoryIcon = "📁"
		case "System":
			categoryIcon = "⚙️"
		case "Obsidian":
			categoryIcon = "📝"
		case "Scheduling":
			categoryIcon = "⏰"
		}

		sb.WriteString(fmt.Sprintf("%s *%s*: %d/%d\n", categoryIcon, category, availCount, len(tools)))
	}

	sb.WriteString("\n💡 Все инструменты доступны через диалог с Duq")

	SendTelegramMessage(deps.Config, chatID, sb.String())
	w.WriteHeader(http.StatusOK)
}
