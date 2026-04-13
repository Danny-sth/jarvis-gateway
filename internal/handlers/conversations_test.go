package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestConversationResponseStruct tests ConversationResponse serialization
func TestConversationResponseStruct(t *testing.T) {
	resp := ConversationResponse{
		ID:            "550e8400-e29b-41d4-a716-446655440000",
		UserID:        123456789,
		Title:         "Test Conversation",
		StartedAt:     1712923200, // 2024-04-12 12:00:00 UTC
		LastMessageAt: 1712926800, // 2024-04-12 13:00:00 UTC
		IsActive:      true,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var parsed ConversationResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if parsed.ID != resp.ID {
		t.Errorf("ID = %s, want %s", parsed.ID, resp.ID)
	}
	if parsed.UserID != resp.UserID {
		t.Errorf("UserID = %d, want %d", parsed.UserID, resp.UserID)
	}
	if parsed.Title != resp.Title {
		t.Errorf("Title = %s, want %s", parsed.Title, resp.Title)
	}
	if parsed.StartedAt != resp.StartedAt {
		t.Errorf("StartedAt = %d, want %d", parsed.StartedAt, resp.StartedAt)
	}
	if parsed.LastMessageAt != resp.LastMessageAt {
		t.Errorf("LastMessageAt = %d, want %d", parsed.LastMessageAt, resp.LastMessageAt)
	}
	if parsed.IsActive != resp.IsActive {
		t.Errorf("IsActive = %v, want %v", parsed.IsActive, resp.IsActive)
	}
}

// TestConversationResponseOmitEmptyTitle tests that empty title is omitted
func TestConversationResponseOmitEmptyTitle(t *testing.T) {
	resp := ConversationResponse{
		ID:       "test-id",
		UserID:   123,
		Title:    "", // Empty title
		IsActive: true,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	jsonStr := string(data)
	if strings.Contains(jsonStr, `"title":`) {
		t.Errorf("Empty title should be omitted, got: %s", jsonStr)
	}
}

// TestCreateConversationRequest tests request parsing
func TestCreateConversationRequestParsing(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantTitle string
	}{
		{
			name:      "with title",
			body:      `{"title": "My Conversation"}`,
			wantTitle: "My Conversation",
		},
		{
			name:      "empty title",
			body:      `{"title": ""}`,
			wantTitle: "",
		},
		{
			name:      "no title field",
			body:      `{}`,
			wantTitle: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req CreateConversationRequest
			if err := json.NewDecoder(strings.NewReader(tt.body)).Decode(&req); err != nil {
				t.Fatalf("Failed to decode: %v", err)
			}

			if req.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", req.Title, tt.wantTitle)
			}
		})
	}
}

// TestUpdateConversationRequest tests request parsing
func TestUpdateConversationRequestParsing(t *testing.T) {
	body := `{"title": "Updated Title"}`

	var req UpdateConversationRequest
	if err := json.NewDecoder(strings.NewReader(body)).Decode(&req); err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if req.Title != "Updated Title" {
		t.Errorf("Title = %q, want %q", req.Title, "Updated Title")
	}
}

// TestConversationsListUnauthorized tests unauthorized access
func TestConversationsListUnauthorized(t *testing.T) {
	handler := ConversationsList(nil)

	// Request without telegram_id in context
	req := httptest.NewRequest("GET", "/api/conversations", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// TestConversationsListWithTelegramID tests that telegram_id is extracted from context
func TestConversationsListWithWrongContextType(t *testing.T) {
	handler := ConversationsList(nil)

	// Request with wrong type in context
	req := httptest.NewRequest("GET", "/api/conversations", nil)
	ctx := context.WithValue(req.Context(), "telegram_id", "not-an-int64") // wrong type
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// TestCreateConversationUnauthorized tests unauthorized access
func TestCreateConversationUnauthorized(t *testing.T) {
	handler := CreateConversation(nil)

	req := httptest.NewRequest("POST", "/api/conversations", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// TestCreateConversationInvalidJSON tests invalid JSON handling
func TestCreateConversationInvalidJSON(t *testing.T) {
	handler := CreateConversation(nil)

	req := httptest.NewRequest("POST", "/api/conversations", strings.NewReader(`{invalid}`))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "telegram_id", int64(123))
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// TestUpdateConversationUnauthorized tests unauthorized access
func TestUpdateConversationUnauthorized(t *testing.T) {
	handler := UpdateConversation(nil)

	req := httptest.NewRequest("PUT", "/api/conversations/test-id", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// TestUpdateConversationMissingID tests missing conversation ID
func TestUpdateConversationMissingID(t *testing.T) {
	handler := UpdateConversation(nil)

	req := httptest.NewRequest("PUT", "/api/conversations/", strings.NewReader(`{"title":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "telegram_id", int64(123))
	req = req.WithContext(ctx)
	// SetPathValue is not available in test, so PathValue returns empty

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// TestUpdateConversationInvalidJSON tests invalid JSON handling
func TestUpdateConversationInvalidJSON(t *testing.T) {
	handler := UpdateConversation(nil)

	req := httptest.NewRequest("PUT", "/api/conversations/test-id", strings.NewReader(`{invalid}`))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "telegram_id", int64(123))
	req = req.WithContext(ctx)
	req.SetPathValue("id", "test-id")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// TestUpdateConversationEmptyTitle tests empty title validation
func TestUpdateConversationEmptyTitle(t *testing.T) {
	handler := UpdateConversation(nil)

	req := httptest.NewRequest("PUT", "/api/conversations/test-id", strings.NewReader(`{"title":""}`))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), "telegram_id", int64(123))
	req = req.WithContext(ctx)
	req.SetPathValue("id", "test-id")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// TestEndConversationUnauthorized tests unauthorized access
func TestEndConversationUnauthorized(t *testing.T) {
	handler := EndConversation(nil)

	req := httptest.NewRequest("DELETE", "/api/conversations/test-id", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

// TestEndConversationMissingID tests missing conversation ID
func TestEndConversationMissingID(t *testing.T) {
	handler := EndConversation(nil)

	req := httptest.NewRequest("DELETE", "/api/conversations/", nil)
	ctx := context.WithValue(req.Context(), "telegram_id", int64(123))
	req = req.WithContext(ctx)
	// PathValue returns empty string

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// TestMessageResponseStruct tests MessageResponse serialization
func TestMessageResponseStruct(t *testing.T) {
	resp := MessageResponse{
		ID:              1,
		ConversationID:  "conv-123",
		Role:            "assistant",
		Content:         "Hello!",
		HasAudio:        true,
		AudioDurationMs: 5000,
		Waveform:        []float64{0.1, 0.5, 0.8},
		CreatedAt:       1712923200,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var parsed MessageResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if parsed.ID != resp.ID {
		t.Errorf("ID = %d, want %d", parsed.ID, resp.ID)
	}
	if parsed.Role != resp.Role {
		t.Errorf("Role = %s, want %s", parsed.Role, resp.Role)
	}
	if parsed.Content != resp.Content {
		t.Errorf("Content = %s, want %s", parsed.Content, resp.Content)
	}
	if parsed.HasAudio != resp.HasAudio {
		t.Errorf("HasAudio = %v, want %v", parsed.HasAudio, resp.HasAudio)
	}
	if parsed.AudioDurationMs != resp.AudioDurationMs {
		t.Errorf("AudioDurationMs = %d, want %d", parsed.AudioDurationMs, resp.AudioDurationMs)
	}
	if len(parsed.Waveform) != len(resp.Waveform) {
		t.Errorf("Waveform length = %d, want %d", len(parsed.Waveform), len(resp.Waveform))
	}
}

// TestMessageResponseOmitEmpty tests optional fields are omitted
func TestMessageResponseOmitEmpty(t *testing.T) {
	resp := MessageResponse{
		ID:             1,
		ConversationID: "conv-123",
		Role:           "user",
		Content:        "Hello",
		HasAudio:       false,
		CreatedAt:      1712923200,
		// AudioDurationMs and Waveform not set
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	jsonStr := string(data)
	if strings.Contains(jsonStr, `"audio_duration_ms"`) {
		t.Errorf("audio_duration_ms should be omitted when zero: %s", jsonStr)
	}
	if strings.Contains(jsonStr, `"waveform"`) {
		t.Errorf("waveform should be omitted when nil: %s", jsonStr)
	}
}
