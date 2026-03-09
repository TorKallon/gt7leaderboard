package webui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/rourkem/gt7leaderboard/local-service/internal/config"
	"github.com/rourkem/gt7leaderboard/local-service/internal/psn"
	"github.com/rourkem/gt7leaderboard/local-service/internal/session"
)

//go:embed templates/*.html
var templateFS embed.FS

// SessionProvider returns the current active session, if any.
type SessionProvider interface {
	CurrentSession() *session.ActiveSession
}

// Server serves the local web UI for status display and PSN token management.
type Server struct {
	addr       string
	configPath string
	cfg        *config.Config
	psnClient  *psn.Client
	sessions   SessionProvider
	startedAt  time.Time
	templates  *template.Template
	httpServer *http.Server
}

// NewServer creates a new web UI server.
func NewServer(addr string, cfg *config.Config, psnClient *psn.Client, sessions SessionProvider, configPath string) *Server {
	tmpl := template.Must(template.ParseFS(templateFS, "templates/*.html"))
	return &Server{
		addr:       addr,
		configPath: configPath,
		cfg:        cfg,
		psnClient:  psnClient,
		sessions:   sessions,
		startedAt:  time.Now(),
		templates:  tmpl,
	}
}

// Start starts the HTTP server. It blocks until the context is cancelled.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleStatus)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/auth", s.handleAuth)

	s.httpServer = &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	// Shutdown when context is cancelled.
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("Web UI listening on %s", s.addr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("web UI server: %w", err)
	}
	return nil
}

// statusData holds data passed to the status page template.
type statusData struct {
	Uptime          string
	UptimeSeconds   int
	HasSession      bool
	DriverName      string
	TrackName       string
	CarID           int32
	LastLap         int16
	SessionStarted  string
	PSNTokenDays    int
	PSNTokenWarning bool
	PSNTokenMessage string
	LeaderboardURL  string
}

// handleStatus renders the status page.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uptime := time.Since(s.startedAt)
	data := statusData{
		Uptime:         formatDuration(uptime),
		UptimeSeconds:  int(uptime.Seconds()),
		LeaderboardURL: s.cfg.API.Endpoint,
	}

	// Populate session info.
	if s.sessions != nil {
		if sess := s.sessions.CurrentSession(); sess != nil {
			data.HasSession = true
			data.DriverName = sess.DriverName
			data.TrackName = sess.TrackName
			data.CarID = sess.CarID
			data.LastLap = sess.LastLap
			data.SessionStarted = sess.StartedAt.Format("15:04:05")
		}
	}

	// Populate PSN token info.
	if s.psnClient != nil {
		tokens := s.psnClient.GetTokens()
		if tokens != nil && !tokens.RefreshTokenExpiresAt.IsZero() {
			data.PSNTokenDays = tokens.DaysUntilRefreshExpiry()
			if data.PSNTokenDays < 14 {
				data.PSNTokenWarning = true
			}
			if needs, msg := tokens.NeedsReminder(); needs {
				data.PSNTokenMessage = msg
			}
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "status.html", data); err != nil {
		log.Printf("Error rendering status template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// healthResponse is the JSON response for the /health endpoint.
type healthResponse struct {
	Status        string `json:"status"`
	UptimeSeconds int    `json:"uptime_seconds"`
}

// handleHealth returns a JSON health check response.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp := healthResponse{
		Status:        "ok",
		UptimeSeconds: int(time.Since(s.startedAt).Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleAuth serves the auth form (GET) or processes NPSSO token submission (POST).
func (s *Server) handleAuth(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleAuthGet(w, r)
	case http.MethodPost:
		s.handleAuthPost(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// authData holds data passed to the auth page template.
type authData struct {
	Error   string
	Success bool
}

// handleAuthGet renders the NPSSO token submission form.
func (s *Server) handleAuthGet(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, "auth.html", authData{}); err != nil {
		log.Printf("Error rendering auth template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleAuthPost processes the NPSSO token form submission.
func (s *Server) handleAuthPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	npsso := r.FormValue("npsso")
	if npsso == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		s.templates.ExecuteTemplate(w, "auth.html", authData{Error: "NPSSO token cannot be empty"})
		return
	}

	if s.psnClient == nil {
		http.Error(w, "PSN client not configured", http.StatusInternalServerError)
		return
	}

	if err := s.psnClient.AuthenticateWithNPSSO(npsso); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		s.templates.ExecuteTemplate(w, "auth.html", authData{
			Error: fmt.Sprintf("Authentication failed: %v", err),
		})
		return
	}

	// Persist the new token to the config file so it survives restarts.
	if s.configPath != "" {
		if err := updateNPSSOToken(s.configPath, npsso); err != nil {
			log.Printf("Warning: failed to persist NPSSO token to config: %v", err)
		} else {
			log.Printf("NPSSO token persisted to %s", s.configPath)
		}
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// formatDuration formats a duration as "Xh Ym Zs".
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
