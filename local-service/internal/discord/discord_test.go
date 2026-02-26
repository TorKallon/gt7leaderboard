package discord

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFormatLapTime(t *testing.T) {
	tests := []struct {
		name     string
		ms       int
		expected string
	}{
		{"1:42.387", 102387, "1:42.387"},
		{"1:02.100", 62100, "1:02.100"},
		{"3:05.432", 185432, "3:05.432"},
		{"0:45.123 (under a minute)", 45123, "45.123"},
		{"exact minute", 60000, "1:00.000"},
		{"zero", 0, "0.000"},
		{"just millis", 500, "0.500"},
		{"10+ minutes", 600000, "10:00.000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatLapTime(tt.ms)
			if got != tt.expected {
				t.Errorf("FormatLapTime(%d) = %q, want %q", tt.ms, got, tt.expected)
			}
		})
	}
}

func TestSendRecordNotification(t *testing.T) {
	var receivedPayload webhookPayload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", contentType)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		if err := json.Unmarshal(body, &receivedPayload); err != nil {
			t.Fatalf("failed to unmarshal payload: %v", err)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL)

	notif := RecordNotification{
		DriverName:     "Alice",
		TrackName:      "Tsukuba Circuit",
		TrackSlug:      "tsukuba-circuit",
		CarName:        "Mazda RX-7",
		Category:       "N300",
		LapTime:        "1:02.100",
		LapTimeMs:      62100,
		PreviousTimeMs: 63500,
		PreviousDriver: "Bob",
		RecordType:     "overall",
	}

	err := client.SendRecordNotification(notif)
	if err != nil {
		t.Fatalf("SendRecordNotification failed: %v", err)
	}

	if len(receivedPayload.Embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(receivedPayload.Embeds))
	}

	emb := receivedPayload.Embeds[0]
	if emb.Title != "New overall Record!" {
		t.Errorf("unexpected embed title: %q", emb.Title)
	}
	if emb.Color != 0xFFD700 {
		t.Errorf("unexpected embed color: %d", emb.Color)
	}

	// Check fields
	fieldMap := make(map[string]string)
	for _, f := range emb.Fields {
		fieldMap[f.Name] = f.Value
	}

	if fieldMap["Lap Time"] != "1:02.100" {
		t.Errorf("unexpected Lap Time field: %q", fieldMap["Lap Time"])
	}
	if fieldMap["Car"] != "Mazda RX-7" {
		t.Errorf("unexpected Car field: %q", fieldMap["Car"])
	}
	if fieldMap["Category"] != "N300" {
		t.Errorf("unexpected Category field: %q", fieldMap["Category"])
	}
	if fieldMap["Previous Holder"] != "Bob" {
		t.Errorf("unexpected Previous Holder field: %q", fieldMap["Previous Holder"])
	}
}

func TestSendRecordNotification_NoPreviousRecord(t *testing.T) {
	var receivedPayload webhookPayload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedPayload)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL)

	notif := RecordNotification{
		DriverName: "Alice",
		TrackName:  "Suzuka",
		CarName:    "Honda NSX",
		Category:   "N500",
		LapTime:    "2:10.000",
		LapTimeMs:  130000,
		RecordType: "category",
	}

	err := client.SendRecordNotification(notif)
	if err != nil {
		t.Fatalf("SendRecordNotification failed: %v", err)
	}

	// Should not have Previous Record or Previous Holder fields
	for _, f := range receivedPayload.Embeds[0].Fields {
		if f.Name == "Previous Record" || f.Name == "Previous Holder" {
			t.Errorf("should not have %q field when no previous record", f.Name)
		}
	}
}

func TestSendRecordNotification_WebhookError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message": "invalid webhook"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)

	notif := RecordNotification{
		DriverName: "Alice",
		TrackName:  "Suzuka",
		CarName:    "Honda NSX",
		LapTime:    "2:10.000",
		LapTimeMs:  130000,
		RecordType: "overall",
	}

	err := client.SendRecordNotification(notif)
	if err == nil {
		t.Fatal("expected error on bad webhook response")
	}
}
