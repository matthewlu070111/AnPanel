package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Listen           string `json:"listen"`
	Port             int    `json:"port"`
	DatabasePath     string `json:"database_path"`
	AgentSocket      string `json:"agent_socket"`
	AgentTokenFile   string `json:"agent_token_file"`
	SessionKeyFile   string `json:"session_key_file"`
	NotificationPath string `json:"notification_path"`
	Region           string `json:"region"`
	MetricsInterval  int    `json:"metrics_interval_seconds"`
	UpdateChannel    string `json:"update_channel"`
	// EntryPath is the secret panel URL prefix (e.g. "s8k2m9" → http://host:port/s8k2m9).
	// Empty means not configured yet (must set after login).
	EntryPath string `json:"entry_path"`
	// DecoyMode is the public root disguise: "404" or "dino" (no redirects).
	DecoyMode string `json:"decoy_mode"`
}

func Default() Config {
	return Config{
		Listen: "0.0.0.0", Port: 8888,
		DatabasePath:     "/var/lib/anpanel/anpanel.db",
		AgentSocket:      "/run/anpanel/agent.sock",
		AgentTokenFile:   "/etc/anpanel/agent.token",
		SessionKeyFile:   "/etc/anpanel/session.key",
		NotificationPath: "/etc/anpanel/notifications.json",
		Region:           "auto", MetricsInterval: 5, UpdateChannel: "stable",
		EntryPath: "", DecoyMode: "404",
	}
}

func Path() string {
	if p := os.Getenv("ANPANEL_CONFIG"); p != "" {
		return p
	}
	return "/etc/anpanel/config.json"
}

func Load() (Config, error) {
	cfg := Default()
	b, err := os.ReadFile(Path())
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return cfg, fmt.Errorf("invalid port %d", cfg.Port)
	}
	if cfg.MetricsInterval < 1 {
		cfg.MetricsInterval = 5
	}
	if cfg.DecoyMode != "dino" {
		cfg.DecoyMode = "404"
	}
	cfg.EntryPath = NormalizeEntryPath(cfg.EntryPath)
	return cfg, nil
}

// NormalizeEntryPath strips slashes and rejects unsafe characters.
func NormalizeEntryPath(p string) string {
	p = strings.Trim(strings.TrimSpace(p), "/")
	if p == "" {
		return ""
	}
	// only allow simple path segments
	for _, c := range p {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			continue
		}
		return ""
	}
	if len(p) < 4 || len(p) > 64 {
		return ""
	}
	// reserve common paths
	switch strings.ToLower(p) {
	case "api", "assets", "static", "favicon.ico", "robots.txt":
		return ""
	}
	return p
}

func Save(cfg Config) error {
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0750); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0640); err != nil {
		return err
	}
	if err := preserveOwnership(tmp, p); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
