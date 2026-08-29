#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:?usage: homebrew-formula.sh VERSION CHECKSUMS_FILE}"
CHECKSUMS="${2:?usage: homebrew-formula.sh VERSION CHECKSUMS_FILE}"
REPOSITORY="${GHANAGEO_GITHUB_REPOSITORY:-ghanageo/ghanageo}"

checksum() {
  local artifact="$1"
  awk -v artifact="$artifact" '$2 == artifact { print $1 }' "$CHECKSUMS"
}

for artifact in \
  ghanageo_darwin_arm64 ghanageo_darwin_amd64 \
  ghanageo_linux_arm64 ghanageo_linux_amd64; do
  if [[ -z "$(checksum "$artifact")" ]]; then
    echo "missing checksum for $artifact" >&2
    exit 1
  fi
done

cat <<RUBY
class Ghanageo < Formula
  desc "Ghana's free public location data, from your terminal"
  homepage "https://geo.digitalghana.dev"
  version "${VERSION}"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/${REPOSITORY}/releases/download/v${VERSION}/ghanageo_darwin_arm64"
      sha256 "$(checksum ghanageo_darwin_arm64)"
    else
      url "https://github.com/${REPOSITORY}/releases/download/v${VERSION}/ghanageo_darwin_amd64"
      sha256 "$(checksum ghanageo_darwin_amd64)"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/${REPOSITORY}/releases/download/v${VERSION}/ghanageo_linux_arm64"
      sha256 "$(checksum ghanageo_linux_arm64)"
    else
      url "https://github.com/${REPOSITORY}/releases/download/v${VERSION}/ghanageo_linux_amd64"
      sha256 "$(checksum ghanageo_linux_amd64)"
    end
  end

  def install
    binary = Dir["ghanageo_*"].first
    bin.install binary => "ghanageo"
  end

  test do
    assert_match "targets GhanaGeo API v1", shell_output("#{bin}/ghanageo version")
  end
end
RUBY
