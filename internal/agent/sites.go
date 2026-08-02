package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/matthewlu070111/anpanel/internal/system"
)

var (
	proxyURLRe = regexp.MustCompile(`(?i)^https?://[a-z0-9._:\[\]/@%-]+$`)
	safeSlug   = regexp.MustCompile(`[^a-z0-9._-]+`)
)

func createWebsite(ctx context.Context, opts map[string]string) (ActionResult, error) {
	domain := normalizedDomain(opts["domain"])
	if !domainName.MatchString(domain) {
		return ActionResult{}, errors.New("invalid domain name")
	}
	server := strings.ToLower(strings.TrimSpace(opts["server"]))
	if server == "" {
		var err error
		server, err = preferredWebServer()
		if err != nil {
			return ActionResult{}, err
		}
	}
	if server != "nginx" && server != "apache" {
		return ActionResult{}, errors.New("web server must be nginx or apache")
	}
	siteType := strings.ToLower(strings.TrimSpace(opts["site_type"]))
	if siteType == "" {
		siteType = "proxy"
	}
	if siteType != "static" && siteType != "proxy" {
		return ActionResult{}, errors.New("site_type must be static or proxy")
	}
	rewrite := strings.TrimSpace(opts["rewrite"])
	if rewrite == "" {
		rewrite = "none"
	}
	if _, err := rewriteByID(rewrite); err != nil {
		return ActionResult{}, err
	}

	path, err := siteConfigPath(server, domain)
	if err != nil {
		return ActionResult{}, err
	}
	if _, err := os.Stat(path); err == nil {
		return ActionResult{}, fmt.Errorf("site config already exists: %s", path)
	}

	var content string
	switch siteType {
	case "static":
		root, err := safeWebRoot(opts["root"], domain)
		if err != nil {
			return ActionResult{}, err
		}
		if err := os.MkdirAll(root, 0755); err != nil {
			return ActionResult{}, err
		}
		index := filepath.Join(root, "index.html")
		if _, err := os.Stat(index); err != nil {
			_ = os.WriteFile(index, []byte(fmt.Sprintf("<!doctype html><html><head><meta charset=utf-8><title>%s</title></head><body><h1>%s</h1><p>Created by AnPanel</p></body></html>\n", domain, domain)), 0644)
		}
		_ = os.MkdirAll("/var/lib/anpanel/acme", 0755)
		content = siteStaticConfig(server, domain, root, "", "", rewrite)
	case "proxy":
		target := strings.TrimSpace(opts["proxy_pass"])
		if target == "" {
			return ActionResult{}, errors.New("proxy_pass is required for reverse proxy sites")
		}
		if !proxyURLRe.MatchString(target) {
			return ActionResult{}, errors.New("invalid proxy_pass URL (use http:// or https:// host:port)")
		}
		_ = os.MkdirAll("/var/lib/anpanel/acme", 0755)
		content = siteProxyConfig(server, domain, target, "", "")
	}

	// Ensure parent directory exists (e.g. conf.d).
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return ActionResult{}, err
	}
	res, err := applyWebConfig(ctx, path, content)
	if err != nil {
		return ActionResult{}, err
	}

	ssl := strings.EqualFold(opts["enable_ssl"], "true") || opts["enable_ssl"] == "1"
	if ssl {
		tool := opts["tool"]
		if tool == "" {
			tool = "certbot"
		}
		certRes, certErr := issueSiteCertificate(ctx, domain, server, tool, opts["email"])
		if certErr != nil {
			// Keep the HTTP site; operator can re-run cert.issue from the UI.
			return ActionResult{
				Output: res.Output + "\nHTTP site created at " + path + ", but SSL failed: " + certErr.Error() + "\nUse cert.issue to retry.",
			}, nil
		}
		return ActionResult{Output: "site created: " + path + "\n" + res.Output + "\n" + certRes.Output}, nil
	}
	return ActionResult{Output: "site created: " + path + "\n" + res.Output}, nil
}

func deleteWebsite(ctx context.Context, domain, server string) (ActionResult, error) {
	domain = normalizedDomain(domain)
	if !domainName.MatchString(domain) {
		return ActionResult{}, errors.New("invalid domain name")
	}
	if server == "" {
		var err error
		server, err = preferredWebServer()
		if err != nil {
			return ActionResult{}, err
		}
	}
	path, err := siteConfigPath(server, domain)
	if err != nil {
		return ActionResult{}, err
	}
	// Only delete AnPanel-managed site files.
	base := filepath.Base(path)
	if !strings.HasPrefix(base, "anpanel-site-") {
		return ActionResult{}, errors.New("only AnPanel-managed site configs (anpanel-site-*) can be deleted from the panel")
	}
	old, readErr := os.ReadFile(path)
	if readErr != nil {
		return ActionResult{}, readErr
	}
	backup := fmt.Sprintf("%s.anpanel.deleted.%d", path, os.Getpid())
	_ = os.WriteFile(backup, old, 0600)
	if err := os.Remove(path); err != nil {
		return ActionResult{}, err
	}
	if err := configTest(ctx, server); err != nil {
		_ = os.WriteFile(path, old, 0644)
		return ActionResult{Output: err.Error(), RolledBack: true}, err
	}
	if err := reloadServer(ctx, server); err != nil {
		_ = os.WriteFile(path, old, 0644)
		_ = reloadServer(ctx, server)
		return ActionResult{Output: err.Error(), RolledBack: true}, err
	}
	return ActionResult{Output: "site deleted; backup: " + backup}, nil
}

func siteConfigPath(server, domain string) (string, error) {
	slug := domainSlug(domain)
	switch server {
	case "nginx":
		// Prefer conf.d (works on RHEL and Debian without sites-enabled).
		if st, err := os.Stat("/etc/nginx/conf.d"); err == nil && st.IsDir() {
			return filepath.Join("/etc/nginx/conf.d", "anpanel-site-"+slug+".conf"), nil
		}
		if st, err := os.Stat("/etc/nginx/sites-enabled"); err == nil && st.IsDir() {
			return filepath.Join("/etc/nginx/sites-enabled", "anpanel-site-"+slug+".conf"), nil
		}
		return filepath.Join("/etc/nginx/conf.d", "anpanel-site-"+slug+".conf"), nil
	case "apache":
		if st, err := os.Stat("/etc/apache2/sites-enabled"); err == nil && st.IsDir() {
			return filepath.Join("/etc/apache2/sites-enabled", "anpanel-site-"+slug+".conf"), nil
		}
		if st, err := os.Stat("/etc/httpd/conf.d"); err == nil && st.IsDir() {
			return filepath.Join("/etc/httpd/conf.d", "anpanel-site-"+slug+".conf"), nil
		}
		return filepath.Join("/etc/httpd/conf.d", "anpanel-site-"+slug+".conf"), nil
	default:
		return "", errors.New("web server must be nginx or apache")
	}
}

func domainSlug(domain string) string {
	d := strings.ToLower(strings.TrimSpace(domain))
	d = safeSlug.ReplaceAllString(d, "-")
	if len(d) > 80 {
		d = d[:80]
	}
	return d
}

func defaultWebRoot(domain string) string {
	return filepath.Join("/var/www", domainSlug(domain))
}

func safeWebRoot(raw, domain string) (string, error) {
	p := strings.TrimSpace(raw)
	if p == "" {
		p = defaultWebRoot(domain)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	ok := false
	for _, root := range []string{"/var/www", "/www", "/srv", "/opt", "/home"} {
		rel, e := filepath.Rel(root, abs)
		if e == nil && rel != ".." && !strings.HasPrefix(rel, "../") {
			ok = true
			break
		}
	}
	if !ok {
		return "", errors.New("document root must be under /var/www, /www, /srv, /opt, or /home")
	}
	return abs, nil
}

func siteStaticConfig(server, domain, root, cert, key, rewriteID string) string {
	if rewriteID == "" {
		rewriteID = "none"
	}
	ngxLoc := "# BEGIN AnPanel rewrite\n" + nginxLocationBlock(rewriteID) + "# END AnPanel rewrite\n"
	apRw := "# BEGIN AnPanel rewrite\n" + apacheRewriteBlock(rewriteID) + "# END AnPanel rewrite\n"
	// PHP-FPM snippet when php is present
	phpNginx := ""
	phpApache := ""
	if system.IsInstalled("php") {
		phpNginx = `  location ~ \.php$ {
    include fastcgi_params;
    fastcgi_param SCRIPT_FILENAME $document_root$fastcgi_script_name;
    fastcgi_pass 127.0.0.1:9000;
  }
`
		phpApache = "  # PHP handled via php-fpm / proxy when configured\n"
	}
	if server == "nginx" {
		if cert == "" {
			return fmt.Sprintf(`# Managed by AnPanel
server {
  listen 80;
  server_name %s;
  root %s;
  index index.php index.html index.htm;
  location /.well-known/acme-challenge/ { root /var/lib/anpanel/acme; }
%s%s}
`, domain, root, ngxLoc, phpNginx)
		}
		return fmt.Sprintf(`# Managed by AnPanel
server {
  listen 80;
  server_name %s;
  location /.well-known/acme-challenge/ { root /var/lib/anpanel/acme; }
  location / { return 301 https://$host$request_uri; }
}
server {
  listen 443 ssl;
  server_name %s;
  ssl_certificate %s;
  ssl_certificate_key %s;
  root %s;
  index index.php index.html index.htm;
%s%s}
`, domain, domain, cert, key, root, ngxLoc, phpNginx)
	}
	// apache
	if cert == "" {
		return fmt.Sprintf(`# Managed by AnPanel
<VirtualHost *:80>
  ServerName %s
  DocumentRoot %s
  Alias /.well-known/acme-challenge/ /var/lib/anpanel/acme/.well-known/acme-challenge/
  <Directory %s>
    AllowOverride All
    Require all granted
%s%s  </Directory>
</VirtualHost>
`, domain, root, root, apRw, phpApache)
	}
	return fmt.Sprintf(`# Managed by AnPanel
<VirtualHost *:80>
  ServerName %s
  Alias /.well-known/acme-challenge/ /var/lib/anpanel/acme/.well-known/acme-challenge/
  RedirectMatch 301 ^/(?!\.well-known/acme-challenge/)(.*)$ https://%s/$1
</VirtualHost>
<VirtualHost *:443>
  ServerName %s
  SSLEngine on
  SSLCertificateFile %s
  SSLCertificateKeyFile %s
  DocumentRoot %s
  <Directory %s>
    AllowOverride All
    Require all granted
%s%s  </Directory>
</VirtualHost>
`, domain, domain, domain, cert, key, root, root, apRw, phpApache)
}

func siteProxyConfig(server, domain, target, cert, key string) string {
	if server == "nginx" {
		if cert == "" {
			return fmt.Sprintf(`# Managed by AnPanel
server {
  listen 80;
  server_name %s;
  location /.well-known/acme-challenge/ { root /var/lib/anpanel/acme; }
  location / {
    proxy_pass %s;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection upgrade;
  }
}
`, domain, target)
		}
		return fmt.Sprintf(`# Managed by AnPanel
server {
  listen 80;
  server_name %s;
  location /.well-known/acme-challenge/ { root /var/lib/anpanel/acme; }
  location / { return 301 https://$host$request_uri; }
}
server {
  listen 443 ssl;
  server_name %s;
  ssl_certificate %s;
  ssl_certificate_key %s;
  location / {
    proxy_pass %s;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto https;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection upgrade;
  }
}
`, domain, domain, cert, key, target)
	}
	// apache — ensure trailing slash form for ProxyPass
	pass := target
	if !strings.HasSuffix(pass, "/") {
		pass += "/"
	}
	if cert == "" {
		return fmt.Sprintf(`# Managed by AnPanel
<VirtualHost *:80>
  ServerName %s
  Alias /.well-known/acme-challenge/ /var/lib/anpanel/acme/.well-known/acme-challenge/
  ProxyPreserveHost On
  ProxyPass /.well-known/acme-challenge/ !
  ProxyPass / %s
  ProxyPassReverse / %s
</VirtualHost>
`, domain, pass, pass)
	}
	return fmt.Sprintf(`# Managed by AnPanel
<VirtualHost *:80>
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
  ProxyPass / %s
  ProxyPassReverse / %s
</VirtualHost>
`, domain, domain, domain, cert, key, pass, pass)
}
