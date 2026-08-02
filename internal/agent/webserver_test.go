package agent

import (
	"runtime"
	"testing"
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
