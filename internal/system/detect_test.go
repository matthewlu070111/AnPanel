package system

import "testing"

func TestCertbotInstallMethod(t *testing.T) {
	for path, want := range map[string]string{
		"/snap/bin/certbot": "snap",
		"/usr/bin/certbot":  "package",
		"/usr/local/certbot/bin/certbot": "source",
	} {
		if got := CertbotInstallMethod(path); got != want {
			t.Fatalf("%s: got %s, want %s", path, got, want)
		}
	}
}
