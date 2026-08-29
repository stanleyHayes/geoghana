#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([+-][0-9A-Za-z.-]+)?$ ]] || {
  echo "usage: $0 <semver>" >&2
  exit 2
}

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
release_dir="$repo_root/release/npm"
rm -rf "$release_dir"
mkdir -p "$release_dir"

packages=(core client react node data proto)
for package_name in "${packages[@]}"; do
  actual_version="$(node -p "require('$repo_root/packages/$package_name/package.json').version")"
  [[ "$actual_version" == "$version" ]] || {
    echo "packages/$package_name is $actual_version; expected $version" >&2
    exit 1
  }
  pnpm --dir "$repo_root/packages/$package_name" pack --pack-destination "$release_dir"
done

expected=(
  "ghanageo-core-$version.tgz"
  "ghanageo-client-$version.tgz"
  "ghanageo-react-$version.tgz"
  "ghanageo-node-$version.tgz"
  "ghanageo-data-$version.tgz"
  "ghanageo-proto-$version.tgz"
)

for archive_name in "${expected[@]}"; do
  archive="$release_dir/$archive_name"
  [[ -f "$archive" ]] || { echo "missing $archive_name" >&2; exit 1; }
  tar -xOf "$archive" package/package.json | node -e '
    let input = "";
    process.stdin.on("data", chunk => input += chunk);
    process.stdin.on("end", () => {
      const manifest = JSON.parse(input);
      for (const value of Object.values(manifest.dependencies ?? {})) {
        if (String(value).startsWith("workspace:")) {
          console.error(`${manifest.name} contains an unpublished workspace dependency`);
          process.exit(1);
        }
      }
      if (!manifest.publishConfig?.provenance || manifest.publishConfig?.access !== "public") {
        console.error(`${manifest.name} is missing public provenance configuration`);
        process.exit(1);
      }
      console.log(`verified ${manifest.name}@${manifest.version}`);
    });
  '
done

if grep -aE 'gh_(live|test)_[A-Za-z0-9]+' "$release_dir"/*.tgz; then
  echo "an SDK tarball contains an API-key-shaped value" >&2
  exit 1
fi

tar -xOf "$release_dir/ghanageo-core-$version.tgz" package/dist/index.js | grep -q '2026.08.3-ulid'
tar -xOf "$release_dir/ghanageo-data-$version.tgz" package/dist/index.js | grep -q '2026.08.3-ulid'
tar -tzf "$release_dir/ghanageo-proto-$version.tgz" | grep -q 'package/proto/ghanageo/v1/geography.proto'

echo "verified ${#expected[@]} publishable SDK artifacts for $version"
