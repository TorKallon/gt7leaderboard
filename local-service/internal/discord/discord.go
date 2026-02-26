package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client sends notifications to a Discord webhook.
type Client struct {
	webhookURL string
	httpClient *http.Client
}

// NewClient creates a new Discord webhook client.
func NewClient(webhookURL string) *Client {
	return &Client{
		webhookURL: webhookURL,
		httpClient: &http.Client{},
	}
}

// RecordNotification contains the details of a new lap record to announce.
type RecordNotification struct {
	DriverName     string
	TrackName      string
	TrackSlug      string
	CarName        string
	Category       string
	LapTime        string
	LapTimeMs      int
	PreviousTimeMs int
	PreviousDriver string
	RecordType     string // "overall", "category", "car"
}

// embed represents a Discord embed object.
type embed struct {
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Color       int          `json:"color"`
	Fields      []embedField `json:"fields"`
}

// embedField represents a field within a Discord embed.
type embedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

// webhookPayload is the JSON body sent to the Discord webhook.
type webhookPayload struct {
	Embeds []embed `json:"embeds"`
}

// SendRecordNotification posts a record notification embed to the Discord webhook.
func (c *Client) SendRecordNotification(notif RecordNotification) error {
	title := fmt.Sprintf("New %s Record!", notif.RecordType)
	description := fmt.Sprintf("**%s** set a new %s record on **%s**!",
		notif.DriverName, notif.RecordType, notif.TrackName)

	fields := []embedField{
		{Name: "Lap Time", Value: notif.LapTime, Inline: true},
		{Name: "Car", Value: notif.CarName, Inline: true},
		{Name: "Category", Value: notif.Category, Inline: true},
	}

	if notif.PreviousTimeMs > 0 {
		improvement := notif.PreviousTimeMs - notif.LapTimeMs
		fields = append(fields, embedField{
			Name:   "Previous Record",
			Value:  fmt.Sprintf("%s (-%s)", FormatLapTime(notif.PreviousTimeMs), FormatLapTime(improvement)),
			Inline: true,
		})
		if notif.PreviousDriver != "" {
			fields = append(fields, embedField{
				Name:   "Previous Holder",
				Value:  notif.PreviousDriver,
				Inline: true,
			})
		}
	}

	// Gold color for records
	color := 0xFFD700

	payload := webhookPayload{
		Embeds: []embed{
			{
				Title:       title,
				Description: description,
				Color:       color,
				Fields:      fields,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling webhook payload: %w", err)
	}

	resp, err := c.httpClient.Post(c.webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("sending webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// FormatLapTime converts milliseconds to a human-readable lap time string.
// Examples: 102387 -> "1:42.387", 62100 -> "1:02.100", 185432 -> "3:05.432"
func FormatLapTime(ms int) string {
	if ms < 0 {
		ms = -ms
	}
	totalSeconds := ms / 1000
	millis := ms % 1000
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60

	if minutes > 0 {
		return fmt.Sprintf("%d:%02d.%03d", minutes, seconds, millis)
	}
	return fmt.Sprintf("%d.%03d", seconds, millis)
}
