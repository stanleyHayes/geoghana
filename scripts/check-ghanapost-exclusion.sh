#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if (( $# > 0 )); then
  targets=("$@")
else
  targets=("$ROOT/data" "$ROOT/services/api/migrations")
  while IFS= read -r directory; do
    targets+=("$directory")
  done < <(
    find "$ROOT" -type d \( -name fixtures -o -name testdata \) \
      -not -path '*/.git/*' -not -path '*/node_modules/*' -print
  )
fi

existing=()
for target in "${targets[@]}"; do
  [[ -e "$target" ]] && existing+=("$target")
done

if (( ${#existing[@]} == 0 )); then
  echo "GhanaPost exclusion check has no paths to scan" >&2
  exit 2
fi

# GhanaPostGPS digital addresses use a two-letter district prefix, a three or
# four-digit area code and a four-digit unique address (for example, AA-123-4567).
# The provider name is also forbidden inside distributable data payloads.
patterns=(
  '(^|[^[:alnum:]_])[A-Z]{2}-[0-9]{3,4}-[0-9]{4}([^[:alnum:]_]|$)'
  'Ghana[[:space:]_-]*Post[[:space:]_-]*GPS'
)

for pattern in "${patterns[@]}"; do
  if rg --line-number --ignore-case --no-messages \
    --glob '*.{csv,json,geojson,jsonl,ndjson,sql,yaml,yml,xml,txt}' \
    --regexp "$pattern" "${existing[@]}"; then
    echo "Unlicensed GhanaPostGPS data detected. Remove the payload; the integration must remain adapter-only until licensed." >&2
    exit 1
  fi
done

echo "GhanaPostGPS exclusion check passed (${#existing[@]} paths scanned)"
