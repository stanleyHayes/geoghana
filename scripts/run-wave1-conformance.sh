#!/usr/bin/env bash
set -euo pipefail

language="${1:-}"
report="${2:-}"
[[ "$language" == "python" || "$language" == "go" ]] || { echo "usage: $0 python|go REPORT" >&2; exit 2; }
[[ -n "$report" ]] || { echo "report path is required" >&2; exit 2; }

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
run_dir="$(mktemp -d "${TMPDIR:-/tmp}/ghanageo-wave1-conformance.XXXXXX")"
fixture_pid=""
cleanup() {
  if [[ -n "$fixture_pid" ]] && kill -0 "$fixture_pid" 2>/dev/null; then kill "$fixture_pid" 2>/dev/null || true; wait "$fixture_pid" 2>/dev/null || true; fi
  rm -rf "$run_dir"
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
base_url="http://127.0.0.1:$port/v1"
report="$(mkdir -p "$(dirname "$report")" && cd "$(dirname "$report")" && pwd -P)/$(basename "$report")"

runner_status=0
if [[ "$language" == "python" ]]; then
  "${GHANAGEO_PYTHON_BIN:-$repo_root/sdks/python/.venv/bin/python}" "$repo_root/sdks/python/conformance/runner.py" --base-url "$base_url" --output "$report" || runner_status=$?
else
  (cd "$repo_root/sdks/go" && GOWORK=off GOTOOLCHAIN=local go run ./conformance --base-url "$base_url" --output "$report" --root ../..) || runner_status=$?
fi
validator_status=0
if [[ -f "$report" ]]; then
  ruby "$repo_root/tools/conformance/validate.rb" --report "$report" || validator_status=$?
else
  echo "$language runner did not produce $report" >&2
  validator_status=1
fi
(( runner_status == 0 && validator_status == 0 ))
