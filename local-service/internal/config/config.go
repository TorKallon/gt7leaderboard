package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config is the top-level configuration for the GT7 leaderboard local service.
type Config struct {
	PlayStation PlayStationConfig `mapstructure:"playstation"`
	PSN         PSNConfig         `mapstructure:"psn"`
	API         APIConfig         `mapstructure:"api"`
	Discord     DiscordConfig     `mapstructure:"discord"`
	Datadog     DatadogConfig     `mapstructure:"datadog"`
	Session     SessionConfig     `mapstructure:"session"`
	DataRefresh DataRefreshConfig `mapstructure:"data_refresh"`
}

// PlayStationConfig holds settings for connecting to the PlayStation console.
type PlayStationConfig struct {
	IP         string `mapstructure:"ip"`
	SendPort   int    `mapstructure:"send_port"`
	ListenPort int    `mapstructure:"listen_port"`
}

// PSNAccount represents a single PSN account to monitor.
type PSNAccount struct {
	OnlineID   string `mapstructure:"online_id"`
	DriverName string `mapstructure:"driver_name"`
}

// PSNConfig holds PSN authentication and account settings.
type PSNConfig struct {
	NpssoToken string       `mapstructure:"npsso_token"`
	Accounts   []PSNAccount `mapstructure:"accounts"`
}

// APIConfig holds settings for the hosted API endpoint.
type APIConfig struct {
	Endpoint string `mapstructure:"endpoint"`
	APIKey   string `mapstructure:"api_key"`
}

// DiscordConfig holds settings for Discord webhook notifications.
type DiscordConfig struct {
	WebhookURL             string `mapstructure:"webhook_url"`
	NotifyOverallRecords   bool   `mapstructure:"notify_overall_records"`
	NotifyCategoryRecords  bool   `mapstructure:"notify_category_records"`
	NotifyCarRecords       bool   `mapstructure:"notify_car_records"`
}

// DatadogConfig holds settings for Datadog metrics reporting.
type DatadogConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	APIKey  string `mapstructure:"api_key"`
	Site    string `mapstructure:"site"`
	Service string `mapstructure:"service"`
	Env     string `mapstructure:"env"`
}

// SessionConfig holds settings for telemetry session management.
type SessionConfig struct {
	IdleTimeoutSeconds       int `mapstructure:"idle_timeout_seconds"`
	TrackDetectionMinPoints  int `mapstructure:"track_detection_min_points"`
}

// DataRefreshConfig holds settings for periodic car and track data refreshes.
type DataRefreshConfig struct {
	CarDataURL                string `mapstructure:"car_data_url"`
	CarRefreshIntervalHours   int    `mapstructure:"car_refresh_interval_hours"`
	TrackDataRepo             string `mapstructure:"track_data_repo"`
	TrackRefreshIntervalHours int    `mapstructure:"track_refresh_interval_hours"`
}

// Load reads configuration from the given file path, applying defaults for
// optional values. The path should include the directory; the filename and
// extension are parsed automatically.
func Load(path string) (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("playstation.send_port", 33739)
	v.SetDefault("playstation.listen_port", 33740)
	v.SetDefault("discord.notify_overall_records", true)
	v.SetDefault("discord.notify_category_records", true)
	v.SetDefault("discord.notify_car_records", false)
	v.SetDefault("session.idle_timeout_seconds", 30)
	v.SetDefault("session.track_detection_min_points", 300)
	v.SetDefault("data_refresh.car_refresh_interval_hours", 24)
	v.SetDefault("data_refresh.track_refresh_interval_hours", 168)
	v.SetDefault("datadog.enabled", false)

	v.SetConfigFile(path)

	// Allow environment variable overrides with GT7_ prefix
	v.SetEnvPrefix("GT7")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	return &cfg, nil
}
