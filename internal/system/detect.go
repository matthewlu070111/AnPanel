package system

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/anpanel/anpanel/internal/domain"
)

func DetectServices() []domain.DetectedService {
	defs := []struct {
		name   string
		bins   []string
		config string
	}{
		{"nginx", []string{"nginx"}, "/etc/nginx/nginx.conf"},
		{"apache", []string{"apache2", "httpd"}, "/etc/apache2/apache2.conf"},
		{"docker", []string{"docker"}, "/etc/docker/daemon.json"},
		{"compose", []string{"docker-compose"}, ""},
		{"certbot", []string{"certbot"}, "/etc/letsencrypt"},
		{"acme.sh", []string{"acme.sh"}, "~/.acme.sh"},
	}
	out := make([]domain.DetectedService, 0, len(defs))
	for _, d := range defs {
		s := domain.DetectedService{Name: d.name, ConfigPath: d.config}
		for _, bin := range d.bins {
			if p, err := exec.LookPath(bin); err == nil {
				s.Installed = true
				s.Path = p
				s.Version = commandVersion(p)
				break
			}
		}
		if s.Installed {
			s.Status = serviceStatus(d.name)
		} else {
			s.Status = "not-installed"
		}
		out = append(out, s)
	}
	return out
}
func commandVersion(bin string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	b, _ := exec.CommandContext(ctx, bin, "--version").CombinedOutput()
	v := strings.TrimSpace(string(b))
	if len(v) > 160 {
		v = v[:160]
	}
	return v
}
func serviceStatus(name string) string {
	if name == "compose" || name == "certbot" || name == "acme.sh" {
		return "available"
	}
	if name == "apache" {
		for _, n := range []string{"apache2", "httpd"} {
			if exec.Command("systemctl", "is-active", "--quiet", n).Run() == nil {
				return "active"
			}
		}
		return "inactive"
	}
	if exec.Command("systemctl", "is-active", "--quiet", name).Run() == nil {
		return "active"
	}
	return "inactive"
}
