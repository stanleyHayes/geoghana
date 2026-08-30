#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
# repo_root is resolved and validated above.
# shellcheck disable=SC1091
source "$repo_root/scripts/sdk-release-lock.sh"

language="${1:-}"
case "$language" in
  dotnet|php) ;;
  *) echo "usage: $0 dotnet|php" >&2; exit 64 ;;
esac

if [[ -n "${REPORT:-}" ]]; then
  report="$REPORT"
else
  report="$(mktemp "${TMPDIR:-/tmp}/ghanageo-${language}-conformance.XXXXXX.json")"
fi
mkdir -p "$(dirname "$report")"
report="$(cd "$(dirname "$report")" && pwd -P)/$(basename "$report")"
run_dir="$(mktemp -d "${TMPDIR:-/tmp}/ghanageo-${language}-wave3.XXXXXX")"
lock_dir="${TMPDIR:-/tmp}/ghanageo-${language}-wave3-source.lock"
fixture_pid=""
lock_held=0

cleanup() {
  if [[ -n "$fixture_pid" ]] && kill -0 "$fixture_pid" 2>/dev/null; then
    kill "$fixture_pid" 2>/dev/null || true
    wait "$fixture_pid" 2>/dev/null || true
  fi
  if [[ "$lock_held" == 1 ]]; then release_sdk_source_lock "$lock_dir" "$$"; fi
  case "$run_dir" in
    "${TMPDIR:-/tmp}"/ghanageo-"$language"-wave3.*) rm -rf "$run_dir" ;;
    *) return 1 ;;
  esac
}
trap cleanup EXIT HUP INT TERM

acquire_sdk_source_lock "$lock_dir" "$$"
lock_held=1

node --experimental-strip-types "$repo_root/tools/conformance/fixture-server.ts" \
  > "$run_dir/port" 2> "$run_dir/fixture.log" &
fixture_pid=$!
for _ in {1..100}; do
  [[ -s "$run_dir/port" ]] && break
  kill -0 "$fixture_pid" 2>/dev/null || { cat "$run_dir/fixture.log" >&2; exit 1; }
  sleep 0.1
done
[[ -s "$run_dir/port" ]] || { echo "fixture server did not report a port" >&2; exit 1; }
port="$(head -n 1 "$run_dir/port")"
[[ "$port" =~ ^[0-9]+$ ]] || { echo "invalid fixture port: $port" >&2; exit 1; }
base_url="http://127.0.0.1:$port/v1"

case "$language" in
  dotnet)
    dotnet restore "$repo_root/sdks/dotnet/GhanaGeo.sln" --locked-mode
    "$repo_root/sdks/dotnet/verify.sh"
    dotnet run --project "$repo_root/sdks/dotnet/conformance/GhanaGeo.Conformance.csproj" \
      -c Release --no-build -- --root "$repo_root" --base-url "$base_url/" --report "$report"
    ;;
  php)
    GHANAGEO_CONFORMANCE_URL="$base_url" CI=1 "$repo_root/sdks/php/verify.sh"
    ruby "$repo_root/tools/conformance/export_cases.rb" > "$run_dir/cases.json"
    php "$repo_root/sdks/php/conformance/runner.php" "$run_dir/cases.json" "$base_url" "$report"
    ;;
esac

ruby "$repo_root/tools/conformance/validate.rb" --report "$report"
printf 'Strict %s conformance report: %s\n' "$language" "$report"
