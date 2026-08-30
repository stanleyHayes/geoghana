#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fixture="$(mktemp -d "${TMPDIR:-/tmp}/ghanageo-java-release-test.XXXXXX")"
cleanup() { rm -rf "$fixture"; }
trap cleanup EXIT HUP INT TERM
source_hashes="$(find "$repo_root/sdks/java" -type f \( -name pom.xml -o -name Version.java \) -print0 | sort -z | xargs -0 shasum -a 256)"
for version in 2.0.0-rc.1 2.0.0; do
  archive="$fixture/ghanageo-java-$version-maven-bundle.tar.gz"
  "$repo_root/scripts/stage-java-sdk-release.sh" "$version" "$archive"
  test "$(tar -xOf "$archive" ./pom.xml | sed -n 's#.*<artifactId>ghanageo-java-parent</artifactId><version>\([^<]*\)</version>.*#\1#p')" = "$version"
  tar -xOf "$archive" ./client/src/main/java/dev/ghanageo/Version.java | grep -q "SDK = \"$version\""
  if tar -tzf "$archive" | grep -qi SNAPSHOT; then echo "$version bundle contains a SNAPSHOT path" >&2; exit 1; fi
done
[[ "$source_hashes" == "$(find "$repo_root/sdks/java" -type f \( -name pom.xml -o -name Version.java \) -print0 | sort -z | xargs -0 shasum -a 256)" ]] || { echo "Java release regression mutated source" >&2; exit 1; }
echo "Java prerelease and stable isolated staging regressions passed"
