#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="${1:-0.0.0-test}"
API_VERSION="${2:-v1}"
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT

(
  cd "$ROOT/cli"
  ./build.sh "$VERSION" "$API_VERSION"
)

mkdir -p "$STAGE/package"
cp -R "$ROOT/packages/cli/." "$STAGE/package/"
cp "$ROOT"/cli/dist/ghanageo_* "$STAGE/package/bin/"

node - "$STAGE/package/package.json" "$VERSION" <<'NODE'
const fs = require("node:fs");
const [file, version] = process.argv.slice(2);
const pkg = JSON.parse(fs.readFileSync(file, "utf8"));
pkg.version = version;
fs.writeFileSync(file, JSON.stringify(pkg, null, 2) + "\n");
NODE

(
  cd "$STAGE/package"
  npm pack --ignore-scripts --pack-destination "$STAGE" >/dev/null
)

TARBALL="$(find "$STAGE" -maxdepth 1 -name 'ghanageo-*.tgz' -print -quit)"
[[ -n "$TARBALL" ]] || { echo "npm tarball was not created" >&2; exit 1; }

for artifact in \
  ghanageo_darwin_amd64 ghanageo_darwin_arm64 \
  ghanageo_linux_amd64 ghanageo_linux_arm64 ghanageo_linux_arm \
  ghanageo_windows_amd64.exe ghanageo_windows_arm64.exe; do
  tar -tzf "$TARBALL" | grep -qx "package/bin/$artifact" || {
    echo "$artifact is missing from the npm package" >&2
    exit 1
  }
done

"$ROOT/cli/release/homebrew-formula.sh" "$VERSION" \
  "$ROOT/cli/dist/checksums.txt" > "$STAGE/ghanageo.rb"
grep -q "version \"$VERSION\"" "$STAGE/ghanageo.rb"
grep -q "targets GhanaGeo API $API_VERSION" "$STAGE/ghanageo.rb"

"$ROOT/cli/dist/ghanageo_$(go env GOOS)_$(go env GOARCH)" version | \
  grep -q "ghanageo $VERSION (targets GhanaGeo API $API_VERSION)"

echo "✓ CLI release bundle verified: 7 binaries, checksums, npm tarball and Homebrew formula"
