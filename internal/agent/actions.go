package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var safeID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.:/@-]{0,255}$`)

func executeAction(ctx context.Context, a ActionRequest) (ActionResult, error) {
	if a.Actor == "" {
		return ActionResult{}, errors.New("actor is required")
	}
	switch a.Kind {
	case "docker.container.start", "docker.container.stop", "docker.container.restart", "docker.container.delete":
		if !safeID.MatchString(a.Resource) {
			return ActionResult{}, errors.New("invalid container id")
		}
		verb := strings.TrimPrefix(a.Kind, "docker.container.")
		if err := dockerContainerAction(ctx, a.Resource, verb); err != nil {
			return ActionResult{}, err
		}
		return ActionResult{Output: "container " + verb + " completed"}, nil
	case "docker.image.pull", "docker.image.delete", "docker.volume.create", "docker.volume.delete", "docker.network.create", "docker.network.delete":
		parts := strings.Split(a.Kind, ".")
		if err := dockerObjectAction(ctx, parts[1], a.Resource, parts[2]); err != nil {
			return ActionResult{}, err
		}
		return ActionResult{Output: a.Kind + " completed"}, nil
	case "docker.compose.up", "docker.compose.down":
		path, err := safeComposePath(a.Resource)
		if err != nil {
			return ActionResult{}, err
		}
		args := []string{"compose", "-f", path}
		if strings.HasSuffix(a.Kind, ".up") {
			args = append(args, "up", "-d", "--remove-orphans")
		} else {
			args = append(args, "down")
		}
		return run(ctx, "docker", args...)
	case "service.start", "service.stop", "service.restart":
		if !allowedService(a.Resource) {
			return ActionResult{}, errors.New("service is not managed by AnPanel")
		}
		verb := strings.TrimPrefix(a.Kind, "service.")
		return run(ctx, "systemctl", verb, a.Resource)
	case "web.apply":
		return applyWebConfig(ctx, a.Resource, a.Options["content"])
	case "web.reload":
		if a.Resource == "nginx" {
			return validatedReload(ctx, "nginx")
		}
		if a.Resource == "apache" {
			return validatedReload(ctx, "apache")
		}
		return ActionResult{}, errors.New("unknown web server")
	case "package.install":
		return installPackage(ctx, a.Resource)
	case "notification.configure":
		return configureNotifications(a.Options["json"])
	case "panel.bind_domain":
		return bindPanelDomain(ctx, normalizedDomain(a.Options["domain"]), a.Options["server"], a.Options["tool"], a.Options["email"])
	case "panel.unbind_domain":
		return unbindPanelDomain(ctx, a.Options["server"])
	default:
		return ActionResult{}, fmt.Errorf("action %q is not allowed", a.Kind)
	}
}
func run(ctx context.Context, name string, args ...string) (ActionResult, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	b, err := cmd.CombinedOutput()
	out := redact(string(b))
	if err != nil {
		return ActionResult{}, fmt.Errorf("%s failed: %s", name, out)
	}
	return ActionResult{Output: out}, nil
}
func redact(v string) string {
	patterns := []*regexp.Regexp{regexp.MustCompile(`(?i)(password|token|secret)=\S+`), regexp.MustCompile(`(?i)authorization:\s*\S+`)}
	for _, p := range patterns {
		v = p.ReplaceAllString(v, "$1=[REDACTED]")
	}
	if len(v) > 1<<20 {
		v = v[:1<<20] + "\n[truncated]"
	}
	return v
}
func allowedService(v string) bool {
	switch v {
	case "nginx", "apache2", "httpd", "docker", "anpanel-web", "anpanel-agent":
		return true
	}
	return false
}
func safeComposePath(v string) (string, error) {
	p, err := filepath.Abs(v)
	if err != nil {
		return "", err
	}
	ok := false
	for _, root := range []string{"/etc/anpanel/compose", "/opt", "/srv"} {
		rel, e := filepath.Rel(root, p)
		if e == nil && rel != ".." && !strings.HasPrefix(rel, "../") {
			ok = true
		}
	}
	if !ok {
		return "", errors.New("compose file must be below /etc/anpanel/compose, /opt, or /srv")
	}
	if ext := filepath.Ext(p); ext != ".yml" && ext != ".yaml" {
		return "", errors.New("compose file must be YAML")
	}
	return p, nil
}
func installPackage(ctx context.Context, component string) (ActionResult, error) {
	if !map[string]bool{"nginx": true, "apache": true, "docker": true, "certbot": true}[component] {
		return ActionResult{}, errors.New("component is not in the embedded compatibility catalog")
	}
	osRelease, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ActionResult{}, err
	}
	values := parseOSRelease(string(osRelease))
	id, version := values["ID"], values["VERSION_ID"]
	apt := exec.Command("apt-get", "--version").Run() == nil
	packages := []string{}
	if apt {
		switch component {
		case "nginx":
			packages = []string{"nginx"}
		case "apache":
			packages = []string{"apache2"}
		case "certbot":
			packages = []string{"certbot"}
		case "docker":
			if hasDockerRepo() {
				packages = []string{"docker-ce", "docker-ce-cli", "containerd.io", "docker-compose-plugin"}
			}
		}
	} else {
		switch component {
		case "nginx":
			packages = []string{"nginx"}
		case "apache":
			packages = []string{"httpd"}
		case "certbot":
			packages = []string{"certbot"}
		case "docker":
			if hasDockerRepo() {
				packages = []string{"docker-ce", "docker-ce-cli", "containerd.io", "docker-compose-plugin"}
			}
		}
	}
	if len(packages) == 0 {
		return ActionResult{}, fmt.Errorf("%s installation is not validated for %s %s without its official repository", component, id, version)
	}
	if apt {
		args := append([]string{"install", "-y", "--no-install-recommends"}, packages...)
		return run(ctx, "apt-get", args...)
	}
	manager := "yum"
	if exec.Command("dnf", "--version").Run() == nil {
		manager = "dnf"
	}
	args := append([]string{"install", "-y"}, packages...)
	return run(ctx, manager, args...)
}

func parseOSRelease(raw string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		p := strings.SplitN(line, "=", 2)
		if len(p) == 2 {
			out[p[0]] = strings.Trim(p[1], "\"'")
		}
	}
	return out
}
func hasDockerRepo() bool {
	matches, _ := filepath.Glob("/etc/apt/sources.list.d/docker*")
	if len(matches) > 0 {
		return true
	}
	matches, _ = filepath.Glob("/etc/yum.repos.d/docker*")
	return len(matches) > 0
}

func configureNotifications(raw string) (ActionResult, error) {
	var v map[string]any
	if len(raw) == 0 || len(raw) > 64<<10 || json.Unmarshal([]byte(raw), &v) != nil {
		return ActionResult{}, errors.New("invalid notification configuration")
	}
	path := "/etc/anpanel/notifications.json"
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(raw), 0600); err != nil {
		return ActionResult{}, err
	}
	if err := os.Rename(tmp, path); err != nil {
		return ActionResult{}, err
	}
	if u, err := user.Lookup("anpanel"); err == nil {
		if uid, e := strconv.Atoi(u.Uid); e == nil {
			gid := -1
			if g, e := user.LookupGroup("anpanel-agent"); e == nil {
				gid, _ = strconv.Atoi(g.Gid)
			}
			_ = os.Chown(path, uid, gid)
		}
	}
	return ActionResult{Output: "notification configuration saved"}, nil
}
