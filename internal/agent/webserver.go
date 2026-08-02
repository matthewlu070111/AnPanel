package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/matthewlu070111/anpanel/internal/domain"
)

var (
	nginxStart  = regexp.MustCompile(`(?m)\bserver\s*\{`)
	nginxNames  = regexp.MustCompile(`(?m)(?:^|[;{])\s*server_name\s+([^;]+);`)
	nginxListen = regexp.MustCompile(`(?m)(?:^|[;{])\s*listen\s+([^;]+);`)
	nginxProxy  = regexp.MustCompile(`(?m)(?:^|[;{])\s*proxy_pass\s+([^;]+);`)
	nginxRoot   = regexp.MustCompile(`(?m)(?:^|[;{])\s*root\s+([^;]+);`)
	apacheVHost = regexp.MustCompile(`(?s)<VirtualHost\s+([^>]+)>(.*?)</VirtualHost>`)
	apacheName  = regexp.MustCompile(`(?mi)^\s*Server(?:Name|Alias)\s+(.+)$`)
	apacheRoot  = regexp.MustCompile(`(?mi)^\s*DocumentRoot\s+(\S+)`)
	apacheProxy = regexp.MustCompile(`(?mi)^\s*ProxyPass\s+/\s+(\S+)`)
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
				if strings.HasSuffix(path, "~") || strings.Contains(path, ".bak.") || strings.Contains(path, ".anpanel.") {
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
	return mergeWebsites(out), nil
}

// mergeWebsites collapses HTTP + HTTPS blocks for the same domain into one row (BT-style).
func mergeWebsites(in []domain.WebSite) []domain.WebSite {
	type key struct{ server, domain string }
	order := []key{}
	by := map[key]*domain.WebSite{}
	for _, s := range in {
		primary := ""
		if len(s.Domains) > 0 {
			primary = strings.ToLower(s.Domains[0])
		} else {
			primary = s.Name
		}
		k := key{server: s.Server, domain: primary}
		if cur, ok := by[k]; ok {
			cur.Listen = uniqueStrings(append(cur.Listen, s.Listen...))
			cur.Domains = uniqueStrings(append(cur.Domains, s.Domains...))
			if s.TLS {
				cur.TLS = true
				cur.HasHTTPS = true
			} else {
				cur.HasHTTP = true
			}
			if cur.ProxyTarget == "" {
				cur.ProxyTarget = s.ProxyTarget
			}
			if cur.DocRoot == "" {
				cur.DocRoot = s.DocRoot
			}
			// Prefer managed anpanel-site path when merging multi-file setups.
			if strings.Contains(s.SourcePath, "anpanel-site-") {
				cur.SourcePath = s.SourcePath
			}
			continue
		}
		cp := s
		cp.Raw = "" // never expose full config in list
		if cp.TLS {
			cp.HasHTTPS = true
		} else {
			cp.HasHTTP = true
		}
		// Same file may have both blocks later; also detect from listen.
		for _, l := range cp.Listen {
			ll := strings.ToLower(l)
			if strings.Contains(ll, "443") || strings.Contains(ll, "ssl") {
				cp.HasHTTPS = true
				cp.TLS = true
			}
			if strings.Contains(ll, "80") || (!strings.Contains(ll, "443") && !strings.Contains(ll, "ssl")) {
				cp.HasHTTP = true
			}
		}
		by[k] = &cp
		order = append(order, k)
	}
	out := make([]domain.WebSite, 0, len(order))
	for _, k := range order {
		out = append(out, *by[k])
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := "", ""
		if len(out[i].Domains) > 0 {
			a = out[i].Domains[0]
		}
		if len(out[j].Domains) > 0 {
			b = out[j].Domains[0]
		}
		return a < b
	})
	return out
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
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
		if m := nginxRoot.FindStringSubmatch(block); len(m) > 1 {
			v.DocRoot = strings.TrimSpace(m[1])
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
		if m := apacheRoot.FindStringSubmatch(b[2]); len(m) > 1 {
			v.DocRoot = strings.TrimSpace(m[1])
		}
		if m := apacheProxy.FindStringSubmatch(b[2]); len(m) > 1 {
			v.ProxyTarget = strings.TrimSpace(m[1])
		}
		out = append(out, v)
	}
	return out
}

func websiteConfig(path string) (string, error) {
	p, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if !isManagedWebPath(p) {
		return "", errors.New("path is outside managed web config roots")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func isManagedWebPath(p string) bool {
	for _, root := range []string{"/etc/nginx", "/etc/apache2", "/etc/httpd"} {
		rel, e := filepath.Rel(root, p)
		if e == nil && rel != ".." && !strings.HasPrefix(rel, "../") {
			return true
		}
	}
	return false
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
		if _, err := exec.LookPath("nginx"); err != nil {
			return err
		}
		c = exec.CommandContext(ctx, "nginx", "-t")
	} else if _, err := exec.LookPath("apachectl"); err == nil {
		c = exec.CommandContext(ctx, "apachectl", "configtest")
	} else if _, err := exec.LookPath("httpd"); err == nil {
		c = exec.CommandContext(ctx, "httpd", "-t")
	} else {
		return errors.New("web server binary not found")
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
	if exec.Command("systemctl", "is-active", "--quiet", name).Run() != nil {
		return fmt.Errorf("%s is not active", name)
	}
	b, err := exec.CommandContext(ctx, "systemctl", "reload", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("reload failed: %s", redact(string(b)))
	}
	return nil
}

// preferredWebServer picks the only installed server, or nginx when both exist.
func preferredWebServer() (string, error) {
	_, nginxErr := exec.LookPath("nginx")
	_, apache2Err := exec.LookPath("apache2")
	_, httpdErr := exec.LookPath("httpd")
	nginx := nginxErr == nil
	apache := apache2Err == nil || httpdErr == nil
	if nginx && !apache {
		return "nginx", nil
	}
	if apache && !nginx {
		return "apache", nil
	}
	if nginx && apache {
		return "nginx", nil
	}
	return "", errors.New("no web server installed; install Nginx or Apache first")
}
