package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCreateSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/ingest/sessions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("unexpected Authorization: %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected Content-Type: %q", r.Header.Get("Content-Type"))
		}

		var req CreateSessionRequest
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &req)

		if req.DriverID != "driver-123" {
			t.Errorf("unexpected driver_id: %q", req.DriverID)
		}
		if req.TrackSlug != "tsukuba" {
			t.Errorf("unexpected track_slug: %q", req.TrackSlug)
		}
		if req.CarID != 42 {
			t.Errorf("unexpected car_id: %d", req.CarID)
		}

		resp := CreateSessionResponse{SessionID: "session-abc"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")

	resp, err := client.CreateSession(CreateSessionRequest{
		DriverID:        "driver-123",
		TrackSlug:       "tsukuba",
		CarID:           42,
		StartedAt:       "2026-01-01T00:00:00Z",
		DetectionMethod: "telemetry",
	})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if resp.SessionID != "session-abc" {
		t.Errorf("expected session_id %q, got %q", "session-abc", resp.SessionID)
	}
}

func TestRecordLap(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/ingest/laps" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var req RecordLapRequest
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &req)

		if req.SessionID != "session-abc" {
			t.Errorf("unexpected session_id: %q", req.SessionID)
		}
		if req.LapTimeMs != 62100 {
			t.Errorf("unexpected lap_time_ms: %d", req.LapTimeMs)
		}
		if req.LapNumber != 3 {
			t.Errorf("unexpected lap_number: %d", req.LapNumber)
		}

		resp := RecordLapResponse{
			LapID: "lap-xyz",
			Records: []RecordInfo{
				{Type: "overall", PreviousTimeMs: 63500, PreviousDriver: "Bob"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")

	resp, err := client.RecordLap(RecordLapRequest{
		SessionID:  "session-abc",
		LapTimeMs:  62100,
		LapNumber:  3,
		RecordedAt: "2026-01-01T00:05:00Z",
	})
	if err != nil {
		t.Fatalf("RecordLap failed: %v", err)
	}
	if resp.LapID != "lap-xyz" {
		t.Errorf("expected lap_id %q, got %q", "lap-xyz", resp.LapID)
	}
	if len(resp.Records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(resp.Records))
	}
	if resp.Records[0].Type != "overall" {
		t.Errorf("expected record type %q, got %q", "overall", resp.Records[0].Type)
	}
	if resp.Records[0].PreviousTimeMs != 63500 {
		t.Errorf("expected previous_time_ms 63500, got %d", resp.Records[0].PreviousTimeMs)
	}
}

func TestEndSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/ingest/sessions/session-abc/end" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("unexpected Authorization: %q", r.Header.Get("Authorization"))
		}

		var body struct {
			EndedAt string `json:"ended_at"`
		}
		raw, _ := io.ReadAll(r.Body)
		json.Unmarshal(raw, &body)

		if body.EndedAt == "" {
			t.Error("expected non-empty ended_at")
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")

	err := client.EndSession("session-abc", time.Date(2026, 1, 1, 0, 10, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("EndSession failed: %v", err)
	}
}

func TestSendHeartbeat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/ingest/heartbeat" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var req HeartbeatRequest
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &req)

		if req.Status != "active" {
			t.Errorf("unexpected status: %q", req.Status)
		}
		if req.UptimeSeconds != 3600 {
			t.Errorf("unexpected uptime: %d", req.UptimeSeconds)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")

	err := client.SendHeartbeat(HeartbeatRequest{
		Status:           "active",
		CurrentSessionID: "session-abc",
		UptimeSeconds:    3600,
	})
	if err != nil {
		t.Fatalf("SendHeartbeat failed: %v", err)
	}
}

func TestSyncCars(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/ingest/cars" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var body struct {
			Cars []CarSync `json:"cars"`
		}
		raw, _ := io.ReadAll(r.Body)
		json.Unmarshal(raw, &body)

		if len(body.Cars) != 2 {
			t.Errorf("expected 2 cars, got %d", len(body.Cars))
		}
		if body.Cars[0].Name != "Mazda RX-7" {
			t.Errorf("unexpected car name: %q", body.Cars[0].Name)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")

	err := client.SyncCars([]CarSync{
		{ID: 1, Name: "Mazda RX-7", Manufacturer: "Mazda", Category: "N300", PPStock: 295.5},
		{ID: 2, Name: "Honda NSX", Manufacturer: "Honda", Category: "N500", PPStock: 480.0},
	})
	if err != nil {
		t.Fatalf("SyncCars failed: %v", err)
	}
}

func TestSyncTrack(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/ingest/tracks" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var track TrackSync
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &track)

		if track.Slug != "tsukuba" {
			t.Errorf("unexpected slug: %q", track.Slug)
		}
		if track.LengthMeters != 2045 {
			t.Errorf("unexpected length: %d", track.LengthMeters)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key")

	err := client.SyncTrack(TrackSync{
		Name:         "Tsukuba Circuit",
		Layout:       "Full Course",
		Slug:         "tsukuba",
		Country:      "Japan",
		LengthMeters: 2045,
		NumCorners:   10,
		HasWeather:   true,
	})
	if err != nil {
		t.Fatalf("SyncTrack failed: %v", err)
	}
}

func TestAPIErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid api key"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "bad-key")

	_, err := client.CreateSession(CreateSessionRequest{
		CarID:     1,
		StartedAt: "2026-01-01T00:00:00Z",
	})
	if err == nil {
		t.Fatal("expected error on 401 response")
	}
}
