package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/rourkem/gt7leaderboard/local-service/internal/api"
	"github.com/rourkem/gt7leaderboard/local-service/internal/cardb"
	"github.com/rourkem/gt7leaderboard/local-service/internal/config"
	"github.com/rourkem/gt7leaderboard/local-service/internal/discord"
	"github.com/rourkem/gt7leaderboard/local-service/internal/metrics"
	"github.com/rourkem/gt7leaderboard/local-service/internal/psn"
	"github.com/rourkem/gt7leaderboard/local-service/internal/refresh"
	"github.com/rourkem/gt7leaderboard/local-service/internal/session"
	"github.com/rourkem/gt7leaderboard/local-service/internal/telemetry"
	"github.com/rourkem/gt7leaderboard/local-service/internal/trackdetect"
	"github.com/rourkem/gt7leaderboard/local-service/internal/webui"
)

const (
	heartbeatInterval = 60 * time.Second
	idleCheckInterval = 5 * time.Second
	defaultWebUIAddr  = ":8081"
)

func main() {
	// 1. Parse flags.
	configPath := flag.String("config", "./config.yaml", "Path to configuration file")
	flag.Parse()

	// 2. Load config.
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Configuration loaded from %s", *configPath)

	// 3. Initialize metrics.
	var m metrics.Metrics
	if cfg.Datadog.Enabled {
		ddAddr := "localhost:8125"
		m, err = metrics.New(ddAddr,
			metrics.WithNamespace("gt7."),
			metrics.WithTags([]string{"service:collector"}),
		)
		if err != nil {
			log.Printf("Warning: failed to initialize Datadog metrics: %v", err)
			m = metrics.NewNoop()
		}
	} else {
		m = metrics.NewNoop()
	}
	defer m.Close()

	// 4. Load car database.
	carCacheDir := filepath.Join(filepath.Dir(*configPath), "data", "cars", "raw")
	var carDB *cardb.Database
	if _, err := os.Stat(filepath.Join(carCacheDir, "cars.csv")); err == nil {
		carDB, err = cardb.LoadFromDir(carCacheDir)
		if err != nil {
			log.Printf("Warning: failed to load car database from %s: %v", carCacheDir, err)
		} else {
			log.Printf("Loaded car database: %d cars", carDB.Count())
		}
	} else {
		log.Printf("No car database found at %s, will be populated on first refresh", carCacheDir)
	}

	// 5. Load track references.
	trackDataDir := filepath.Join(filepath.Dir(*configPath), "data", "tracks")
	var tracks []*trackdetect.TrackReference
	if _, err := os.Stat(trackDataDir); err == nil {
		tracks, err = trackdetect.LoadAllTracks(trackDataDir)
		if err != nil {
			log.Printf("Warning: failed to load track data from %s: %v", trackDataDir, err)
		} else {
			log.Printf("Loaded track references: %d tracks", len(tracks))
		}
	} else {
		log.Printf("No track data found at %s, will be populated on first refresh", trackDataDir)
	}

	// 6. Initialize PSN client and restore tokens if available.
	psnClient := psn.NewClient()
	// PSN accounts from config.
	psnAccounts := make([]psn.AccountConfig, len(cfg.PSN.Accounts))
	for i, a := range cfg.PSN.Accounts {
		psnAccounts[i] = psn.AccountConfig{
			OnlineID:   a.OnlineID,
			DriverName: a.DriverName,
		}
	}

	if cfg.PSN.NpssoToken != "" {
		if err := psnClient.AuthenticateWithNPSSO(cfg.PSN.NpssoToken); err != nil {
			log.Printf("Warning: PSN authentication failed: %v", err)
		} else {
			if tokens := psnClient.GetTokens(); tokens != nil {
				log.Printf("PSN authenticated, refresh token expires in %d days", tokens.DaysUntilRefreshExpiry())
			}

			// Resolve online IDs to numeric account IDs for presence lookups.
			for i, a := range psnAccounts {
				if a.AccountID == "" && a.OnlineID != "" {
					accountID, err := psnClient.ResolveOnlineID(a.OnlineID)
					if err != nil {
						log.Printf("Warning: could not resolve account ID for %s: %v", a.OnlineID, err)
					} else {
						psnAccounts[i].AccountID = accountID
						log.Printf("Resolved %s -> account ID %s", a.OnlineID, accountID)
					}
				}
			}
		}
	} else {
		log.Printf("No PSN NPSSO token configured, driver detection will be unavailable")
	}

	// 7. Initialize API client.
	apiClient := api.NewClient(cfg.API.Endpoint, cfg.API.APIKey)

	// 8. Initialize Discord client.
	var discordClient *discord.Client
	var notifier session.NotificationSender
	if cfg.Discord.WebhookURL != "" {
		discordClient = discord.NewClient(cfg.Discord.WebhookURL)
		notifier = discordClient
		log.Printf("Discord notifications enabled")
	}

	// 9. Create track detector.
	trackDetector := trackdetect.NewDetector(tracks, trackdetect.DefaultConfig())

	// 10. Create session manager.
	idleTimeout := time.Duration(cfg.Session.IdleTimeoutSeconds) * time.Second
	sessionMgr := session.NewManager(
		apiClient,
		psnClient,
		trackDetector,
		notifier,
		carDB,
		m,
		psnAccounts,
		cfg.Discord,
		idleTimeout,
	)

	// Set up graceful shutdown.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// 11. Start data refresh scheduler.
	refresher := refresh.NewRefresher(cfg.DataRefresh, carCacheDir, carDB, apiClient, m)
	carInterval := time.Duration(cfg.DataRefresh.CarRefreshIntervalHours) * time.Hour
	trackInterval := time.Duration(cfg.DataRefresh.TrackRefreshIntervalHours) * time.Hour
	if carInterval <= 0 {
		carInterval = 24 * time.Hour
	}
	if trackInterval <= 0 {
		trackInterval = 168 * time.Hour
	}
	go refresher.StartScheduler(ctx, carInterval, trackInterval, trackDataDir)

	// 12. Start local web UI.
	webServer := webui.NewServer(defaultWebUIAddr, cfg, psnClient, sessionMgr, *configPath)
	go func() {
		if err := webServer.Start(ctx); err != nil {
			log.Printf("Web UI error: %v", err)
		}
	}()

	// 13. Start heartbeat sender.
	startTime := time.Now()
	sendHeartbeat := func() {
		hbReq := api.HeartbeatRequest{
			Status:        "running",
			UptimeSeconds: int(time.Since(startTime).Seconds()),
		}
		if sess := sessionMgr.CurrentSession(); sess != nil {
			hbReq.CurrentSessionID = sess.ID
		}
		if err := apiClient.SendHeartbeat(hbReq); err != nil {
			log.Printf("Heartbeat error: %v", err)
		}
	}
	go func() {
		sendHeartbeat() // Send immediately on startup.
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		var lastReminderDay int
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sendHeartbeat()
				// Check PSN token expiry once per day and send Discord reminder.
				today := time.Now().YearDay()
				if today != lastReminderDay {
					if tokens := psnClient.GetTokens(); tokens != nil && !tokens.RefreshTokenExpiresAt.IsZero() {
						if needs, msg := tokens.NeedsReminder(); needs {
							log.Printf("WARNING: %s", msg)
							if discordClient != nil {
								if err := discordClient.SendMessage("⚠️ " + msg); err != nil {
									log.Printf("Discord reminder error: %v", err)
								}
							}
							lastReminderDay = today
						}
					}
				}
			}
		}
	}()

	// 14. Start idle checker.
	go func() {
		ticker := time.NewTicker(idleCheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sessionMgr.CheckIdle()
			}
		}
	}()

	// 15. Create telemetry listener with session manager's HandlePacket as handler.
	listener := telemetry.NewListener(
		cfg.PlayStation.IP,
		cfg.PlayStation.SendPort,
		cfg.PlayStation.ListenPort,
		sessionMgr.HandlePacket,
	)

	log.Printf("GT7 Collector starting...")
	if cfg.PlayStation.IP != "" {
		log.Printf("  PlayStation IP: %s", cfg.PlayStation.IP)
	} else {
		log.Printf("  PlayStation IP: broadcast auto-discovery")
	}
	log.Printf("  Send port: %d, Listen port: %d", cfg.PlayStation.SendPort, cfg.PlayStation.ListenPort)
	log.Printf("  API endpoint: %s", cfg.API.Endpoint)
	log.Printf("  Web UI: %s", defaultWebUIAddr)

	// 16. Run telemetry listener (blocking).
	if err := listener.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatalf("Telemetry listener error: %v", err)
	}

	// 17. Graceful shutdown.
	log.Printf("GT7 Collector shutting down...")
}
