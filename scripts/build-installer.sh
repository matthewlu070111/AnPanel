#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
OUT=${1:-$ROOT/dist/install.sh}
INSTALLER_VERSION=${ANPANEL_INSTALLER_VERSION:-latest}
[[ "$INSTALLER_VERSION" =~ ^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$ ]] || { echo "Invalid installer version: $INSTALLER_VERSION" >&2; exit 2; }
mkdir -p "$(dirname "$OUT")"
awk -v lib="$ROOT/scripts/lib/sources.sh" -v version="$INSTALLER_VERSION" '
  /^VERSION=.*# anpanel:version$/ {
    printf "VERSION=${ANPANEL_VERSION:-%s}\n", version
    next
  }
  /^SCRIPT_DIR=.*BASH_SOURCE/ { next }
  /^# shellcheck source=lib\/sources.sh$/ {
    getline
    while ((getline line < lib) > 0) {
      if (line !~ /^#!/) print line
    }
    close(lib)
    next
  }
  { print }
' "$ROOT/scripts/install.sh" > "$OUT"
chmod 0755 "$OUT"
printf 'Built self-contained installer for %s: %s\n' "$INSTALLER_VERSION" "$OUT"
