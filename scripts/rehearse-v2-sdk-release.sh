#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
semver='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$'
[[ "$version" =~ $semver ]] || { echo "usage: $0 <semver>" >&2; exit 2; }

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
temp_root="$(cd "${TMPDIR:-/tmp}" && pwd -P)"
stage="$(mktemp -d "$temp_root/ghanageo-v2-release.XXXXXX")"
final="$repo_root/release/v2/$version"
cleanup() { rm -rf "$stage"; }
trap cleanup EXIT HUP INT TERM
[[ ! -e "$final" ]] || { echo "immutable rehearsal already exists: $final" >&2; exit 1; }
mkdir -p "$stage"/{npm,python,go,dart,java,dotnet,php}

echo "[1/7] TypeScript/npm"
"$repo_root/scripts/verify-sdk-release.sh" "$version"
cp "$repo_root"/release/npm/*.tgz "$stage/npm/"

echo "[2/7] Python"
(cd "$repo_root/sdks/python" && ./scripts/verify_release.sh && uv build --out-dir "$stage/python")

echo "[3/7] Go"
(cd "$repo_root/sdks/go" && ./verify.sh)
tar -C "$repo_root" --exclude='.git' -czf "$stage/go/ghanageo-go-$version.tar.gz" sdks/go proto/ghanageo/v1

echo "[4/7] Dart and Flutter"
(cd "$repo_root/sdks/dart" && dart run tool/verify.dart)
(cd "$repo_root/sdks/dart/packages/ghanageo_flutter" && dart run tool/verify.dart)
tar -C "$repo_root/sdks/dart" --exclude='.dart_tool' --exclude='packages' --exclude='dart-conformance-report.json' -czf "$stage/dart/ghanageo-$version.tar.gz" .
tar -C "$repo_root/sdks/dart/packages/ghanageo_flutter" --exclude='.dart_tool' -czf "$stage/dart/ghanageo_flutter-$version.tar.gz" .

echo "[5/7] Java and Spring Boot"
(cd "$repo_root/sdks/java" && ./verify.sh)
java_source_hash="$(find "$repo_root/sdks/java" -type f \( -name pom.xml -o -name Version.java \) -print0 | sort -z | xargs -0 shasum -a 256)"
"$repo_root/scripts/stage-java-sdk-release.sh" "$version" "$stage/java/ghanageo-java-$version-maven-bundle.tar.gz"
[[ "$java_source_hash" == "$(find "$repo_root/sdks/java" -type f \( -name pom.xml -o -name Version.java \) -print0 | sort -z | xargs -0 shasum -a 256)" ]] || { echo "Java staging mutated source metadata" >&2; exit 1; }

echo "[6/7] .NET"
(cd "$repo_root/sdks/dotnet" && ./verify.sh)
cp "$repo_root"/sdks/dotnet/artifacts/pack-a/*.{nupkg,snupkg} "$stage/dotnet/"

echo "[7/7] PHP"
(cd "$repo_root/sdks/php" && CI='' composer archive --format=zip --dir="$stage/php" && CI='' ./verify.sh)

"$repo_root/tools/conformance/scan-secrets" "$stage"
node "$repo_root/scripts/generate-sdk-release-manifest.mjs" "$stage" "$version"
mkdir -p "$(dirname "$final")"
mv "$stage" "$final"
trap - EXIT HUP INT TERM
echo "V2 SDK release rehearsal complete: $final"
