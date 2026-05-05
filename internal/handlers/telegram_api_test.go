package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"duq-gateway/internal/config"
)

// ==================== Group Chat API Tests ====================
// TDD: These tests are written BEFORE the implementation (RED phase)

// TestSendMessageWithReply_Success tests reply_parameters are sent correctly
func TestSendMessageWithReply_Success(t *testing.T) {
	// Mock Telegram API server
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's a POST to sendMessage
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "sendMessage") {
			t.Errorf("Expected path containing sendMessage, got %s", r.URL.Path)
		}

		// Parse request body
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)

		// Return success
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true,"result":{"message_id":123}}`))
	}))
	defer server.Close()

	// Create config with mock server URL
	cfg := &config.Config{
		Telegram: config.TelegramConfig{
			BotToken: "test-token",
		},
	}

	// Call the function (this should FAIL in RED phase - function doesn't exist)
	err := SendMessageWithReply(cfg, 12345, "Hello reply!", 999, server.URL)
	if err != nil {
		t.Fatalf("SendMessageWithReply failed: %v", err)
	}

	// Verify request contains reply_parameters
	replyParams, ok := receivedBody["reply_parameters"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected reply_parameters in request body")
	}

	// Verify message_id in reply_parameters
	msgID, ok := replyParams["message_id"].(float64)
	if !ok || int64(msgID) != 999 {
		t.Errorf("Expected reply_parameters.message_id=999, got %v", replyParams["message_id"])
	}

	// Verify chat_id
	chatID, ok := receivedBody["chat_id"].(float64)
	if !ok || int64(chatID) != 12345 {
		t.Errorf("Expected chat_id=12345, got %v", receivedBody["chat_id"])
	}

	// Verify text
	text, ok := receivedBody["text"].(string)
	if !ok || text != "Hello reply!" {
		t.Errorf("Expected text='Hello reply!', got %v", receivedBody["text"])
	}
}

// TestSendMessageWithReply_NoToken tests error when bot token is missing
func TestSendMessageWithReply_NoToken(t *testing.T) {
	cfg := &config.Config{
		Telegram: config.TelegramConfig{
			BotToken: "", // Empty token
		},
	}

	err := SendMessageWithReply(cfg, 12345, "Hello", 999, "http://localhost")
	if err == nil {
		t.Fatal("Expected error when bot token is empty")
	}

	if !strings.Contains(err.Error(), "token") {
		t.Errorf("Error should mention token, got: %s", err.Error())
	}
}

// TestSendMessageWithReply_APIError tests handling of Telegram API errors
func TestSendMessageWithReply_APIError(t *testing.T) {
	// Mock server returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: message not found"}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		Telegram: config.TelegramConfig{
			BotToken: "test-token",
		},
	}

	err := SendMessageWithReply(cfg, 12345, "Hello", 999, server.URL)
	if err == nil {
		t.Fatal("Expected error on API failure")
	}
}

// ==================== GetChatInfo Tests ====================

func TestGetChatInfoHandler_Success(t *testing.T) {
	// This test verifies the HTTP handler for getting chat info
	// The handler should call Telegram's getChat API and return the result

	// Mock Telegram API
	telegramServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "getChat") {
			t.Errorf("Expected path containing getChat, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"ok": true,
			"result": {
				"id": -1001234567890,
				"type": "supergroup",
				"title": "Test Group",
				"description": "A test group"
			}
		}`))
	}))
	defer telegramServer.Close()

	cfg := &config.Config{
		Telegram: config.TelegramConfig{
			BotToken: "test-token",
		},
	}

	// Create handler (should FAIL - function doesn't exist)
	handler := GetChatInfoHandler(cfg, telegramServer.URL)

	// Create request
	reqBody := GetChatInfoRequest{ChatID: -1001234567890}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/telegram/chat/info", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	// Execute
	handler.ServeHTTP(rr, req)

	// Verify response
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	// Parse response
	var resp ChatFullInfo
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Title != "Test Group" {
		t.Errorf("Expected title 'Test Group', got '%s'", resp.Title)
	}
	if resp.Type != "supergroup" {
		t.Errorf("Expected type 'supergroup', got '%s'", resp.Type)
	}
}

// ==================== PinMessage Tests ====================

func TestPinMessageHandler_Success(t *testing.T) {
	// Mock Telegram API
	var receivedBody map[string]interface{}
	telegramServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "pinChatMessage") {
			t.Errorf("Expected path containing pinChatMessage, got %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer telegramServer.Close()

	cfg := &config.Config{
		Telegram: config.TelegramConfig{
			BotToken: "test-token",
		},
	}

	// Create handler (should FAIL - function doesn't exist)
	handler := PinMessageHandler(cfg, telegramServer.URL)

	// Create request
	reqBody := PinMessageRequest{
		ChatID:              -1001234567890,
		MessageID:           999,
		DisableNotification: true,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/telegram/pin", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	// Execute
	handler.ServeHTTP(rr, req)

	// Verify
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	// Verify Telegram API received correct params
	if receivedBody["chat_id"].(float64) != -1001234567890 {
		t.Errorf("Unexpected chat_id: %v", receivedBody["chat_id"])
	}
	if receivedBody["message_id"].(float64) != 999 {
		t.Errorf("Unexpected message_id: %v", receivedBody["message_id"])
	}
}

// ==================== UnpinMessage Tests ====================

func TestUnpinMessageHandler_Success(t *testing.T) {
	telegramServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "unpinChatMessage") {
			t.Errorf("Expected path containing unpinChatMessage, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer telegramServer.Close()

	cfg := &config.Config{
		Telegram: config.TelegramConfig{
			BotToken: "test-token",
		},
	}

	handler := UnpinMessageHandler(cfg, telegramServer.URL)

	reqBody := UnpinMessageRequest{
		ChatID:    -1001234567890,
		MessageID: 999,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/telegram/unpin", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

// ==================== EditMessage Tests ====================

func TestEditMessageHandler_Success(t *testing.T) {
	var receivedBody map[string]interface{}
	telegramServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "editMessageText") {
			t.Errorf("Expected path containing editMessageText, got %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true,"result":{"message_id":999}}`))
	}))
	defer telegramServer.Close()

	cfg := &config.Config{
		Telegram: config.TelegramConfig{
			BotToken: "test-token",
		},
	}

	handler := EditMessageHandler(cfg, telegramServer.URL)

	reqBody := EditMessageRequest{
		ChatID:    -1001234567890,
		MessageID: 999,
		Text:      "Updated text",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/telegram/edit", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if receivedBody["text"].(string) != "Updated text" {
		t.Errorf("Expected text 'Updated text', got %v", receivedBody["text"])
	}
}

// ==================== GetChatMember Tests ====================

func TestGetChatMemberHandler_Success(t *testing.T) {
	telegramServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "getChatMember") {
			t.Errorf("Expected path containing getChatMember, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"ok": true,
			"result": {
				"user": {
					"id": 123456789,
					"first_name": "John",
					"username": "johndoe"
				},
				"status": "administrator",
				"custom_title": "Admin"
			}
		}`))
	}))
	defer telegramServer.Close()

	cfg := &config.Config{
		Telegram: config.TelegramConfig{
			BotToken: "test-token",
		},
	}

	handler := GetChatMemberHandler(cfg, telegramServer.URL)

	reqBody := GetChatMemberRequest{
		ChatID: -1001234567890,
		UserID: 123456789,
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/telegram/chat/member", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d: %s", http.StatusOK, rr.Code, rr.Body.String())
	}

	var resp ChatMember
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Status != "administrator" {
		t.Errorf("Expected status 'administrator', got '%s'", resp.Status)
	}
	if resp.User.Username != "johndoe" {
		t.Errorf("Expected username 'johndoe', got '%s'", resp.User.Username)
	}
}
