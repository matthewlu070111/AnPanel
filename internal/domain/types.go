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
	Name            string   `json:"name"`
	DisplayName     string   `json:"display_name,omitempty"`
	Version         string   `json:"version"`
	Path            string   `json:"path"`
	Status          string   `json:"status"`
	ConfigPath      string   `json:"config_path"`
	Installed       bool     `json:"installed"`
	Group           string   `json:"group,omitempty"`            // web | ssl | runtime | container
	Conflicts       []string `json:"conflicts,omitempty"`       // mutually exclusive components
	InstallMethods  []string `json:"install_methods,omitempty"` // source | package | script
	DefaultMethod   string   `json:"default_method,omitempty"`
	Versions        []string `json:"versions,omitempty"` // e.g. PHP
	CanInstall      bool     `json:"can_install"`
	CanUpdate       bool     `json:"can_update"`
	BlockReason     string   `json:"block_reason,omitempty"`
	Note            string   `json:"note,omitempty"`
	// Deploy: native = host install; docker = one-click container deploy
	Deploy        string `json:"deploy,omitempty"`
	Image         string `json:"image,omitempty"`
	HostPort      string `json:"host_port,omitempty"`
	ContainerPort string `json:"container_port,omitempty"`
	DockerName    string `json:"docker_name,omitempty"`
}

type RewriteRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Nginx       string `json:"nginx"`
	Apache      string `json:"apache"`
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
	DocRoot     string   `json:"doc_root,omitempty"`
	TLS         bool     `json:"tls"`
	HasHTTP     bool     `json:"has_http"`
	HasHTTPS    bool     `json:"has_https"`
	Enabled     bool     `json:"enabled"`
	SourcePath  string   `json:"source_path"`
	// Raw is only filled for config detail requests, not site lists.
	Raw string `json:"raw,omitempty"`
}

type FileEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime string `json:"mod_time"`
}

type CronJob struct {
	ID       string `json:"id"`
	Schedule string `json:"schedule"`
	Command  string `json:"command"`
	Raw      string `json:"raw"`
	Enabled  bool   `json:"enabled"`
}

type Certificate struct {
	Domain    string    `json:"domain"`
	Issuer    string    `json:"issuer"`
	Path      string    `json:"path"`
	KeyPath   string    `json:"key_path,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
	Source    string    `json:"source"`
	AutoRenew bool      `json:"auto_renew"`
	DaysLeft  int       `json:"days_left"`
}

type AlertRule struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	Metric          string  `json:"metric"`
	Operator        string  `json:"operator"`
	Threshold       float64 `json:"threshold"`
	DurationSeconds int     `json:"duration_seconds"`
	SilenceSeconds  int     `json:"silence_seconds"`
	RepeatSeconds   int     `json:"repeat_seconds"`
	Enabled         bool    `json:"enabled"`
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
	ID        int64     `json:"id"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Detail    string    `json:"detail"`
	RemoteIP  string    `json:"remote_ip"`
	CreatedAt time.Time `json:"created_at"`
}
