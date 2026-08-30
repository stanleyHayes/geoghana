#!/usr/bin/env bash
set -euo pipefail
bundle="${1:-}"
version="${2:-}"
ledger="${3:-release-ledger.json}"
[[ -d "$bundle/npm" && -n "$version" ]] || { echo "usage: $0 <bundle> <version>" >&2; exit 2; }
verify_or_publish() {
  local name="$1" archive="$2" checkpoint="$3" evidence
  local sha1 sha512 remote_sha1 remote_integrity
  sha1="$(shasum -a 1 "$archive" | awk '{print $1}')"
  sha512="sha512-$(openssl dgst -sha512 -binary "$archive" | openssl base64 -A)"
  if npm view "$name@$version" version >/dev/null 2>&1; then
    remote_sha1="$(npm view "$name@$version" dist.shasum)"
    remote_integrity="$(npm view "$name@$version" dist.integrity)"
    [[ "$remote_sha1" == "$sha1" && "$remote_integrity" == "$sha512" ]] || { echo "$name@$version exists with bytes that do not match the attested tarball" >&2; exit 1; }
  else
    npm publish --access public --provenance "$archive"
    for attempt in {1..30}; do
      remote_sha1="$(npm view "$name@$version" dist.shasum 2>/dev/null || true)"
      remote_integrity="$(npm view "$name@$version" dist.integrity 2>/dev/null || true)"
      [[ "$remote_sha1" == "$sha1" && "$remote_integrity" == "$sha512" ]] && break
      [[ "$attempt" -lt 30 ]] || { echo "$name@$version did not become visible with expected integrity" >&2; exit 1; }
      sleep 10
    done
  fi
  evidence="$(mktemp)"
  node -e 'require("fs").writeFileSync(process.argv[1],JSON.stringify({registry:"npm",name:process.argv[2],version:process.argv[3],sha1:process.argv[4],integrity:process.argv[5]}))' "$evidence" "$name" "$version" "$sha1" "$sha512"
  node scripts/sdk-release-ledger.mjs complete "$ledger" _ "$checkpoint" "$evidence"
  rm -f "$evidence"
}
for package in core client react node data proto; do verify_or_publish "@ghanageo/$package" "$bundle/npm/ghanageo-$package-$version.tgz" "npm-$package"; done
verify_or_publish ghanageo "$bundle/npm/ghanageo-$version.tgz" npm-umbrella
