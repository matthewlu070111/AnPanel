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

	"github.com/matthewlu070111/anpanel/internal/system"
)

var phpVersionRe = regexp.MustCompile(`^8\.[1-4]$`)

// installSoftware installs a component with mutual exclusion and method selection.
// method: source (default for nginx/apache/php/certbot), package, script (Docker/acme.sh)
func installSoftware(ctx context.Context, component string, opts map[string]string) (ActionResult, error) {
	component = strings.ToLower(strings.TrimSpace(component))
	method := strings.ToLower(strings.TrimSpace(opts["method"]))
	version := strings.TrimSpace(opts["version"])

	if component == "compose" {
		return ActionResult{}, errors.New("Docker Compose V2 已包含在 Docker Engine 中，请安装 Docker；无需单独安装 compose")
	}

	if err := checkSoftwareConflict(component); err != nil {
		return ActionResult{}, err
	}
	if system.IsInstalled(component) && component != "php" {
		return ActionResult{}, fmt.Errorf("%s 已安装", component)
	}

	switch component {
	case "nginx", "apache", "php", "certbot":
		if method == "" {
			method = "source"
		}
	case "acme.sh":
		if method == "" {
			method = "script"
		}
	case "docker":
		if method == "" {
			method = "script"
		}
	default:
		return ActionResult{}, fmt.Errorf("unsupported component %q", component)
	}

	switch method {
	case "package":
		return installFromPackage(ctx, component, version)
	case "source":
		return installFromSource(ctx, component, version)
	case "snap":
		if component != "certbot" {
			return ActionResult{}, errors.New("snap method is only valid for certbot")
		}
		return installCertbotSnap(ctx)
	case "script":
		if component == "docker" {
			return installDockerScript(ctx)
		}
		if component != "acme.sh" {
			return ActionResult{}, errors.New("script method is only valid for docker or acme.sh")
		}
		return installAcmeSh(ctx)
	default:
		return ActionResult{}, fmt.Errorf("unknown install method %q (source|package|script)", method)
	}
}

func updateSoftware(ctx context.Context, component string, opts map[string]string) (ActionResult, error) {
	component = strings.ToLower(strings.TrimSpace(component))
	if component == "compose" {
		return ActionResult{}, errors.New("compose 随 Docker 更新，请更新 Docker Engine")
	}
	if !system.IsInstalled(component) {
		return ActionResult{}, fmt.Errorf("%s 未安装", component)
	}
	method := strings.ToLower(strings.TrimSpace(opts["method"]))
	version := strings.TrimSpace(opts["version"])
	if component == "certbot" {
		path, _ := system.LookPath("certbot")
		method = system.CertbotInstallMethod(path)
	}
	// Prefer re-running source for compiled stacks; package for package installs.
	if method == "" {
		if component == "docker" || component == "acme.sh" {
			method = map[string]string{"docker": "package", "acme.sh": "script"}[component]
		} else {
			method = "source"
		}
	}
	switch method {
	case "package":
		return upgradeFromPackage(ctx, component, version)
	case "source":
		return installFromSource(ctx, component, version) // recompile / reinstall latest
	case "script":
		return installAcmeSh(ctx)
	case "snap":
		return run(ctx, "snap", "refresh", "certbot")
	default:
		return ActionResult{}, fmt.Errorf("unknown update method %q", method)
	}
}

func checkSoftwareConflict(component string) error {
	switch component {
	case "nginx":
		if system.IsInstalled("apache") {
			return errors.New("已安装 Apache，与 Nginx 互斥，不能同时安装。请先卸载 Apache")
		}
	case "apache":
		if system.IsInstalled("nginx") {
			return errors.New("已安装 Nginx，与 Apache 互斥，不能同时安装。请先卸载 Nginx")
		}
	case "certbot":
		if system.IsInstalled("acme.sh") {
			return errors.New("已安装 acme.sh，与 Certbot 互斥，不能同时安装。请先卸载 acme.sh")
		}
	case "acme.sh":
		if system.IsInstalled("certbot") {
			return errors.New("已安装 Certbot，与 acme.sh 互斥，不能同时安装。请先卸载 Certbot")
		}
	}
	return nil
}

func installFromPackage(ctx context.Context, component, version string) (ActionResult, error) {
	apt := exec.Command("apt-get", "--version").Run() == nil
	var packages []string
	if apt {
		switch component {
		case "nginx":
			packages = []string{"nginx"}
		case "apache":
			packages = []string{"apache2"}
		case "certbot":
			packages = []string{"certbot", "python3-certbot-nginx", "python3-certbot-apache"}
		case "docker":
			if !hasDockerRepo() {
				return ActionResult{}, errors.New("未检测到 Docker 官方仓库，请先配置 docker-ce 源")
			}
			packages = []string{"docker-ce", "docker-ce-cli", "containerd.io", "docker-compose-plugin"}
		case "php":
			if version == "" {
				version = "8.3"
			}
			if !phpVersionRe.MatchString(version) {
				return ActionResult{}, errors.New("php version must be 8.1–8.4")
			}
			// Debian/Ubuntu packages
			packages = []string{
				"php" + version + "-fpm", "php" + version + "-cli", "php" + version + "-common",
				"php" + version + "-mysql", "php" + version + "-xml", "php" + version + "-mbstring",
				"php" + version + "-curl", "php" + version + "-zip", "php" + version + "-gd",
			}
		default:
			return ActionResult{}, fmt.Errorf("package install not supported for %s", component)
		}
		// refresh index lightly
		_, _ = run(ctx, "apt-get", "update", "-y")
		args := append([]string{"install", "-y", "--no-install-recommends"}, packages...)
		res, err := run(ctx, "apt-get", args...)
		if err != nil {
			return ActionResult{}, err
		}
		_ = enableServiceAfterInstall(ctx, component, version)
		return ActionResult{Output: "installed via package\n" + res.Output}, nil
	}
	// dnf/yum
	manager := "yum"
	if exec.Command("dnf", "--version").Run() == nil {
		manager = "dnf"
	}
	switch component {
	case "nginx":
		packages = []string{"nginx"}
	case "apache":
		packages = []string{"httpd"}
	case "certbot":
		packages = []string{"certbot"}
	case "docker":
		if !hasDockerRepo() {
			return ActionResult{}, errors.New("未检测到 Docker 官方仓库")
		}
		packages = []string{"docker-ce", "docker-ce-cli", "containerd.io", "docker-compose-plugin"}
	case "php":
		if version == "" {
			version = "8.3"
		}
		packages = []string{"php", "php-fpm", "php-cli", "php-mysqlnd", "php-xml", "php-mbstring", "php-json"}
	default:
		return ActionResult{}, fmt.Errorf("package install not supported for %s", component)
	}
	args := append([]string{"install", "-y"}, packages...)
	res, err := run(ctx, manager, args...)
	if err != nil {
		return ActionResult{}, err
	}
	_ = enableServiceAfterInstall(ctx, component, version)
	return ActionResult{Output: "installed via package\n" + res.Output}, nil
}

func upgradeFromPackage(ctx context.Context, component, version string) (ActionResult, error) {
	apt := exec.Command("apt-get", "--version").Run() == nil
	if apt {
		_, _ = run(ctx, "apt-get", "update", "-y")
		pkgs := packageNames(component, version, true)
		args := append([]string{"install", "-y", "--only-upgrade"}, pkgs...)
		return run(ctx, "apt-get", args...)
	}
	manager := "yum"
	if exec.Command("dnf", "--version").Run() == nil {
		manager = "dnf"
	}
	pkgs := packageNames(component, version, false)
	args := append([]string{"update", "-y"}, pkgs...)
	return run(ctx, manager, args...)
}

func packageNames(component, version string, apt bool) []string {
	switch component {
	case "nginx":
		return []string{"nginx"}
	case "apache":
		if apt {
			return []string{"apache2"}
		}
		return []string{"httpd"}
	case "certbot":
		return []string{"certbot"}
	case "docker":
		return []string{"docker-ce", "docker-ce-cli", "containerd.io", "docker-compose-plugin"}
	case "php":
		if apt {
			if version == "" {
				version = "8.3"
			}
			return []string{"php" + version + "-fpm", "php" + version + "-cli"}
		}
		return []string{"php", "php-fpm"}
	}
	return nil
}

func enableServiceAfterInstall(ctx context.Context, component, version string) error {
	switch component {
	case "nginx":
		_, _ = run(ctx, "systemctl", "enable", "--now", "nginx")
	case "apache":
		if exec.Command("systemctl", "status", "apache2").Run() == nil || exec.Command("systemctl", "cat", "apache2").Run() == nil {
			_, _ = run(ctx, "systemctl", "enable", "--now", "apache2")
		} else {
			_, _ = run(ctx, "systemctl", "enable", "--now", "httpd")
		}
	case "docker":
		_, _ = run(ctx, "systemctl", "enable", "--now", "docker")
	case "php":
		if version == "" {
			version = "8.3"
		}
		for _, u := range []string{"php" + version + "-fpm", "php-fpm"} {
			if exec.Command("systemctl", "cat", u).Run() == nil {
				_, _ = run(ctx, "systemctl", "enable", "--now", u)
				break
			}
		}
	}
	return nil
}

func installFromSource(ctx context.Context, component, version string) (ActionResult, error) {
	switch component {
	case "nginx":
		return compileNginx(ctx, version)
	case "apache":
		return compileApache(ctx, version)
	case "php":
		if version == "" {
			version = "8.3"
		}
		if !phpVersionRe.MatchString(version) {
			return ActionResult{}, errors.New("php version must be one of 8.1, 8.2, 8.3, 8.4")
		}
		return compilePHP(ctx, version)
	case "certbot":
		return installCertbotPip(ctx)
	default:
		return ActionResult{}, fmt.Errorf("source install not supported for %s", component)
	}
}

func installBuildDeps(ctx context.Context) error {
	if exec.Command("apt-get", "--version").Run() == nil {
		_, _ = run(ctx, "apt-get", "update", "-y")
		_, err := run(ctx, "apt-get", "install", "-y", "--no-install-recommends",
			"build-essential", "curl", "wget", "ca-certificates", "tar", "xz-utils",
			"libpcre3-dev", "zlib1g-dev", "libssl-dev", "libxml2-dev", "libsqlite3-dev",
			"libcurl4-openssl-dev", "libpng-dev", "libjpeg-dev", "libonig-dev", "libzip-dev",
			"pkg-config", "autoconf", "re2c", "bison", "libapr1-dev", "libaprutil1-dev",
		)
		return err
	}
	manager := "yum"
	if exec.Command("dnf", "--version").Run() == nil {
		manager = "dnf"
	}
	_, err := run(ctx, manager, "install", "-y",
		"gcc", "gcc-c++", "make", "curl", "wget", "tar", "pcre-devel", "zlib-devel",
		"openssl-devel", "libxml2-devel", "sqlite-devel", "libcurl-devel", "oniguruma-devel",
		"libzip-devel", "autoconf", "bison", "re2c", "apr-devel", "apr-util-devel",
	)
	return err
}

func compileNginx(ctx context.Context, version string) (ActionResult, error) {
	if version == "" {
		version = "1.26.2"
	}
	if err := installBuildDeps(ctx); err != nil {
		return ActionResult{}, fmt.Errorf("install build deps: %w", err)
	}
	src := "/usr/local/src"
	_ = os.MkdirAll(src, 0755)
	tarball := fmt.Sprintf("nginx-%s.tar.gz", version)
	url := "https://nginx.org/download/" + tarball
	script := fmt.Sprintf(`set -euo pipefail
cd %s
curl -fsSL -o %s %s
tar -xzf %s
cd nginx-%s
./configure --prefix=/usr/local/nginx --with-http_ssl_module --with-http_v2_module --with-http_realip_module --with-http_gzip_static_module --with-stream --with-stream_ssl_module
make -j"$(nproc)"
make install
ln -sfn /usr/local/nginx/sbin/nginx /usr/local/sbin/nginx
mkdir -p /etc/nginx/conf.d /var/log/nginx /var/lib/anpanel/acme
if [ ! -f /etc/nginx/nginx.conf ]; then
  cat > /etc/nginx/nginx.conf <<'EOF'
user nobody;
worker_processes auto;
error_log /var/log/nginx/error.log;
pid /run/nginx.pid;
events { worker_connections 1024; }
http {
  include /usr/local/nginx/conf/mime.types;
  default_type application/octet-stream;
  sendfile on;
  keepalive_timeout 65;
  include /etc/nginx/conf.d/*.conf;
}
EOF
fi
cat > /etc/systemd/system/nginx.service <<'EOF'
[Unit]
Description=The nginx HTTP and reverse proxy server
After=network-online.target
[Service]
Type=forking
PIDFile=/run/nginx.pid
ExecStartPre=/usr/local/sbin/nginx -t -c /etc/nginx/nginx.conf
ExecStart=/usr/local/sbin/nginx -c /etc/nginx/nginx.conf
ExecReload=/usr/local/sbin/nginx -s reload
ExecStop=/usr/local/sbin/nginx -s quit
[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable --now nginx
nginx -v
`, src, tarball, url, tarball, version)
	return runShell(ctx, script)
}

func compileApache(ctx context.Context, version string) (ActionResult, error) {
	if version == "" {
		version = "2.4.62"
	}
	if err := installBuildDeps(ctx); err != nil {
		return ActionResult{}, fmt.Errorf("install build deps: %w", err)
	}
	// Use a consolidated install via package when APR toolchain is incomplete is fallback —
	// still attempt source with --with-included-apr if available; otherwise clear error.
	src := "/usr/local/src"
	_ = os.MkdirAll(src, 0755)
	script := fmt.Sprintf(`set -euo pipefail
cd %s
VER=%s
curl -fsSL -o httpd-$VER.tar.gz https://dlcdn.apache.org/httpd/httpd-$VER.tar.gz || curl -fsSL -o httpd-$VER.tar.gz https://archive.apache.org/dist/httpd/httpd-$VER.tar.gz
tar -xzf httpd-$VER.tar.gz
cd httpd-$VER
./configure --prefix=/usr/local/apache2 --enable-so --enable-ssl --enable-rewrite --enable-proxy --enable-proxy-http --with-mpm=event
make -j"$(nproc)"
make install
ln -sfn /usr/local/apache2/bin/httpd /usr/local/sbin/httpd
mkdir -p /etc/httpd/conf.d
cat > /etc/systemd/system/httpd.service <<'EOF'
[Unit]
Description=Apache HTTP Server
After=network-online.target
[Service]
Type=forking
ExecStart=/usr/local/apache2/bin/apachectl start
ExecReload=/usr/local/apache2/bin/apachectl graceful
ExecStop=/usr/local/apache2/bin/apachectl stop
[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable --now httpd
httpd -v
`, src, version)
	return runShell(ctx, script)
}

func compilePHP(ctx context.Context, version string) (ActionResult, error) {
	if err := installBuildDeps(ctx); err != nil {
		return ActionResult{}, fmt.Errorf("install build deps: %w", err)
	}
	src := "/usr/local/src"
	_ = os.MkdirAll(src, 0755)
	// latest patch series placeholders — use x.y.0 if patch unknown; php.net /distributions
	full := version + ".0"
	// try known recent patches via php.net releases list is heavy; allow override later
	switch version {
	case "8.1":
		full = "8.1.31"
	case "8.2":
		full = "8.2.27"
	case "8.3":
		full = "8.3.16"
	case "8.4":
		full = "8.4.3"
	}
	prefix := "/usr/local/php" + version
	script := fmt.Sprintf(`set -euo pipefail
cd %s
FULL=%s
curl -fsSL -o php-$FULL.tar.gz https://www.php.net/distributions/php-$FULL.tar.gz
tar -xzf php-$FULL.tar.gz
cd php-$FULL
./configure --prefix=%s --with-config-file-path=%s/etc --enable-fpm --with-fpm-user=www-data --with-fpm-group=www-data --enable-mbstring --with-curl --with-openssl --with-zlib --enable-gd --with-pdo-mysql --with-mysqli --enable-opcache --with-zip
make -j"$(nproc)"
make install
mkdir -p %s/etc
cp php.ini-production %s/etc/php.ini
cp %s/etc/php-fpm.conf.default %s/etc/php-fpm.conf 2>/dev/null || true
cp %s/etc/php-fpm.d/www.conf.default %s/etc/php-fpm.d/www.conf 2>/dev/null || true
ln -sfn %s/bin/php /usr/local/bin/php
ln -sfn %s/sbin/php-fpm /usr/local/sbin/php-fpm
cat > /etc/systemd/system/php-fpm.service <<EOF
[Unit]
Description=PHP FastCGI Process Manager
After=network.target
[Service]
Type=simple
ExecStart=%s/sbin/php-fpm --nodaemonize --fpm-config %s/etc/php-fpm.conf
ExecReload=/bin/kill -USR2 \$MAINPID
[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable --now php-fpm
php -v
`, src, full, prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix, prefix)
	return runShell(ctx, script)
}

func installCertbotPip(ctx context.Context) (ActionResult, error) {
	// "source-like" isolated install via pipx/venv (not distro package)
	script := `set -euo pipefail
if command -v apt-get >/dev/null; then
  apt-get update -y
  apt-get install -y --no-install-recommends python3 python3-venv python3-pip
fi
python3 -m venv /usr/local/certbot
/usr/local/certbot/bin/pip install -U pip wheel
/usr/local/certbot/bin/pip install certbot certbot-nginx certbot-apache
ln -sfn /usr/local/certbot/bin/certbot /usr/local/bin/certbot
certbot --version
`
	return runShell(ctx, script)
}

func installCertbotSnap(ctx context.Context) (ActionResult, error) {
	if _, err := exec.LookPath("snap"); err != nil {
		return ActionResult{}, errors.New("snap is not installed")
	}
	res, err := run(ctx, "snap", "install", "--classic", "certbot")
	if err != nil {
		return ActionResult{}, err
	}
	return ActionResult{Output: "installed via snap\n" + res.Output}, nil
}

func installAcmeSh(ctx context.Context) (ActionResult, error) {
	script := `set -euo pipefail
export HOME=/root
curl -fsSL https://get.acme.sh | sh -s email=anpanel@localhost
ln -sfn /root/.acme.sh/acme.sh /usr/local/bin/acme.sh
acme.sh --version || /root/.acme.sh/acme.sh --version
`
	return runShell(ctx, script)
}

func installDockerScript(ctx context.Context) (ActionResult, error) {
	script := `set -euo pipefail
tmp=/tmp/anpanel-get-docker.sh
trap 'rm -f "$tmp"' EXIT
curl -fsSL https://get.docker.com -o "$tmp"
sh "$tmp"
systemctl enable --now docker
docker --version
docker compose version
`
	return runShell(ctx, script)
}

func runShell(ctx context.Context, script string) (ActionResult, error) {
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("anpanel-install-%d.sh", time.Now().UnixNano()))
	if err := os.WriteFile(tmp, []byte(script), 0700); err != nil {
		return ActionResult{}, err
	}
	defer os.Remove(tmp)
	cmd := exec.CommandContext(ctx, "bash", tmp)
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	b, err := cmd.CombinedOutput()
	out := redact(string(b))
	if err != nil {
		return ActionResult{}, fmt.Errorf("install failed: %s", out)
	}
	return ActionResult{Output: out}, nil
}
