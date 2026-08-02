package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/matthewlu070111/anpanel/internal/domain"
)

var (
	nginxStart  = regexp.MustCompile(`(?m)\bserver\s*\{`)
	nginxNames  = regexp.MustCompile(`(?m)(?:^|[;{])\s*server_name\s+([^;]+);`)
	nginxListen = regexp.MustCompile(`(?m)(?:^|[;{])\s*listen\s+([^;]+);`)
	nginxProxy  = regexp.MustCompile(`(?m)(?:^|[;{])\s*proxy_pass\s+([^;]+);`)
	apacheVHost = regexp.MustCompile(`(?s)<VirtualHost\s+([^>]+)>(.*?)</VirtualHost>`)
	apacheName  = regexp.MustCompile(`(?mi)^\s*Server(?:Name|Alias)\s+(.+)$`)
)

func discoverWebsites() ([]domain.WebSite, error) {
	out := []domain.WebSite{}
	for _, g := range []struct {
		server string
		roots  []string
	}{{"nginx", []string{"/etc/nginx/conf.d", "/etc/nginx/sites-enabled"}}, {"apache", []string{"/etc/apache2/sites-enabled", "/etc/httpd/conf.d"}}} {
		for _, root := range g.roots {
			_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				if strings.HasSuffix(path, "~") || strings.Contains(path, ".bak.") {
					return nil
				}
				b, e := os.ReadFile(path)
				if e != nil {
					return nil
				}
				if g.server == "nginx" {
					out = append(out, parseNginx(path, string(b))...)
				} else {
					out = append(out, parseApache(path, string(b))...)
				}
				return nil
			})
		}
	}
	return out, nil
}
func parseNginx(path, raw string) []domain.WebSite {
	blocks := nginxBlocks(raw)
	out := []domain.WebSite{}
	for i, block := range blocks {
		v := domain.WebSite{ID: fmt.Sprintf("nginx:%s:%d", path, i), Server: "nginx", Name: filepath.Base(path), SourcePath: path, Raw: raw, Enabled: true, TLS: strings.Contains(block, "ssl")}
		if m := nginxNames.FindStringSubmatch(block); len(m) > 1 {
			v.Domains = strings.Fields(m[1])
		}
		for _, m := range nginxListen.FindAllStringSubmatch(block, -1) {
			v.Listen = append(v.Listen, strings.TrimSpace(m[1]))
		}
		if m := nginxProxy.FindStringSubmatch(block); len(m) > 1 {
			v.ProxyTarget = strings.TrimSpace(m[1])
		}
		out = append(out, v)
	}
	return out
}

func nginxBlocks(raw string) []string {
	out := []string{}
	for offset := 0; offset < len(raw); {
		loc := nginxStart.FindStringIndex(raw[offset:])
		if loc == nil {
			break
		}
		start := offset + loc[0]
		brace := offset + loc[1] - 1
		depth, quote, escaped := 0, byte(0), false
		end := -1
		for i := brace; i < len(raw); i++ {
			c := raw[i]
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' && quote != 0 {
				escaped = true
				continue
			}
			if quote != 0 {
				if c == quote {
					quote = 0
				}
				continue
			}
			if c == '\'' || c == '"' {
				quote = c
				continue
			}
			if c == '{' {
				depth++
			} else if c == '}' {
				depth--
				if depth == 0 {
					end = i + 1
					break
				}
			}
		}
		if end < 0 {
			break
		}
		out = append(out, raw[start:end])
		offset = end
	}
	return out
}
func parseApache(path, raw string) []domain.WebSite {
	blocks := apacheVHost.FindAllStringSubmatch(raw, -1)
	out := []domain.WebSite{}
	for i, b := range blocks {
		v := domain.WebSite{ID: fmt.Sprintf("apache:%s:%d", path, i), Server: "apache", Name: filepath.Base(path), SourcePath: path, Raw: raw, Enabled: true, TLS: strings.Contains(strings.ToLower(b[1]+b[2]), "443")}
		v.Listen = strings.Fields(b[1])
		for _, m := range apacheName.FindAllStringSubmatch(b[2], -1) {
			v.Domains = append(v.Domains, strings.Fields(m[1])...)
		}
		out = append(out, v)
	}
	return out
}
func applyWebConfig(ctx context.Context, path, content string) (ActionResult, error) {
	p, err := filepath.Abs(path)
	if err != nil {
		return ActionResult{}, err
	}
	server := ""
	for _, root := range []string{"/etc/nginx", "/etc/apache2", "/etc/httpd"} {
		rel, e := filepath.Rel(root, p)
		if e == nil && rel != ".." && !strings.HasPrefix(rel, "../") {
			if root == "/etc/nginx" {
				server = "nginx"
			} else {
				server = "apache"
			}
		}
	}
	if server == "" {
		return ActionResult{}, errors.New("web configuration path is outside supported roots")
	}
	if len(content) == 0 || len(content) > 1<<20 {
		return ActionResult{}, errors.New("configuration content has invalid size")
	}
	old, readErr := os.ReadFile(p)
	mode := os.FileMode(0644)
	if st, e := os.Stat(p); e == nil {
		mode = st.Mode()
	}
	backup := fmt.Sprintf("%s.anpanel.bak.%d", p, time.Now().Unix())
	if readErr == nil {
		if err := os.WriteFile(backup, old, 0600); err != nil {
			return ActionResult{}, err
		}
	}
	tmp := p + ".anpanel.tmp"
	if err := os.WriteFile(tmp, []byte(content), mode); err != nil {
		return ActionResult{}, err
	}
	if err := os.Rename(tmp, p); err != nil {
		return ActionResult{}, err
	}
	check := configTest(ctx, server)
	if check != nil {
		if readErr == nil {
			_ = os.WriteFile(p, old, mode)
		} else {
			_ = os.Remove(p)
		}
		return ActionResult{Output: check.Error(), RolledBack: true}, check
	}
	reload := reloadServer(ctx, server)
	if reload != nil {
		if readErr == nil {
			_ = os.WriteFile(p, old, mode)
		} else {
			_ = os.Remove(p)
		}
		_ = reloadServer(ctx, server)
		return ActionResult{Output: reload.Error(), RolledBack: true}, reload
	}
	return ActionResult{Output: "configuration applied; backup: " + backup}, nil
}
func validatedReload(ctx context.Context, server string) (ActionResult, error) {
	if err := configTest(ctx, server); err != nil {
		return ActionResult{}, err
	}
	if err := reloadServer(ctx, server); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{Output: server + " reloaded"}, nil
}
func configTest(ctx context.Context, server string) error {
	var c *exec.Cmd
	if server == "nginx" {
		c = exec.CommandContext(ctx, "nginx", "-t")
	} else if _, err := exec.LookPath("apachectl"); err == nil {
		c = exec.CommandContext(ctx, "apachectl", "configtest")
	} else {
		c = exec.CommandContext(ctx, "httpd", "-t")
	}
	b, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("configuration test failed: %s", redact(string(b)))
	}
	return nil
}
func reloadServer(ctx context.Context, server string) error {
	name := server
	if server == "apache" {
		if exec.Command("systemctl", "status", "apache2").Run() == nil {
			name = "apache2"
		} else {
			name = "httpd"
		}
	}
	b, err := exec.CommandContext(ctx, "systemctl", "reload", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("reload failed: %s", redact(string(b)))
	}
	return nil
}
