package refresh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// carDataFiles maps logical names to URL path suffixes relative to the base URL.
var carDataFiles = map[string]string{
	"cars.csv":           "/data/db/cars.csv",
	"maker.csv":          "/data/db/maker.csv",
	"cargrp.csv":         "/data/db/cargrp.csv",
	"data-stock-perf.csv": "/data-stock-perf.csv",
}

// TrackReloadFunc is called after new track files are downloaded so the
// detector can reload its references from disk.
type TrackReloadFunc func(trackDir string) error

// Refresher handles periodic refresh of car and track data.
type Refresher struct {
	carDataBaseURL string
	carCacheDir    string
	trackDataRepo  string
	carDB          *cardb.Database
	apiClient      APIClient
	metrics        metrics.Metrics
	httpClient     *http.Client
	onTrackReload  TrackReloadFunc

	LastCarRefresh   time.Time
	LastTrackRefresh time.Time
}

// NewRefresher creates a new data refresher. The optional onTrackReload
// callback is invoked after new track files are downloaded so that the
// detector can reload its references from disk.
func NewRefresher(cfg config.DataRefreshConfig, carCacheDir string, carDB *cardb.Database, apiClient APIClient, m metrics.Metrics, onTrackReload TrackReloadFunc) *Refresher {
	return &Refresher{
		carDataBaseURL: cfg.CarDataBaseURL,
		carCacheDir:    carCacheDir,
		trackDataRepo:  cfg.TrackDataRepo,
		carDB:          carDB,
		apiClient:      apiClient,
		metrics:        m,
		httpClient:     &http.Client{Timeout: 60 * time.Second},
		onTrackReload:  onTrackReload,
	}
}

// fetchResult holds the result of a single file download.
type fetchResult struct {
	name string
	data []byte
	err  error
}

// fetchBytes downloads a URL and returns its body as bytes.
func (r *Refresher) fetchBytes(url string) ([]byte, error) {
	resp, err := r.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned status %d", url, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", url, err)
	}
	return data, nil
}

// saveCarCache writes downloaded CSV data to the cache directory.
func saveCarCache(dir string, data map[string][]byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}
	for name, content := range data {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
	}
	return nil
}

// RefreshCars downloads the four car data CSVs from the configured base URL,
// parses them, and syncs the data to the hosted API.
func (r *Refresher) RefreshCars() error {
	if r.carDataBaseURL == "" {
		return fmt.Errorf("car data base URL not configured")
	}

	// Download all four files in parallel.
	var wg sync.WaitGroup
	results := make(chan fetchResult, len(carDataFiles))

	for name, suffix := range carDataFiles {
		wg.Add(1)
		go func(name, url string) {
			defer wg.Done()
			data, err := r.fetchBytes(url)
			results <- fetchResult{name: name, data: data, err: err}
		}(name, r.carDataBaseURL+suffix)
	}

	wg.Wait()
	close(results)

	// Collect results — all-or-nothing.
	downloaded := make(map[string][]byte, len(carDataFiles))
	for res := range results {
		if res.err != nil {
			return fmt.Errorf("fetching %s: %w", res.name, res.err)
		}
		downloaded[res.name] = res.data
	}

	// Parse with LoadFromSources.
	db, err := cardb.LoadFromSources(cardb.Sources{
		Cars:      bytes.NewReader(downloaded["cars.csv"]),
		Makers:    bytes.NewReader(downloaded["maker.csv"]),
		CarGroups: bytes.NewReader(downloaded["cargrp.csv"]),
		StockPerf: bytes.NewReader(downloaded["data-stock-perf.csv"]),
	})
	if err != nil {
		return fmt.Errorf("parsing car data: %w", err)
	}

	// Save raw CSVs to cache (non-fatal if save fails).
	if r.carCacheDir != "" {
		if err := saveCarCache(r.carCacheDir, downloaded); err != nil {
			log.Printf("Warning: failed to save car cache: %v", err)
		}
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

	// Reload track references into the detector if any new files were downloaded
	// (or on first run when the detector may have been initialized with 0 tracks).
	if downloaded > 0 && r.onTrackReload != nil {
		if err := r.onTrackReload(localDir); err != nil {
			log.Printf("Warning: failed to reload track references: %v", err)
		}
	}

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
