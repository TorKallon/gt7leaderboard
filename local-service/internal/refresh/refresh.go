package refresh

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rourkem/gt7leaderboard/local-service/internal/api"
	"github.com/rourkem/gt7leaderboard/local-service/internal/cardb"
	"github.com/rourkem/gt7leaderboard/local-service/internal/config"
	"github.com/rourkem/gt7leaderboard/local-service/internal/metrics"
)

// APIClient is the interface for syncing data to the hosted API.
type APIClient interface {
	SyncCars([]api.CarSync) error
	SyncTrack(api.TrackSync) error
}

// Refresher handles periodic refresh of car and track data.
type Refresher struct {
	carDataURL    string
	trackDataRepo string
	carDB         *cardb.Database
	apiClient     APIClient
	metrics       metrics.Metrics
	httpClient    *http.Client

	LastCarRefresh   time.Time
	LastTrackRefresh time.Time
}

// NewRefresher creates a new data refresher.
func NewRefresher(cfg config.DataRefreshConfig, carDB *cardb.Database, apiClient APIClient, m metrics.Metrics) *Refresher {
	return &Refresher{
		carDataURL:    cfg.CarDataURL,
		trackDataRepo: cfg.TrackDataRepo,
		carDB:         carDB,
		apiClient:     apiClient,
		metrics:       m,
		httpClient:    &http.Client{Timeout: 60 * time.Second},
	}
}

// RefreshCars downloads the car CSV from the configured URL, parses it,
// and syncs the data to the hosted API.
func (r *Refresher) RefreshCars() error {
	if r.carDataURL == "" {
		return fmt.Errorf("car data URL not configured")
	}

	resp, err := r.httpClient.Get(r.carDataURL)
	if err != nil {
		return fmt.Errorf("downloading car data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("car data download returned status %d", resp.StatusCode)
	}

	db, err := cardb.LoadFromReader(resp.Body)
	if err != nil {
		return fmt.Errorf("parsing car data: %w", err)
	}

	// Build sync payload from the parsed database.
	allCars := db.All()
	syncCars := make([]api.CarSync, 0, len(allCars))
	for _, car := range allCars {
		syncCars = append(syncCars, api.CarSync{
			ID:           car.ID,
			Name:         car.Name,
			Manufacturer: car.Manufacturer,
			Category:     car.Category,
			PPStock:      car.PPStock,
		})
	}

	if err := r.apiClient.SyncCars(syncCars); err != nil {
		return fmt.Errorf("syncing cars to API: %w", err)
	}

	r.LastCarRefresh = time.Now()
	r.metrics.Incr("refresh.cars", nil)
	log.Printf("Refreshed car data: %d cars synced", len(syncCars))
	return nil
}

// githubContent represents a file entry from the GitHub contents API.
type githubContent struct {
	Name        string `json:"name"`
	DownloadURL string `json:"download_url"`
	SHA         string `json:"sha"`
}

// RefreshTracks checks the configured GitHub repo for track files and downloads
// any new or changed .gt7track files to the local directory.
func (r *Refresher) RefreshTracks(localDir string) error {
	if r.trackDataRepo == "" {
		return fmt.Errorf("track data repo not configured")
	}

	// Parse owner/repo from the config.
	parts := strings.SplitN(r.trackDataRepo, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid track data repo format %q (expected owner/repo)", r.trackDataRepo)
	}
	owner, repo := parts[0], parts[1]

	// List files in the tracks/ directory via GitHub API.
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/tracks", owner, repo)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("creating GitHub API request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetching track file list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	var contents []githubContent
	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		return fmt.Errorf("parsing GitHub API response: %w", err)
	}

	// Ensure local directory exists.
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		return fmt.Errorf("creating local track directory: %w", err)
	}

	downloaded := 0
	for _, entry := range contents {
		if !strings.HasSuffix(entry.Name, ".gt7track") {
			continue
		}
		if entry.DownloadURL == "" {
			continue
		}

		localPath := filepath.Join(localDir, entry.Name)

		// Check if file already exists locally. We use the presence of the file
		// as a simple "already downloaded" check. A more robust approach would
		// store SHAs, but file presence is sufficient for this use case.
		if _, err := os.Stat(localPath); err == nil {
			continue // file exists, skip
		}

		if err := r.downloadFile(entry.DownloadURL, localPath); err != nil {
			log.Printf("Warning: failed to download track file %s: %v", entry.Name, err)
			continue
		}
		downloaded++
	}

	r.LastTrackRefresh = time.Now()
	r.metrics.Incr("refresh.tracks", nil)
	log.Printf("Refreshed track data: %d new files downloaded", downloaded)
	return nil
}

// downloadFile downloads a URL to a local file path.
func (r *Refresher) downloadFile(url, destPath string) error {
	resp, err := r.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", destPath, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("writing file %s: %w", destPath, err)
	}

	return nil
}

// StartScheduler runs RefreshCars and RefreshTracks on startup and then
// periodically at the configured intervals. It blocks until the context is cancelled.
func (r *Refresher) StartScheduler(ctx context.Context, carInterval, trackInterval time.Duration, localTrackDir string) {
	// Run immediately on startup.
	if err := r.RefreshCars(); err != nil {
		log.Printf("Initial car refresh failed: %v", err)
	}
	if err := r.RefreshTracks(localTrackDir); err != nil {
		log.Printf("Initial track refresh failed: %v", err)
	}

	carTicker := time.NewTicker(carInterval)
	defer carTicker.Stop()

	trackTicker := time.NewTicker(trackInterval)
	defer trackTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-carTicker.C:
			if err := r.RefreshCars(); err != nil {
				log.Printf("Car refresh failed: %v", err)
			}
		case <-trackTicker.C:
			if err := r.RefreshTracks(localTrackDir); err != nil {
				log.Printf("Track refresh failed: %v", err)
			}
		}
	}
}
