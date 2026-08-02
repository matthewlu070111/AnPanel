package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	return cfg, nil
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
