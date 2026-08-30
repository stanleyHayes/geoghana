#!/usr/bin/env bash
set -euo pipefail
version="${1:-}"
commit="${2:-}"
asset="${3:-}"
ledger="${4:-release-ledger.json}"
[[ -n "$version" && -n "$commit" && -f "$asset" ]] || { echo "usage: $0 <version> <commit> <asset> [ledger]" >&2; exit 2; }
tag="v$version"
name="$(basename "$asset")"
expected="$(shasum -a 256 "$asset" | awk '{print $1}')"
work="$(mktemp -d "${TMPDIR:-/tmp}/ghanageo-gh-release.XXXXXX")"
trap 'rm -rf "$work"' EXIT HUP INT TERM
if gh release view "$tag" >/dev/null 2>&1; then
  gh release download "$tag" --pattern "$name" --dir "$work"
  actual="$(shasum -a 256 "$work/$name" | awk '{print $1}')"
  [[ "$actual" == "$expected" ]] || { echo "GitHub release asset exists with a non-attested digest" >&2; exit 1; }
else
  gh release create "$tag" "$asset" --target "$commit" --verify-tag --generate-notes --title "GhanaGeo v$version"
  gh release download "$tag" --pattern "$name" --dir "$work"
  actual="$(shasum -a 256 "$work/$name" | awk '{print $1}')"
  [[ "$actual" == "$expected" ]] || { echo "uploaded GitHub release asset digest mismatch" >&2; exit 1; }
fi
evidence="$work/evidence.json"
node -e 'require("fs").writeFileSync(process.argv[1],JSON.stringify({registry:"github-releases",tag:process.argv[2],asset:process.argv[3],sha256:process.argv[4]}))' "$evidence" "$tag" "$name" "$expected"
node scripts/sdk-release-ledger.mjs complete "$ledger" _ github-release "$evidence"
