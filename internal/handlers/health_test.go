package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestHealthEndpoint tests the health handler
func TestHealthEndpoint(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	Health(rr, req)

	// Check status code
	if rr.Code != http.StatusOK {
		t.Errorf("Health() status = %d, want %d", rr.Code, http.StatusOK)
	}

	// Check content type
	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %s, want application/json", contentType)
	}

	// Parse response
	var resp HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Check status field
	if resp.Status != "ok" {
		t.Errorf("status = %s, want ok", resp.Status)
	}

	// Check version field
	if resp.Version != "1.0.0" {
		t.Errorf("version = %s, want 1.0.0", resp.Version)
	}

	// Check timestamp is valid RFC3339
	if _, err := time.Parse(time.RFC3339, resp.Timestamp); err != nil {
		t.Errorf("timestamp %q is not valid RFC3339: %v", resp.Timestamp, err)
	}
}

// TestHealthResponseStruct tests HealthResponse serialization
func TestHealthResponseStruct(t *testing.T) {
	resp := HealthResponse{
		Status:    "ok",
		Timestamp: "2026-04-12T10:00:00Z",
		Version:   "1.0.0",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Check JSON contains expected fields
	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"status":"ok"`) {
		t.Errorf("JSON missing status field: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"timestamp":`) {
		t.Errorf("JSON missing timestamp field: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"version":"1.0.0"`) {
		t.Errorf("JSON missing version field: %s", jsonStr)
	}
}

// TestHealthTimestampFormat tests that timestamp is in UTC
func TestHealthTimestampFormat(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	Health(rr, req)

	var resp HealthResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	// Timestamp should end with Z (UTC)
	if !strings.HasSuffix(resp.Timestamp, "Z") {
		t.Errorf("timestamp should be UTC (end with Z), got %s", resp.Timestamp)
	}

	// Parse and verify it's close to now
	parsed, err := time.Parse(time.RFC3339, resp.Timestamp)
	if err != nil {
		t.Fatalf("Failed to parse timestamp: %v", err)
	}

	// Should be within last 5 seconds
	diff := time.Since(parsed)
	if diff > 5*time.Second || diff < -5*time.Second {
		t.Errorf("timestamp %v is not close to current time (diff: %v)", parsed, diff)
	}
}

// TestHealthMethodNotAllowed tests that other methods still work (not enforced)
func TestHealthWithDifferentMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "HEAD"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/health", nil)
			rr := httptest.NewRecorder()

			Health(rr, req)

			// Health endpoint doesn't check method, should always return OK
			if rr.Code != http.StatusOK {
				t.Errorf("%s /health status = %d, want %d", method, rr.Code, http.StatusOK)
			}
		})
	}
}
