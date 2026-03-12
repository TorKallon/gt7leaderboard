package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client pushes data to the hosted Vercel API.
type Client struct {
	endpoint   string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new API push client.
func NewClient(endpoint, apiKey string) *Client {
	return &Client{
		endpoint:   endpoint,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// CreateSessionRequest is the request body for creating a new session.
type CreateSessionRequest struct {
	DriverID        string `json:"driver_id,omitempty"`
	DriverName      string `json:"driver_name,omitempty"`
	TrackSlug       string `json:"track_slug,omitempty"`
	CarID           int    `json:"car_id"`
	StartedAt       string `json:"started_at"`
	DetectionMethod string `json:"detection_method"`
}

// UpdateSessionRequest is the request body for updating a session.
type UpdateSessionRequest struct {
	TrackSlug       string `json:"track_slug,omitempty"`
	DetectionMethod string `json:"detection_method,omitempty"`
	DriverID        string `json:"driver_id,omitempty"`
	DriverName      string `json:"driver_name,omitempty"`
}

// CreateSessionResponse is the response from session creation.
type CreateSessionResponse struct {
	SessionID string `json:"session_id"`
}

// RecordLapRequest is the request body for recording a lap.
type RecordLapRequest struct {
	SessionID  string `json:"session_id"`
	LapTimeMs  int    `json:"lap_time_ms"`
	LapNumber  int    `json:"lap_number"`
	RecordedAt string `json:"recorded_at"`
}

// RecordInfo describes a record broken by a lap.
type RecordInfo struct {
	Type           string `json:"type"`
	PreviousTimeMs int    `json:"previous_time_ms"`
	PreviousDriver string `json:"previous_driver"`
}

// RecordLapResponse is the response from recording a lap.
type RecordLapResponse struct {
	LapID   string       `json:"lap_id"`
	Records []RecordInfo `json:"records"`
}

// HeartbeatRequest is the request body for a heartbeat.
type HeartbeatRequest struct {
	Status           string `json:"status"`
	CurrentSessionID string `json:"current_session_id,omitempty"`
	UptimeSeconds    int    `json:"uptime_seconds"`
}

// CarSync represents a car to sync to the API.
type CarSync struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	Manufacturer string  `json:"manufacturer"`
	Category     string  `json:"category"`
	PPStock      float64 `json:"pp_stock"`
}

// TrackSync represents a track to sync to the API.
type TrackSync struct {
	Name         string `json:"name"`
	Layout       string `json:"layout"`
	Slug         string `json:"slug"`
	Country      string `json:"country"`
	LengthMeters int    `json:"length_meters"`
	NumCorners   int    `json:"num_corners"`
	HasWeather   bool   `json:"has_weather"`
}

// CreateSession creates a new driving session.
func (c *Client) CreateSession(req CreateSessionRequest) (*CreateSessionResponse, error) {
	var resp CreateSessionResponse
	if err := c.doPost("/api/ingest/sessions", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdateSession updates a session with track or detection info.
func (c *Client) UpdateSession(sessionID string, req UpdateSessionRequest) error {
	return c.doPost("/api/ingest/sessions/"+sessionID+"/update", req, nil)
}

// RecordLap records a lap in an existing session.
func (c *Client) RecordLap(req RecordLapRequest) (*RecordLapResponse, error) {
	var resp RecordLapResponse
	if err := c.doPost("/api/ingest/laps", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// EndSession marks a session as ended.
func (c *Client) EndSession(sessionID string, endedAt time.Time) error {
	body := struct {
		EndedAt string `json:"ended_at"`
	}{
		EndedAt: endedAt.UTC().Format(time.RFC3339),
	}
	return c.doPost("/api/ingest/sessions/"+sessionID+"/end", body, nil)
}

// SendHeartbeat sends a service heartbeat.
func (c *Client) SendHeartbeat(req HeartbeatRequest) error {
	return c.doPost("/api/ingest/heartbeat", req, nil)
}

// SyncCars syncs car data to the API.
func (c *Client) SyncCars(cars []CarSync) error {
	body := struct {
		Cars []CarSync `json:"cars"`
	}{Cars: cars}
	return c.doPost("/api/ingest/cars", body, nil)
}

// SyncTrack syncs a single track to the API.
func (c *Client) SyncTrack(track TrackSync) error {
	return c.doPost("/api/ingest/tracks", track, nil)
}

// doPost sends a POST request with JSON body and optional response decoding.
func (c *Client) doPost(path string, reqBody interface{}, respBody interface{}) error {
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequest("POST", c.endpoint+path, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request to %s failed with status %d: %s", path, resp.StatusCode, string(body))
	}

	if respBody != nil {
		if err := json.NewDecoder(resp.Body).Decode(respBody); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}

	return nil
}
