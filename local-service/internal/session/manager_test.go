package session

import (
	"strings"
	"sync"
	"testing"
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

// --- Mock implementations ---

type mockAPIClient struct {
	mu              sync.Mutex
	createCalls     []api.CreateSessionRequest
	updateCalls     []string
	recordCalls     []api.RecordLapRequest
	endCalls        []string
	createResp      *api.CreateSessionResponse
	recordResp      *api.RecordLapResponse
	createErr       error
	updateErr       error
	recordErr       error
	endErr          error
}

func (m *mockAPIClient) CreateSession(req api.CreateSessionRequest) (*api.CreateSessionResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createCalls = append(m.createCalls, req)
	if m.createErr != nil {
		return nil, m.createErr
	}
	if m.createResp != nil {
		return m.createResp, nil
	}
	return &api.CreateSessionResponse{SessionID: "test-session-1"}, nil
}

func (m *mockAPIClient) UpdateSession(id string, _ api.UpdateSessionRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCalls = append(m.updateCalls, id)
	return m.updateErr
}

func (m *mockAPIClient) SyncTrack(_ api.TrackSync) error {
	return nil
}

func (m *mockAPIClient) RecordLap(req api.RecordLapRequest) (*api.RecordLapResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recordCalls = append(m.recordCalls, req)
	if m.recordErr != nil {
		return nil, m.recordErr
	}
	if m.recordResp != nil {
		return m.recordResp, nil
	}
	return &api.RecordLapResponse{LapID: "lap-1"}, nil
}

func (m *mockAPIClient) EndSession(id string, _ time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.endCalls = append(m.endCalls, id)
	return m.endErr
}

type mockDriverDetector struct {
	accountID  string
	driverName string
	err        error
}

func (m *mockDriverDetector) IdentifyDriver(_ []psn.AccountConfig) (string, string, error) {
	return m.accountID, m.driverName, m.err
}

type mockTrackDetector struct {
	result     *trackdetect.DetectionResult
	resetCount int
	addCount   int
}

func (m *mockTrackDetector) AddPoint(_ *telemetry.Packet) *trackdetect.DetectionResult {
	m.addCount++
	return m.result
}

func (m *mockTrackDetector) Reset() {
	m.resetCount++
}

type mockNotifier struct {
	mu            sync.Mutex
	notifications []discord.RecordNotification
	err           error
}

func (m *mockNotifier) SendRecordNotification(notif discord.RecordNotification) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifications = append(m.notifications, notif)
	return m.err
}

// --- Helper functions ---

func newTestManager(
	apiClient *mockAPIClient,
	driver *mockDriverDetector,
	track *mockTrackDetector,
	notifier *mockNotifier,
) *Manager {
	return NewManager(
		apiClient,
		driver,
		track,
		notifier,
		nil, // carDB
		metrics.NewNoop(),
		nil, // psnAccounts
		config.DiscordConfig{
			NotifyOverallRecords:  true,
			NotifyCategoryRecords: true,
			NotifyCarRecords:      false,
		},
		30*time.Second,
	)
}

func makePacket(carID int32, currentLap int16, lastLapTimeMs int32) *telemetry.Packet {
	return &telemetry.Packet{
		CarID:       carID,
		CurrentLap:  currentLap,
		LastLapTime: lastLapTimeMs,
		InRace:      true,
	}
}

// --- Tests ---

func TestNewSessionStartsOnFirstPacket(t *testing.T) {
	apiClient := &mockAPIClient{}
	trackDet := &mockTrackDetector{}
	mgr := newTestManager(apiClient, nil, trackDet, nil)

	pkt := makePacket(100, 1, 0)
	mgr.HandlePacket(pkt)

	if len(apiClient.createCalls) != 1 {
		t.Fatalf("expected 1 CreateSession call, got %d", len(apiClient.createCalls))
	}
	if apiClient.createCalls[0].CarID != 100 {
		t.Errorf("expected CarID 100, got %d", apiClient.createCalls[0].CarID)
	}

	session := mgr.CurrentSession()
	if session == nil {
		t.Fatal("expected active session, got nil")
	}
	if session.ID != "test-session-1" {
		t.Errorf("expected session ID 'test-session-1', got %q", session.ID)
	}
	if session.CarID != 100 {
		t.Errorf("expected CarID 100, got %d", session.CarID)
	}
}

func TestLapCompletionRecordedWhenLapCounterIncrements(t *testing.T) {
	apiClient := &mockAPIClient{}
	trackDet := &mockTrackDetector{}
	mgr := newTestManager(apiClient, nil, trackDet, nil)

	// Lap 1: start session
	mgr.HandlePacket(makePacket(100, 1, 0))

	// Lap 2: first lap completion (should be skipped as first lap)
	mgr.HandlePacket(makePacket(100, 2, 65000))

	// Lap 3: second lap completion (should be recorded)
	mgr.HandlePacket(makePacket(100, 3, 72000))

	if len(apiClient.recordCalls) != 1 {
		t.Fatalf("expected 1 RecordLap call, got %d", len(apiClient.recordCalls))
	}
	if apiClient.recordCalls[0].LapTimeMs != 72000 {
		t.Errorf("expected lap time 72000ms, got %d", apiClient.recordCalls[0].LapTimeMs)
	}
	if apiClient.recordCalls[0].LapNumber != 2 {
		t.Errorf("expected lap number 2, got %d", apiClient.recordCalls[0].LapNumber)
	}
}

func TestFirstLapIsSkipped(t *testing.T) {
	apiClient := &mockAPIClient{}
	trackDet := &mockTrackDetector{}
	mgr := newTestManager(apiClient, nil, trackDet, nil)

	// Start session on lap 1
	mgr.HandlePacket(makePacket(100, 1, 0))

	// Complete first lap: should be skipped
	mgr.HandlePacket(makePacket(100, 2, 65000))

	if len(apiClient.recordCalls) != 0 {
		t.Fatalf("expected 0 RecordLap calls (first lap should be skipped), got %d", len(apiClient.recordCalls))
	}
}

func TestInvalidLapsDiscarded_NegativeTime(t *testing.T) {
	apiClient := &mockAPIClient{}
	trackDet := &mockTrackDetector{}
	mgr := newTestManager(apiClient, nil, trackDet, nil)

	mgr.HandlePacket(makePacket(100, 1, 0))
	mgr.HandlePacket(makePacket(100, 2, 65000)) // first lap, skipped

	// Invalid: negative lap time
	mgr.HandlePacket(makePacket(100, 3, -1))

	if len(apiClient.recordCalls) != 0 {
		t.Fatalf("expected 0 RecordLap calls for negative lap time, got %d", len(apiClient.recordCalls))
	}
}

func TestInvalidLapsDiscarded_TooShort(t *testing.T) {
	apiClient := &mockAPIClient{}
	trackDet := &mockTrackDetector{}
	mgr := newTestManager(apiClient, nil, trackDet, nil)

	mgr.HandlePacket(makePacket(100, 1, 0))
	mgr.HandlePacket(makePacket(100, 2, 65000)) // first lap, skipped

	// Invalid: less than 10 seconds
	mgr.HandlePacket(makePacket(100, 3, 5000))

	if len(apiClient.recordCalls) != 0 {
		t.Fatalf("expected 0 RecordLap calls for sub-10s lap, got %d", len(apiClient.recordCalls))
	}
}

func TestInvalidLapsDiscarded_Zero(t *testing.T) {
	apiClient := &mockAPIClient{}
	trackDet := &mockTrackDetector{}
	mgr := newTestManager(apiClient, nil, trackDet, nil)

	mgr.HandlePacket(makePacket(100, 1, 0))
	mgr.HandlePacket(makePacket(100, 2, 65000)) // first lap, skipped

	// Invalid: zero lap time
	mgr.HandlePacket(makePacket(100, 3, 0))

	if len(apiClient.recordCalls) != 0 {
		t.Fatalf("expected 0 RecordLap calls for zero lap time, got %d", len(apiClient.recordCalls))
	}
}

func TestPausedLapsDiscarded(t *testing.T) {
	apiClient := &mockAPIClient{}
	trackDet := &mockTrackDetector{}
	mgr := newTestManager(apiClient, nil, trackDet, nil)

	mgr.HandlePacket(makePacket(100, 1, 0))
	mgr.HandlePacket(makePacket(100, 2, 65000)) // first lap, skipped

	// Paused lap
	pkt := makePacket(100, 3, 72000)
	pkt.IsPaused = true
	mgr.HandlePacket(pkt)

	if len(apiClient.recordCalls) != 0 {
		t.Fatalf("expected 0 RecordLap calls for paused lap, got %d", len(apiClient.recordCalls))
	}
}

func TestLoadingLapsDiscarded(t *testing.T) {
	apiClient := &mockAPIClient{}
	trackDet := &mockTrackDetector{}
	mgr := newTestManager(apiClient, nil, trackDet, nil)

	mgr.HandlePacket(makePacket(100, 1, 0))
	mgr.HandlePacket(makePacket(100, 2, 65000)) // first lap, skipped

	// Loading lap
	pkt := makePacket(100, 3, 72000)
	pkt.IsLoading = true
	mgr.HandlePacket(pkt)

	if len(apiClient.recordCalls) != 0 {
		t.Fatalf("expected 0 RecordLap calls for loading lap, got %d", len(apiClient.recordCalls))
	}
}

func TestNotInRaceLapsDiscarded(t *testing.T) {
	apiClient := &mockAPIClient{}
	trackDet := &mockTrackDetector{}
	mgr := newTestManager(apiClient, nil, trackDet, nil)

	mgr.HandlePacket(makePacket(100, 1, 0))
	mgr.HandlePacket(makePacket(100, 2, 65000)) // first lap, skipped

	// Not in race (e.g. in menus or photo mode)
	pkt := makePacket(100, 3, 72000)
	pkt.InRace = false
	mgr.HandlePacket(pkt)

	if len(apiClient.recordCalls) != 0 {
		t.Fatalf("expected 0 RecordLap calls for not-in-race lap, got %d", len(apiClient.recordCalls))
	}
}

func TestCarChangeTriggersNewSession(t *testing.T) {
	apiClient := &mockAPIClient{
		createResp: &api.CreateSessionResponse{SessionID: "session-1"},
	}
	trackDet := &mockTrackDetector{}
	mgr := newTestManager(apiClient, nil, trackDet, nil)

	// Start with car 100
	mgr.HandlePacket(makePacket(100, 1, 0))

	if len(apiClient.createCalls) != 1 {
		t.Fatalf("expected 1 CreateSession call after first packet, got %d", len(apiClient.createCalls))
	}

	// Change to car 200: should end old session and start new one
	apiClient.createResp = &api.CreateSessionResponse{SessionID: "session-2"}
	mgr.HandlePacket(makePacket(200, 1, 0))

	if len(apiClient.endCalls) != 1 {
		t.Fatalf("expected 1 EndSession call, got %d", len(apiClient.endCalls))
	}
	if apiClient.endCalls[0] != "session-1" {
		t.Errorf("expected EndSession for session-1, got %s", apiClient.endCalls[0])
	}
	if len(apiClient.createCalls) != 2 {
		t.Fatalf("expected 2 CreateSession calls, got %d", len(apiClient.createCalls))
	}
	if apiClient.createCalls[1].CarID != 200 {
		t.Errorf("expected new session with CarID 200, got %d", apiClient.createCalls[1].CarID)
	}

	session := mgr.CurrentSession()
	if session == nil {
		t.Fatal("expected active session, got nil")
	}
	if session.ID != "session-2" {
		t.Errorf("expected session ID 'session-2', got %q", session.ID)
	}
}

func TestIdleTimeoutEndsSession(t *testing.T) {
	apiClient := &mockAPIClient{}
	trackDet := &mockTrackDetector{}
	mgr := NewManager(
		apiClient, nil, trackDet, nil,
		nil, metrics.NewNoop(), nil,
		config.DiscordConfig{},
		100*time.Millisecond, // very short timeout for testing
	)

	mgr.HandlePacket(makePacket(100, 1, 0))

	if mgr.CurrentSession() == nil {
		t.Fatal("expected active session after first packet")
	}

	// Wait for idle timeout to expire.
	time.Sleep(200 * time.Millisecond)

	// CheckIdle should end the session.
	mgr.CheckIdle()

	if mgr.CurrentSession() != nil {
		t.Fatal("expected session to be ended after idle timeout")
	}
	if len(apiClient.endCalls) != 1 {
		t.Fatalf("expected 1 EndSession call from idle timeout, got %d", len(apiClient.endCalls))
	}
}

func TestIdleTimeoutViaCheckIdle(t *testing.T) {
	apiClient := &mockAPIClient{
		createResp: &api.CreateSessionResponse{SessionID: "session-1"},
	}
	trackDet := &mockTrackDetector{}
	mgr := NewManager(
		apiClient, nil, trackDet, nil,
		nil, metrics.NewNoop(), nil,
		config.DiscordConfig{},
		100*time.Millisecond,
	)

	mgr.HandlePacket(makePacket(100, 1, 0))

	// Wait for idle timeout to expire.
	time.Sleep(200 * time.Millisecond)

	// CheckIdle should detect the timeout and end the session.
	mgr.CheckIdle()

	if len(apiClient.endCalls) != 1 {
		t.Fatalf("expected 1 EndSession call, got %d", len(apiClient.endCalls))
	}

	// Next packet should start a new session.
	apiClient.createResp = &api.CreateSessionResponse{SessionID: "session-2"}
	mgr.HandlePacket(makePacket(100, 1, 0))

	if len(apiClient.createCalls) != 2 {
		t.Fatalf("expected 2 CreateSession calls, got %d", len(apiClient.createCalls))
	}

	session := mgr.CurrentSession()
	if session == nil {
		t.Fatal("expected new session after idle restart")
	}
	if session.ID != "session-2" {
		t.Errorf("expected session ID 'session-2', got %q", session.ID)
	}
}

func TestDriverDetectionPopulatesSession(t *testing.T) {
	apiClient := &mockAPIClient{}
	driver := &mockDriverDetector{
		accountID:  "acc-123",
		driverName: "TestDriver",
	}
	trackDet := &mockTrackDetector{}
	accounts := []psn.AccountConfig{
		{OnlineID: "testplayer", AccountID: "acc-123", DriverName: "TestDriver"},
	}

	mgr := NewManager(
		apiClient, driver, trackDet, nil,
		nil, metrics.NewNoop(), accounts,
		config.DiscordConfig{},
		30*time.Second,
	)

	mgr.HandlePacket(makePacket(100, 1, 0))

	session := mgr.CurrentSession()
	if session == nil {
		t.Fatal("expected active session, got nil")
	}
	if session.DriverID != "acc-123" {
		t.Errorf("expected DriverID 'acc-123', got %q", session.DriverID)
	}
	if session.DriverName != "TestDriver" {
		t.Errorf("expected DriverName 'TestDriver', got %q", session.DriverName)
	}
}

func TestTrackDetectionUpdatesSession(t *testing.T) {
	apiClient := &mockAPIClient{}
	trackResult := &trackdetect.DetectionResult{
		Track: &trackdetect.TrackReference{
			Info: trackdetect.TrackInfo{
				Name:   "Suzuka Circuit",
				Layout: "Full Course",
				Slug:   "suzuka-circuit-full-course",
			},
		},
		IsReverse: false,
	}
	trackDet := &mockTrackDetector{result: trackResult}
	mgr := newTestManager(apiClient, nil, trackDet, nil)

	mgr.HandlePacket(makePacket(100, 1, 0))

	session := mgr.CurrentSession()
	if session == nil {
		t.Fatal("expected active session")
	}
	if session.TrackSlug != "suzuka-circuit-full-course" {
		t.Errorf("expected TrackSlug 'suzuka-circuit-full-course', got %q", session.TrackSlug)
	}
	if session.TrackName != "Suzuka Circuit - Full Course" {
		t.Errorf("expected TrackName 'Suzuka Circuit - Full Course', got %q", session.TrackName)
	}
}

func TestRecordNotificationsSent(t *testing.T) {
	notifier := &mockNotifier{}
	apiClient := &mockAPIClient{
		recordResp: &api.RecordLapResponse{
			LapID: "lap-1",
			Records: []api.RecordInfo{
				{Type: "overall", PreviousTimeMs: 75000, PreviousDriver: "OldChamp"},
			},
		},
	}
	trackDet := &mockTrackDetector{}
	mgr := newTestManager(apiClient, nil, trackDet, notifier)

	mgr.HandlePacket(makePacket(100, 1, 0))
	mgr.HandlePacket(makePacket(100, 2, 65000)) // first lap, skipped
	mgr.HandlePacket(makePacket(100, 3, 72000)) // recorded, has record

	if len(notifier.notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifier.notifications))
	}
	n := notifier.notifications[0]
	if n.RecordType != "overall" {
		t.Errorf("expected RecordType 'overall', got %q", n.RecordType)
	}
	if n.PreviousTimeMs != 75000 {
		t.Errorf("expected PreviousTimeMs 75000, got %d", n.PreviousTimeMs)
	}
	if n.PreviousDriver != "OldChamp" {
		t.Errorf("expected PreviousDriver 'OldChamp', got %q", n.PreviousDriver)
	}
}

func TestCategoryRecordNotificationRespectConfig(t *testing.T) {
	notifier := &mockNotifier{}
	apiClient := &mockAPIClient{
		recordResp: &api.RecordLapResponse{
			LapID: "lap-1",
			Records: []api.RecordInfo{
				{Type: "overall", PreviousTimeMs: 75000},
				{Type: "category", PreviousTimeMs: 76000},
				{Type: "car", PreviousTimeMs: 77000},
			},
		},
	}
	trackDet := &mockTrackDetector{}
	// Default config: overall=true, category=true, car=false
	mgr := newTestManager(apiClient, nil, trackDet, notifier)

	mgr.HandlePacket(makePacket(100, 1, 0))
	mgr.HandlePacket(makePacket(100, 2, 65000)) // first lap, skipped
	mgr.HandlePacket(makePacket(100, 3, 72000)) // recorded

	// Should have 2 notifications (overall + category, not car)
	if len(notifier.notifications) != 2 {
		t.Fatalf("expected 2 notifications (overall + category), got %d", len(notifier.notifications))
	}
	if notifier.notifications[0].RecordType != "overall" {
		t.Errorf("expected first notification type 'overall', got %q", notifier.notifications[0].RecordType)
	}
	if notifier.notifications[1].RecordType != "category" {
		t.Errorf("expected second notification type 'category', got %q", notifier.notifications[1].RecordType)
	}
}

func TestTrackDetectorReset(t *testing.T) {
	apiClient := &mockAPIClient{}
	trackDet := &mockTrackDetector{}
	mgr := newTestManager(apiClient, nil, trackDet, nil)

	mgr.HandlePacket(makePacket(100, 1, 0))

	if trackDet.resetCount != 1 {
		t.Errorf("expected track detector to be reset once on session start, got %d", trackDet.resetCount)
	}
}

func TestNoSessionWithoutAPIResponse(t *testing.T) {
	apiClient := &mockAPIClient{
		createErr: errTest,
	}
	trackDet := &mockTrackDetector{}
	mgr := newTestManager(apiClient, nil, trackDet, nil)

	mgr.HandlePacket(makePacket(100, 1, 0))

	if mgr.CurrentSession() != nil {
		t.Fatal("expected no session when API returns error")
	}
}

var errTest = &testError{}

type testError struct{}

func (e *testError) Error() string { return "test error" }

func TestTrackDetectionStopsAfterDetection(t *testing.T) {
	trackResult := &trackdetect.DetectionResult{
		Track: &trackdetect.TrackReference{
			Info: trackdetect.TrackInfo{
				Name: "Test Track",
				Slug: "test-track",
			},
		},
	}
	trackDet := &mockTrackDetector{result: trackResult}
	apiClient := &mockAPIClient{}
	mgr := newTestManager(apiClient, nil, trackDet, nil)

	// First packet: starts session, detects track
	mgr.HandlePacket(makePacket(100, 1, 0))
	firstAddCount := trackDet.addCount

	// Second packet: track already detected, should NOT call AddPoint again
	mgr.HandlePacket(makePacket(100, 1, 0))

	if trackDet.addCount != firstAddCount {
		t.Errorf("expected track detector not to be called again after detection, addCount was %d now %d", firstAddCount, trackDet.addCount)
	}
}

func TestCarDBLookupInNotifications(t *testing.T) {
	// Build a small car database.
	db := buildTestCarDB(t)

	notifier := &mockNotifier{}
	apiClient := &mockAPIClient{
		recordResp: &api.RecordLapResponse{
			LapID: "lap-1",
			Records: []api.RecordInfo{
				{Type: "overall"},
			},
		},
	}
	trackDet := &mockTrackDetector{}

	mgr := NewManager(
		apiClient, nil, trackDet, notifier,
		db, metrics.NewNoop(), nil,
		config.DiscordConfig{NotifyOverallRecords: true},
		30*time.Second,
	)

	mgr.HandlePacket(makePacket(100, 1, 0))
	mgr.HandlePacket(makePacket(100, 2, 65000)) // first lap, skipped
	mgr.HandlePacket(makePacket(100, 3, 72000)) // recorded

	if len(notifier.notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifier.notifications))
	}
	if notifier.notifications[0].CarName != "Test Car" {
		t.Errorf("expected CarName 'Test Car', got %q", notifier.notifications[0].CarName)
	}
	if notifier.notifications[0].Category != "Gr.3" {
		t.Errorf("expected Category 'Gr.3', got %q", notifier.notifications[0].Category)
	}
}

func TestRaceRestartSplitsSession(t *testing.T) {
	apiClient := &mockAPIClient{
		createResp: &api.CreateSessionResponse{SessionID: "session-1"},
	}
	trackDet := &mockTrackDetector{}
	mgr := newTestManager(apiClient, nil, trackDet, nil)

	// Start a session and advance to lap 3.
	mgr.HandlePacket(makePacket(100, 1, 0))
	mgr.HandlePacket(makePacket(100, 2, 65000))
	mgr.HandlePacket(makePacket(100, 3, 70000))

	if len(apiClient.createCalls) != 1 {
		t.Fatalf("expected 1 CreateSession call, got %d", len(apiClient.createCalls))
	}

	// Lap counter resets to 1 (new race started, potentially on a new track).
	apiClient.createResp = &api.CreateSessionResponse{SessionID: "session-2"}
	mgr.HandlePacket(makePacket(100, 1, 0))

	if len(apiClient.endCalls) != 1 {
		t.Fatalf("expected EndSession called once on race restart, got %d", len(apiClient.endCalls))
	}
	if apiClient.endCalls[0] != "session-1" {
		t.Errorf("expected EndSession for session-1, got %s", apiClient.endCalls[0])
	}
	if len(apiClient.createCalls) != 2 {
		t.Fatalf("expected 2 CreateSession calls after race restart, got %d", len(apiClient.createCalls))
	}

	session := mgr.CurrentSession()
	if session == nil {
		t.Fatal("expected active session after race restart")
	}
	if session.ID != "session-2" {
		t.Errorf("expected new session ID 'session-2', got %q", session.ID)
	}
	// Track detector should be reset for the new session.
	if trackDet.resetCount != 2 {
		t.Errorf("expected track detector reset twice (session start + race restart), got %d", trackDet.resetCount)
	}
}

// buildTestCarDB creates a small car database for testing.
func buildTestCarDB(t *testing.T) *cardb.Database {
	t.Helper()
	db, err := cardb.LoadFromSources(cardb.Sources{
		Cars:      strings.NewReader("ID,ShortName,Maker\n100,Short Test Car,1\n"),
		Makers:    strings.NewReader("ID,Name,Country\n1,TestMfr,US\n"),
		CarGroups: strings.NewReader("ID,Group\n100,3\n"),
		StockPerf: strings.NewReader("carid,manufacturer,name,group,CH\n100,TestMfr,Test Car,3,500\n"),
	})
	if err != nil {
		t.Fatalf("failed to load test car DB: %v", err)
	}
	return db
}
