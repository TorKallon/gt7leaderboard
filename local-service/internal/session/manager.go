package session

import (
	"log"
	"sync"
	"time"

	"github.com/rourkem/gt7leaderboard/local-service/internal/api"
	"github.com/rourkem/gt7leaderboard/local-service/internal/cardb"
	"github.com/rourkem/gt7leaderboard/local-service/internal/config"
	"github.com/rourkem/gt7leaderboard/local-service/internal/discord"
	"github.com/rourkem/gt7leaderboard/local-service/internal/metrics"
	"github.com/rourkem/gt7leaderboard/local-service/internal/psn"
	"github.com/rourkem/gt7leaderboard/local-service/internal/telemetry"
	"github.com/rourkem/gt7leaderboard/local-service/internal/trackdetect"
)

// APIClient is the interface for the hosted API.
type APIClient interface {
	CreateSession(api.CreateSessionRequest) (*api.CreateSessionResponse, error)
	RecordLap(api.RecordLapRequest) (*api.RecordLapResponse, error)
	EndSession(string, time.Time) error
}

// DriverDetector identifies which PSN account is currently driving.
type DriverDetector interface {
	IdentifyDriver(accounts []psn.AccountConfig) (accountID, driverName string, err error)
}

// TrackDetector processes telemetry packets to identify the current track.
type TrackDetector interface {
	AddPoint(pkt *telemetry.Packet) *trackdetect.DetectionResult
	Reset()
}

// NotificationSender sends record notifications to Discord.
type NotificationSender interface {
	SendRecordNotification(discord.RecordNotification) error
}

// ActiveSession represents the state of an in-progress driving session.
type ActiveSession struct {
	ID              string
	DriverID        string
	DriverName      string
	TrackSlug       string
	TrackName       string
	CarID           int32
	LastLap         int16
	StartedAt       time.Time
	DetectionMethod string
	FirstLapSkipped bool
}

// Manager orchestrates telemetry sessions. It receives packets and manages
// driver detection, track detection, lap recording, and session lifecycle.
type Manager struct {
	api           APIClient
	driver        DriverDetector
	track         TrackDetector
	notifier      NotificationSender
	carDB         *cardb.Database
	metrics       metrics.Metrics
	psnAccounts   []psn.AccountConfig
	discordConfig config.DiscordConfig

	mu             sync.Mutex
	currentSession *ActiveSession
	lastPacketTime time.Time
	idleTimeout    time.Duration
}

// NewManager creates a new session Manager.
func NewManager(
	apiClient APIClient,
	driverDetector DriverDetector,
	trackDetector TrackDetector,
	notifier NotificationSender,
	carDB *cardb.Database,
	m metrics.Metrics,
	psnAccounts []psn.AccountConfig,
	discordCfg config.DiscordConfig,
	idleTimeout time.Duration,
) *Manager {
	return &Manager{
		api:           apiClient,
		driver:        driverDetector,
		track:         trackDetector,
		notifier:      notifier,
		carDB:         carDB,
		metrics:       m,
		psnAccounts:   psnAccounts,
		discordConfig: discordCfg,
		idleTimeout:   idleTimeout,
	}
}

// CurrentSession returns a copy of the current active session, or nil if none.
func (m *Manager) CurrentSession() *ActiveSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.currentSession == nil {
		return nil
	}
	s := *m.currentSession
	return &s
}

// HandlePacket processes a single telemetry packet.
func (m *Manager) HandlePacket(pkt *telemetry.Packet) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// Check idle timeout: if we have a session and haven't received a packet
	// for longer than the idle timeout, end the current session.
	if m.currentSession != nil && !m.lastPacketTime.IsZero() &&
		now.Sub(m.lastPacketTime) > m.idleTimeout {
		m.endSessionLocked()
	}

	// If no current session, start a new one.
	if m.currentSession == nil {
		m.startSessionLocked(pkt, now)
	} else if pkt.CarID != m.currentSession.CarID {
		// Car changed: end current session and start a new one.
		m.endSessionLocked()
		m.startSessionLocked(pkt, now)
	}

	// Feed packet to track detector.
	if m.currentSession != nil && m.currentSession.TrackSlug == "" {
		if result := m.track.AddPoint(pkt); result != nil {
			m.currentSession.TrackSlug = result.Track.Info.Slug
			m.currentSession.TrackName = result.Track.Info.Name
			if result.Track.Info.Layout != "" {
				m.currentSession.TrackName += " - " + result.Track.Info.Layout
			}
			if result.IsReverse {
				m.currentSession.DetectionMethod = "reverse"
			} else {
				m.currentSession.DetectionMethod = "forward"
			}
			log.Printf("Track detected: %s (slug: %s)", m.currentSession.TrackName, m.currentSession.TrackSlug)
		}
	}

	// Check for lap completion.
	if m.currentSession != nil && pkt.CurrentLap > m.currentSession.LastLap && m.currentSession.LastLap > 0 {
		m.handleLapCompletionLocked(pkt, now)
	}

	// Update lap counter and last packet time.
	if m.currentSession != nil {
		m.currentSession.LastLap = pkt.CurrentLap
	}
	m.lastPacketTime = now
}

// CheckIdle checks whether the current session has been idle too long and ends it if so.
// This is intended to be called periodically from a ticker.
func (m *Manager) CheckIdle() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentSession == nil {
		return
	}
	if m.lastPacketTime.IsZero() {
		return
	}
	if time.Since(m.lastPacketTime) > m.idleTimeout {
		log.Printf("Session idle for %v, ending session %s", m.idleTimeout, m.currentSession.ID)
		m.endSessionLocked()
	}
}

// startSessionLocked creates a new session. Must be called with mu held.
func (m *Manager) startSessionLocked(pkt *telemetry.Packet, now time.Time) {
	// Detect driver via PSN presence.
	var driverID, driverName string
	if m.driver != nil && len(m.psnAccounts) > 0 {
		var err error
		driverID, driverName, err = m.driver.IdentifyDriver(m.psnAccounts)
		if err != nil {
			log.Printf("Warning: failed to identify driver: %v", err)
		}
	}

	// Look up car name for logging.
	carName := "Unknown"
	if m.carDB != nil {
		if car, ok := m.carDB.Lookup(int(pkt.CarID)); ok {
			carName = car.Name
		}
	}

	// Create session via API.
	req := api.CreateSessionRequest{
		DriverID:        driverID,
		CarID:           int(pkt.CarID),
		StartedAt:       now.UTC().Format(time.RFC3339),
		DetectionMethod: "telemetry",
	}

	resp, err := m.api.CreateSession(req)
	if err != nil {
		log.Printf("Error creating session: %v", err)
		return
	}

	m.track.Reset()

	m.currentSession = &ActiveSession{
		ID:         resp.SessionID,
		DriverID:   driverID,
		DriverName: driverName,
		CarID:      pkt.CarID,
		LastLap:    pkt.CurrentLap,
		StartedAt:  now,
	}

	m.metrics.Incr("session.started", nil)
	log.Printf("Session started: id=%s driver=%s car=%s(%d)", resp.SessionID, driverName, carName, pkt.CarID)
}

// endSessionLocked ends the current session. Must be called with mu held.
func (m *Manager) endSessionLocked() {
	if m.currentSession == nil {
		return
	}
	sessionID := m.currentSession.ID
	if err := m.api.EndSession(sessionID, time.Now()); err != nil {
		log.Printf("Error ending session %s: %v", sessionID, err)
	}
	m.metrics.Incr("session.ended", nil)
	log.Printf("Session ended: id=%s", sessionID)
	m.currentSession = nil
}

// handleLapCompletionLocked processes a completed lap. Must be called with mu held.
func (m *Manager) handleLapCompletionLocked(pkt *telemetry.Packet, now time.Time) {
	lapTimeMs := int(pkt.LastLapTime)

	// Validate lap time: skip invalid laps.
	if lapTimeMs <= 0 {
		return
	}
	if lapTimeMs < 10000 { // Less than 10 seconds
		return
	}

	// Skip if not in a race or game is paused/loading.
	if !pkt.InRace || pkt.IsPaused || pkt.IsLoading {
		return
	}

	// Skip the first completed lap (typically an out-lap or partial lap).
	if !m.currentSession.FirstLapSkipped {
		m.currentSession.FirstLapSkipped = true
		return
	}

	lapNumber := int(pkt.CurrentLap) - 1 // The completed lap is the previous one.

	req := api.RecordLapRequest{
		SessionID:  m.currentSession.ID,
		LapTimeMs:  lapTimeMs,
		LapNumber:  lapNumber,
		RecordedAt: now.UTC().Format(time.RFC3339),
	}

	resp, err := m.api.RecordLap(req)
	if err != nil {
		log.Printf("Error recording lap: %v", err)
		return
	}

	m.metrics.Incr("lap.recorded", nil)
	m.metrics.Histogram("lap.time_ms", float64(lapTimeMs), nil)
	log.Printf("Lap recorded: session=%s lap=%d time=%dms", m.currentSession.ID, lapNumber, lapTimeMs)

	// Send Discord notifications for any records broken.
	if resp != nil && len(resp.Records) > 0 {
		m.sendRecordNotifications(resp.Records, lapTimeMs)
	}
}

// sendRecordNotifications dispatches Discord notifications for broken records.
func (m *Manager) sendRecordNotifications(records []api.RecordInfo, lapTimeMs int) {
	if m.notifier == nil {
		return
	}

	// Look up car name.
	carName := "Unknown"
	category := ""
	if m.carDB != nil {
		if car, ok := m.carDB.Lookup(int(m.currentSession.CarID)); ok {
			carName = car.Name
			category = car.Category
		}
	}

	for _, rec := range records {
		// Check config flags to determine whether to send this notification.
		switch rec.Type {
		case "overall":
			if !m.discordConfig.NotifyOverallRecords {
				continue
			}
		case "category":
			if !m.discordConfig.NotifyCategoryRecords {
				continue
			}
		case "car":
			if !m.discordConfig.NotifyCarRecords {
				continue
			}
		}

		notif := discord.RecordNotification{
			DriverName:     m.currentSession.DriverName,
			TrackName:      m.currentSession.TrackName,
			TrackSlug:      m.currentSession.TrackSlug,
			CarName:        carName,
			Category:       category,
			LapTime:        discord.FormatLapTime(lapTimeMs),
			LapTimeMs:      lapTimeMs,
			PreviousTimeMs: rec.PreviousTimeMs,
			PreviousDriver: rec.PreviousDriver,
			RecordType:     rec.Type,
		}

		if err := m.notifier.SendRecordNotification(notif); err != nil {
			log.Printf("Error sending Discord notification: %v", err)
		}
	}
}
