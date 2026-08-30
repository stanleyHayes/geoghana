#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
output="${2:-}"
semver='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$'
[[ "$version" =~ $semver && "$version" != *SNAPSHOT* ]] || { echo "usage: $0 <non-SNAPSHOT-semver> <output.tar.gz>" >&2; exit 2; }
[[ -n "$output" ]] || { echo "output path is required" >&2; exit 2; }

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
temp_root="$(cd "${TMPDIR:-/tmp}" && pwd -P)"
stage="$(mktemp -d "$temp_root/ghanageo-java-stage.XXXXXX")"
reactor="$stage/java"
cleanup() { rm -rf "$stage"; }
trap cleanup EXIT HUP INT TERM
mkdir -p "$reactor" "$(dirname "$output")"

tar -C "$repo_root/sdks/java" \
  --exclude='.build-tools' --exclude='.gradle' --exclude='build' --exclude='target' \
  --exclude='consumer-smoke' --exclude='java-conformance-report.json' \
  -cf - . | tar -C "$reactor" -xf -

ruby -e '
  version, root = ARGV
  files = Dir[File.join(root, "**", "pom.xml")] + [File.join(root, "client/src/main/java/dev/ghanageo/Version.java")]
  files.each do |path|
    source = File.read(path)
    replaced = source.gsub("2.0.0-SNAPSHOT", version)
    abort "expected version marker missing in #{path}" if source == replaced
    File.write(path, replaced)
  end
' "$version" "$reactor"

if rg -n '2\.0\.0-SNAPSHOT|<version>[^<]*SNAPSHOT|SDK\s*=\s*"[^"]*SNAPSHOT' "$reactor" -g 'pom.xml' -g 'Version.java'; then
  echo "SNAPSHOT metadata remains in staged Java reactor" >&2
  exit 1
fi
(cd "$reactor" && ./mvnw clean install)
(cd "$reactor" && tar -czf "$output" \
  mvnw pom.xml client/pom.xml spring-boot-starter/pom.xml examples/pom.xml conformance/pom.xml \
  client/src spring-boot-starter/src examples/src conformance/src \
  client/target/ghanageo-java-*.jar spring-boot-starter/target/ghanageo-spring-boot-starter-*.jar)
tar -xOf "$output" ./pom.xml | grep -q "<version>$version</version>"
tar -xOf "$output" ./client/src/main/java/dev/ghanageo/Version.java | grep -q "SDK = \"$version\""
if tar -xOf "$output" ./pom.xml | grep -q SNAPSHOT; then echo "staged Java bundle contains SNAPSHOT version" >&2; exit 1; fi
echo "staged Java SDK $version at $output"
