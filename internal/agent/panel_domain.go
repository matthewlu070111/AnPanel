package agent

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/matthewlu070111/anpanel/internal/config"
)

var domainName = regexp.MustCompile(`^(?i:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+)$`)

func bindPanelDomain(ctx context.Context, domain, server, tool, email string) (ActionResult, error) {
	if !domainName.MatchString(domain) {
		return ActionResult{}, errors.New("invalid domain name")
	}
	if _, err := net.DefaultResolver.LookupHost(ctx, domain); err != nil {
		return ActionResult{}, fmt.Errorf("domain does not resolve: %w", err)
	}
	cfg, err := config.Load()
	if err != nil {
		return ActionResult{}, err
	}
	path, err := panelConfigPath(server)
	if err != nil {
		return ActionResult{}, err
	}
	old, oldErr := os.ReadFile(path)
	if server == "apache" {
		if _, err := exec.LookPath("a2enmod"); err == nil {
			if _, err := run(ctx, "a2enmod", "proxy", "proxy_http", "ssl", "alias"); err != nil {
				return ActionResult{}, err
			}
		}
	}
	rollback := func() {
		if oldErr == nil {
			_ = os.WriteFile(path, old, 0644)
		} else {
			_ = os.Remove(path)
		}
		_ = reloadServer(context.Background(), server)
	}
	httpConfig := panelProxyConfig(server, domain, cfg.Port, "", "")
	if _, err := applyWebConfig(ctx, path, httpConfig); err != nil {
		return ActionResult{}, err
	}
	var output string
	switch tool {
	case "certbot":
		certbot := certbotPath()
		if certbot == "" {
			rollback()
			return ActionResult{}, errors.New("certbot is not installed")
		}
		plugin := "--nginx"
		if server == "apache" {
			plugin = "--apache"
		}
		args := []string{plugin, "-d", domain, "--non-interactive", "--agree-tos", "--redirect"}
		if email != "" {
			args = append(args, "--email", email)
		} else {
			args = append(args, "--register-unsafely-without-email")
		}
		res, err := run(ctx, certbot, args...)
		if err != nil {
			rollback()
			return ActionResult{}, err
		}
		output = res.Output
	case "acme.sh":
		acme := acmePath()
		if acme == "" {
			rollback()
			return ActionResult{}, errors.New("acme.sh is not installed")
		}
		challenge := "/var/lib/anpanel/acme"
		if err := os.MkdirAll(challenge, 0755); err != nil {
			rollback()
			return ActionResult{}, err
		}
		certDir := filepath.Join("/etc/anpanel/certs", domain)
		if err := os.MkdirAll(certDir, 0700); err != nil {
			rollback()
			return ActionResult{}, err
		}
		if res, err := run(ctx, acme, "--issue", "--webroot", challenge, "-d", domain); err != nil {
			rollback()
			return ActionResult{}, err
		} else {
			output = res.Output
		}
		fullchain := filepath.Join(certDir, "fullchain.pem")
		key := filepath.Join(certDir, "key.pem")
		if _, err := run(ctx, acme, "--install-cert", "-d", domain, "--key-file", key, "--fullchain-file", fullchain); err != nil {
			rollback()
			return ActionResult{}, err
		}
		final := panelProxyConfig(server, domain, cfg.Port, fullchain, key)
		if _, err := applyWebConfig(ctx, path, final); err != nil {
			rollback()
			return ActionResult{}, err
		}
	default:
		rollback()
		return ActionResult{}, errors.New("certificate tool must be certbot or acme.sh")
	}
	if err := configTest(ctx, server); err != nil {
		rollback()
		return ActionResult{}, err
	}
	cfg.Listen = "127.0.0.1"
	if err := config.Save(cfg); err != nil {
		rollback()
		return ActionResult{}, err
	}
	scheduleWebRestart()
	return ActionResult{Output: "domain bound successfully\n" + redact(output)}, nil
}

func unbindPanelDomain(ctx context.Context, server string) (ActionResult, error) {
	cfg, err := config.Load()
	if err != nil {
		return ActionResult{}, err
	}
	path, err := panelConfigPath(server)
	if err != nil {
		return ActionResult{}, err
	}
	old, readErr := os.ReadFile(path)
	if readErr == nil {
		backup := fmt.Sprintf("%s.anpanel.unbound.%d", path, time.Now().Unix())
		if err = os.WriteFile(backup, old, 0600); err != nil {
			return ActionResult{}, err
		}
		if err = os.Remove(path); err != nil {
			return ActionResult{}, err
		}
	}
	restore := func() {
		if readErr == nil {
			_ = os.WriteFile(path, old, 0644)
		}
		_ = reloadServer(context.Background(), server)
	}
	if err = configTest(ctx, server); err != nil {
		restore()
		return ActionResult{}, err
	}
	if err = reloadServer(ctx, server); err != nil {
		restore()
		return ActionResult{}, err
	}
	cfg.Listen = "0.0.0.0"
	if err = config.Save(cfg); err != nil {
		restore()
		return ActionResult{}, err
	}
	scheduleWebRestart()
	return ActionResult{Output: fmt.Sprintf("IP access restored on port %d", cfg.Port)}, nil
}

func panelConfigPath(server string) (string, error) {
	switch server {
	case "nginx":
		return "/etc/nginx/conf.d/anpanel-panel.conf", nil
	case "apache":
		if _, err := os.Stat("/etc/apache2"); err == nil {
			return "/etc/apache2/sites-enabled/anpanel-panel.conf", nil
		}
		return "/etc/httpd/conf.d/anpanel-panel.conf", nil
	default:
		return "", errors.New("web server must be nginx or apache")
	}
}
func acmePath() string {
	if p, err := exec.LookPath("acme.sh"); err == nil {
		return p
	}
	for _, p := range []string{"/root/.acme.sh/acme.sh", "/home/anpanel/.acme.sh/acme.sh"} {
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
			return p
		}
	}
	return ""
}
func scheduleWebRestart() {
	go func() { time.Sleep(2 * time.Second); _ = exec.Command("systemctl", "restart", "anpanel-web").Run() }()
}

func panelProxyConfig(server, domain string, port int, cert, key string) string {
	if server == "nginx" {
		if cert == "" {
			return fmt.Sprintf(`server {
  listen 80;
  server_name %s;
  location /.well-known/acme-challenge/ { root /var/lib/anpanel/acme; }
  location / {
    proxy_pass http://127.0.0.1:%d;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection upgrade;
  }
}
`, domain, port)
		}
		return fmt.Sprintf(`server {
  listen 80; server_name %s;
  location /.well-known/acme-challenge/ { root /var/lib/anpanel/acme; }
  location / { return 301 https://$host$request_uri; }
}
server {
  listen 443 ssl; server_name %s;
  ssl_certificate %s; ssl_certificate_key %s;
  location / {
    proxy_pass http://127.0.0.1:%d; proxy_http_version 1.1;
    proxy_set_header Host $host; proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-Proto https; proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection upgrade;
  }
}
`, domain, domain, cert, key, port)
	}
	if cert == "" {
		return fmt.Sprintf(`<VirtualHost *:80>
  ServerName %s
  Alias /.well-known/acme-challenge/ /var/lib/anpanel/acme/.well-known/acme-challenge/
  ProxyPreserveHost On
  ProxyPass /.well-known/acme-challenge/ !
  ProxyPass / http://127.0.0.1:%d/
  ProxyPassReverse / http://127.0.0.1:%d/
</VirtualHost>
`, domain, port, port)
	}
	return fmt.Sprintf(`<VirtualHost *:80>
  ServerName %s
  Alias /.well-known/acme-challenge/ /var/lib/anpanel/acme/.well-known/acme-challenge/
  RedirectMatch 301 ^/(?!\.well-known/acme-challenge/)(.*)$ https://%s/$1
</VirtualHost>
<VirtualHost *:443>
  ServerName %s
  SSLEngine on
  SSLCertificateFile %s
  SSLCertificateKeyFile %s
  ProxyPreserveHost On
  ProxyPass / http://127.0.0.1:%d/
  ProxyPassReverse / http://127.0.0.1:%d/
</VirtualHost>
`, domain, domain, domain, cert, key, port, port)
}

func normalizedDomain(v string) string { return strings.ToLower(strings.TrimSpace(v)) }
