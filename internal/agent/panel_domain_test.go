package agent

import (
	"strings"
	"testing"
)

func TestPanelProxyConfig(t *testing.T) {
	for _, server := range []string{"nginx", "apache"} {
		plain := panelProxyConfig(server, "panel.example.com", 38888, "", "")
		if !strings.Contains(plain, "127.0.0.1:38888") || !strings.Contains(plain, "panel.example.com") {
			t.Fatalf("bad %s config: %s", server, plain)
		}
		tls := panelProxyConfig(server, "panel.example.com", 38888, "/cert", "/key")
		if !strings.Contains(tls, "/cert") || !strings.Contains(tls, "443") {
			t.Fatalf("bad TLS %s config", server)
		}
	}
}
func TestDomainValidation(t *testing.T) {
	if !domainName.MatchString("panel.example.com") {
		t.Fatal("valid domain rejected")
	}
	for _, v := range []string{"localhost", "bad domain", "-bad.example.com", "example"} {
		if domainName.MatchString(v) {
			t.Fatalf("invalid domain accepted: %s", v)
		}
	}
}
