package system

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/matthewlu070111/anpanel/internal/domain"
)

func DetectServices() []domain.DetectedService {
	installed := map[string]bool{}
	// probe helpers
	nginxPath, nginxOK := lookAny("nginx")
	apachePath, apacheOK := lookAny("apache2", "httpd")
	dockerPath, dockerOK := lookAny("docker")
	certbotPath, certbotOK := lookAny("certbot")
	_, snapOK := lookAny("snap")
	acmePath, acmeOK := findAcme()
	composeOK, composeVer, _ := detectCompose()

	installed["nginx"] = nginxOK
	installed["apache"] = apacheOK
	installed["docker"] = dockerOK
	installed["certbot"] = certbotOK
	installed["acme.sh"] = acmeOK

	certbotMethods, certbotDefault := []string{"source", "package"}, "source"
	if certbotOK {
		certbotDefault = CertbotInstallMethod(certbotPath)
	}
	if snapOK && (!certbotOK || certbotDefault == "snap") {
		certbotMethods, certbotDefault = append([]string{"snap"}, certbotMethods...), "snap"
	}

	// Compose is NOT listed as a separate app store entry — it ships with Docker Engine.
	dockerNote := ""
	if dockerOK {
		if composeOK {
			dockerNote = "已包含 Docker Compose（" + composeVer + "）"
		} else {
			dockerNote = "未检测到 docker compose 插件，更新 Docker 时可一并安装"
		}
	} else {
		dockerNote = "安装 Docker Engine 时会同时提供 Compose V2 插件（docker compose）"
	}

	// Only host-native installs: nginx/apache/docker/certbot/acme.sh.
	// Everything else (PHP, 3x-ui, …) is offered as Docker one-click deploy.
	out := []domain.DetectedService{
		soft("nginx", "Nginx", "web", []string{"apache"}, []string{"source", "package"}, "source", nil, nginxOK, nginxPath, "/etc/nginx/nginx.conf", serviceUnitStatus("nginx"), installed),
		soft("apache", "Apache", "web", []string{"nginx"}, []string{"source", "package"}, "source", nil, apacheOK, apachePath, apacheConfig(), serviceUnitStatusApache(), installed),
		soft("docker", "Docker Engine", "container", nil, []string{"script", "package"}, "script", nil, dockerOK, dockerPath, "/etc/docker/daemon.json", serviceUnitStatus("docker"), installed),
		soft("certbot", "Certbot", "ssl", []string{"acme.sh"}, certbotMethods, certbotDefault, nil, certbotOK, certbotPath, "/etc/letsencrypt", toolStatus(certbotOK), installed),
		soft("acme.sh", "acme.sh", "ssl", []string{"certbot"}, []string{"script"}, "script", nil, acmeOK, acmePath, "/root/.acme.sh", toolStatus(acmeOK), installed),
	}
	for i := range out {
		out[i].Deploy = "native"
		if out[i].Name == "docker" {
			out[i].Note = dockerNote
		}
	}
	out = append(out, dockerCatalogApps(dockerOK)...)
	return out
}

// dockerCatalogApps lists marketplace apps that deploy only via Docker.
func dockerCatalogApps(dockerOK bool) []domain.DetectedService {
	type item struct {
		name, display, group, image, hostPort, containerPort, dockerName, note string
		versions                                                               []string
	}
	items := []item{
		{
			name: "3x-ui", display: "3x-ui", group: "apps",
			image: "ghcr.io/mhsanaei/3x-ui:latest", hostPort: "2053", containerPort: "2053",
			dockerName: "anpanel-3x-ui",
			note:       "通过 Docker 部署 · 面板默认端口 2053",
		},
		{
			name: "php", display: "PHP", group: "runtime",
			image: "php:8.3-fpm", hostPort: "9000", containerPort: "9000",
			dockerName: "anpanel-php",
			versions:   []string{"8.1", "8.2", "8.3", "8.4"},
			note:       "通过 Docker 部署 PHP-FPM（非本机编译）",
		},
	}
	out := make([]domain.DetectedService, 0, len(items))
	for _, it := range items {
		running, version := dockerAppStatus(it.dockerName, it.image)
		s := domain.DetectedService{
			Name: it.name, DisplayName: it.display, Group: it.group,
			Deploy: "docker", Image: it.image, HostPort: it.hostPort, ContainerPort: it.containerPort,
			DockerName: it.dockerName, Note: it.note, Versions: it.versions,
			Installed: running, Status: map[bool]string{true: "running", false: "not-installed"}[running],
			Version: version, CanInstall: dockerOK && !running, CanUpdate: dockerOK && running,
		}
		if !dockerOK {
			s.CanInstall = false
			s.BlockReason = "请先安装 Docker Engine，再通过 Docker 部署此应用"
		}
		out = append(out, s)
	}
	return out
}

func dockerAppStatus(name, image string) (running bool, version string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// docker inspect running container by name
	b, err := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Running}}|{{.Config.Image}}", name).CombinedOutput()
	if err != nil {
		return false, ""
	}
	parts := strings.SplitN(strings.TrimSpace(string(b)), "|", 2)
	if len(parts) > 0 && parts[0] == "true" {
		img := image
		if len(parts) > 1 {
			img = parts[1]
		}
		return true, img
	}
	return false, ""
}

// toolStatus: installed tools that are not systemd services show as "installed".
func toolStatus(ok bool) string {
	if ok {
		return "installed"
	}
	return "not-installed"
}

func CertbotInstallMethod(path string) string {
	original := filepath.ToSlash(path)
	if strings.Contains(original, "/snap/") {
		return "snap"
	}
	if target, err := os.Readlink(path); err == nil {
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		if strings.Contains(filepath.ToSlash(target), "/snap/") {
			return "snap"
		}
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		path = resolved
	}
	path = filepath.ToSlash(path)
	if strings.Contains(path, "/snap/") {
		return "snap"
	}
	if strings.HasPrefix(path, "/usr/bin/") || strings.HasPrefix(path, "/bin/") {
		return "package"
	}
	return "source"
}

func soft(name, display, group string, conflicts, methods []string, defMethod string, versions []string, ok bool, path, config, status string, installed map[string]bool) domain.DetectedService {
	s := domain.DetectedService{
		Name: name, DisplayName: display, Group: group, Conflicts: conflicts,
		InstallMethods: methods, DefaultMethod: defMethod, Versions: versions,
		Installed: ok, Path: path, ConfigPath: config, Status: status,
		CanInstall: !ok, CanUpdate: ok && name != "compose",
	}
	if ok {
		s.Version = commandVersion(path)
		s.CanInstall = false
	}
	// exclusivity block
	for _, c := range conflicts {
		if installed[c] && !ok {
			s.CanInstall = false
			s.BlockReason = "已安装 " + c + "，与 " + display + " 互斥，不能同时安装。请先卸载冲突软件。"
			break
		}
	}
	if name == "compose" {
		s.CanInstall = false
	}
	return s
}

func lookAny(bins ...string) (string, bool) {
	for _, b := range bins {
		if p, err := exec.LookPath(b); err == nil {
			return p, true
		}
	}
	// also check common prefix installs
	for _, b := range bins {
		for _, p := range []string{"/usr/local/nginx/sbin/" + b, "/usr/local/sbin/" + b, "/usr/local/bin/" + b, "/snap/bin/" + b} {
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p, true
			}
		}
	}
	return "", false
}

func findAcme() (string, bool) {
	if p, err := exec.LookPath("acme.sh"); err == nil {
		return p, true
	}
	for _, p := range []string{"/root/.acme.sh/acme.sh", "/home/anpanel/.acme.sh/acme.sh"} {
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
			return p, true
		}
	}
	return "", false
}

func detectCompose() (bool, string, string) {
	// Prefer Docker Compose V2 plugin
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := exec.LookPath("docker"); err == nil {
		b, err := exec.CommandContext(ctx, "docker", "compose", "version").CombinedOutput()
		if err == nil {
			return true, firstLine(string(b)), "docker compose"
		}
	}
	if p, err := exec.LookPath("docker-compose"); err == nil {
		return true, commandVersion(p), p
	}
	return false, "", ""
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 120 {
		s = s[:120]
	}
	return s
}

func commandVersion(bin string) string {
	if bin == "" || bin == "docker compose" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	b, _ := exec.CommandContext(ctx, bin, "--version").CombinedOutput()
	return firstLine(string(b))
}

func serviceUnitStatus(name string) string {
	if exec.Command("systemctl", "is-active", "--quiet", name).Run() == nil {
		return "running"
	}
	if _, err := exec.LookPath(name); err == nil {
		return "stopped"
	}
	// also detect compiled installs not necessarily on PATH for systemctl
	if CompiledBin(name) != "" {
		return "stopped"
	}
	return "not-installed"
}

func serviceUnitStatusApache() string {
	for _, n := range []string{"apache2", "httpd"} {
		if exec.Command("systemctl", "is-active", "--quiet", n).Run() == nil {
			return "running"
		}
	}
	if _, ok := lookAny("apache2", "httpd"); ok {
		return "stopped"
	}
	return "not-installed"
}

func apacheConfig() string {
	for _, p := range []string{"/etc/apache2/apache2.conf", "/etc/httpd/conf/httpd.conf", "/usr/local/apache2/conf/httpd.conf"} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "/etc/apache2/apache2.conf"
}

func phpIniPath(phpBin string) string {
	if phpBin == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	b, _ := exec.CommandContext(ctx, phpBin, "-i").CombinedOutput()
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "Loaded Configuration File") {
			parts := strings.SplitN(line, "=>", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

func phpStatus() string {
	for _, n := range []string{"php-fpm", "php8.3-fpm", "php8.2-fpm", "php8.1-fpm", "php8.4-fpm"} {
		if exec.Command("systemctl", "is-active", "--quiet", n).Run() == nil {
			return "running"
		}
	}
	if _, ok := lookAny("php"); ok {
		return "installed"
	}
	return "not-installed"
}

// LookPath is used by agent software checks.
func LookPath(bin string) (string, bool) {
	return lookAny(bin)
}

func FindAcme() (string, bool) { return findAcme() }

func IsInstalled(name string) bool {
	switch name {
	case "nginx":
		_, ok := lookAny("nginx")
		return ok
	case "apache":
		_, ok := lookAny("apache2", "httpd")
		return ok
	case "docker":
		_, ok := lookAny("docker")
		return ok
	case "certbot":
		_, ok := lookAny("certbot")
		return ok
	case "acme.sh":
		_, ok := findAcme()
		return ok
	case "php":
		_, ok := lookAny("php")
		return ok
	case "compose":
		ok, _, _ := detectCompose()
		return ok
	}
	return false
}

// PHPBin returns resolved php binary path.
func PHPBin() string {
	p, _ := lookAny("php")
	return p
}

// Ensure common prefix is on PATH for compiled installs.
func CompiledBin(name string) string {
	candidates := []string{
		filepath.Join("/usr/local", name, "sbin", name),
		filepath.Join("/usr/local", name, "bin", name),
		filepath.Join("/usr/local/sbin", name),
		filepath.Join("/usr/local/bin", name),
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}
