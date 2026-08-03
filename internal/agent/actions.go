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
	case "web.site.create":
		return createWebsite(ctx, a.Options)
	case "web.site.configure":
		if a.Options == nil {
			a.Options = map[string]string{}
		}
		if a.Options["domain"] == "" {
			a.Options["domain"] = a.Resource
		}
		return configureWebsite(ctx, a.Options)
	case "web.site.delete":
		return deleteWebsite(ctx, a.Resource, a.Options["server"])
	case "cert.issue":
		server := a.Options["server"]
		if server == "" {
			var err error
			server, err = preferredWebServer()
			if err != nil {
				return ActionResult{}, err
			}
		}
		return issueSiteCertificate(ctx, a.Resource, server, a.Options["tool"], a.Options["email"])
	case "cert.renew":
		force := strings.EqualFold(a.Options["force"], "true") || a.Options["force"] == "1"
		return renewCertificate(ctx, a.Resource, a.Options["tool"], force)
	case "cert.delete":
		return deleteCertificate(ctx, a.Resource, a.Options["source"])
	case "files.write":
		return writeFileContent(a.Resource, a.Options["content"])
	case "files.mkdir":
		return mkdirPath(a.Resource)
	case "files.delete":
		return deletePath(a.Resource)
	case "files.rename":
		return renamePath(a.Resource, a.Options["to"])
	case "crontab.add":
		return addCrontab(ctx, a.Options["schedule"], a.Options["command"])
	case "crontab.remove":
		return removeCrontab(ctx, a.Resource)
	case "panel.self_update":
		ch := a.Options["channel"]
		if ch == "" {
			ch = a.Resource
		}
		return selfUpdate(ctx, ch)
	case "package.install":
		// Docker marketplace apps (php, 3x-ui, …) install via container deploy.
		if a.Options != nil && a.Options["deploy"] == "docker" {
			return deployDockerApp(ctx, a.Resource, a.Options)
		}
		return installSoftware(ctx, a.Resource, a.Options)
	case "package.update":
		if a.Options != nil && a.Options["deploy"] == "docker" {
			return updateDockerApp(ctx, a.Resource, a.Options)
		}
		return updateSoftware(ctx, a.Resource, a.Options)
	case "web.site.rewrite":
		return setSiteRewrite(ctx, a.Resource, a.Options["rewrite"], a.Options["server"])
	case "notification.configure":
		return configureNotifications(a.Options["json"])
	case "panel.bind_domain":
		server := a.Options["server"]
		if server == "" {
			var err error
			server, err = preferredWebServer()
			if err != nil {
				return ActionResult{}, err
			}
		}
		return bindPanelDomain(ctx, normalizedDomain(a.Options["domain"]), server, a.Options["tool"], a.Options["email"])
	case "panel.unbind_domain":
		server := a.Options["server"]
		if server == "" {
			var err error
			server, err = preferredWebServer()
			if err != nil {
				return ActionResult{}, err
			}
		}
		return unbindPanelDomain(ctx, server)
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
	case "nginx", "apache2", "httpd", "docker", "php-fpm", "anpanel-web", "anpanel-agent":
		return true
	}
	if strings.HasPrefix(v, "php") && strings.HasSuffix(v, "-fpm") {
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
