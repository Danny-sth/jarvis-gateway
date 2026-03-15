package session

// HandlerAdapter wraps Service to implement handlers.SessionServiceInterface
type HandlerAdapter struct {
	service *Service
}

// HistoryMessage for handler interface compatibility
type HistoryMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// NewHandlerAdapter creates a new adapter
func NewHandlerAdapter(service *Service) *HandlerAdapter {
	return &HandlerAdapter{service: service}
}

// GetOrCreateConversationID implements handlers.SessionServiceInterface
func (a *HandlerAdapter) GetOrCreateConversationID(userID int64) (string, error) {
	return a.service.GetOrCreateConversationID(userID)
}

// GetRecentMessagesSimple implements handlers.SessionServiceInterface
func (a *HandlerAdapter) GetRecentMessagesSimple(conversationID string, limit int) ([]HistoryMessage, error) {
	messages, err := a.service.GetRecentMessages(conversationID, limit)
	if err != nil {
		return nil, err
	}

	result := make([]HistoryMessage, len(messages))
	for i, m := range messages {
		result[i] = HistoryMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}
	return result, nil
}

// SaveMessageSimple implements handlers.SessionServiceInterface
func (a *HandlerAdapter) SaveMessageSimple(conversationID string, role, content string) error {
	return a.service.SaveMessageSimple(conversationID, role, content)
}
