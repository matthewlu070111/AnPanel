package agent

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/matthewlu070111/anpanel/internal/domain"
	"github.com/matthewlu070111/anpanel/internal/system"
)

func certbotPath() string {
	p, _ := system.LookPath("certbot")
	return p
}

func discoverCertificates() ([]domain.Certificate, error) {
	byDomain := map[string]domain.Certificate{}

	live := "/etc/letsencrypt/live"
	if entries, err := os.ReadDir(live); err == nil {
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			name := e.Name()
			fullchain := filepath.Join(live, name, "fullchain.pem")
			if _, err := os.Stat(fullchain); err != nil {
				fullchain = filepath.Join(live, name, "cert.pem")
			}
			key := filepath.Join(live, name, "privkey.pem")
			if cert, err := parseCertificateFile(name, fullchain, key, "letsencrypt", true); err == nil {
				byDomain[strings.ToLower(cert.Domain)] = cert
			}
		}
	}

	anpanelCerts := "/etc/anpanel/certs"
	if entries, err := os.ReadDir(anpanelCerts); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			fullchain := filepath.Join(anpanelCerts, name, "fullchain.pem")
			key := filepath.Join(anpanelCerts, name, "key.pem")
			if cert, err := parseCertificateFile(name, fullchain, key, "anpanel", true); err == nil {
				k := strings.ToLower(cert.Domain)
				if _, ok := byDomain[k]; !ok {
					byDomain[k] = cert
				}
			}
		}
	}

	for _, home := range []string{"/root/.acme.sh", "/home/anpanel/.acme.sh"} {
		entries, err := os.ReadDir(home)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			base := strings.TrimSuffix(e.Name(), "_ecc")
			if !domainName.MatchString(base) {
				continue
			}
			dir := filepath.Join(home, e.Name())
			fullchain := filepath.Join(dir, "fullchain.cer")
			if _, err := os.Stat(fullchain); err != nil {
				fullchain = filepath.Join(dir, base+".cer")
			}
			key := filepath.Join(dir, base+".key")
			if cert, err := parseCertificateFile(base, fullchain, key, "acme.sh", true); err == nil {
				k := strings.ToLower(cert.Domain)
				if _, ok := byDomain[k]; !ok {
					byDomain[k] = cert
				}
			}
		}
	}

	out := make([]domain.Certificate, 0, len(byDomain))
	for _, c := range byDomain {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].DaysLeft != out[j].DaysLeft {
			return out[i].DaysLeft < out[j].DaysLeft
		}
		return out[i].Domain < out[j].Domain
	})
	return out, nil
}

func parseCertificateFile(domainHint, certPath, keyPath, source string, autoRenew bool) (domain.Certificate, error) {
	b, err := os.ReadFile(certPath)
	if err != nil {
		return domain.Certificate{}, err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return domain.Certificate{}, errors.New("no PEM block in certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return domain.Certificate{}, err
	}
	name := domainHint
	if name == "" && len(cert.DNSNames) > 0 {
		name = cert.DNSNames[0]
	}
	if name == "" {
		name = cert.Subject.CommonName
	}
	issuer := cert.Issuer.CommonName
	if issuer == "" && len(cert.Issuer.Organization) > 0 {
		issuer = cert.Issuer.Organization[0]
	}
	days := int(time.Until(cert.NotAfter).Hours() / 24)
	return domain.Certificate{
		Domain:    name,
		Issuer:    issuer,
		Path:      certPath,
		KeyPath:   keyPath,
		ExpiresAt: cert.NotAfter.UTC(),
		Source:    source,
		AutoRenew: autoRenew,
		DaysLeft:  days,
	}, nil
}

func renewCertificate(ctx context.Context, domain, tool string, force bool) (ActionResult, error) {
	domain = normalizedDomain(domain)
	if domain != "" && !domainName.MatchString(domain) {
		return ActionResult{}, errors.New("invalid domain name")
	}
	if tool == "" {
		tool = detectCertTool(domain)
	}
	switch tool {
	case "certbot":
		certbot := certbotPath()
		if certbot == "" {
			return ActionResult{}, errors.New("certbot is not installed")
		}
		args := []string{"renew", "--non-interactive"}
		if domain != "" {
			args = append(args, "--cert-name", domain)
		}
		if force {
			args = append(args, "--force-renewal")
		}
		res, err := run(ctx, certbot, args...)
		if err != nil {
			return ActionResult{}, err
		}
		_ = tryReloadWebServers(ctx)
		return ActionResult{Output: "certificate renew completed\n" + res.Output}, nil
	case "acme.sh":
		if domain == "" {
			return ActionResult{}, errors.New("domain is required for acme.sh renew")
		}
		acme := acmePath()
		if acme == "" {
			return ActionResult{}, errors.New("acme.sh is not installed")
		}
		args := []string{"--renew", "-d", domain}
		if force {
			args = append(args, "--force")
		}
		res, err := run(ctx, acme, args...)
		if err != nil {
			return ActionResult{}, err
		}
		certDir := filepath.Join("/etc/anpanel/certs", domain)
		fullchain := filepath.Join(certDir, "fullchain.pem")
		key := filepath.Join(certDir, "key.pem")
		if st, err := os.Stat(certDir); err == nil && st.IsDir() {
			if _, err := run(ctx, acme, "--install-cert", "-d", domain, "--key-file", key, "--fullchain-file", fullchain); err != nil {
				return ActionResult{}, err
			}
		}
		_ = tryReloadWebServers(ctx)
		return ActionResult{Output: "certificate renew completed\n" + res.Output}, nil
	default:
		return ActionResult{}, errors.New("certificate tool must be certbot or acme.sh")
	}
}

func tryReloadWebServers(ctx context.Context) error {
	var last error
	for _, s := range []string{"nginx", "apache"} {
		if err := configTest(ctx, s); err != nil {
			continue
		}
		if err := reloadServer(ctx, s); err != nil {
			last = err
		}
	}
	return last
}

func detectCertTool(domain string) string {
	if domain != "" {
		if _, err := os.Stat(filepath.Join("/etc/letsencrypt/live", domain)); err == nil {
			return "certbot"
		}
		if _, err := os.Stat(filepath.Join("/etc/anpanel/certs", domain)); err == nil {
			return "acme.sh"
		}
		if acmePath() != "" {
			for _, home := range []string{"/root/.acme.sh", "/home/anpanel/.acme.sh"} {
				if _, err := os.Stat(filepath.Join(home, domain)); err == nil {
					return "acme.sh"
				}
				if _, err := os.Stat(filepath.Join(home, domain+"_ecc")); err == nil {
					return "acme.sh"
				}
			}
		}
	}
	if certbotPath() != "" {
		return "certbot"
	}
	if acmePath() != "" {
		return "acme.sh"
	}
	return "certbot"
}

func issueSiteCertificate(ctx context.Context, domain, server, tool, email string) (ActionResult, error) {
	domain = normalizedDomain(domain)
	if !domainName.MatchString(domain) {
		return ActionResult{}, errors.New("invalid domain name")
	}
	if server != "nginx" && server != "apache" {
		return ActionResult{}, errors.New("web server must be nginx or apache")
	}
	if tool == "" {
		tool = "certbot"
	}
	if err := os.MkdirAll("/var/lib/anpanel/acme", 0755); err != nil {
		return ActionResult{}, err
	}
	switch tool {
	case "certbot":
		certbot := certbotPath()
		if certbot == "" {
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
		return run(ctx, certbot, args...)
	case "acme.sh":
		acme := acmePath()
		if acme == "" {
			return ActionResult{}, errors.New("acme.sh is not installed")
		}
		challenge := "/var/lib/anpanel/acme"
		certDir := filepath.Join("/etc/anpanel/certs", domain)
		if err := os.MkdirAll(certDir, 0700); err != nil {
			return ActionResult{}, err
		}
		res, err := run(ctx, acme, "--issue", "--webroot", challenge, "-d", domain)
		if err != nil {
			return ActionResult{}, err
		}
		fullchain := filepath.Join(certDir, "fullchain.pem")
		key := filepath.Join(certDir, "key.pem")
		if _, err := run(ctx, acme, "--install-cert", "-d", domain, "--key-file", key, "--fullchain-file", fullchain); err != nil {
			return ActionResult{}, err
		}
		path, err := siteConfigPath(server, domain)
		if err != nil {
			return ActionResult{}, err
		}
		if raw, err := os.ReadFile(path); err == nil {
			updated := injectTLSIntoSiteConfig(server, string(raw), domain, fullchain, key)
			if updated != string(raw) {
				if _, err := applyWebConfig(ctx, path, updated); err != nil {
					return ActionResult{}, err
				}
			}
		}
		return ActionResult{Output: "certificate issued\n" + res.Output}, nil
	default:
		return ActionResult{}, errors.New("certificate tool must be certbot or acme.sh")
	}
}

func injectTLSIntoSiteConfig(server, raw, domain, cert, key string) string {
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "listen 443") || strings.Contains(lower, "sslengine") {
		return raw
	}
	proxy := ""
	if m := nginxProxy.FindStringSubmatch(raw); len(m) > 1 {
		proxy = strings.TrimSpace(m[1])
	}
	root := ""
	if m := nginxRoot.FindStringSubmatch(raw); len(m) > 1 {
		root = strings.TrimSpace(m[1])
	}
	if proxy != "" {
		return siteProxyConfig(server, domain, proxy, cert, key)
	}
	if root == "" {
		root = defaultWebRoot(domain)
	}
	return siteStaticConfig(server, domain, root, cert, key, "none")
}
