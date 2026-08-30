#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
report="${REPORT:-${TMPDIR:-/tmp}/ghanageo-go-grpc-conformance.json}"
mkdir -p "$(dirname "$report")"
report="$(cd "$(dirname "$report")" && pwd -P)/$(basename "$report")"
run_dir="$(mktemp -d "${TMPDIR:-/tmp}/ghanageo-grpc-conformance.XXXXXX")"
server_pid=""
cleanup() {
  if [[ -n "$server_pid" ]] && kill -0 "$server_pid" 2>/dev/null; then kill "$server_pid" 2>/dev/null || true; wait "$server_pid" 2>/dev/null || true; fi
  case "$run_dir" in "${TMPDIR:-/tmp}"/ghanageo-grpc-conformance.*) rm -rf "$run_dir" ;; *) return 1 ;; esac
}
trap cleanup EXIT HUP INT TERM

ruby "$repo_root/tools/conformance/export_cases.rb" > "$run_dir/cases.json"
(cd "$repo_root/tools/conformance/grpc" && GOWORK=off GOTOOLCHAIN=local go run ./cmd/server --cases "$run_dir/cases.json") > "$run_dir/port" 2> "$run_dir/server.log" &
server_pid=$!
for _ in {1..100}; do
  [[ -s "$run_dir/port" ]] && break
  kill -0 "$server_pid" 2>/dev/null || { cat "$run_dir/server.log" >&2; exit 1; }
  sleep 0.1
done
[[ -s "$run_dir/port" ]] || { echo "gRPC fixture did not report a port" >&2; exit 1; }
port="$(head -n1 "$run_dir/port")"
[[ "$port" =~ ^[0-9]+$ ]] || { echo "invalid gRPC fixture port" >&2; exit 1; }

(cd "$repo_root/tools/conformance/grpc" && GOWORK=off GOTOOLCHAIN=local go run ./cmd/runner --address "127.0.0.1:$port" --cases "$run_dir/cases.json" --output "$report")
ruby "$repo_root/tools/conformance/validate.rb" --report "$report"
printf 'Strict Go gRPC conformance report: %s\n' "$report"
