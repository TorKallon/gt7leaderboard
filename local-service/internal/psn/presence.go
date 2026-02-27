package psn

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// presenceBaseURL can be overridden in tests.
var presenceBaseURL = "https://m.np.playstation.com/api/userProfile/v2/internal/users"

// profileBaseURL can be overridden in tests.
var profileBaseURL = "https://us-prof.np.community.playstation.net/userProfile/v1/users"

// GetPresence fetches the presence status for a single PSN account ID.
func (c *Client) GetPresence(accountID string) (*BasicPresence, error) {
	c.mu.Lock()
	if err := c.ensureValidToken(); err != nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("ensuring valid token: %w", err)
	}
	accessToken := c.tokens.AccessToken
	c.mu.Unlock()

	reqURL := fmt.Sprintf("%s/%s/basicPresences?type=primary", presenceBaseURL, accountID)

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating presence request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing presence request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("presence request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var presenceResp BasicPresence
	if err := json.NewDecoder(resp.Body).Decode(&presenceResp); err != nil {
		return nil, fmt.Errorf("decoding presence response: %w", err)
	}
	presenceResp.AccountID = accountID

	return &presenceResp, nil
}

// IdentifyDriver checks presence for the given accounts and returns the one
// currently playing GT7. Returns empty strings if no one or multiple users
// are playing GT7.
func (c *Client) IdentifyDriver(accounts []AccountConfig) (accountID string, driverName string, err error) {
	if len(accounts) == 0 {
		return "", "", nil
	}

	var matches []AccountConfig
	for _, a := range accounts {
		if a.AccountID == "" {
			continue
		}
		presence, err := c.GetPresence(a.AccountID)
		if err != nil {
			log.Printf("Warning: presence check failed for %s: %v", a.OnlineID, err)
			continue
		}
		if presence.IsPlayingGT7() {
			matches = append(matches, a)
		}
	}

	if len(matches) == 0 {
		return "", "", nil
	}
	if len(matches) > 1 {
		return "", "", nil
	}

	return matches[0].AccountID, matches[0].DriverName, nil
}

// profileResponse represents the relevant parts of a PSN profile lookup.
type profileResponse struct {
	Profile struct {
		AccountID string `json:"accountId"`
	} `json:"profile"`
}

// ResolveOnlineID converts a PSN online ID (gamertag) to the numeric account ID.
func (c *Client) ResolveOnlineID(onlineID string) (string, error) {
	c.mu.Lock()
	if err := c.ensureValidToken(); err != nil {
		c.mu.Unlock()
		return "", fmt.Errorf("ensuring valid token: %w", err)
	}
	accessToken := c.tokens.AccessToken
	c.mu.Unlock()

	reqURL := fmt.Sprintf(profileBaseURL+"/%s/profile2?fields=accountId,onlineId,currentOnlineId", onlineID)

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating profile request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("executing profile request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("profile request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var profileResp profileResponse
	if err := json.NewDecoder(resp.Body).Decode(&profileResp); err != nil {
		return "", fmt.Errorf("decoding profile response: %w", err)
	}

	return profileResp.Profile.AccountID, nil
}
