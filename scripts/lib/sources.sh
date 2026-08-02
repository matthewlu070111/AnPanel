#!/usr/bin/env bash

source_log() { printf '[AnPanel:sources] %s\n' "$*"; }

backup_sources() {
  SOURCE_BACKUP="/var/lib/anpanel/backups/sources-$(date +%Y%m%d-%H%M%S)"
  mkdir -p "$SOURCE_BACKUP"
  if [[ "$PKG_FAMILY" == apt ]]; then
    cp -a /etc/apt/sources.list "$SOURCE_BACKUP/" 2>/dev/null || true
    cp -a /etc/apt/sources.list.d "$SOURCE_BACKUP/" 2>/dev/null || true
	[[ -f /etc/apt/apt.conf.d/99anpanel-archive ]] && cp -a /etc/apt/apt.conf.d/99anpanel-archive "$SOURCE_BACKUP/" || true
  else
    cp -a /etc/yum.repos.d "$SOURCE_BACKUP/" 2>/dev/null || true
  fi
  source_log "backup saved to $SOURCE_BACKUP"
}

restore_sources() {
  source_log "repository validation failed; restoring $SOURCE_BACKUP"
  if [[ "$PKG_FAMILY" == apt ]]; then
    [[ -f "$SOURCE_BACKUP/sources.list" ]] && cp -a "$SOURCE_BACKUP/sources.list" /etc/apt/sources.list
    [[ -d "$SOURCE_BACKUP/sources.list.d" ]] && { rm -rf /etc/apt/sources.list.d; cp -a "$SOURCE_BACKUP/sources.list.d" /etc/apt/; }
	if [[ -f "$SOURCE_BACKUP/99anpanel-archive" ]]; then cp -a "$SOURCE_BACKUP/99anpanel-archive" /etc/apt/apt.conf.d/; else rm -f /etc/apt/apt.conf.d/99anpanel-archive; fi
  else
    [[ -d "$SOURCE_BACKUP/yum.repos.d" ]] && { rm -rf /etc/yum.repos.d; cp -a "$SOURCE_BACKUP/yum.repos.d" /etc/; }
  fi
}

replace_uri_in_file() {
  local file=$1 from=$2 to=$3
  [[ -f "$file" ]] || return 0
  sed -i "s#${from}#${to}#g" "$file"
}

ubuntu_advantage_attached() { command -v pro >/dev/null && pro status 2>/dev/null | grep -qiE 'esm-(apps|infra).*enabled'; }
freexian_elts_active() { find /etc/apt/sources.list.d -maxdepth 1 -type f -iname '*freexian*' 2>/dev/null | grep -q .; }

configure_apt_sources() {
  local files=()
  [[ -f /etc/apt/sources.list ]] && files+=(/etc/apt/sources.list)
  while IFS= read -r -d '' f; do files+=("$f"); done < <(find /etc/apt/sources.list.d -maxdepth 1 -type f \( -name '*.list' -o -name '*.sources' \) -print0 2>/dev/null)
  local f
  for f in "${files[@]}"; do
	    case "$OS_ID:$OS_VERSION" in
      ubuntu:18.04)
	        ubuntu_advantage_attached && continue
        replace_uri_in_file "$f" 'archive.ubuntu.com/ubuntu' 'old-releases.ubuntu.com/ubuntu'
        replace_uri_in_file "$f" 'security.ubuntu.com/ubuntu' 'old-releases.ubuntu.com/ubuntu'
		EOL_WARNING='Ubuntu 18.04 standard support ended; archive/ESM status must be reviewed.'
		printf 'Acquire::Check-Valid-Until "false";\n' > /etc/apt/apt.conf.d/99anpanel-archive
        ;;
      debian:10)
	        freexian_elts_active && continue
		if [[ "$REGION" == cn ]]; then target='mirrors.tuna.tsinghua.edu.cn/debian-archive/debian'; security_target='mirrors.tuna.tsinghua.edu.cn/debian-archive/debian-security'; else target='archive.debian.org/debian'; security_target='archive.debian.org/debian-security'; fi
		replace_uri_in_file "$f" 'deb.debian.org/debian' "$target"
		replace_uri_in_file "$f" 'security.debian.org/debian-security' "$security_target"
		EOL_WARNING='Debian 10 uses an archive unless a detected ELTS subscription is active.'
		printf 'Acquire::Check-Valid-Until "false";\n' > /etc/apt/apt.conf.d/99anpanel-archive
        ;;
      ubuntu:*)
        [[ "$REGION" == cn ]] || continue
        replace_uri_in_file "$f" 'archive.ubuntu.com/ubuntu' "$APT_MIRROR/ubuntu"
        replace_uri_in_file "$f" 'ports.ubuntu.com/ubuntu-ports' "$APT_PORTS_MIRROR/ubuntu-ports"
        ;;
      debian:*)
        [[ "$REGION" == cn ]] || continue
        replace_uri_in_file "$f" 'deb.debian.org/debian' "$APT_MIRROR/debian"
        # Keep security.debian.org by design: security mirrors may lag.
        ;;
    esac
  done
  if ! apt-get update -o Acquire::Retries=2; then restore_sources; return 1; fi
}

configure_rpm_sources() {
  if [[ "$OS_ID" == centos && "$OS_VERSION" == 7* ]]; then
	if find /etc/yum.repos.d -maxdepth 1 -type f -iname '*tuxcare*' 2>/dev/null | grep -q .; then
	  EOL_WARNING='TuxCare ELS repository detected; CentOS base packages still use the frozen Vault.'
	else
	  EOL_WARNING='CentOS 7 Vault is frozen and receives no security updates.'
	fi
    for f in /etc/yum.repos.d/CentOS-*.repo; do
      [[ -f "$f" ]] || continue
      sed -i 's/^mirrorlist=/#mirrorlist=/' "$f"
	      sed -i 's|^#baseurl=http://mirror.centos.org/centos/\$releasever|baseurl=https://vault.centos.org/7.9.2009|' "$f"
    done
	  elif [[ "$REGION" == cn ]]; then
    # Only modify distribution-owned Rocky/Alma repo files; Docker and EPEL are preserved.
    for f in /etc/yum.repos.d/Rocky-*.repo /etc/yum.repos.d/almalinux*.repo; do
      [[ -f "$f" ]] || continue
	      if [[ "$OS_ID" == rocky ]]; then
	        sed -i -e 's/^mirrorlist=/#mirrorlist=/' -e "s|^#baseurl=http://dl.rockylinux.org/\$contentdir|baseurl=${RPM_MIRROR}/rocky|" -e "s|^#baseurl=https://dl.rockylinux.org/\$contentdir|baseurl=${RPM_MIRROR}/rocky|" "$f"
	      else
	        sed -i -e 's/^mirrorlist=/#mirrorlist=/' -e "s|^#baseurl=https://repo.almalinux.org/almalinux|baseurl=${RPM_MIRROR}/almalinux|" "$f"
	      fi
    done
  fi
  if command -v dnf >/dev/null; then dnf -q makecache; else yum -q makecache; fi || { restore_sources; return 1; }
}

configure_sources() {
  backup_sources
  if [[ "$PKG_FAMILY" == apt ]]; then configure_apt_sources; else configure_rpm_sources; fi
}
