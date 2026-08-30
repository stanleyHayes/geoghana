#!/usr/bin/env bash
set -euo pipefail
version="${1:-}"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([+-][0-9A-Za-z.-]+)?$ ]] || { echo "usage: $0 VERSION" >&2; exit 2; }
packages=(@ghanageo/core @ghanageo/client @ghanageo/react @ghanageo/node @ghanageo/data @ghanageo/proto)
for attempt in {1..30}; do
  missing=()
  for package_name in "${packages[@]}"; do
    published="$(npm view "$package_name@$version" version 2>/dev/null || true)"
    [[ "$published" == "$version" ]] || missing+=("$package_name")
  done
  ((${#missing[@]} == 0)) && { echo "all six scoped SDKs are published at $version"; exit 0; }
  ((attempt == 30)) && { echo "umbrella release blocked; missing $version: ${missing[*]}" >&2; exit 1; }
  sleep 10
done
