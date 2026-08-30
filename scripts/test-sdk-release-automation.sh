#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fixture="$(mktemp -d "${TMPDIR:-/tmp}/ghanageo-release-test.XXXXXX")"
trap 'rm -rf "$fixture"' EXIT HUP INT TERM
for spec in 'npm 7' 'python 2' 'go 1' 'dart 2' 'java 1' 'dotnet 2' 'php 1'; do
  read -r family count <<< "$spec"
  mkdir -p "$fixture/$family"
  for ((index=1; index<=count; index++)); do printf '%s\n' "$family-$index" > "$fixture/$family/artifact-$index"; done
done
GHANAGEO_SYNTHETIC_RELEASE=true node "$repo_root/scripts/generate-sdk-release-manifest.mjs" "$fixture" 2.0.0-rc.1
commit="$(git -C "$repo_root" rev-parse HEAD)"
node "$repo_root/scripts/verify-sdk-release-bundle.mjs" "$fixture" 2.0.0-rc.1 "$commit"
node "$repo_root/scripts/check-sdk-release-ownership.mjs"
node "$repo_root/scripts/sdk-release-ledger.mjs" init "$fixture/release-ledger.json" "$fixture/compatibility-manifest.json"
printf '{"sha256":"first"}' > "$fixture/evidence.json"
node "$repo_root/scripts/sdk-release-ledger.mjs" complete "$fixture/release-ledger.json" _ npm-core "$fixture/evidence.json"
node "$repo_root/scripts/sdk-release-ledger.mjs" has "$fixture/release-ledger.json" npm-core
printf '{"sha256":"different"}' > "$fixture/evidence.json"
if node "$repo_root/scripts/sdk-release-ledger.mjs" complete "$fixture/release-ledger.json" _ npm-core "$fixture/evidence.json" >/dev/null 2>&1; then echo "ledger accepted mismatched checkpoint evidence" >&2; exit 1; fi
printf '{"tree":"go-source"}' > "$fixture/evidence.json"; node "$repo_root/scripts/sdk-release-ledger.mjs" complete "$fixture/release-ledger.json" _ go-source "$fixture/evidence.json"
# Simulate failure after the Go tag: reload the same bound ledger, preserve Go, and resume PHP/dispatch independently.
node "$repo_root/scripts/sdk-release-ledger.mjs" init "$fixture/release-ledger.json" "$fixture/compatibility-manifest.json"
node "$repo_root/scripts/sdk-release-ledger.mjs" has "$fixture/release-ledger.json" go-source
if node "$repo_root/scripts/sdk-release-ledger.mjs" has "$fixture/release-ledger.json" php-source; then echo "source resume skipped an incomplete PHP checkpoint" >&2; exit 1; fi
for checkpoint in php-source go-dispatch; do printf '{"tree":"%s"}' "$checkpoint" > "$fixture/evidence.json"; node "$repo_root/scripts/sdk-release-ledger.mjs" complete "$fixture/release-ledger.json" _ "$checkpoint" "$fixture/evidence.json"; done
node "$repo_root/scripts/sdk-release-ledger.mjs" has "$fixture/release-ledger.json" go-source
node "$repo_root/scripts/sdk-release-ledger.mjs" has "$fixture/release-ledger.json" php-source
node "$repo_root/scripts/sdk-release-ledger.mjs" has "$fixture/release-ledger.json" go-dispatch

# Inject a mid-npm failure, then prove the rerun verifies completed packages and publishes only the remainder.
mkdir -p "$fixture/mock-bin" "$fixture/npm-state" "$fixture/npm-bundle/npm"
for package in core client react node data proto; do printf '%s' "$package" > "$fixture/npm-bundle/npm/ghanageo-$package-2.0.0-rc.1.tgz"; done
printf umbrella > "$fixture/npm-bundle/npm/ghanageo-2.0.0-rc.1.tgz"
cat > "$fixture/mock-bin/npm" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
cmd="$1"; shift
if [[ "$cmd" == publish ]]; then archive="${@: -1}"; base="$(basename "$archive")"; if [[ "${MOCK_FAIL_ONCE:-}" == "$base" && ! -e "$MOCK_NPM_STATE/failed" ]]; then touch "$MOCK_NPM_STATE/failed"; exit 73; fi; touch "$MOCK_NPM_STATE/$base"; exit 0; fi
[[ "$cmd" == view ]]; spec="$1"; field="${2:-version}"; name="${spec%@*}"; version="${spec##*@}"; [[ "$name" == "$spec" ]] && { name="${spec%%@*}"; version="${spec#*@}"; }
if [[ "$name" == @ghanageo/* ]]; then base="ghanageo-${name#@ghanageo/}-$version.tgz"; else base="ghanageo-$version.tgz"; fi
[[ -e "$MOCK_NPM_STATE/$base" ]] || exit 1
case "$field" in version) echo "$version";; dist.shasum) shasum -a 1 "$MOCK_NPM_BUNDLE/npm/$base"|awk '{print $1}';; dist.integrity) printf 'sha512-'; openssl dgst -sha512 -binary "$MOCK_NPM_BUNDLE/npm/$base"|openssl base64 -A; echo;; esac
MOCK
chmod +x "$fixture/mock-bin/npm"
node "$repo_root/scripts/sdk-release-ledger.mjs" init "$fixture/npm-ledger.json" "$fixture/compatibility-manifest.json"
if PATH="$fixture/mock-bin:$PATH" MOCK_NPM_STATE="$fixture/npm-state" MOCK_NPM_BUNDLE="$fixture/npm-bundle" MOCK_FAIL_ONCE=ghanageo-react-2.0.0-rc.1.tgz "$repo_root/scripts/publish-npm-sdk-bundle.sh" "$fixture/npm-bundle" 2.0.0-rc.1 "$fixture/npm-ledger.json" >/dev/null 2>&1; then echo "npm failure injection did not fail" >&2; exit 1; fi
PATH="$fixture/mock-bin:$PATH" MOCK_NPM_STATE="$fixture/npm-state" MOCK_NPM_BUNDLE="$fixture/npm-bundle" "$repo_root/scripts/publish-npm-sdk-bundle.sh" "$fixture/npm-bundle" 2.0.0-rc.1 "$fixture/npm-ledger.json"
for checkpoint in npm-core npm-client npm-react npm-node npm-data npm-proto npm-umbrella; do node "$repo_root/scripts/sdk-release-ledger.mjs" has "$fixture/npm-ledger.json" "$checkpoint"; done
# Inject GitHub release creation failure, then resume and verify the exact asset checkpoint.
cat > "$fixture/mock-bin/gh" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
[[ "$1" == release ]]; action="$2"; shift 2
case "$action" in
  view) [[ -f "$MOCK_GH_STATE/asset" ]];;
  create) if [[ ! -e "$MOCK_GH_STATE/failed" ]]; then touch "$MOCK_GH_STATE/failed"; exit 74; fi; cp "$2" "$MOCK_GH_STATE/asset";;
  download) while (($#)); do [[ "$1" == --dir ]] && { shift; dir="$1"; }; shift || true; done; cp "$MOCK_GH_STATE/asset" "$dir/$MOCK_GH_ASSET";;
esac
MOCK
chmod +x "$fixture/mock-bin/gh"
mkdir -p "$fixture/gh-state"
gh_asset="$fixture/npm-bundle/npm/ghanageo-2.0.0-rc.1.tgz"
if PATH="$fixture/mock-bin:$PATH" MOCK_GH_STATE="$fixture/gh-state" MOCK_GH_ASSET="$(basename "$gh_asset")" "$repo_root/scripts/publish-github-sdk-release.sh" 2.0.0-rc.1 "$commit" "$gh_asset" "$fixture/npm-ledger.json" >/dev/null 2>&1; then echo "GitHub release failure injection did not fail" >&2; exit 1; fi
PATH="$fixture/mock-bin:$PATH" MOCK_GH_STATE="$fixture/gh-state" MOCK_GH_ASSET="$(basename "$gh_asset")" "$repo_root/scripts/publish-github-sdk-release.sh" 2.0.0-rc.1 "$commit" "$gh_asset" "$fixture/npm-ledger.json"
node "$repo_root/scripts/sdk-release-ledger.mjs" has "$fixture/npm-ledger.json" github-release
PATH="$fixture/mock-bin:$PATH" MOCK_GH_STATE="$fixture/gh-state" MOCK_GH_ASSET="$(basename "$gh_asset")" "$repo_root/scripts/publish-github-sdk-release.sh" 2.0.0-rc.1 "$commit" "$gh_asset" "$fixture/npm-ledger.json"
printf tampered > "$fixture/gh-state/asset"
if PATH="$fixture/mock-bin:$PATH" MOCK_GH_STATE="$fixture/gh-state" MOCK_GH_ASSET="$(basename "$gh_asset")" "$repo_root/scripts/publish-github-sdk-release.sh" 2.0.0-rc.1 "$commit" "$gh_asset" "$fixture/npm-ledger.json" >/dev/null 2>&1; then echo "GitHub release resume accepted a mismatched asset" >&2; exit 1; fi
node -e 'const fs=require("node:fs"); const p=process.argv[1]; const m=JSON.parse(fs.readFileSync(p)); m.release.commit="mismatch"; fs.writeFileSync(p,JSON.stringify(m));' "$fixture/compatibility-manifest.json"
if node "$repo_root/scripts/sdk-release-ledger.mjs" init "$fixture/release-ledger.json" "$fixture/compatibility-manifest.json" >/dev/null 2>&1; then
  echo "release ledger accepted a mismatched manifest" >&2
  exit 1
fi
if GHANAGEO_SYNTHETIC_RELEASE=true node "$repo_root/scripts/generate-sdk-release-manifest.mjs" "$fixture" 2.0.0 >/dev/null 2>&1; then
  echo "stable release unexpectedly accepted prerelease SDK metadata" >&2
  exit 1
fi
printf 'tampered\n' >> "$fixture/npm/artifact-1"
if node "$repo_root/scripts/verify-sdk-release-bundle.mjs" "$fixture" 2.0.0-rc.1 "$commit" >/dev/null 2>&1; then
  echo "tampered bundle unexpectedly passed" >&2
  exit 1
fi
grep -Fq "if: github.event_name == 'push' || inputs.publish" "$repo_root/.github/workflows/v2-sdk-release.yml"
grep -Fq './scripts/publish-npm-sdk-bundle.sh bundle' "$repo_root/.github/workflows/v2-sdk-publish.yml"
grep -Fq 'publish-github-sdk-release.sh' "$repo_root/.github/workflows/v2-sdk-publish.yml"
grep -Fq 'GitHub release asset exists with a non-attested digest' "$repo_root/scripts/publish-github-sdk-release.sh"
core_line="$(rg -n 'Publish Dart core through' "$repo_root/.github/workflows/v2-sdk-publish.yml" | cut -d: -f1)"
wait_line="$(rg -n 'Wait for Dart core registry visibility' "$repo_root/.github/workflows/v2-sdk-publish.yml" | cut -d: -f1)"
flutter_line="$(rg -n 'Publish Flutter companion after Dart core' "$repo_root/.github/workflows/v2-sdk-publish.yml" | cut -d: -f1)"
[[ "$core_line" -lt "$wait_line" && "$wait_line" -lt "$flutter_line" ]] || { echo "Dart/Flutter publish order regressed" >&2; exit 1; }
if grep -Eq '^\s+tags:' "$repo_root/.github/workflows/cli-release.yml" "$repo_root/.github/workflows/sdk-release.yml"; then
  echo "legacy npm workflow still owns a tag trigger" >&2
  exit 1
fi
if rg -n 'uses: .*@(v[0-9]|main|master|release/)' "$repo_root/.github/workflows"; then
  echo "a GitHub Action is not pinned to an immutable commit" >&2
  exit 1
fi
echo "release automation synthetic tests passed"
