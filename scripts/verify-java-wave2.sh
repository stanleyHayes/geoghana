#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
report="${REPORT:-/tmp/ghanageo-java-conformance.json}"
run_dir="$(mktemp -d "${TMPDIR:-/tmp}/ghanageo-java-wave2.XXXXXX")"
fixture_pid=""
cleanup() {
  if [[ -n "$fixture_pid" ]] && kill -0 "$fixture_pid" 2>/dev/null; then kill "$fixture_pid" 2>/dev/null || true; wait "$fixture_pid" 2>/dev/null || true; fi
  case "$run_dir" in "${TMPDIR:-/tmp}"/ghanageo-java-wave2.*) rm -rf "$run_dir" ;; *) return 1 ;; esac
}
trap cleanup EXIT HUP INT TERM

node --experimental-strip-types "$repo_root/tools/conformance/fixture-server.ts" > "$run_dir/port" 2> "$run_dir/fixture.log" &
fixture_pid=$!
for _ in {1..100}; do
  [[ -s "$run_dir/port" ]] && break
  kill -0 "$fixture_pid" 2>/dev/null || { cat "$run_dir/fixture.log" >&2; exit 1; }
  sleep 0.1
done
[[ -s "$run_dir/port" ]] || { echo "fixture server did not report a port" >&2; exit 1; }
port="$(head -n 1 "$run_dir/port")"
[[ "$port" =~ ^[0-9]+$ ]] || { echo "invalid fixture port: $port" >&2; exit 1; }

GHANAGEO_CONFORMANCE_URL="http://127.0.0.1:$port/v1" "$repo_root/sdks/java/verify.sh"
report="$(mkdir -p "$(dirname "$report")" && cd "$(dirname "$report")" && pwd -P)/$(basename "$report")"
cp "$repo_root/sdks/java/java-conformance-report.json" "$report"
ruby "$repo_root/tools/conformance/validate.rb" --report "$report"
