#!/usr/bin/env bash
set -euo pipefail

sdk_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$sdk_dir/../.." && pwd)"
artifacts_dir="$sdk_dir/artifacts"
pack_a="$artifacts_dir/pack-a"
pack_b="$artifacts_dir/pack-b"
raw_a="$artifacts_dir/raw-a"
raw_b="$artifacts_dir/raw-b"

export DOTNET_CLI_TELEMETRY_OPTOUT=1
export DOTNET_NOLOGO=1
export SOURCE_DATE_EPOCH=0

cd "$sdk_dir"
dotnet format GhanaGeo.sln --verify-no-changes --no-restore
dotnet restore GhanaGeo.sln --locked-mode
dotnet build GhanaGeo.sln -c Release --no-restore
dotnet run --project tests/GhanaGeo.Tests/GhanaGeo.Tests.csproj -c Release --no-build
dotnet build conformance/GhanaGeo.Conformance.csproj -c Release
dotnet build examples/ConsoleExample/ConsoleExample.csproj -c Release

rm -rf "$artifacts_dir"
mkdir -p "$pack_a" "$pack_b" "$raw_a" "$raw_b"
dotnet build tools/NormalizePackage/NormalizePackage.csproj -c Release
dotnet pack src/GhanaGeo/GhanaGeo.csproj -c Release --no-build -o "$raw_a"
dotnet pack src/GhanaGeo/GhanaGeo.csproj -c Release --no-build -o "$raw_b"
for package in GhanaGeo.1.0.0.nupkg GhanaGeo.1.0.0.snupkg; do
  dotnet run --project tools/NormalizePackage/NormalizePackage.csproj -c Release --no-build -- "$raw_a/$package" "$pack_a/$package"
  dotnet run --project tools/NormalizePackage/NormalizePackage.csproj -c Release --no-build -- "$raw_b/$package" "$pack_b/$package"
  test -f "$pack_a/$package"
  test -f "$pack_b/$package"
  cmp "$pack_a/$package" "$pack_b/$package"
done

dotnet restore consumer-smoke/Consumer.csproj --source "$pack_a" --source "https://api.nuget.org/v3/index.json"
dotnet build consumer-smoke/Consumer.csproj -c Release --no-restore

ruby -rjson -rdigest -e '
  lock = JSON.parse(File.read(ARGV.shift)).fetch("contracts")
  root = ARGV.shift
  bad = lock.map { |path, expected| actual = Digest::SHA256.file(File.join(root, path)).hexdigest; "#{path}: #{actual}" unless actual == expected }.compact
  abort("contract lock mismatch\n#{bad.join("\n")}") unless bad.empty?
' contract.lock.json "$repo_root"

unpack_dir="$artifacts_dir/unpacked"
mkdir -p "$unpack_dir/nupkg" "$unpack_dir/snupkg"
unzip -qq "$pack_a/GhanaGeo.1.0.0.nupkg" -d "$unpack_dir/nupkg"
unzip -qq "$pack_a/GhanaGeo.1.0.0.snupkg" -d "$unpack_dir/snupkg"
"$repo_root/tools/conformance/scan-secrets" "$sdk_dir/src" "$sdk_dir/examples" "$sdk_dir/consumer-smoke" "$unpack_dir/nupkg" "$unpack_dir/snupkg"

if rg -a -n -F "$repo_root" "$unpack_dir"; then
  echo "package contains an absolute development path" >&2
  exit 1
fi
if rg -a -n '(/Users/|/home/|Desktop/Dev|sdks/dotnet)' "$unpack_dir"; then
  echo "package contains development-only leakage" >&2
  exit 1
fi

unzip -l "$pack_a/GhanaGeo.1.0.0.nupkg" | rg 'lib/net8.0/GhanaGeo.dll|README.md|LICENSE|\.nuspec' >/dev/null
unzip -l "$pack_a/GhanaGeo.1.0.0.snupkg" | rg 'lib/net8.0/GhanaGeo.pdb|\.nuspec' >/dev/null
dependency_audit="$(dotnet list src/GhanaGeo/GhanaGeo.csproj package --vulnerable --include-transitive)"
printf '%s\n' "$dependency_audit"
if printf '%s\n' "$dependency_audit" | rg -q 'has the following vulnerable packages'; then
  echo "vulnerable package dependency found" >&2
  exit 1
fi
echo "Deterministic package SHA-256: $(shasum -a 256 "$pack_a/GhanaGeo.1.0.0.nupkg" | awk '{print $1}')"
echo "Deterministic symbols SHA-256: $(shasum -a 256 "$pack_a/GhanaGeo.1.0.0.snupkg" | awk '{print $1}')"
echo "GhanaGeo .NET verification passed."
