package psn

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestGetBulkPresence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}

		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-access-token" {
			t.Errorf("unexpected Authorization: %q", auth)
		}

		ids := r.URL.Query().Get("accountIds")
		if ids == "" {
			t.Error("expected accountIds query parameter")
		}

		resp := PresenceResponse{
			BasicPresences: []BasicPresence{
				{
					AccountID:    "1234567890",
					Availability: "availableToPlay",
					GameTitleInfoList: []GameTitleInfo{
						{NpTitleID: GT7TitlePS5, TitleName: "Gran Turismo 7", Format: "PS5"},
					},
				},
				{
					AccountID:    "9876543210",
					Availability: "availableToPlay",
					GameTitleInfoList: []GameTitleInfo{
						{NpTitleID: "CUSA00001_00", TitleName: "Some Other Game", Format: "PS5"},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Override the base URL for testing
	oldURL := presenceBaseURL
	presenceBaseURL = server.URL
	defer func() { presenceBaseURL = oldURL }()

	client := newTestClientWithToken(server)

	presences, err := client.GetBulkPresence([]string{"1234567890", "9876543210"})
	if err != nil {
		t.Fatalf("GetBulkPresence failed: %v", err)
	}

	if len(presences) != 2 {
		t.Fatalf("expected 2 presences, got %d", len(presences))
	}

	if presences[0].AccountID != "1234567890" {
		t.Errorf("expected account ID %q, got %q", "1234567890", presences[0].AccountID)
	}
	if !presences[0].IsPlayingGT7() {
		t.Error("first user should be playing GT7")
	}
	if presences[1].IsPlayingGT7() {
		t.Error("second user should not be playing GT7")
	}
}

func TestIdentifyDriver_OnePlayingGT7(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := PresenceResponse{
			BasicPresences: []BasicPresence{
				{
					AccountID:    "111",
					Availability: "availableToPlay",
					GameTitleInfoList: []GameTitleInfo{
						{NpTitleID: "CUSA00001_00", TitleName: "Other Game"},
					},
				},
				{
					AccountID:    "222",
					Availability: "availableToPlay",
					GameTitleInfoList: []GameTitleInfo{
						{NpTitleID: GT7TitlePS5, TitleName: "Gran Turismo 7", Format: "PS5"},
					},
				},
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

	if accountID != "222" {
		t.Errorf("expected account ID %q, got %q", "222", accountID)
	}
	if driverName != "Bob" {
		t.Errorf("expected driver name %q, got %q", "Bob", driverName)
	}
}

func TestIdentifyDriver_NonePlayingGT7(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := PresenceResponse{
			BasicPresences: []BasicPresence{
				{
					AccountID:    "111",
					Availability: "availableToPlay",
					GameTitleInfoList: []GameTitleInfo{
						{NpTitleID: "CUSA00001_00", TitleName: "Other Game"},
					},
				},
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
		resp := PresenceResponse{
			BasicPresences: []BasicPresence{
				{
					AccountID:         "111",
					Availability:      "availableToPlay",
					GameTitleInfoList:  []GameTitleInfo{{NpTitleID: GT7TitlePS5, TitleName: "Gran Turismo 7"}},
				},
				{
					AccountID:         "222",
					Availability:      "availableToPlay",
					GameTitleInfoList:  []GameTitleInfo{{NpTitleID: GT7TitlePS4, TitleName: "Gran Turismo 7"}},
				},
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
