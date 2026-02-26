package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	yamlContent := `
playstation:
  ip: "192.168.1.100"
  send_port: 33739
  listen_port: 33740

psn:
  npsso_token: "test-npsso-token"
  accounts:
    - online_id: "player1"
      driver_name: "Alice"
    - online_id: "player2"
      driver_name: "Bob"

api:
  endpoint: "https://api.example.com"
  api_key: "test-api-key"

discord:
  webhook_url: "https://discord.com/api/webhooks/123/abc"

datadog:
  enabled: true
  api_key: "dd-api-key"
  site: "datadoghq.com"
  service: "gt7-leaderboard"
  env: "production"

session:
  idle_timeout_seconds: 45

data_refresh:
  car_data_url: "https://example.com/cars.csv"
  track_data_repo: "owner/repo"
`

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// PlayStation
	if cfg.PlayStation.IP != "192.168.1.100" {
		t.Errorf("PlayStation.IP = %q, want %q", cfg.PlayStation.IP, "192.168.1.100")
	}
	if cfg.PlayStation.SendPort != 33739 {
		t.Errorf("PlayStation.SendPort = %d, want %d", cfg.PlayStation.SendPort, 33739)
	}
	if cfg.PlayStation.ListenPort != 33740 {
		t.Errorf("PlayStation.ListenPort = %d, want %d", cfg.PlayStation.ListenPort, 33740)
	}

	// PSN
	if cfg.PSN.NpssoToken != "test-npsso-token" {
		t.Errorf("PSN.NpssoToken = %q, want %q", cfg.PSN.NpssoToken, "test-npsso-token")
	}
	if len(cfg.PSN.Accounts) != 2 {
		t.Fatalf("PSN.Accounts length = %d, want 2", len(cfg.PSN.Accounts))
	}
	if cfg.PSN.Accounts[0].OnlineID != "player1" {
		t.Errorf("PSN.Accounts[0].OnlineID = %q, want %q", cfg.PSN.Accounts[0].OnlineID, "player1")
	}
	if cfg.PSN.Accounts[1].DriverName != "Bob" {
		t.Errorf("PSN.Accounts[1].DriverName = %q, want %q", cfg.PSN.Accounts[1].DriverName, "Bob")
	}

	// API
	if cfg.API.Endpoint != "https://api.example.com" {
		t.Errorf("API.Endpoint = %q, want %q", cfg.API.Endpoint, "https://api.example.com")
	}
	if cfg.API.APIKey != "test-api-key" {
		t.Errorf("API.APIKey = %q, want %q", cfg.API.APIKey, "test-api-key")
	}

	// Discord - explicit + defaults
	if cfg.Discord.WebhookURL != "https://discord.com/api/webhooks/123/abc" {
		t.Errorf("Discord.WebhookURL = %q, want webhook URL", cfg.Discord.WebhookURL)
	}
	if !cfg.Discord.NotifyOverallRecords {
		t.Error("Discord.NotifyOverallRecords should default to true")
	}
	if !cfg.Discord.NotifyCategoryRecords {
		t.Error("Discord.NotifyCategoryRecords should default to true")
	}
	if cfg.Discord.NotifyCarRecords {
		t.Error("Discord.NotifyCarRecords should default to false")
	}

	// Datadog
	if !cfg.Datadog.Enabled {
		t.Error("Datadog.Enabled should be true")
	}
	if cfg.Datadog.APIKey != "dd-api-key" {
		t.Errorf("Datadog.APIKey = %q, want %q", cfg.Datadog.APIKey, "dd-api-key")
	}
	if cfg.Datadog.Service != "gt7-leaderboard" {
		t.Errorf("Datadog.Service = %q, want %q", cfg.Datadog.Service, "gt7-leaderboard")
	}

	// Session - explicit override
	if cfg.Session.IdleTimeoutSeconds != 45 {
		t.Errorf("Session.IdleTimeoutSeconds = %d, want 45", cfg.Session.IdleTimeoutSeconds)
	}
	// Session - default value
	if cfg.Session.TrackDetectionMinPoints != 300 {
		t.Errorf("Session.TrackDetectionMinPoints = %d, want 300 (default)", cfg.Session.TrackDetectionMinPoints)
	}

	// DataRefresh - explicit + defaults
	if cfg.DataRefresh.CarDataURL != "https://example.com/cars.csv" {
		t.Errorf("DataRefresh.CarDataURL = %q", cfg.DataRefresh.CarDataURL)
	}
	if cfg.DataRefresh.CarRefreshIntervalHours != 24 {
		t.Errorf("DataRefresh.CarRefreshIntervalHours = %d, want 24 (default)", cfg.DataRefresh.CarRefreshIntervalHours)
	}
	if cfg.DataRefresh.TrackDataRepo != "owner/repo" {
		t.Errorf("DataRefresh.TrackDataRepo = %q", cfg.DataRefresh.TrackDataRepo)
	}
	if cfg.DataRefresh.TrackRefreshIntervalHours != 168 {
		t.Errorf("DataRefresh.TrackRefreshIntervalHours = %d, want 168 (default)", cfg.DataRefresh.TrackRefreshIntervalHours)
	}
}

func TestLoadDefaults(t *testing.T) {
	// Minimal config file - test that all defaults are applied
	yamlContent := `
playstation:
  ip: "10.0.0.1"
`

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.PlayStation.SendPort != 33739 {
		t.Errorf("default SendPort = %d, want 33739", cfg.PlayStation.SendPort)
	}
	if cfg.PlayStation.ListenPort != 33740 {
		t.Errorf("default ListenPort = %d, want 33740", cfg.PlayStation.ListenPort)
	}
	if cfg.Session.IdleTimeoutSeconds != 30 {
		t.Errorf("default IdleTimeoutSeconds = %d, want 30", cfg.Session.IdleTimeoutSeconds)
	}
	if cfg.Session.TrackDetectionMinPoints != 300 {
		t.Errorf("default TrackDetectionMinPoints = %d, want 300", cfg.Session.TrackDetectionMinPoints)
	}
	if !cfg.Discord.NotifyOverallRecords {
		t.Error("default NotifyOverallRecords should be true")
	}
	if !cfg.Discord.NotifyCategoryRecords {
		t.Error("default NotifyCategoryRecords should be true")
	}
	if cfg.Discord.NotifyCarRecords {
		t.Error("default NotifyCarRecords should be false")
	}
	if cfg.Datadog.Enabled {
		t.Error("default Datadog.Enabled should be false")
	}
	if cfg.DataRefresh.CarRefreshIntervalHours != 24 {
		t.Errorf("default CarRefreshIntervalHours = %d, want 24", cfg.DataRefresh.CarRefreshIntervalHours)
	}
	if cfg.DataRefresh.TrackRefreshIntervalHours != 168 {
		t.Errorf("default TrackRefreshIntervalHours = %d, want 168", cfg.DataRefresh.TrackRefreshIntervalHours)
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent config file")
	}
}
