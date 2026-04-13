package session

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// newMockService creates a mock service for testing
func newMockService(t *testing.T) (*Service, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	return NewService(db), mock
}

func TestNewServiceWithMock(t *testing.T) {
	db, _, _ := sqlmock.New()
	defer db.Close()

	svc := NewService(db)
	if svc == nil {
		t.Fatal("expected service, got nil")
	}
	if svc.db != db {
		t.Error("db not set correctly")
	}
}

func TestGetOrCreateConversation_ExistingActive(t *testing.T) {
	svc, mock := newMockService(t)
	defer svc.db.Close()

	userID := int64(42)
	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "user_id", "title", "started_at", "last_message_at", "is_active",
	}).AddRow("conv-123", userID, "Test", now, now, true)

	mock.ExpectQuery(`SELECT id, user_id, title, started_at, last_message_at, is_active`).
		WithArgs(userID).
		WillReturnRows(rows)

	conv, err := svc.GetOrCreateConversation(userID)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if conv == nil {
		t.Fatal("expected conversation, got nil")
	}
	if conv.ID != "conv-123" {
		t.Errorf("expected ID conv-123, got %s", conv.ID)
	}
	if conv.Title != "Test" {
		t.Errorf("expected title Test, got %s", conv.Title)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetOrCreateConversation_NoActive_CreatesNew(t *testing.T) {
	svc, mock := newMockService(t)
	defer svc.db.Close()

	userID := int64(42)
	now := time.Now()

	// First query returns no rows
	mock.ExpectQuery(`SELECT id, user_id, title, started_at, last_message_at, is_active`).
		WithArgs(userID).
		WillReturnError(sql.ErrNoRows)

	// Then insert new conversation
	insertRows := sqlmock.NewRows([]string{"started_at", "last_message_at"}).
		AddRow(now, now)
	mock.ExpectQuery(`INSERT INTO conversations`).
		WithArgs(sqlmock.AnyArg(), userID).
		WillReturnRows(insertRows)

	conv, err := svc.GetOrCreateConversation(userID)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if conv == nil {
		t.Fatal("expected conversation, got nil")
	}
	if conv.UserID != userID {
		t.Errorf("expected user ID %d, got %d", userID, conv.UserID)
	}
	if !conv.IsActive {
		t.Error("expected new conversation to be active")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetOrCreateConversation_QueryError(t *testing.T) {
	svc, mock := newMockService(t)
	defer svc.db.Close()

	userID := int64(42)

	mock.ExpectQuery(`SELECT id, user_id, title, started_at, last_message_at, is_active`).
		WithArgs(userID).
		WillReturnError(sql.ErrConnDone)

	conv, err := svc.GetOrCreateConversation(userID)

	if err == nil {
		t.Error("expected error")
	}
	if conv != nil {
		t.Error("expected nil conversation on error")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetOrCreateConversationID(t *testing.T) {
	svc, mock := newMockService(t)
	defer svc.db.Close()

	userID := int64(42)
	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "user_id", "title", "started_at", "last_message_at", "is_active",
	}).AddRow("conv-456", userID, nil, now, now, true)

	mock.ExpectQuery(`SELECT id, user_id, title, started_at, last_message_at, is_active`).
		WithArgs(userID).
		WillReturnRows(rows)

	convID, err := svc.GetOrCreateConversationID(userID)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if convID != "conv-456" {
		t.Errorf("expected conv-456, got %s", convID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSaveMessage_Success(t *testing.T) {
	svc, mock := newMockService(t)
	defer svc.db.Close()

	convID := "conv-123"
	role := "user"
	content := "Hello world"
	toolCalls := json.RawMessage(`[{"name":"test"}]`)

	mock.ExpectExec(`INSERT INTO messages`).
		WithArgs(convID, role, content, toolCalls).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(`UPDATE conversations SET last_message_at`).
		WithArgs(convID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := svc.SaveMessage(convID, role, content, toolCalls)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSaveMessage_NoToolCalls(t *testing.T) {
	svc, mock := newMockService(t)
	defer svc.db.Close()

	convID := "conv-123"
	role := "assistant"
	content := "Hi there"

	mock.ExpectExec(`INSERT INTO messages`).
		WithArgs(convID, role, content, nil).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(`UPDATE conversations SET last_message_at`).
		WithArgs(convID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := svc.SaveMessage(convID, role, content, nil)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSaveMessageSimple(t *testing.T) {
	svc, mock := newMockService(t)
	defer svc.db.Close()

	convID := "conv-123"
	role := "user"
	content := "Simple message"

	mock.ExpectExec(`INSERT INTO messages`).
		WithArgs(convID, role, content, nil).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec(`UPDATE conversations SET last_message_at`).
		WithArgs(convID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := svc.SaveMessageSimple(convID, role, content)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetRecentMessages_Success(t *testing.T) {
	svc, mock := newMockService(t)
	defer svc.db.Close()

	convID := "conv-123"
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "role", "content", "tool_calls", "created_at"}).
		AddRow(2, "assistant", "Response", nil, now).
		AddRow(1, "user", "Hello", nil, now.Add(-time.Minute))

	mock.ExpectQuery(`SELECT id, role, content, tool_calls, created_at`).
		WithArgs(convID, 50).
		WillReturnRows(rows)

	messages, err := svc.GetRecentMessages(convID, 50)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}
	// Should be reversed to chronological order
	if messages[0].Content != "Hello" {
		t.Errorf("expected first message to be Hello, got %s", messages[0].Content)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetRecentMessages_DefaultLimit(t *testing.T) {
	svc, mock := newMockService(t)
	defer svc.db.Close()

	convID := "conv-123"

	rows := sqlmock.NewRows([]string{"id", "role", "content", "tool_calls", "created_at"})

	mock.ExpectQuery(`SELECT id, role, content, tool_calls, created_at`).
		WithArgs(convID, 50). // Default limit
		WillReturnRows(rows)

	_, err := svc.GetRecentMessages(convID, 0) // 0 should default to 50

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetRecentMessages_WithToolCalls(t *testing.T) {
	svc, mock := newMockService(t)
	defer svc.db.Close()

	convID := "conv-123"
	now := time.Now()
	toolCallsStr := `[{"name":"search"}]`

	rows := sqlmock.NewRows([]string{"id", "role", "content", "tool_calls", "created_at"}).
		AddRow(1, "assistant", "Searching...", toolCallsStr, now)

	mock.ExpectQuery(`SELECT id, role, content, tool_calls, created_at`).
		WithArgs(convID, 10).
		WillReturnRows(rows)

	messages, err := svc.GetRecentMessages(convID, 10)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if string(messages[0].ToolCalls) != toolCallsStr {
		t.Errorf("expected tool calls %s, got %s", toolCallsStr, messages[0].ToolCalls)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestEndConversation_Success(t *testing.T) {
	svc, mock := newMockService(t)
	defer svc.db.Close()

	convID := "conv-123"

	mock.ExpectExec(`UPDATE conversations SET is_active = FALSE`).
		WithArgs(convID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := svc.EndConversation(convID)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSetConversationTitle_Success(t *testing.T) {
	svc, mock := newMockService(t)
	defer svc.db.Close()

	convID := "conv-123"
	title := "New Title"

	mock.ExpectExec(`UPDATE conversations SET title`).
		WithArgs(title, convID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := svc.SetConversationTitle(convID, title)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetUserConversations_Success(t *testing.T) {
	svc, mock := newMockService(t)
	defer svc.db.Close()

	userID := int64(42)
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "title", "started_at", "last_message_at", "is_active",
	}).
		AddRow("conv-1", userID, "Conversation 1", now, now, true).
		AddRow("conv-2", userID, nil, now.Add(-time.Hour), now.Add(-time.Hour), false)

	mock.ExpectQuery(`SELECT id, user_id, title, started_at, last_message_at, is_active`).
		WithArgs(userID, 20).
		WillReturnRows(rows)

	convs, err := svc.GetUserConversations(userID, 20)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(convs) != 2 {
		t.Errorf("expected 2 conversations, got %d", len(convs))
	}
	if convs[0].Title != "Conversation 1" {
		t.Errorf("expected title 'Conversation 1', got '%s'", convs[0].Title)
	}
	if convs[1].Title != "" {
		t.Errorf("expected empty title, got '%s'", convs[1].Title)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetUserConversations_DefaultLimit(t *testing.T) {
	svc, mock := newMockService(t)
	defer svc.db.Close()

	userID := int64(42)

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "title", "started_at", "last_message_at", "is_active",
	})

	mock.ExpectQuery(`SELECT id, user_id, title, started_at, last_message_at, is_active`).
		WithArgs(userID, 20). // Default limit
		WillReturnRows(rows)

	_, err := svc.GetUserConversations(userID, 0) // 0 should default to 20

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetMessagesAudio_Success(t *testing.T) {
	svc, mock := newMockService(t)
	defer svc.db.Close()

	convID := "conv-123"
	waveformJSON := `[0.1, 0.5, 0.3]`

	rows := sqlmock.NewRows([]string{"message_id", "duration_ms", "waveform"}).
		AddRow(1, 5000, waveformJSON).
		AddRow(2, 3000, nil)

	mock.ExpectQuery(`SELECT ma.message_id, ma.duration_ms, ma.waveform`).
		WithArgs(convID).
		WillReturnRows(rows)

	audioMap, err := svc.GetMessagesAudio(convID)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(audioMap) != 2 {
		t.Errorf("expected 2 audio entries, got %d", len(audioMap))
	}
	if audioMap[1].DurationMs != 5000 {
		t.Errorf("expected duration 5000, got %d", audioMap[1].DurationMs)
	}
	if len(audioMap[1].Waveform) != 3 {
		t.Errorf("expected 3 waveform points, got %d", len(audioMap[1].Waveform))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetMessageAudio_Success(t *testing.T) {
	svc, mock := newMockService(t)
	defer svc.db.Close()

	messageID := int64(1)
	audioData := []byte{0x00, 0x01, 0x02}
	waveformJSON := `[0.1, 0.2]`

	rows := sqlmock.NewRows([]string{"audio_data", "duration_ms", "waveform"}).
		AddRow(audioData, 5000, waveformJSON)

	mock.ExpectQuery(`SELECT audio_data, duration_ms, waveform`).
		WithArgs(messageID).
		WillReturnRows(rows)

	data, meta, err := svc.GetMessageAudio(messageID)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(data) != 3 {
		t.Errorf("expected 3 bytes, got %d", len(data))
	}
	if meta.MessageID != messageID {
		t.Errorf("expected message ID %d, got %d", messageID, meta.MessageID)
	}
	if meta.DurationMs != 5000 {
		t.Errorf("expected duration 5000, got %d", meta.DurationMs)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetMessageAudio_NotFound(t *testing.T) {
	svc, mock := newMockService(t)
	defer svc.db.Close()

	messageID := int64(999)

	mock.ExpectQuery(`SELECT audio_data, duration_ms, waveform`).
		WithArgs(messageID).
		WillReturnError(sql.ErrNoRows)

	data, meta, err := svc.GetMessageAudio(messageID)

	if err == nil {
		t.Error("expected error for not found")
	}
	if data != nil {
		t.Error("expected nil data")
	}
	if meta != nil {
		t.Error("expected nil meta")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSaveMessageAudio_Success(t *testing.T) {
	svc, mock := newMockService(t)
	defer svc.db.Close()

	messageID := int64(1)
	audioData := []byte{0x00, 0x01, 0x02}
	durationMs := 5000
	waveform := []float64{0.1, 0.5, 0.3}

	mock.ExpectExec(`INSERT INTO message_audio`).
		WithArgs(messageID, audioData, durationMs, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := svc.SaveMessageAudio(messageID, audioData, durationMs, waveform)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSaveMessageAudio_NoWaveform(t *testing.T) {
	svc, mock := newMockService(t)
	defer svc.db.Close()

	messageID := int64(1)
	audioData := []byte{0x00, 0x01}
	durationMs := 3000

	mock.ExpectExec(`INSERT INTO message_audio`).
		WithArgs(messageID, audioData, durationMs, nil).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := svc.SaveMessageAudio(messageID, audioData, durationMs, nil)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestGetConversationHistory_Success(t *testing.T) {
	svc, mock := newMockService(t)
	defer svc.db.Close()

	userID := int64(42)
	now := time.Now()

	// First get/create conversation
	convRows := sqlmock.NewRows([]string{
		"id", "user_id", "title", "started_at", "last_message_at", "is_active",
	}).AddRow("conv-123", userID, "Test", now, now, true)

	mock.ExpectQuery(`SELECT id, user_id, title, started_at, last_message_at, is_active`).
		WithArgs(userID).
		WillReturnRows(convRows)

	// Then get messages
	msgRows := sqlmock.NewRows([]string{"id", "role", "content", "tool_calls", "created_at"}).
		AddRow(1, "user", "Hi", nil, now)

	mock.ExpectQuery(`SELECT id, role, content, tool_calls, created_at`).
		WithArgs("conv-123", 10).
		WillReturnRows(msgRows)

	convID, messages, err := svc.GetConversationHistory(userID, 10)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if convID != "conv-123" {
		t.Errorf("expected conv-123, got %s", convID)
	}
	if len(messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(messages))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
