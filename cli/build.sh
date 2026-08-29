#!/usr/bin/env bash
# Cross-compile release binaries. Go's static linking means a user needs no
# runtime installed — which matters for a tool aimed at a broad public.
set -euo pipefail
VERSION="${1:-dev}"
OUT="dist"
rm -rf "$OUT" && mkdir -p "$OUT"

PLATFORMS=(
  "darwin/amd64" "darwin/arm64"
  "linux/amd64"  "linux/arm64" "linux/arm"
  "windows/amd64" "windows/arm64"
)

for p in "${PLATFORMS[@]}"; do
  GOOS="${p%/*}"; GOARCH="${p#*/}"
  name="ghanageo_${GOOS}_${GOARCH}"
  [ "$GOOS" = "windows" ] && name="${name}.exe"
  CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o "$OUT/$name" .
  echo "  built $name"
done

( cd "$OUT" && shasum -a 256 * > checksums.txt )
echo "✓ $(ls "$OUT" | grep -c ghanageo) binaries and checksums.txt in $OUT/"
