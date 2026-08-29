#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT

printf '%s\n' '{"id":"gh-place-osu","name":"Osu"}' > "$STAGE/clean.json"
"$ROOT/scripts/check-ghanapost-exclusion.sh" "$STAGE" >/dev/null

printf '%s\n' 'name,digital_address' 'Example,GA-123-4567' > "$STAGE/contaminated.csv"
if "$ROOT/scripts/check-ghanapost-exclusion.sh" "$STAGE" >/dev/null 2>&1; then
  echo "expected a digital-address payload to fail the exclusion check" >&2
  exit 1
fi

rm "$STAGE/contaminated.csv"
printf '%s\n' '{"source":"GhanaPostGPS"}' > "$STAGE/provider.json"
if "$ROOT/scripts/check-ghanapost-exclusion.sh" "$STAGE" >/dev/null 2>&1; then
  echo "expected a GhanaPostGPS source payload to fail the exclusion check" >&2
  exit 1
fi

echo "GhanaPostGPS exclusion regression tests passed"
