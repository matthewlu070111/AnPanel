package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/matthewlu070111/anpanel/internal/agent"
	"github.com/matthewlu070111/anpanel/internal/auth"
	"github.com/matthewlu070111/anpanel/internal/config"
	"github.com/matthewlu070111/anpanel/internal/domain"
	"github.com/matthewlu070111/anpanel/internal/notify"
	"github.com/matthewlu070111/anpanel/internal/store"
	"github.com/matthewlu070111/anpanel/internal/webui"
)

// Version is injected from main at process start.
var Version = "dev"

type server struct {
	cfg         config.Config
	db          *store.Store
	agent       *agent.Client
	log         *slog.Logger
	attempts    *loginLimiter
	latestMu    sync.RWMutex
	latest      domain.HostSnapshot
	alertMu     sync.Mutex
	alertStates map[int64]*alertState
}

func Run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := store.Open(cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer db.Close()
	ac, err := agent.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("connect agent: %w", err)
	}
	s := &server{cfg: cfg, db: db, agent: ac, log: logger, attempts: newLoginLimiter(), alertStates: map[int64]*alertState{}}
	mux := http.NewServeMux()
	s.routes(mux)
	h := &http.Server{Addr: net.JoinHostPort(cfg.Listen, strconv.Itoa(cfg.Port)), Handler: securityHeaders(mux), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 0, IdleTimeout: 60 * time.Second}
	go s.collect(ctx)
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = h.Shutdown(shutdown)
	}()
	logger.Info("web listening", "address", h.Addr)
	err = h.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *server) routes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/auth/login", s.login)
	mux.HandleFunc("/api/v1/auth/logout", s.withSession(s.logout, false))
	mux.HandleFunc("/api/v1/me", s.withSession(s.me, false))
	mux.HandleFunc("/api/v1/me/change", s.withSession(s.changeCredentials, true))
	mux.HandleFunc("/api/v1/me/totp/setup", s.withSession(s.totpSetup, true))
	mux.HandleFunc("/api/v1/me/totp/enable", s.withSession(s.totpEnable, true))
	mux.HandleFunc("/api/v1/me/totp/disable", s.withSession(s.totpDisable, true))
	mux.HandleFunc("/api/v1/overview", s.withSession(s.overview, false))
	mux.HandleFunc("/api/v1/metrics/history", s.withSession(s.metricsHistory, false))
	mux.HandleFunc("/api/v1/ws/metrics", s.metricsWS)
	mux.HandleFunc("/api/v1/ws/docker/terminal", s.dockerTerminalWS)
	mux.HandleFunc("/api/v1/services", s.withSession(s.proxyGet("/v1/services"), false))
	mux.HandleFunc("/api/v1/docker/containers", s.withSession(s.proxyGet("/v1/docker/containers"), false))
	mux.HandleFunc("/api/v1/docker/inventory/", s.withSession(s.proxyInventory, false))
	mux.HandleFunc("/api/v1/websites", s.withSession(s.proxyGet("/v1/websites"), false))
	mux.HandleFunc("/api/v1/websites/config", s.withSession(s.proxyQuery("/v1/websites/config"), false))
	mux.HandleFunc("/api/v1/rewrite-rules", s.withSession(s.proxyGet("/v1/rewrite-rules"), false))
	mux.HandleFunc("/api/v1/certificates", s.withSession(s.proxyGet("/v1/certificates"), false))
	mux.HandleFunc("/api/v1/files", s.withSession(s.proxyQuery("/v1/files"), false))
	mux.HandleFunc("/api/v1/files/content", s.withSession(s.proxyQuery("/v1/files/content"), false))
	mux.HandleFunc("/api/v1/crontab", s.withSession(s.proxyGet("/v1/crontab"), false))
	mux.HandleFunc("/api/v1/system", s.withSession(s.systemInfo, false))
	mux.HandleFunc("/api/v1/tasks", s.withSession(s.tasks, false))
	mux.HandleFunc("/api/v1/audits", s.withSession(s.audits, false))
	mux.HandleFunc("/api/v1/alerts/rules", s.withSession(s.alertRules, false))
	mux.HandleFunc("/api/v1/alerts/rules/update", s.withSession(s.updateAlertRule, true))
	mux.HandleFunc("/api/v1/alerts/test", s.withSession(s.testNotification, true))
	mux.HandleFunc("/api/v1/actions", s.withSession(s.action, true))
	dist, _ := fs.Sub(webui.Dist, "dist")
	files := http.FileServer(http.FS(dist))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path != "/" {
			if _, err := fs.Stat(dist, strings.TrimPrefix(r.URL.Path, "/")); err == nil {
				files.ServeHTTP(w, r)
				return
			}
		}
		r.URL.Path = "/"
		files.ServeHTTP(w, r)
	})
}

type contextKey string

const sessionKey contextKey = "session"

func (s *server) withSession(next http.HandlerFunc, csrf bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("anpanel_session")
		if err != nil {
			apiError(w, 401, "authentication required")
			return
		}
		ss, err := s.db.Session(c.Value)
		if err != nil {
			apiError(w, 401, "session expired")
			return
		}
		if csrf && r.Header.Get("X-CSRF-Token") != ss.CSRF {
			apiError(w, 403, "invalid CSRF token")
			return
		}
		if ss.User.MustChange && !strings.HasPrefix(r.URL.Path, "/api/v1/me") && r.URL.Path != "/api/v1/auth/logout" {
			apiError(w, 428, "credentials must be changed")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), sessionKey, ss)))
	}
}
func current(r *http.Request) store.Session { return r.Context().Value(sessionKey).(store.Session) }

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		apiError(w, 405, "method not allowed")
		return
	}
	ip := remoteIP(r)
	if !s.attempts.allow(ip) {
		apiError(w, 429, "too many login attempts; try again later")
		return
	}
	var in struct{ Username, Password, TOTP string }
	if !decode(w, r, &in) {
		return
	}
	u, err := s.db.Authenticate(in.Username, in.Password)
	if err != nil || u.TOTPSecret != "" && !auth.VerifyTOTP(u.TOTPSecret, in.TOTP, time.Now()) {
		s.attempts.fail(ip)
		_ = s.db.Audit(in.Username, "auth.login_failed", "session", "invalid credentials", ip)
		apiError(w, 401, "invalid credentials")
		return
	}
	s.attempts.success(ip)
	ss, err := s.db.CreateSession(u)
	if err != nil {
		apiError(w, 500, "could not create session")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "anpanel_session", Value: ss.Token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Expires: ss.ExpiresAt, Secure: r.TLS != nil})
	_ = s.db.Audit(u.Username, "auth.login", "session", "login succeeded", ip)
	apiJSON(w, map[string]any{"username": u.Username, "must_change": u.MustChange, "csrf_token": ss.CSRF, "totp_enabled": u.TOTPSecret != ""})
}
func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	ss := current(r)
	_ = s.db.DeleteSession(ss.Token)
	http.SetCookie(w, &http.Cookie{Name: "anpanel_session", Path: "/", MaxAge: -1, HttpOnly: true})
	_ = s.db.Audit(ss.User.Username, "auth.logout", "session", "", remoteIP(r))
	apiJSON(w, map[string]bool{"ok": true})
}
func (s *server) me(w http.ResponseWriter, r *http.Request) {
	ss := current(r)
	apiJSON(w, map[string]any{"username": ss.User.Username, "must_change": ss.User.MustChange, "csrf_token": ss.CSRF, "totp_enabled": ss.User.TOTPSecret != ""})
}
func (s *server) changeCredentials(w http.ResponseWriter, r *http.Request) {
	var in struct{ Username, Password string }
	if !decode(w, r, &in) {
		return
	}
	if len(in.Username) < 3 || strings.ContainsAny(in.Username, " \t\r\n") {
		apiError(w, 400, "invalid username")
		return
	}
	if err := s.db.ChangeAdmin(in.Username, in.Password); err != nil {
		apiError(w, 400, err.Error())
		return
	}
	ss := current(r)
	_ = s.db.Audit(ss.User.Username, "account.change", "user:1", "credentials changed", remoteIP(r))
	apiJSON(w, map[string]bool{"ok": true})
}
func (s *server) totpSetup(w http.ResponseWriter, r *http.Request) {
	secret, err := auth.NewTOTPSecret()
	if err != nil {
		apiError(w, 500, err.Error())
		return
	}
	ss := current(r)
	if err := s.db.SetSetting("pending_totp", secret); err != nil {
		apiError(w, 500, err.Error())
		return
	}
	apiJSON(w, map[string]string{"secret": secret, "uri": auth.TOTPURI(secret, ss.User.Username)})
}
func (s *server) totpEnable(w http.ResponseWriter, r *http.Request) {
	var in struct{ Code string }
	if !decode(w, r, &in) {
		return
	}
	secret, err := s.db.Setting("pending_totp")
	if err != nil || !auth.VerifyTOTP(secret, in.Code, time.Now()) {
		apiError(w, 400, "invalid TOTP code")
		return
	}
	if err := s.db.SetTOTP(secret); err != nil {
		apiError(w, 500, err.Error())
		return
	}
	ss := current(r)
	_ = s.db.Audit(ss.User.Username, "account.totp_enable", "user:1", "", remoteIP(r))
	apiJSON(w, map[string]bool{"ok": true})
}
func (s *server) totpDisable(w http.ResponseWriter, r *http.Request) {
	var in struct{ Code string }
	if !decode(w, r, &in) {
		return
	}
	ss := current(r)
	if ss.User.TOTPSecret == "" || !auth.VerifyTOTP(ss.User.TOTPSecret, in.Code, time.Now()) {
		apiError(w, 400, "invalid TOTP code")
		return
	}
	_ = s.db.DisableTOTP()
	_ = s.db.Audit(ss.User.Username, "account.totp_disable", "user:1", "", remoteIP(r))
	apiJSON(w, map[string]bool{"ok": true})
}

func (s *server) overview(w http.ResponseWriter, r *http.Request) {
	s.latestMu.RLock()
	latest := s.latest
	s.latestMu.RUnlock()
	services := any(nil)
	_ = s.agent.Get(r.Context(), "/v1/services", &services)
	containers := any(nil)
	_ = s.agent.Get(r.Context(), "/v1/docker/containers", &containers)
	apiJSON(w, map[string]any{"snapshot": latest, "services": services, "containers": containers, "insecure_http": r.TLS == nil, "listen": s.cfg.Listen, "port": s.cfg.Port})
}
func (s *server) metricsHistory(w http.ResponseWriter, r *http.Request) {
	since := time.Now().Add(-24 * time.Hour)
	if v := r.URL.Query().Get("hours"); v != "" {
		if h, e := strconv.Atoi(v); e == nil && h > 0 && h <= 2160 {
			since = time.Now().Add(-time.Duration(h) * time.Hour)
		}
	}
	items, err := s.db.Metrics(since, 10000)
	if err != nil {
		apiError(w, 500, err.Error())
		return
	}
	apiJSON(w, items)
}
func (s *server) proxyGet(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var out any
		if err := s.agent.Get(r.Context(), path, &out); err != nil {
			apiError(w, 503, err.Error())
			return
		}
		apiJSON(w, out)
	}
}

// proxyQuery forwards GET with the original query string to the agent.
func (s *server) proxyQuery(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		full := path
		if q := r.URL.RawQuery; q != "" {
			full = path + "?" + q
		}
		var out any
		if err := s.agent.Get(r.Context(), full, &out); err != nil {
			apiError(w, 503, err.Error())
			return
		}
		apiJSON(w, out)
	}
}

func (s *server) systemInfo(w http.ResponseWriter, r *http.Request) {
	var out any
	if err := s.agent.Get(r.Context(), "/v1/system", &out); err != nil {
		// Fall back to local version if agent is briefly unavailable.
		apiJSON(w, map[string]any{"version": Version, "channel": s.cfg.UpdateChannel})
		return
	}
	apiJSON(w, out)
}
func (s *server) proxyInventory(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimPrefix(r.URL.Path, "/api/v1/docker/inventory/")
	var out any
	if err := s.agent.Get(r.Context(), "/v1/docker/inventory/"+kind, &out); err != nil {
		apiError(w, 503, err.Error())
		return
	}
	apiJSON(w, out)
}
func (s *server) tasks(w http.ResponseWriter, r *http.Request) {
	items, err := s.db.Tasks(100)
	if err != nil {
		apiError(w, 500, err.Error())
		return
	}
	apiJSON(w, items)
}
func (s *server) audits(w http.ResponseWriter, r *http.Request) {
	items, err := s.db.Audits(100)
	if err != nil {
		apiError(w, 500, err.Error())
		return
	}
	apiJSON(w, items)
}

func (s *server) alertRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		apiError(w, 405, "method not allowed")
		return
	}
	items, err := s.db.AlertRules()
	if err != nil {
		apiError(w, 500, err.Error())
		return
	}
	apiJSON(w, items)
}
func (s *server) updateAlertRule(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Operation string           `json:"operation"`
		Rule      domain.AlertRule `json:"rule"`
	}
	if !decode(w, r, &in) {
		return
	}
	ss := current(r)
	if in.Operation == "delete" {
		if in.Rule.ID < 1 {
			apiError(w, 400, "rule id is required")
			return
		}
		if err := s.db.DeleteAlertRule(in.Rule.ID); err != nil {
			apiError(w, 500, err.Error())
			return
		}
		_ = s.db.Audit(ss.User.Username, "alert.delete", strconv.FormatInt(in.Rule.ID, 10), "", remoteIP(r))
		apiJSON(w, map[string]bool{"ok": true})
		return
	}
	if !validAlertRule(in.Rule) {
		apiError(w, 400, "invalid alert rule")
		return
	}
	id, err := s.db.SaveAlertRule(in.Rule)
	if err != nil {
		apiError(w, 500, err.Error())
		return
	}
	_ = s.db.Audit(ss.User.Username, "alert.save", strconv.FormatInt(id, 10), in.Rule.Name, remoteIP(r))
	apiJSON(w, map[string]int64{"id": id})
}
func validAlertRule(r domain.AlertRule) bool {
	metrics := map[string]bool{"cpu": true, "memory": true, "disk": true, "load": true}
	return len(r.Name) > 0 && len(r.Name) <= 100 && metrics[r.Metric] && (r.Operator == "gt" || r.Operator == "lt") && r.DurationSeconds >= 0 && r.DurationSeconds <= 86400 && r.RepeatSeconds >= 60
}
func (s *server) testNotification(w http.ResponseWriter, r *http.Request) {
	cfg, err := notify.Load(s.cfg.NotificationPath)
	if err != nil {
		apiError(w, 400, err.Error())
		return
	}
	if err := notify.Send(cfg, notify.Event{Title: "AnPanel test notification", Message: "Notification delivery is configured correctly.", Severity: "info", Time: time.Now()}); err != nil {
		apiError(w, 502, err.Error())
		return
	}
	apiJSON(w, map[string]bool{"ok": true})
}
func (s *server) action(w http.ResponseWriter, r *http.Request) {
	var in agent.ActionRequest
	if !decode(w, r, &in) {
		return
	}
	ss := current(r)
	in.Actor = ss.User.Username
	id := randomID()
	now := time.Now()
	t := domain.Task{ID: id, Kind: in.Kind, Status: "queued", Summary: in.Kind + " " + in.Resource, CreatedAt: now, UpdatedAt: now}
	if err := s.db.CreateTask(t); err != nil {
		apiError(w, 500, err.Error())
		return
	}
	_ = s.db.Audit(ss.User.Username, "task.create", id, t.Summary, remoteIP(r))
	go func() {
		_ = s.db.UpdateTask(id, "running", "", nil)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		result, err := s.agent.Action(ctx, in)
		if err != nil {
			status := "failed"
			if result.RolledBack {
				status = "rolled_back"
			}
			_ = s.db.UpdateTask(id, status, err.Error(), result)
			return
		}
		_ = s.db.UpdateTask(id, "succeeded", result.Output, result)
	}()
	apiJSONStatus(w, 202, map[string]string{"task_id": id})
}

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
	return r.Header.Get("Origin") == "" || strings.Contains(r.Header.Get("Origin"), r.Host)
}}

func (s *server) metricsWS(w http.ResponseWriter, r *http.Request) {
	wrapped := s.withSession(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		ticker := time.NewTicker(time.Duration(s.cfg.MetricsInterval) * time.Second)
		defer ticker.Stop()
		for {
			s.latestMu.RLock()
			latest := s.latest
			s.latestMu.RUnlock()
			if err := c.WriteJSON(latest); err != nil {
				return
			}
			<-ticker.C
		}
	}, false)
	wrapped(w, r)
}

func (s *server) dockerTerminalWS(w http.ResponseWriter, r *http.Request) {
	wrapped := s.withSession(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		agentConn, err := s.agent.DialTerminal(r.Context(), id)
		if err != nil {
			apiError(w, 502, err.Error())
			return
		}
		defer agentConn.Close()
		browser, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer browser.Close()
		done := make(chan struct{}, 2)
		relay := func(dst, src *websocket.Conn) {
			defer func() { done <- struct{}{} }()
			for {
				kind, msg, err := src.ReadMessage()
				if err != nil {
					return
				}
				if err = dst.WriteMessage(kind, msg); err != nil {
					return
				}
			}
		}
		go relay(agentConn, browser)
		go relay(browser, agentConn)
		<-done
	}, false)
	wrapped(w, r)
}
func (s *server) collect(ctx context.Context) {
	collectOnce := func() {
		m, err := s.agent.Snapshot(ctx)
		if err != nil {
			s.log.Debug("metrics collect failed", "error", err)
			return
		}
		s.latestMu.Lock()
		s.latest = m
		s.latestMu.Unlock()
		_ = s.db.SaveMetric(m)
		s.evaluateAlerts(m)
	}
	// Collect immediately so the dashboard is not empty until the first tick.
	collectOnce()
	ticker := time.NewTicker(time.Duration(s.cfg.MetricsInterval) * time.Second)
	defer ticker.Stop()
	prune := time.NewTicker(24 * time.Hour)
	defer prune.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			collectOnce()
		case <-prune.C:
			_ = s.db.PruneMetrics()
		}
	}
}

type alertState struct {
	Since, timeSent time.Time
	Active          bool
}

func (s *server) evaluateAlerts(m domain.HostSnapshot) {
	rules, err := s.db.AlertRules()
	if err != nil {
		return
	}
	now := time.Now()
	s.alertMu.Lock()
	defer s.alertMu.Unlock()
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		value := alertValue(r.Metric, m)
		firing := r.Operator == "gt" && value > r.Threshold || r.Operator == "lt" && value < r.Threshold
		st := s.alertStates[r.ID]
		if st == nil {
			st = &alertState{}
			s.alertStates[r.ID] = st
		}
		if !firing {
			if st.Active {
				s.sendAlert(r, value, true)
			}
			*st = alertState{}
			continue
		}
		if st.Since.IsZero() {
			st.Since = now
		}
		if now.Sub(st.Since) < time.Duration(r.DurationSeconds)*time.Second {
			continue
		}
		repeat := time.Duration(r.RepeatSeconds) * time.Second
		if !st.Active || now.Sub(st.timeSent) >= repeat {
			st.Active = true
			st.timeSent = now
			s.sendAlert(r, value, false)
		}
	}
}
func alertValue(metric string, m domain.HostSnapshot) float64 {
	switch metric {
	case "cpu":
		return m.CPUPercent
	case "memory":
		if m.MemoryTotal > 0 {
			return 100 * float64(m.MemoryUsed) / float64(m.MemoryTotal)
		}
	case "disk":
		if m.DiskTotal > 0 {
			return 100 * float64(m.DiskUsed) / float64(m.DiskTotal)
		}
	case "load":
		return m.Load1
	}
	return 0
}
func (s *server) sendAlert(r domain.AlertRule, value float64, recovered bool) {
	title := "Alert: " + r.Name
	severity := "warning"
	message := fmt.Sprintf("%s is %.2f (threshold %.2f)", r.Metric, value, r.Threshold)
	if recovered {
		title = "Recovered: " + r.Name
		severity = "recovery"
	}
	go func() {
		cfg, err := notify.Load(s.cfg.NotificationPath)
		if err != nil {
			return
		}
		if err := notify.Send(cfg, notify.Event{Title: title, Message: message, Severity: severity, Time: time.Now()}); err != nil {
			s.log.Error("send alert", "error", err)
		}
	}()
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Method != "POST" && r.Method != "PUT" && r.Method != "DELETE" {
		apiError(w, 405, "method not allowed")
		return false
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(v); err != nil {
		apiError(w, 400, "invalid JSON")
		return false
	}
	return true
}
func apiJSON(w http.ResponseWriter, v any) { apiJSONStatus(w, 200, v) }
func apiJSONStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func apiError(w http.ResponseWriter, status int, msg string) {
	apiJSONStatus(w, status, map[string]string{"error": msg})
}
func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
func randomID() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self' ws: wss:")
		next.ServeHTTP(w, r)
	})
}

type loginState struct {
	failures      int
	first, locked time.Time
}
type loginLimiter struct {
	mu sync.Mutex
	m  map[string]loginState
}

func newLoginLimiter() *loginLimiter { return &loginLimiter{m: map[string]loginState{}} }
func (l *loginLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	v := l.m[ip]
	return v.locked.IsZero() || time.Now().After(v.locked)
}
func (l *loginLimiter) fail(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	v := l.m[ip]
	now := time.Now()
	if v.first.IsZero() || now.Sub(v.first) > 15*time.Minute {
		v = loginState{first: now}
	}
	v.failures++
	if v.failures >= 5 {
		v.locked = now.Add(15 * time.Minute)
	}
	l.m[ip] = v
}
func (l *loginLimiter) success(ip string) { l.mu.Lock(); delete(l.m, ip); l.mu.Unlock() }
