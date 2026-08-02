#!/usr/bin/env bash
set -euo pipefail
ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
OUT=${1:-$ROOT/dist/install.sh}
mkdir -p "$(dirname "$OUT")"
awk -v lib="$ROOT/scripts/lib/sources.sh" '
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
printf 'Built self-contained installer: %s\n' "$OUT"
