#!/usr/bin/env bash
set -euo pipefail

INSTALLER=${1:?generated installer path is required}
[[ -f "$INSTALLER" ]] || { echo "Installer not found: $INSTALLER" >&2; exit 1; }

FUNCTIONS_FILE=$(mktemp)
trap 'rm -f "$FUNCTIONS_FILE"' EXIT
awk '/^download_release_asset\(\)/,/^}/' "$INSTALLER" > "$FUNCTIONS_FILE"
grep -q '^download_release_asset()' "$FUNCTIONS_FILE"

bash -uc '
  source "$1"
  curl() { :; }
  RELEASE_BASE=https://github.com/matthewlu070111/anpanel/releases
  TARGET_VERSION=build-selftest
  download_release_asset anpanel-linux-amd64 /tmp/anpanel-selftest
' _ "$FUNCTIONS_FILE"

if grep -q 'BASH_SOURCE' "$INSTALLER"; then
  echo 'Self-contained installer still depends on BASH_SOURCE.' >&2
  exit 1
fi
if grep -q '^VERSION=${ANPANEL_VERSION' "$INSTALLER"; then
  echo 'Release version collides with /etc/os-release VERSION.' >&2
  exit 1
fi

printf 'Generated installer self-test passed: %s\n' "$INSTALLER"
