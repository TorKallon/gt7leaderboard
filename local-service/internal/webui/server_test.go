package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rourkem/gt7leaderboard/local-service/internal/config"
	"github.com/rourkem/gt7leaderboard/local-service/internal/session"
)

// --- Mock session provider ---

type mockSessionProvider struct {
	session *session.ActiveSession
}

func (m *mockSessionProvider) CurrentSession() *session.ActiveSession {
	return m.session
}

// --- Tests ---

func TestHealthEndpoint(t *testing.T) {
	srv := NewServer(":0", &config.Config{}, nil, nil, "")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	srv.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", contentType)
	}

	var resp healthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.Status)
	}

	if resp.UptimeSeconds < 0 {
		t.Errorf("expected non-negative uptime, got %d", resp.UptimeSeconds)
	}
}

func TestHealthEndpointMethodNotAllowed(t *testing.T) {
	srv := NewServer(":0", &config.Config{}, nil, nil, "")

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()

	srv.handleHealth(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestStatusPageNoSession(t *testing.T) {
	sessions := &mockSessionProvider{session: nil}
	srv := NewServer(":0", &config.Config{}, nil, sessions, "")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	srv.handleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "No active session") {
		t.Error("expected 'No active session' text in response body")
	}
	if !strings.Contains(body, "GT7 Leaderboard") {
		t.Error("expected 'GT7 Leaderboard' title in response body")
	}
}

func TestStatusPageWithSession(t *testing.T) {
	sessions := &mockSessionProvider{
		session: &session.ActiveSession{
			ID:         "sess-123",
			DriverName: "TestDriver",
			TrackName:  "Suzuka Circuit",
			CarID:      42,
			LastLap:    5,
			StartedAt:  time.Now(),
		},
	}
	srv := NewServer(":0", &config.Config{}, nil, sessions, "")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	srv.handleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "TestDriver") {
		t.Error("expected driver name in response body")
	}
	if !strings.Contains(body, "Suzuka Circuit") {
		t.Error("expected track name in response body")
	}
}

func TestAuthPageGet(t *testing.T) {
	srv := NewServer(":0", &config.Config{}, nil, nil, "")

	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	w := httptest.NewRecorder()

	srv.handleAuth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "NPSSO") {
		t.Error("expected 'NPSSO' text in auth page")
	}
	if !strings.Contains(body, "ssocookie") {
		t.Error("expected ssocookie URL in auth page")
	}
}

func TestAuthPagePostEmptyToken(t *testing.T) {
	srv := NewServer(":0", &config.Config{}, nil, nil, "")

	req := httptest.NewRequest(http.MethodPost, "/auth", strings.NewReader("npsso="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	srv.handleAuth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "cannot be empty") {
		t.Error("expected empty token error message")
	}
}

func TestStatusPageNotFound(t *testing.T) {
	srv := NewServer(":0", &config.Config{}, nil, nil, "")

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.handleStatus(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{5 * time.Second, "5s"},
		{65 * time.Second, "1m 5s"},
		{3661 * time.Second, "1h 1m 1s"},
	}

	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}
