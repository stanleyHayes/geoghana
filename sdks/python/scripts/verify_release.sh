#!/bin/sh
set -eu

sdk_dir=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
cd "$sdk_dir"
command -v uv >/dev/null 2>&1 || {
  echo "uv is required for locked release verification" >&2
  exit 1
}

consumer=$(mktemp -d)
artifacts=$(mktemp -d)
owned_environment=''
if [ -z "${UV_PROJECT_ENVIRONMENT:-}" ]; then
  owned_environment=$(mktemp -d)
  UV_PROJECT_ENVIRONMENT="$owned_environment/project"
  export UV_PROJECT_ENVIRONMENT
fi

cleanup() {
  rm -rf "$consumer" "$artifacts"
  if [ -n "$owned_environment" ]; then
    rm -rf "$owned_environment"
  fi
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

uv sync --frozen --extra dev
python_bin=$(uv run --frozen --extra dev python -c 'import sys; print(sys.executable)')
scanner="$sdk_dir/../../tools/conformance/scan-secrets"
test -x "$scanner" || {
  echo "canonical GhanaGeo secret scanner not found: $scanner" >&2
  exit 1
}
ruby -rjson -rdigest -e '
  lock = JSON.parse(File.read(ARGV.shift)).fetch("contracts")
  root = ARGV.shift
  bad = lock.each_with_object([]) do |(_name, entry), drift|
    next unless entry.is_a?(Hash) && entry["path"] && entry["sha256"]
    path = entry.fetch("path")
    actual = Digest::SHA256.file(File.join(root, path)).hexdigest
    drift << path unless actual == entry.fetch("sha256")
  end
  abort "Python contract lock drift: #{bad.join(", ")}" unless bad.empty?
' "$sdk_dir/contract.lock.json" "$sdk_dir/../.."

"$python_bin" -m ruff check .
"$python_bin" -m mypy src conformance/runner.py scripts/check_contract_lock.py
"$python_bin" -m pytest
"$python_bin" -m compileall -q src examples conformance
"$python_bin" scripts/check_contract_lock.py

build_output="$artifacts/dist"
mkdir -p "$build_output"
"$python_bin" -m build --no-isolation --outdir "$build_output"
wheel=$(find "$build_output" -maxdepth 1 -name '*.whl' -print -quit)
sdist=$(find "$build_output" -maxdepth 1 -name '*.tar.gz' -print -quit)
test -n "$wheel" && test -n "$sdist"
"$python_bin" -m zipfile -l "$wheel" | grep 'ghanageo/py.typed'
"$python_bin" -m zipfile -l "$wheel" | grep 'ghanageo/contract.lock.json'
"$python_bin" -m tarfile -l "$sdist" | grep 'contract.lock.json'

"$python_bin" -m zipfile -e "$wheel" "$artifacts/wheel"
"$python_bin" -m tarfile --filter data -e "$sdist" "$artifacts/sdist"
scan_files=$(find src examples tests conformance -type f ! -path '*/__pycache__/*' ! -name '*.pyc' -print)
# Paths in this SDK are deliberately whitespace-free; pass each source file to
# the canonical scanner so generated caches never become release evidence.
# shellcheck disable=SC2086
"$scanner" $scan_files README.md contract.lock.json pyproject.toml uv.lock "$artifacts/wheel" "$artifacts/sdist"

"$python_bin" -m venv "$consumer/venv"
"$consumer/venv/bin/python" -m pip install --quiet "$wheel"
"$consumer/venv/bin/python" -c 'import ghanageo; assert ghanageo.api_version == "v1"; client = ghanageo.GhanaGeo(); assert client._client.headers.get("authorization") is None; client.close()'
