package agent

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/matthewlu070111/anpanel/internal/config"
	"github.com/matthewlu070111/anpanel/internal/system"
)

type ActionRequest struct {
	Kind     string            `json:"kind"`
	Resource string            `json:"resource"`
	Options  map[string]string `json:"options"`
	Actor    string            `json:"actor"`
}
type ActionResult struct {
	Output     string `json:"output"`
	Data       any    `json:"data,omitempty"`
	RolledBack bool   `json:"rolled_back"`
}

func Run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	token, err := os.ReadFile(cfg.AgentTokenFile)
	if err != nil {
		return fmt.Errorf("read agent token: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.AgentSocket), 0750); err != nil {
		return err
	}
	_ = os.Remove(cfg.AgentSocket)
	ln, err := net.Listen("unix", cfg.AgentSocket)
	if err != nil {
		return err
	}
	defer ln.Close()
	_ = os.Chmod(cfg.AgentSocket, 0660)
	mux := http.NewServeMux()
	s := &server{token: strings.TrimSpace(string(token)), logger: logger}
	mux.HandleFunc("/v1/health", s.auth(s.health))
	mux.HandleFunc("/v1/metrics", s.auth(s.metrics))
	mux.HandleFunc("/v1/services", s.auth(s.services))
	mux.HandleFunc("/v1/docker/containers", s.auth(s.containers))
	mux.HandleFunc("/v1/websites", s.auth(s.websites))
	mux.HandleFunc("/v1/websites/config", s.auth(s.websiteConfig))
	mux.HandleFunc("/v1/rewrite-rules", s.auth(s.rewriteRules))
	mux.HandleFunc("/v1/certificates", s.auth(s.certificates))
	mux.HandleFunc("/v1/files", s.auth(s.files))
	mux.HandleFunc("/v1/files/content", s.auth(s.fileContent))
	mux.HandleFunc("/v1/files/upload", s.auth(s.fileUpload))
	mux.HandleFunc("/v1/files/download", s.auth(s.fileDownload))
	mux.HandleFunc("/v1/crontab", s.auth(s.crontab))
	mux.HandleFunc("/v1/system", s.auth(s.system))
	mux.HandleFunc("/v1/action", s.auth(s.action))
	mux.HandleFunc("/v1/docker/inventory/", s.auth(s.inventory))
	mux.HandleFunc("/v1/docker/terminal", s.auth(s.terminal))
	mux.HandleFunc("/v1/host/terminal", s.auth(s.hostTerminal))
	h := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = h.Shutdown(shutdown)
	}()
	logger.Info("agent listening", "socket", cfg.AgentSocket)
	err = h.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

type server struct {
	token  string
	logger *slog.Logger
}

func (s *server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) != 1 {
			http.Error(w, "unauthorized", 401)
			return
		}
		next(w, r)
	}
}
func jsonOut(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func (s *server) health(w http.ResponseWriter, r *http.Request) {
	jsonOut(w, map[string]any{"ok": true, "time": time.Now()})
}
func (s *server) metrics(w http.ResponseWriter, r *http.Request) {
	m, err := system.Snapshot()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	jsonOut(w, m)
}
func (s *server) services(w http.ResponseWriter, r *http.Request) {
	jsonOut(w, system.DetectServices())
}
func (s *server) containers(w http.ResponseWriter, r *http.Request) {
	items, err := dockerContainers(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 503)
		return
	}
	jsonOut(w, items)
}
func (s *server) inventory(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimPrefix(r.URL.Path, "/v1/docker/inventory/")
	items, err := dockerInventory(r.Context(), kind)
	if err != nil {
		http.Error(w, err.Error(), 503)
		return
	}
	jsonOut(w, items)
}
func (s *server) websites(w http.ResponseWriter, r *http.Request) {
	items, err := discoverWebsites()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	jsonOut(w, items)
}
func (s *server) websiteConfig(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	raw, err := websiteConfig(path)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	jsonOut(w, map[string]string{"path": path, "content": raw})
}
func (s *server) rewriteRules(w http.ResponseWriter, r *http.Request) {
	jsonOut(w, rewriteTemplates())
}
func (s *server) certificates(w http.ResponseWriter, r *http.Request) {
	items, err := discoverCertificates()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	jsonOut(w, items)
}
func (s *server) files(w http.ResponseWriter, r *http.Request) {
	items, err := listFiles(r.URL.Query().Get("path"))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	jsonOut(w, items)
}
func (s *server) fileContent(w http.ResponseWriter, r *http.Request) {
	content, err := readFileContent(r.URL.Query().Get("path"))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	jsonOut(w, map[string]string{"path": r.URL.Query().Get("path"), "content": content})
}

func (s *server) fileUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	dir := r.URL.Query().Get("path")
	name := r.URL.Query().Get("name")
	overwrite := r.URL.Query().Get("overwrite") == "1" || strings.EqualFold(r.URL.Query().Get("overwrite"), "true")
	// Cap body slightly above max so saveUploadedFile can report a clean error.
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+1024)
	size := r.ContentLength
	if size <= 0 {
		size = -1
	}
	dst, err := saveUploadedFile(dir, name, r.Body, size, overwrite)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	s.logger.Info("file uploaded", "path", dst)
	jsonOut(w, map[string]string{"path": dst})
}

func (s *server) fileDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	f, size, name, err := openDownload(r.URL.Query().Get("path"))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(name, `"`, "")+`"`)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, name, time.Time{}, f)
}
func (s *server) crontab(w http.ResponseWriter, r *http.Request) {
	items, err := listCrontab(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	jsonOut(w, items)
}
func (s *server) system(w http.ResponseWriter, r *http.Request) {
	jsonOut(w, systemInfo(r.Context()))
}
func (s *server) action(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var a ActionRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&a); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	out, err := executeAction(r.Context(), a)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	s.logger.Info("privileged action", "kind", a.Kind, "resource", a.Resource, "actor", a.Actor)
	jsonOut(w, out)
}
