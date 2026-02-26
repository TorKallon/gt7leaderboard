package psn

import (
	"math"
	"strings"
	"time"
)

const (
	// AuthorizeURL is the Sony OAuth authorization endpoint.
	AuthorizeURL = "https://ca.account.sony.com/api/authz/v3/oauth/authorize"
	// TokenURL is the Sony OAuth token endpoint.
	TokenURL = "https://ca.account.sony.com/api/authz/v3/oauth/token"
	// PresenceURL is the PSN bulk presence API endpoint.
	PresenceURL = "https://m.np.playstation.com/api/userProfile/v2/internal/users//basicPresences"
	// ProfileURL is the PSN profile API endpoint (format with online ID).
	ProfileURL = "https://us-prof.np.community.playstation.net/userProfile/v1/users/%s/profile2"
	// ClientID is the PSN Android app client ID.
	ClientID = "09515159-7237-4370-9b40-3806e67c0891"
	// ClientSecret is the PSN Android app client secret.
	ClientSecret = "ucPjka5tntB2KqsP"
	// RedirectURI is the PSN Android app redirect URI.
	RedirectURI = "com.scee.psxandroid.scecompcall://redirect"
	// Scopes are the required OAuth scopes.
	Scopes = "psn:mobile.v2.core psn:clientapp"
	// BasicAuth is the base64-encoded client_id:client_secret.
	BasicAuth = "MDk1MTUxNTktNzIzNy00MzcwLTliNDAtMzgwNmU2N2MwODkxOnVjUGprYTV0bnRCMktxc1A="
	// GT7TitlePS5 is the PS5 title ID for Gran Turismo 7.
	GT7TitlePS5 = "PPSA01316_00"
	// GT7TitlePS4 is the PS4 title ID for Gran Turismo 7.
	GT7TitlePS4 = "CUSA24767_00"
)

// Tokens holds PSN OAuth tokens and their expiry times.
type Tokens struct {
	AccessToken          string    `json:"access_token"`
	RefreshToken         string    `json:"refresh_token"`
	AccessTokenExpiresAt time.Time `json:"access_token_expires_at"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at"`
	NpssoSetAt           time.Time `json:"npsso_set_at"`
}

// DaysUntilRefreshExpiry returns the number of whole days until the refresh token expires.
func (t *Tokens) DaysUntilRefreshExpiry() int {
	d := time.Until(t.RefreshTokenExpiresAt).Hours() / 24
	return int(math.Floor(d))
}

// NeedsReminder returns true and a message at specific day thresholds before
// the refresh token expires: 14, 7, 3, 1, and 0 days.
func (t *Tokens) NeedsReminder() (bool, string) {
	days := t.DaysUntilRefreshExpiry()
	switch days {
	case 14:
		return true, "PSN refresh token expires in 14 days. Please renew your NPSSO token soon."
	case 7:
		return true, "PSN refresh token expires in 7 days. Please renew your NPSSO token."
	case 3:
		return true, "PSN refresh token expires in 3 days! Renew your NPSSO token immediately."
	case 1:
		return true, "PSN refresh token expires tomorrow! Renew your NPSSO token now."
	case 0:
		return true, "PSN refresh token expires today! Service will stop working without a new NPSSO token."
	default:
		return false, ""
	}
}

// PresenceResponse is the top-level response from the PSN presence API.
type PresenceResponse struct {
	BasicPresences []BasicPresence `json:"basicPresences"`
}

// BasicPresence represents a user's online presence information.
type BasicPresence struct {
	AccountID         string          `json:"accountId"`
	Availability      string          `json:"availability"`
	GameTitleInfoList []GameTitleInfo `json:"gameTitleInfoList"`
}

// GameTitleInfo describes a game title a user is currently playing.
type GameTitleInfo struct {
	NpTitleID string `json:"npTitleId"`
	TitleName string `json:"titleName"`
	Format    string `json:"format"`
}

// PlatformInfo describes a platform a user is active on.
type PlatformInfo struct {
	OnlineStatus string `json:"onlineStatus"`
	Platform     string `json:"platform"`
}

// AccountConfig maps a PSN online ID to a driver name and account ID.
type AccountConfig struct {
	OnlineID   string `json:"online_id"`
	AccountID  string `json:"account_id"`
	DriverName string `json:"driver_name"`
}

// IsPlayingGT7 checks whether the user is currently playing Gran Turismo 7,
// matching by title ID (PS4 or PS5) or by title name (case-insensitive).
func (bp *BasicPresence) IsPlayingGT7() bool {
	for _, g := range bp.GameTitleInfoList {
		if g.NpTitleID == GT7TitlePS5 || g.NpTitleID == GT7TitlePS4 {
			return true
		}
		if strings.Contains(strings.ToLower(g.TitleName), "gran turismo 7") {
			return true
		}
	}
	return false
}
