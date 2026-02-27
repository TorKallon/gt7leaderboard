package psn

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Client handles PSN OAuth authentication and API requests.
type Client struct {
	httpClient   *http.Client
	mu           sync.Mutex
	tokens       *Tokens
	authorizeURL string
	tokenURL     string
}

// NewClient creates a new PSN API client.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		tokens:       &Tokens{},
		authorizeURL: AuthorizeURL,
		tokenURL:     TokenURL,
	}
}

// newClientWithURLs creates a client with custom endpoint URLs (for testing).
func newClientWithURLs(authorizeURL, tokenURL string) *Client {
	c := NewClient()
	c.authorizeURL = authorizeURL
	c.tokenURL = tokenURL
	return c
}

// tokenResponse is the JSON response from the Sony token endpoint.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
}

// AuthenticateWithNPSSO authenticates using an NPSSO cookie value.
// Step 1: Exchange NPSSO for an authorization code via the authorize endpoint.
// Step 2: Exchange the authorization code for access and refresh tokens.
func (c *Client) AuthenticateWithNPSSO(npsso string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Step 1: Get authorization code
	code, err := c.getAuthCode(npsso)
	if err != nil {
		return fmt.Errorf("getting auth code: %w", err)
	}

	// Step 2: Exchange code for tokens
	if err := c.exchangeCodeForTokens(code); err != nil {
		return fmt.Errorf("exchanging code for tokens: %w", err)
	}

	c.tokens.NpssoSetAt = time.Now()
	return nil
}

func (c *Client) getAuthCode(npsso string) (string, error) {
	params := url.Values{
		"access_type":   {"offline"},
		"client_id":     {ClientID},
		"redirect_uri":  {RedirectURI},
		"response_type": {"code"},
		"scope":         {Scopes},
	}

	reqURL := c.authorizeURL + "?" + params.Encode()
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Cookie", "npsso="+npsso)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		return "", fmt.Errorf("unexpected status %d from authorize endpoint", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location == "" {
		return "", fmt.Errorf("no Location header in authorize response")
	}

	parsed, err := url.Parse(location)
	if err != nil {
		return "", fmt.Errorf("parsing redirect URL: %w", err)
	}

	code := parsed.Query().Get("code")
	if code == "" {
		return "", fmt.Errorf("no code in redirect URL: %s", location)
	}

	return code, nil
}

func (c *Client) exchangeCodeForTokens(code string) error {
	data := url.Values{
		"code":         {code},
		"grant_type":   {"authorization_code"},
		"redirect_uri": {RedirectURI},
	}

	req, err := http.NewRequest("POST", c.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+BasicAuth)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("decoding token response: %w", err)
	}

	now := time.Now()
	c.tokens.AccessToken = tokenResp.AccessToken
	c.tokens.RefreshToken = tokenResp.RefreshToken
	c.tokens.AccessTokenExpiresAt = now.Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	// Refresh tokens typically last ~60 days
	c.tokens.RefreshTokenExpiresAt = now.Add(60 * 24 * time.Hour)

	return nil
}

// RefreshAccessToken uses the refresh token to obtain a new access token.
func (c *Client) RefreshAccessToken() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.refreshAccessTokenLocked()
}

// refreshAccessTokenLocked performs token refresh. Must be called with c.mu held.
func (c *Client) refreshAccessTokenLocked() error {
	if c.tokens.RefreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	data := url.Values{
		"refresh_token": {c.tokens.RefreshToken},
		"grant_type":    {"refresh_token"},
		"scope":         {Scopes},
	}

	req, err := http.NewRequest("POST", c.tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("creating refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+BasicAuth)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing refresh request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("refresh request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("decoding refresh response: %w", err)
	}

	now := time.Now()
	c.tokens.AccessToken = tokenResp.AccessToken
	if tokenResp.RefreshToken != "" {
		c.tokens.RefreshToken = tokenResp.RefreshToken
	}
	c.tokens.AccessTokenExpiresAt = now.Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return nil
}

// SetTokens restores previously saved tokens.
func (c *Client) SetTokens(tokens *Tokens) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tokens = tokens
}

// GetTokens returns a copy of the current tokens, or nil if no tokens are set.
func (c *Client) GetTokens() *Tokens {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tokens == nil {
		return nil
	}
	t := *c.tokens
	return &t
}

// ensureValidToken checks if the access token is still valid and refreshes it if needed.
// Must be called with c.mu held.
func (c *Client) ensureValidToken() error {
	if c.tokens.AccessToken == "" {
		return fmt.Errorf("no access token available")
	}
	if time.Now().After(c.tokens.AccessTokenExpiresAt) {
		return c.refreshAccessTokenLocked()
	}
	return nil
}
