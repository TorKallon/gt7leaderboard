package psn

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestAuthenticateWithNPSSO(t *testing.T) {
	testCode := "v3.test-auth-code-123"
	testAccessToken := "test-access-token-abc"
	testRefreshToken := "test-refresh-token-xyz"

	authorizeHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("authorize: expected GET, got %s", r.Method)
		}

		cookie := r.Header.Get("Cookie")
		if cookie != "npsso=test-npsso-value" {
			t.Errorf("authorize: expected npsso cookie, got %q", cookie)
		}

		q := r.URL.Query()
		if q.Get("client_id") != ClientID {
			t.Errorf("authorize: unexpected client_id %q", q.Get("client_id"))
		}
		if q.Get("response_type") != "code" {
			t.Errorf("authorize: unexpected response_type %q", q.Get("response_type"))
		}

		redirectURL := RedirectURI + "?code=" + testCode
		w.Header().Set("Location", redirectURL)
		w.WriteHeader(http.StatusFound)
	})

	tokenHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("token: expected POST, got %s", r.Method)
		}

		auth := r.Header.Get("Authorization")
		if auth != "Basic "+BasicAuth {
			t.Errorf("token: unexpected Authorization header %q", auth)
		}

		if err := r.ParseForm(); err != nil {
			t.Fatalf("token: failed to parse form: %v", err)
		}

		if r.FormValue("code") != testCode {
			t.Errorf("token: unexpected code %q", r.FormValue("code"))
		}
		if r.FormValue("grant_type") != "authorization_code" {
			t.Errorf("token: unexpected grant_type %q", r.FormValue("grant_type"))
		}

		resp := tokenResponse{
			AccessToken:  testAccessToken,
			RefreshToken: testRefreshToken,
			ExpiresIn:    3600,
			TokenType:    "bearer",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	mux := http.NewServeMux()
	mux.Handle("/authorize", authorizeHandler)
	mux.Handle("/token", tokenHandler)
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newClientWithURLs(server.URL+"/authorize", server.URL+"/token")

	err := client.AuthenticateWithNPSSO("test-npsso-value")
	if err != nil {
		t.Fatalf("AuthenticateWithNPSSO failed: %v", err)
	}

	tokens := client.GetTokens()
	if tokens.AccessToken != testAccessToken {
		t.Errorf("expected access token %q, got %q", testAccessToken, tokens.AccessToken)
	}
	if tokens.RefreshToken != testRefreshToken {
		t.Errorf("expected refresh token %q, got %q", testRefreshToken, tokens.RefreshToken)
	}
	if tokens.AccessTokenExpiresAt.Before(time.Now()) {
		t.Error("access token should not already be expired")
	}
	if tokens.NpssoSetAt.IsZero() {
		t.Error("NpssoSetAt should be set")
	}
}

func TestAuthenticateWithNPSSO_NoCodeInRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", RedirectURI+"?error=access_denied")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	client := newClientWithURLs(server.URL, server.URL+"/token")
	err := client.AuthenticateWithNPSSO("bad-npsso")
	if err == nil {
		t.Fatal("expected error when no code in redirect")
	}
}

func TestRefreshAccessToken(t *testing.T) {
	newAccessToken := "refreshed-access-token"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}

		if r.FormValue("grant_type") != "refresh_token" {
			t.Errorf("expected grant_type=refresh_token, got %q", r.FormValue("grant_type"))
		}
		if r.FormValue("refresh_token") != "existing-refresh-token" {
			t.Errorf("unexpected refresh_token %q", r.FormValue("refresh_token"))
		}

		resp := tokenResponse{
			AccessToken: newAccessToken,
			ExpiresIn:   3600,
			TokenType:   "bearer",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := newClientWithURLs(server.URL+"/authorize", server.URL)
	client.SetTokens(&Tokens{
		AccessToken:          "old-access-token",
		RefreshToken:         "existing-refresh-token",
		AccessTokenExpiresAt: time.Now().Add(-1 * time.Hour), // expired
	})

	err := client.RefreshAccessToken()
	if err != nil {
		t.Fatalf("RefreshAccessToken failed: %v", err)
	}

	tokens := client.GetTokens()
	if tokens.AccessToken != newAccessToken {
		t.Errorf("expected access token %q, got %q", newAccessToken, tokens.AccessToken)
	}
	// Refresh token should remain unchanged since server didn't return a new one
	if tokens.RefreshToken != "existing-refresh-token" {
		t.Errorf("refresh token should not have changed, got %q", tokens.RefreshToken)
	}
}

func TestRefreshAccessToken_NoRefreshToken(t *testing.T) {
	client := NewClient()
	err := client.RefreshAccessToken()
	if err == nil {
		t.Fatal("expected error when no refresh token is set")
	}
}

func TestEnsureValidToken_StillValid(t *testing.T) {
	// Create a server that should NOT be called (token still valid)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("token endpoint should not be called when token is still valid")
	}))
	defer server.Close()

	client := newClientWithURLs(server.URL, server.URL)
	client.SetTokens(&Tokens{
		AccessToken:          "valid-token",
		RefreshToken:         "refresh-token",
		AccessTokenExpiresAt: time.Now().Add(1 * time.Hour),
	})

	err := client.ensureValidToken()
	if err != nil {
		t.Fatalf("ensureValidToken failed: %v", err)
	}
}

func TestEnsureValidToken_Expired(t *testing.T) {
	refreshed := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshed = true
		resp := tokenResponse{
			AccessToken: "new-token",
			ExpiresIn:   3600,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := newClientWithURLs(server.URL, server.URL)
	client.SetTokens(&Tokens{
		AccessToken:          "expired-token",
		RefreshToken:         "refresh-token",
		AccessTokenExpiresAt: time.Now().Add(-1 * time.Hour),
	})

	err := client.ensureValidToken()
	if err != nil {
		t.Fatalf("ensureValidToken failed: %v", err)
	}
	if !refreshed {
		t.Error("expected token refresh to occur")
	}
}

func TestEnsureValidToken_NoToken(t *testing.T) {
	client := NewClient()
	err := client.ensureValidToken()
	if err == nil {
		t.Fatal("expected error when no access token is set")
	}
}

func TestTokenDaysUntilRefreshExpiry(t *testing.T) {
	tests := []struct {
		name     string
		expiry   time.Time
		expected int
	}{
		{"30 days", time.Now().Add(30 * 24 * time.Hour), 29},
		{"1 day", time.Now().Add(1*24*time.Hour + 12*time.Hour), 1},
		{"expired", time.Now().Add(-1*24*time.Hour + 12*time.Hour), -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := &Tokens{RefreshTokenExpiresAt: tt.expiry}
			days := tokens.DaysUntilRefreshExpiry()
			if days != tt.expected {
				t.Errorf("expected %d days, got %d", tt.expected, days)
			}
		})
	}
}

func TestNeedsReminder(t *testing.T) {
	tests := []struct {
		name         string
		daysUntil    int
		expectRemind bool
	}{
		{"14 days", 14, true},
		{"7 days", 7, true},
		{"3 days", 3, true},
		{"1 day", 1, true},
		{"0 days", 0, true},
		{"15 days", 15, false},
		{"10 days", 10, false},
		{"5 days", 5, false},
		{"2 days", 2, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set expiry to exactly N days + a small buffer from now
			tokens := &Tokens{
				RefreshTokenExpiresAt: time.Now().Add(time.Duration(tt.daysUntil)*24*time.Hour + 12*time.Hour),
			}
			remind, msg := tokens.NeedsReminder()
			if remind != tt.expectRemind {
				t.Errorf("NeedsReminder() at %d days: got remind=%v, want %v (msg=%q)", tt.daysUntil, remind, tt.expectRemind, msg)
			}
			if tt.expectRemind && msg == "" {
				t.Error("expected non-empty message when reminder is needed")
			}
			if !tt.expectRemind && msg != "" {
				t.Errorf("expected empty message when no reminder, got %q", msg)
			}
		})
	}
}

func TestGetAuthCode_ExtractsCodeFromRedirect(t *testing.T) {
	expectedCode := "v3.abc123"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectURL := RedirectURI + "?code=" + url.QueryEscape(expectedCode)
		w.Header().Set("Location", redirectURL)
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	client := newClientWithURLs(server.URL, server.URL+"/token")
	code, err := client.getAuthCode("test-npsso")
	if err != nil {
		t.Fatalf("getAuthCode failed: %v", err)
	}
	if code != expectedCode {
		t.Errorf("expected code %q, got %q", expectedCode, code)
	}
}
