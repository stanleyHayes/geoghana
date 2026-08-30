#!/usr/bin/env bash
set -euo pipefail
# shellcheck disable=SC1091
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/sdk-release-lock.sh"
test_root="$(mktemp -d "${TMPDIR:-/tmp}/ghanageo-lock-test.XXXXXX")"
trap 'rm -rf "$test_root"' EXIT
lock="$test_root/lock"
mkdir "$lock"
printf '99999999\n' > "$lock/owner"
acquire_sdk_source_lock "$lock" "$$"
[[ "$(cat "$lock/owner")" == "$$" ]]
release_sdk_source_lock "$lock" "$$"
[[ ! -e "$lock" ]]
