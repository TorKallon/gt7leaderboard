package refresh

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/rourkem/gt7leaderboard/local-service/internal/api"
	"github.com/rourkem/gt7leaderboard/local-service/internal/config"
	"github.com/rourkem/gt7leaderboard/local-service/internal/metrics"
)

// --- Mock API Client ---

type mockAPIClient struct {
	syncCarsCalls  [][]api.CarSync
	syncTrackCalls []api.TrackSync
	syncCarsErr    error
	syncTrackErr   error
}

func (m *mockAPIClient) SyncCars(cars []api.CarSync) error {
	m.syncCarsCalls = append(m.syncCarsCalls, cars)
	return m.syncCarsErr
}

func (m *mockAPIClient) SyncTrack(track api.TrackSync) error {
	m.syncTrackCalls = append(m.syncTrackCalls, track)
	return m.syncTrackErr
}

// --- Test CSV data ---

const (
	testCarsCSV     = "ID,ShortName,Maker\n100,Short Test Car,1\n200,Short Other Car,2\n"
	testMakerCSV    = "ID,Name,Country\n1,TestMfr,US\n2,OtherMfr,JP\n"
	testCarGrpCSV   = "ID,Group\n100,3\n200,N\n"
	testStockPerf   = "carid,manufacturer,name,group,CH\n100,TestMfr,Test Car,3,500\n200,OtherMfr,Other Car,N,350\n"
)

// --- Tests ---

func TestRefreshCars(t *testing.T) {
	// Set up a test HTTP server that serves four CSV files.
	mux := http.NewServeMux()
	mux.HandleFunc("/data/db/cars.csv", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.Write([]byte(testCarsCSV))
	})
	mux.HandleFunc("/data/db/maker.csv", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.Write([]byte(testMakerCSV))
	})
	mux.HandleFunc("/data/db/cargrp.csv", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.Write([]byte(testCarGrpCSV))
	})
	mux.HandleFunc("/data-stock-perf.csv", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.Write([]byte(testStockPerf))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	cacheDir := t.TempDir()
	apiClient := &mockAPIClient{}
	cfg := config.DataRefreshConfig{
		CarDataBaseURL: srv.URL,
	}

	refresher := NewRefresher(cfg, cacheDir, nil, apiClient, metrics.NewNoop())

	if err := refresher.RefreshCars(); err != nil {
		t.Fatalf("RefreshCars() error: %v", err)
	}

	if len(apiClient.syncCarsCalls) != 1 {
		t.Fatalf("expected 1 SyncCars call, got %d", len(apiClient.syncCarsCalls))
	}

	synced := apiClient.syncCarsCalls[0]
	if len(synced) != 2 {
		t.Fatalf("expected 2 cars synced, got %d", len(synced))
	}

	// Find car 100 in the synced cars.
	found := false
	for _, c := range synced {
		if c.ID == 100 {
			found = true
			if c.Name != "Test Car" {
				t.Errorf("expected car name 'Test Car', got %q", c.Name)
			}
			if c.Category != "Gr.3" {
				t.Errorf("expected category 'Gr.3', got %q", c.Category)
			}
			if c.Manufacturer != "TestMfr" {
				t.Errorf("expected manufacturer 'TestMfr', got %q", c.Manufacturer)
			}
		}
	}
	if !found {
		t.Error("car ID 100 not found in synced cars")
	}

	if refresher.LastCarRefresh.IsZero() {
		t.Error("expected LastCarRefresh to be set")
	}

	// Verify cache files were written.
	for _, name := range []string{"cars.csv", "maker.csv", "cargrp.csv", "data-stock-perf.csv"} {
		if _, err := os.Stat(filepath.Join(cacheDir, name)); err != nil {
			t.Errorf("expected cache file %s to exist: %v", name, err)
		}
	}
}

func TestRefreshCarsEmptyURL(t *testing.T) {
	cfg := config.DataRefreshConfig{}
	refresher := NewRefresher(cfg, "", nil, &mockAPIClient{}, metrics.NewNoop())

	err := refresher.RefreshCars()
	if err == nil {
		t.Fatal("expected error for empty car data base URL")
	}
}

func TestRefreshCarsHTTPError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := config.DataRefreshConfig{CarDataBaseURL: srv.URL}
	refresher := NewRefresher(cfg, "", nil, &mockAPIClient{}, metrics.NewNoop())

	err := refresher.RefreshCars()
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

func TestRefreshTracks(t *testing.T) {
	// Create temp directory for downloaded tracks.
	tmpDir := t.TempDir()

	// File content to serve as a track file download.
	trackContent := "fake-track-data"

	// Set up a mock GitHub API + download server.
	mux := http.NewServeMux()
	var srvURL string
	mux.HandleFunc("/repos/owner/repo/contents/tracks", func(w http.ResponseWriter, r *http.Request) {
		contents := []githubContent{
			{Name: "track1.gt7track", DownloadURL: srvURL + "/download/track1.gt7track", SHA: "abc123"},
			{Name: "track2.gt7track", DownloadURL: srvURL + "/download/track2.gt7track", SHA: "def456"},
			{Name: "readme.md", DownloadURL: srvURL + "/download/readme.md", SHA: "ghi789"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(contents)
	})
	mux.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(trackContent))
	})

	srv := httptest.NewServer(mux)
	srvURL = srv.URL
	defer srv.Close()

	cfg := config.DataRefreshConfig{
		TrackDataRepo: "owner/repo",
	}
	refresher := NewRefresher(cfg, "", nil, &mockAPIClient{}, metrics.NewNoop())
	// Override the HTTP client's base URL approach by patching the refresher to use the test server.
	// We need to modify the GitHub API URL. Since it's hardcoded, we'll use a workaround:
	// Override the httpClient on the refresher and use a transport that rewrites URLs.
	refresher.httpClient = srv.Client()
	// Use a custom transport that redirects GitHub API calls to our test server.
	refresher.httpClient.Transport = &rewriteTransport{
		base:    http.DefaultTransport,
		testURL: srvURL,
	}

	if err := refresher.RefreshTracks(tmpDir); err != nil {
		t.Fatalf("RefreshTracks() error: %v", err)
	}

	// Check that track files were downloaded (not readme.md).
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("reading tmp dir: %v", err)
	}

	trackFiles := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".gt7track" {
			trackFiles++
			data, err := os.ReadFile(filepath.Join(tmpDir, e.Name()))
			if err != nil {
				t.Errorf("reading track file %s: %v", e.Name(), err)
				continue
			}
			if string(data) != trackContent {
				t.Errorf("track file %s has unexpected content: %q", e.Name(), string(data))
			}
		}
	}

	if trackFiles != 2 {
		t.Errorf("expected 2 track files, got %d", trackFiles)
	}

	if refresher.LastTrackRefresh.IsZero() {
		t.Error("expected LastTrackRefresh to be set")
	}

	// Running again should not re-download (files already exist).
	refresher2 := NewRefresher(cfg, "", nil, &mockAPIClient{}, metrics.NewNoop())
	refresher2.httpClient = srv.Client()
	refresher2.httpClient.Transport = &rewriteTransport{
		base:    http.DefaultTransport,
		testURL: srvURL,
	}

	// We can verify by counting downloads would be 0, but the simplest
	// check is that it doesn't error.
	if err := refresher2.RefreshTracks(tmpDir); err != nil {
		t.Fatalf("second RefreshTracks() error: %v", err)
	}
}

func TestRefreshTracksEmptyRepo(t *testing.T) {
	cfg := config.DataRefreshConfig{}
	refresher := NewRefresher(cfg, "", nil, &mockAPIClient{}, metrics.NewNoop())

	err := refresher.RefreshTracks(t.TempDir())
	if err == nil {
		t.Fatal("expected error for empty track data repo")
	}
}

func TestRefreshTracksInvalidRepoFormat(t *testing.T) {
	cfg := config.DataRefreshConfig{TrackDataRepo: "invalid-no-slash"}
	refresher := NewRefresher(cfg, "", nil, &mockAPIClient{}, metrics.NewNoop())

	err := refresher.RefreshTracks(t.TempDir())
	if err == nil {
		t.Fatal("expected error for invalid repo format")
	}
}

// rewriteTransport is an http.RoundTripper that rewrites all requests to go
// to the test server URL instead of the real GitHub API.
type rewriteTransport struct {
	base    http.RoundTripper
	testURL string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite the URL to point to the test server.
	req.URL.Scheme = "http"
	req.URL.Host = t.testURL[len("http://"):]
	return t.base.RoundTrip(req)
}
