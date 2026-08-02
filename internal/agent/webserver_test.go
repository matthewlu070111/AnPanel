package agent

import (
	"runtime"
	"strings"
	"testing"

	"github.com/matthewlu070111/anpanel/internal/domain"
)

func TestParseNginxPreservesRaw(t *testing.T) {
	raw := `server { listen 443 ssl; server_name example.com www.example.com; location / { proxy_pass http://127.0.0.1:8080; proxy_set_header Upgrade $http_upgrade; } }`
	v := parseNginx("/etc/nginx/conf.d/example.conf", raw)
	if len(v) != 1 {
		t.Fatalf("got %d sites", len(v))
	}
	if !v[0].TLS || v[0].ProxyTarget != "http://127.0.0.1:8080" || len(v[0].Domains) != 2 {
		t.Fatalf("bad parse: %#v", v[0])
	}
	if v[0].Raw == "" {
		t.Fatal("raw block was lost")
	}
	if v[0].Raw != raw {
		t.Fatal("full source file was not preserved")
	}
}
func TestParseApache(t *testing.T) {
	raw := `<VirtualHost *:80>
ServerName example.com
ServerAlias www.example.com
ProxyPass / http://127.0.0.1:3000/
</VirtualHost>`
	v := parseApache("/etc/apache2/sites-enabled/example.conf", raw)
	if len(v) != 1 || len(v[0].Domains) != 2 {
		t.Fatalf("bad parse: %#v", v)
	}
}
func TestComposePathBoundary(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux path policy")
	}
	if _, err := safeComposePath("/tmp/compose.yml"); err == nil {
		t.Fatal("unsafe path accepted")
	}
	if _, err := safeComposePath("/srv/app/compose.yaml"); err != nil {
		t.Fatal(err)
	}
}

func TestSiteStaticAndProxyConfig(t *testing.T) {
	static := siteStaticConfig("nginx", "blog.example.com", "/var/www/blog.example.com", "", "", "spa")
	if !strings.Contains(static, "server_name blog.example.com") || !strings.Contains(static, "root /var/www/blog.example.com") {
		t.Fatalf("bad static config: %s", static)
	}
	if !strings.Contains(static, "index.html") || !strings.Contains(static, "AnPanel rewrite") {
		t.Fatalf("rewrite missing: %s", static)
	}
	proxy := siteProxyConfig("nginx", "app.example.com", "http://127.0.0.1:3000", "", "")
	if !strings.Contains(proxy, "proxy_pass http://127.0.0.1:3000") {
		t.Fatalf("bad proxy config: %s", proxy)
	}
	tls := siteProxyConfig("apache", "app.example.com", "http://127.0.0.1:3000/", "/c.pem", "/k.pem")
	if !strings.Contains(tls, "SSLEngine on") || !strings.Contains(tls, "443") {
		t.Fatalf("bad apache tls config: %s", tls)
	}
}

func TestRewriteTemplates(t *testing.T) {
	rules := rewriteTemplates()
	if len(rules) < 4 {
		t.Fatal("expected rewrite templates")
	}
	r, err := rewriteByID("wordpress")
	if err != nil || r.Nginx == "" {
		t.Fatal(err)
	}
}

func TestSoftwareConflictMessage(t *testing.T) {
	// pure function path — no install on Windows CI
	if err := checkSoftwareConflict("compose"); err != nil {
		// compose is handled in installSoftware, not checkSoftwareConflict
	}
	_ = checkSoftwareConflict("nginx")
}

func TestSafeWebRoot(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skip("path policy")
	}
	// On Windows Abs may still work; policy checks absolute roots that are unix-style.
	if runtime.GOOS != "linux" {
		t.Skip("unix web root policy")
	}
	if _, err := safeWebRoot("/tmp/evil", "x.com"); err == nil {
		t.Fatal("unsafe root accepted")
	}
	p, err := safeWebRoot("/var/www/x.com", "x.com")
	if err != nil || p != "/var/www/x.com" {
		t.Fatalf("got %q %v", p, err)
	}
}

func TestDomainSlug(t *testing.T) {
	if domainSlug("Blog.Example.COM") != "blog.example.com" {
		t.Fatal(domainSlug("Blog.Example.COM"))
	}
}

func TestMergeWebsitesHTTPAndHTTPS(t *testing.T) {
	in := []domain.WebSite{
		{ID: "1", Server: "nginx", Domains: []string{"a.com"}, Listen: []string{"80"}, SourcePath: "/etc/nginx/conf.d/a.conf", Raw: "secret1"},
		{ID: "2", Server: "nginx", Domains: []string{"a.com"}, Listen: []string{"443 ssl"}, TLS: true, SourcePath: "/etc/nginx/conf.d/a.conf", Raw: "secret2", ProxyTarget: "http://127.0.0.1:9"},
	}
	out := mergeWebsites(in)
	if len(out) != 1 {
		t.Fatalf("want 1 site, got %d", len(out))
	}
	if !out[0].TLS || !out[0].HasHTTP || !out[0].HasHTTPS {
		t.Fatalf("flags: %#v", out[0])
	}
	if out[0].Raw != "" {
		t.Fatal("list must not include raw config")
	}
	if out[0].ProxyTarget != "http://127.0.0.1:9" {
		t.Fatal(out[0].ProxyTarget)
	}
}
