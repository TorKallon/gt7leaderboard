package psn

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestClientWithToken(server *httptest.Server) *Client {
	client := newClientWithURLs(server.URL+"/authorize", server.URL+"/token")
	client.SetTokens(&Tokens{
		AccessToken:          "test-access-token",
		RefreshToken:         "test-refresh-token",
		AccessTokenExpiresAt: time.Now().Add(1 * time.Hour),
	})
	return client
}

func TestGetPresence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}

		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-access-token" {
			t.Errorf("unexpected Authorization: %q", auth)
		}

		// Verify the URL format: /{accountId}/basicPresences?type=primary
		if !strings.Contains(r.URL.Path, "/1234567890/basicPresences") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("type") != "primary" {
			t.Errorf("expected type=primary, got %q", r.URL.Query().Get("type"))
		}

		resp := BasicPresence{
			Availability: "availableToPlay",
			GameTitleInfoList: []GameTitleInfo{
				{NpTitleID: GT7TitlePS5, TitleName: "Gran Turismo 7", Format: "PS5"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	oldURL := presenceBaseURL
	presenceBaseURL = server.URL
	defer func() { presenceBaseURL = oldURL }()

	client := newTestClientWithToken(server)

	presence, err := client.GetPresence("1234567890")
	if err != nil {
		t.Fatalf("GetPresence failed: %v", err)
	}

	if presence.AccountID != "1234567890" {
		t.Errorf("expected account ID %q, got %q", "1234567890", presence.AccountID)
	}
	if !presence.IsPlayingGT7() {
		t.Error("user should be playing GT7")
	}
}

func TestIdentifyDriver_OnePlayingGT7(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Respond per-account based on URL path.
		var resp BasicPresence
		if strings.Contains(r.URL.Path, "/111/") {
			resp = BasicPresence{
				Availability: "availableToPlay",
				GameTitleInfoList: []GameTitleInfo{
					{NpTitleID: "CUSA00001_00", TitleName: "Other Game"},
				},
			}
		} else if strings.Contains(r.URL.Path, "/222/") {
			resp = BasicPresence{
				Availability: "availableToPlay",
				GameTitleInfoList: []GameTitleInfo{
					{NpTitleID: GT7TitlePS5, TitleName: "Gran Turismo 7", Format: "PS5"},
				},
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	oldURL := presenceBaseURL
	presenceBaseURL = server.URL
	defer func() { presenceBaseURL = oldURL }()

	client := newTestClientWithToken(server)

	accounts := []AccountConfig{
		{OnlineID: "player1", AccountID: "111", DriverName: "Alice"},
		{OnlineID: "player2", AccountID: "222", DriverName: "Bob"},
	}

	accountID, driverName, err := client.IdentifyDriver(accounts)
	if err != nil {
		t.Fatalf("IdentifyDriver failed: %v", err)
	}

	if accountID != "222" {
		t.Errorf("expected account ID %q, got %q", "222", accountID)
	}
	if driverName != "Bob" {
		t.Errorf("expected driver name %q, got %q", "Bob", driverName)
	}
}

func TestIdentifyDriver_NonePlayingGT7(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := BasicPresence{
			Availability: "availableToPlay",
			GameTitleInfoList: []GameTitleInfo{
				{NpTitleID: "CUSA00001_00", TitleName: "Other Game"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	oldURL := presenceBaseURL
	presenceBaseURL = server.URL
	defer func() { presenceBaseURL = oldURL }()

	client := newTestClientWithToken(server)

	accounts := []AccountConfig{
		{OnlineID: "player1", AccountID: "111", DriverName: "Alice"},
	}

	accountID, driverName, err := client.IdentifyDriver(accounts)
	if err != nil {
		t.Fatalf("IdentifyDriver failed: %v", err)
	}

	if accountID != "" {
		t.Errorf("expected empty account ID, got %q", accountID)
	}
	if driverName != "" {
		t.Errorf("expected empty driver name, got %q", driverName)
	}
}

func TestIdentifyDriver_MultiplePlayingGT7(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := BasicPresence{
			Availability: "availableToPlay",
			GameTitleInfoList: []GameTitleInfo{
				{NpTitleID: GT7TitlePS5, TitleName: "Gran Turismo 7"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	oldURL := presenceBaseURL
	presenceBaseURL = server.URL
	defer func() { presenceBaseURL = oldURL }()

	client := newTestClientWithToken(server)

	accounts := []AccountConfig{
		{OnlineID: "player1", AccountID: "111", DriverName: "Alice"},
		{OnlineID: "player2", AccountID: "222", DriverName: "Bob"},
	}

	accountID, driverName, err := client.IdentifyDriver(accounts)
	if err != nil {
		t.Fatalf("IdentifyDriver failed: %v", err)
	}

	if accountID != "" {
		t.Errorf("expected empty account ID when multiple playing, got %q", accountID)
	}
	if driverName != "" {
		t.Errorf("expected empty driver name when multiple playing, got %q", driverName)
	}
}

func TestResolveOnlineID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}

		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-access-token" {
			t.Errorf("unexpected Authorization: %q", auth)
		}

		resp := profileResponse{}
		resp.Profile.AccountID = "9876543210"
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	oldURL := profileBaseURL
	profileBaseURL = server.URL
	defer func() { profileBaseURL = oldURL }()

	client := newTestClientWithToken(server)

	accountID, err := client.ResolveOnlineID("testplayer")
	if err != nil {
		t.Fatalf("ResolveOnlineID failed: %v", err)
	}

	if accountID != "9876543210" {
		t.Errorf("expected account ID %q, got %q", "9876543210", accountID)
	}
}

func TestIsPlayingGT7(t *testing.T) {
	tests := []struct {
		name     string
		presence BasicPresence
		expected bool
	}{
		{
			name: "PS5 title ID",
			presence: BasicPresence{
				GameTitleInfoList: []GameTitleInfo{{NpTitleID: GT7TitlePS5}},
			},
			expected: true,
		},
		{
			name: "PS4 title ID",
			presence: BasicPresence{
				GameTitleInfoList: []GameTitleInfo{{NpTitleID: GT7TitlePS4}},
			},
			expected: true,
		},
		{
			name: "title name match case insensitive",
			presence: BasicPresence{
				GameTitleInfoList: []GameTitleInfo{{NpTitleID: "UNKNOWN", TitleName: "GRAN TURISMO 7 - Digital Edition"}},
			},
			expected: true,
		},
		{
			name: "no GT7",
			presence: BasicPresence{
				GameTitleInfoList: []GameTitleInfo{{NpTitleID: "CUSA00001_00", TitleName: "Forza Motorsport"}},
			},
			expected: false,
		},
		{
			name: "empty game list",
			presence: BasicPresence{
				GameTitleInfoList: nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.presence.IsPlayingGT7()
			if got != tt.expected {
				t.Errorf("IsPlayingGT7() = %v, want %v", got, tt.expected)
			}
		})
	}
}
