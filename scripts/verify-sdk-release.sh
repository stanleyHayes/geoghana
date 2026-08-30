#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
semver='^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(\.(0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'
[[ "$version" =~ $semver ]] || { echo "usage: $0 <semver>" >&2; exit 2; }
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
temp_root="$(cd "${TMPDIR:-/tmp}" && pwd -P)"
stage_root="$(mktemp -d "$temp_root/ghanageo-sdk-release.XXXXXX")"
inspection_dir="$stage_root/inspection"
consumer_dir="$(mktemp -d "$temp_root/ghanageo-sdk-consumer.XXXXXX")"
release_stage="$stage_root/npm"
root_manifest_hash="$(shasum -a 256 "$repo_root/package.json" | awk '{print $1}')"
source_manifest_hashes="$(find "$repo_root/packages" -mindepth 2 -maxdepth 2 -name package.json -print0 | sort -z | xargs -0 shasum -a 256)"
source_lock="$temp_root/ghanageo-sdk-source.lock"
lock_owned=false
source "$repo_root/scripts/sdk-release-lock.sh"

safe_remove() {
  case "${1:-}" in
    "$temp_root"/ghanageo-sdk-release.*|"$temp_root"/ghanageo-sdk-consumer.*) rm -rf "$1" ;;
    *) echo "refusing unsafe cleanup path: ${1:-}" >&2; return 1 ;;
  esac
}
release_lock() {
  if [[ "$lock_owned" == true ]]; then release_sdk_source_lock "$source_lock" "$$"; lock_owned=false; fi
}
cleanup() { release_lock; safe_remove "$stage_root"; safe_remove "$consumer_dir"; }
trap cleanup EXIT
[[ "$stage_root" != "$repo_root" && "$consumer_dir" != "$repo_root" ]] || { echo "unsafe temporary directory" >&2; exit 1; }
mkdir -p "$release_stage" "$inspection_dir"
node "$repo_root/scripts/check-sdk-release-ownership.mjs"
bash "$repo_root/scripts/sdk-release-lock.test.sh"
node "$repo_root/scripts/check-sdk-clean-order.mjs"

acquire_sdk_source_lock "$source_lock" "$$"
lock_owned=true

packages=(core client react node data proto)
for package_name in "${packages[@]}"; do
  # JavaScript owns its template literals.
  # shellcheck disable=SC2016
  node -e 'const value = require(process.argv[1]).version; if (!/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-(?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*))*)?(\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/.test(value)) throw new Error(`invalid source package version: ${value}`)' "$repo_root/packages/$package_name/package.json"
  # Dependency order matters on a clean checkout: React's example typecheck
  # resolves the freshly-built core/client declaration exports.
  pnpm --dir "$repo_root/packages/$package_name" build
  pnpm --dir "$repo_root/packages/$package_name" test
  base_dir="$stage_root/base-$package_name"
  repack_dir="$stage_root/repack-$package_name"
  mkdir -p "$base_dir" "$repack_dir"
  pnpm --dir "$repo_root/packages/$package_name" pack --pack-destination "$base_dir"
  tar -xzf "$base_dir"/*.tgz -C "$repack_dir"
  node "$repo_root/scripts/stamp-staged-sdk-package.mjs" "$repack_dir/package" "$version"
  (cd "$repack_dir/package" && npm pack --ignore-scripts --pack-destination "$release_stage" >/dev/null)
done

# Use the production cross-compiler in an isolated CLI copy, then inject its
# output into an equally isolated copy of the pnpm-produced umbrella package.
cli_build="$stage_root/cli"
cp -R "$repo_root/cli" "$cli_build"
(cd "$cli_build" && ./build.sh "$version" v1)
pnpm --dir "$repo_root/packages/cli" build
pnpm --dir "$repo_root/packages/cli" test
cli_version="$(node -p "require('$repo_root/packages/cli/package.json').version")"
pnpm --dir "$repo_root/packages/cli" pack --pack-destination "$stage_root"
mkdir -p "$stage_root/umbrella"
tar -xzf "$stage_root/ghanageo-$cli_version.tgz" -C "$stage_root/umbrella"
cp "$cli_build"/dist/ghanageo_* "$stage_root/umbrella/package/bin/"
rm "$stage_root/umbrella/package/bin/resolve.test.js"
node "$repo_root/scripts/stamp-staged-sdk-package.mjs" "$stage_root/umbrella/package" "$version"
(cd "$stage_root/umbrella/package" && npm pack --ignore-scripts --pack-destination "$release_stage" >/dev/null)
release_lock

expected=("ghanageo-core-$version.tgz" "ghanageo-client-$version.tgz" "ghanageo-react-$version.tgz" "ghanageo-node-$version.tgz" "ghanageo-data-$version.tgz" "ghanageo-proto-$version.tgz" "ghanageo-$version.tgz")
for archive_name in "${expected[@]}"; do
  archive="$release_stage/$archive_name"
  [[ -f "$archive" ]] || { echo "missing $archive_name" >&2; exit 1; }
  # JavaScript owns its template literals.
  # shellcheck disable=SC2016
  tar -xOf "$archive" package/package.json | node -e '
    let input = "";
    process.stdin.on("data", chunk => input += chunk);
    process.stdin.on("end", () => {
      const manifest = JSON.parse(input);
      for (const value of Object.values(manifest.dependencies ?? {})) {
        if (String(value).startsWith("workspace:")) throw new Error(`${manifest.name} contains a workspace dependency`);
      }
      if (!manifest.publishConfig?.provenance || manifest.publishConfig?.access !== "public") throw new Error(`${manifest.name} is missing public provenance configuration`);
      console.log(`verified ${manifest.name}@${manifest.version}`);
    });'
  package_dir="$inspection_dir/${archive_name%.tgz}"
  mkdir -p "$package_dir"
  tar -xzf "$archive" -C "$package_dir"
done

ruby "$repo_root/tools/conformance/scan-secrets" "$inspection_dir"
tar -xOf "$release_stage/ghanageo-core-$version.tgz" package/dist/index.js | grep -q '2026.08.3-ulid'
tar -xOf "$release_stage/ghanageo-data-$version.tgz" package/dist/index.js | grep -q '2026.08.3-ulid'
tar -tzf "$release_stage/ghanageo-proto-$version.tgz" | grep -q 'package/proto/ghanageo/v1/geography.proto'

native=(ghanageo_darwin_amd64 ghanageo_darwin_arm64 ghanageo_linux_amd64 ghanageo_linux_arm64 ghanageo_linux_arm ghanageo_windows_amd64.exe ghanageo_windows_arm64.exe)
for binary in "${native[@]}"; do
  tar -tzf "$release_stage/ghanageo-$version.tgz" | grep -qx "package/bin/$binary" || { echo "umbrella is missing $binary" >&2; exit 1; }
done
if tar -tzf "$release_stage/ghanageo-$version.tgz" | grep -q 'package/bin/resolve.test.js'; then echo "umbrella contains its private launcher test" >&2; exit 1; fi

node "$repo_root/scripts/prepare-sdk-consumer.mjs" "$consumer_dir" "$release_stage" "$version"
(cd "$consumer_dir" && npm install --package-lock-only --ignore-scripts --no-audit --no-fund)
if grep -RIE 'workspace:|link:' "$consumer_dir/package.json" "$consumer_dir/package-lock.json"; then echo "fresh consumer lock contains a workspace-only dependency" >&2; exit 1; fi
(cd "$consumer_dir" && npm ci --ignore-scripts --no-audit --no-fund)
(cd "$consumer_dir" && npm run runtime)
(cd "$consumer_dir" && npm run examples)
[[ "$(shasum -a 256 "$repo_root/package.json" | awk '{print $1}')" == "$root_manifest_hash" ]] || { echo "release verification mutated the root workspace manifest" >&2; exit 1; }
[[ "$(find "$repo_root/packages" -mindepth 2 -maxdepth 2 -name package.json -print0 | sort -z | xargs -0 shasum -a 256)" == "$source_manifest_hashes" ]] || { echo "release verification mutated a source package manifest" >&2; exit 1; }

# Each final file is replaced atomically; concurrent successful runs never read
# or validate one another's staging trees.
final_dir="$repo_root/release/npm"
mkdir -p "$final_dir"
for archive_name in "${expected[@]}"; do
  temporary="$final_dir/.${archive_name}.$$"
  cp "$release_stage/$archive_name" "$temporary"
  mv -f "$temporary" "$final_dir/$archive_name"
done
echo "verified ${#expected[@]} publishable SDK artifacts for $version"
