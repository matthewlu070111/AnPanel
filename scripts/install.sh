#!/usr/bin/env bash
set -Eeuo pipefail

TARGET_VERSION=${ANPANEL_VERSION:-latest} # anpanel:version
REGION_OVERRIDE=auto
OPEN_FIREWALL=0
RELEASE_BASE=${ANPANEL_RELEASE_BASE:-https://github.com/matthewlu070111/anpanel/releases}
EOL_WARNING=''

for arg in "$@"; do
  case "$arg" in
    --region=cn|--region=global) REGION_OVERRIDE=${arg#*=} ;;
    --version=*) TARGET_VERSION=${arg#*=} ;;
    --open-firewall) OPEN_FIREWALL=1 ;;
    *) printf 'Unknown option: %s\n' "$arg" >&2; exit 2 ;;
  esac
done

[[ ${EUID:-$(id -u)} -eq 0 ]] || { echo 'Run as root.' >&2; exit 1; }
[[ -r /etc/os-release ]] || { echo '/etc/os-release is required.' >&2; exit 1; }
command -v systemctl >/dev/null || { echo 'systemd is required.' >&2; exit 1; }
command -v curl >/dev/null || { echo 'curl is required.' >&2; exit 1; }

# shellcheck disable=SC1091
source /etc/os-release
OS_ID=${ID,,}; OS_VERSION=${VERSION_ID:-unknown}
case "$OS_ID:$OS_VERSION" in
  ubuntu:18.04|ubuntu:20.04|ubuntu:22.04|ubuntu:24.04|ubuntu:26.04|debian:10|debian:11|debian:12|debian:13|centos:7*|rocky:8*|rocky:9*|rocky:10*|almalinux:8*|almalinux:9*|almalinux:10*) ;;
  *) echo "Unsupported distribution: $OS_ID $OS_VERSION" >&2; exit 1 ;;
esac
case "$(uname -m)" in x86_64) ARCH=amd64;; aarch64|arm64) ARCH=arm64;; *) echo 'Only amd64 and arm64 are supported.' >&2; exit 1;; esac
if command -v apt-get >/dev/null; then PKG_FAMILY=apt; else PKG_FAMILY=rpm; fi

detect_region() {
  [[ "$REGION_OVERRIDE" != auto ]] && { echo "$REGION_OVERRIDE"; return; }
  local country=''
  country=$(curl -qfsS --max-time 3 https://www.cloudflare.com/cdn-cgi/trace 2>/dev/null | awk -F= '$1=="loc"{print $2}') || true
  [[ "$country" == CN ]] && echo cn || echo global
}
REGION=$(detect_region)

pick_fastest() {
  local best='' best_time=999999 candidate result
  for candidate in "$@"; do
	    result=$(curl -qLso /dev/null --connect-timeout 2 --max-time 5 -w '%{time_total}' "$candidate/debian/README" 2>/dev/null) || continue
    awk "BEGIN{exit !($result < $best_time)}" && { best=$candidate; best_time=$result; }
  done
	  [[ -n "$best" ]] && echo "$best" || return 1
}
if [[ "$REGION" == cn ]]; then
	  APT_MIRROR=$(pick_fastest https://mirrors.tuna.tsinghua.edu.cn https://mirrors.ustc.edu.cn https://mirrors.aliyun.com || echo https://mirrors.tuna.tsinghua.edu.cn)
  APT_PORTS_MIRROR=https://mirrors.tuna.tsinghua.edu.cn
  RPM_MIRROR=https://mirrors.tuna.tsinghua.edu.cn
else
  APT_MIRROR=https://deb.debian.org; APT_PORTS_MIRROR=https://ports.ubuntu.com; RPM_MIRROR=''
fi

mkdir -p /var/lib/anpanel/backups
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=lib/sources.sh
source "$SCRIPT_DIR/lib/sources.sh"

if [[ "$TARGET_VERSION" == latest ]]; then
  TARGET_VERSION=$(curl -qfsSL --max-time 10 "$RELEASE_BASE/latest" -o /dev/null -w '%{url_effective}' | sed 's#.*/tag/##')
fi
[[ -n "$TARGET_VERSION" ]] || { echo 'Could not resolve a release version.' >&2; exit 1; }
RELEASE_BASE=${RELEASE_BASE%/}
[[ "$TARGET_VERSION" =~ ^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$ ]] || { echo "Invalid release version: $TARGET_VERSION" >&2; exit 2; }
[[ "$RELEASE_BASE" =~ ^https://[^[:space:]]+$ ]] || { echo "Invalid release base URL: $RELEASE_BASE" >&2; exit 2; }
UPDATE_CHANNEL=stable
case "$TARGET_VERSION" in
  build-*|*-alpha*|*-beta*|*-rc*) UPDATE_CHANNEL=prerelease ;;
esac
ASSET="anpanel-linux-$ARCH"
TMP=$(mktemp -d); trap 'rm -rf "$TMP"' EXIT
download_release_asset() {
  local name destination url
  name=${1:?release asset name is required}
  destination=${2:?release asset destination is required}
  url="$RELEASE_BASE/download/$TARGET_VERSION/$name"
  printf '[AnPanel] Downloading %s from %s\n' "$name" "$url"
  # -q disables ~/.curlrc so host-specific curl settings cannot corrupt URLs.
  curl -qfL --show-error --retry 3 --connect-timeout 10 --max-time 300 \
    --proto '=https' --tlsv1.2 -o "$destination" -- "$url"
}
printf '[AnPanel] Release: %s (%s)\n' "$TARGET_VERSION" "$ARCH"
download_release_asset "$ASSET" "$TMP/$ASSET"
download_release_asset "$ASSET.sha256" "$TMP/$ASSET.sha256"
(cd "$TMP" && sha256sum -c "$ASSET.sha256")
if [[ -n ${ANPANEL_RELEASE_PUBLIC_KEY:-} ]]; then
  command -v openssl >/dev/null || { echo 'openssl is required for signature verification.' >&2; exit 1; }
  download_release_asset "$ASSET.sig" "$TMP/$ASSET.sig"
  openssl pkeyutl -verify -pubin -inkey "$ANPANEL_RELEASE_PUBLIC_KEY" -rawin -in "$TMP/$ASSET" -sigfile "$TMP/$ASSET.sig"
fi

# Do not change system repositories until the requested release has been
# downloaded and verified successfully.
configure_sources

install -m 0755 "$TMP/$ASSET" /usr/local/bin/anpanel
ln -sf /usr/local/bin/anpanel /usr/local/bin/anpanelctl
getent group anpanel-agent >/dev/null || groupadd --system anpanel-agent
id anpanel >/dev/null 2>&1 || useradd --system --home /var/lib/anpanel --shell /usr/sbin/nologin --gid anpanel-agent anpanel
install -d -o anpanel -g anpanel-agent -m 0750 /var/lib/anpanel /var/log/anpanel
install -d -o root -g anpanel-agent -m 0750 /etc/anpanel /run/anpanel
install -d -o root -g root -m 0750 /etc/anpanel/compose

PORT=''
if [[ -f /etc/anpanel/config.json ]]; then
  PORT=$(/usr/local/bin/anpanel ctl show-port)
  [[ "$PORT" =~ ^[0-9]+$ ]] || { echo 'Existing AnPanel port is invalid.' >&2; exit 1; }
  echo 'Existing AnPanel configuration and administrator will be preserved.'
else
  for _ in $(seq 1 100); do
    candidate=$(shuf -i 20000-60000 -n 1)
    if ! ss -H -ltn "sport = :$candidate" 2>/dev/null | grep -q .; then PORT=$candidate; break; fi
  done
  [[ -n "$PORT" ]] || { echo 'Could not find a free port.' >&2; exit 1; }
  umask 0077
  head -c 48 /dev/urandom | base64 > /etc/anpanel/agent.token
  head -c 48 /dev/urandom | base64 > /etc/anpanel/session.key
  cat > /etc/anpanel/config.json <<EOF
{"listen":"0.0.0.0","port":$PORT,"database_path":"/var/lib/anpanel/anpanel.db","agent_socket":"/run/anpanel/agent.sock","agent_token_file":"/etc/anpanel/agent.token","session_key_file":"/etc/anpanel/session.key","notification_path":"/etc/anpanel/notifications.json","region":"$REGION","metrics_interval_seconds":5,"update_channel":"$UPDATE_CHANNEL"}
EOF
fi
if [[ ! -f /etc/anpanel/notifications.json ]]; then
  umask 0077
  printf '{}\n' > /etc/anpanel/notifications.json
fi
chown anpanel:anpanel-agent /etc/anpanel/notifications.json
chmod 0600 /etc/anpanel/notifications.json
chown root:anpanel-agent /etc/anpanel/config.json /etc/anpanel/agent.token /etc/anpanel/session.key
chmod 0640 /etc/anpanel/config.json /etc/anpanel/agent.token /etc/anpanel/session.key

cat > /etc/systemd/system/anpanel-agent.service <<'UNIT'
[Unit]
Description=AnPanel privileged agent
After=network-online.target
Wants=network-online.target
[Service]
Type=simple
Group=anpanel-agent
ExecStart=/usr/local/bin/anpanel agent
Restart=on-failure
RestartSec=3
RuntimeDirectory=anpanel
RuntimeDirectoryMode=0750
UMask=0027
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=read-only
ProtectSystem=false
ReadWritePaths=/etc/anpanel /var/lib/anpanel /var/log/anpanel /run/anpanel -/etc/nginx -/etc/apache2 -/etc/httpd -/etc/letsencrypt -/var/lib/letsencrypt -/var/log/letsencrypt -/root/.acme.sh
[Install]
WantedBy=multi-user.target
UNIT
cat > /etc/systemd/system/anpanel-web.service <<'UNIT'
[Unit]
Description=AnPanel web interface
After=anpanel-agent.service
Requires=anpanel-agent.service
[Service]
Type=simple
User=anpanel
Group=anpanel-agent
ExecStart=/usr/local/bin/anpanel web
Restart=on-failure
RestartSec=3
UMask=0027
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
ProtectSystem=strict
ReadOnlyPaths=/etc/anpanel
ReadWritePaths=/var/lib/anpanel /var/log/anpanel /run/anpanel
[Install]
WantedBy=multi-user.target
UNIT
chmod 0644 /etc/systemd/system/anpanel-agent.service /etc/systemd/system/anpanel-web.service
ADMIN_CREDENTIALS=$(/usr/local/bin/anpanel ctl init-admin)
chown -R anpanel:anpanel-agent /var/lib/anpanel
systemctl daemon-reload
systemctl enable anpanel-agent.service anpanel-web.service
# enable --now does not reload an already-running process after binary replace.
# Always restart so upgrades (including panel.self_update) pick up the new binary.
systemctl restart anpanel-agent.service anpanel-web.service
systemctl is-active --quiet anpanel-agent.service anpanel-web.service || {
  echo 'AnPanel services failed to start after install/upgrade.' >&2
  systemctl --no-pager --full status anpanel-agent.service anpanel-web.service || true
  exit 1
}

if [[ "$OPEN_FIREWALL" == 1 ]]; then
  if command -v firewall-cmd >/dev/null; then firewall-cmd --permanent --add-port="$PORT/tcp" && firewall-cmd --reload
  elif command -v ufw >/dev/null; then ufw allow "$PORT/tcp"; fi
fi
IP=$(hostname -I 2>/dev/null | awk '{print $1}')
printf '\nAnPanel installed successfully.\nURL: http://%s:%s\n%s\n' "${IP:-SERVER_IP}" "$PORT" "$ADMIN_CREDENTIALS"
[[ -z "$EOL_WARNING" ]] || printf '\nSECURITY WARNING: %s\n' "$EOL_WARNING"
printf 'Plain HTTP is enabled until you bind a domain and configure HTTPS.\n'
