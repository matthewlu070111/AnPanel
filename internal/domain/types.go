package domain

import "time"

type HostSnapshot struct {
	Time        time.Time `json:"time"`
	CPUPercent  float64   `json:"cpu_percent"`
	Load1       float64   `json:"load1"`
	MemoryTotal uint64    `json:"memory_total"`
	MemoryUsed  uint64    `json:"memory_used"`
	SwapTotal   uint64    `json:"swap_total"`
	SwapUsed    uint64    `json:"swap_used"`
	DiskTotal   uint64    `json:"disk_total"`
	DiskUsed    uint64    `json:"disk_used"`
	NetRX       uint64    `json:"net_rx"`
	NetTX       uint64    `json:"net_tx"`
	Uptime      uint64    `json:"uptime"`
}

type DetectedService struct {
	Name, Version, Path, Status, ConfigPath string
	Installed                               bool
}

type Container struct {
	ID     string   `json:"id"`
	Names  []string `json:"names"`
	Image  string   `json:"image"`
	State  string   `json:"state"`
	Status string   `json:"status"`
}

type ComposeProject struct{ Name, Status, ConfigPath string }

type WebSite struct {
	ID          string   `json:"id"`
	Server      string   `json:"server"`
	Name        string   `json:"name"`
	Domains     []string `json:"domains"`
	Listen      []string `json:"listen"`
	ProxyTarget string   `json:"proxy_target"`
	TLS         bool     `json:"tls"`
	Enabled     bool     `json:"enabled"`
	SourcePath  string   `json:"source_path"`
	Raw         string   `json:"raw"`
}

type Certificate struct {
	Domain, Issuer, Path string
	ExpiresAt            time.Time
}

type AlertRule struct {
	ID                                             int64 `json:"id"`
	Name, Metric, Operator                         string
	Threshold                                      float64
	DurationSeconds, SilenceSeconds, RepeatSeconds int
	Enabled                                        bool
}

type Task struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Status    string    `json:"status"`
	Summary   string    `json:"summary"`
	Log       string    `json:"log"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AuditEvent struct {
	ID                                        int64 `json:"id"`
	Actor, Action, Resource, Detail, RemoteIP string
	CreatedAt                                 time.Time `json:"created_at"`
}
