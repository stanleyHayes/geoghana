#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
python_version="${PYTHON_VERSION:-3.12}"
report="${REPORT:-/tmp/ghanageo-python-conformance.json}"
environment_root="$(mktemp -d "${TMPDIR:-/tmp}/ghanageo-python-env.XXXXXX")"
cleanup() { case "$environment_root" in "${TMPDIR:-/tmp}"/ghanageo-python-env.*) rm -rf "$environment_root" ;; *) return 1 ;; esac; }
trap cleanup EXIT HUP INT TERM

export UV_PROJECT_ENVIRONMENT="$environment_root/project"
(cd "$repo_root/sdks/python" && uv sync --frozen --python "$python_version" --extra dev)
python_bin="$(cd "$repo_root/sdks/python" && uv run --frozen --extra dev python -c 'import sys; print(sys.executable)')"
"$repo_root/sdks/python/scripts/verify_release.sh"
GHANAGEO_PYTHON_BIN="$python_bin" "$repo_root/scripts/run-wave1-conformance.sh" python "$report"
