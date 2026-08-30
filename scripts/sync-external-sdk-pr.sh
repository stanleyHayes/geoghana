#!/usr/bin/env bash
set -euo pipefail
family="${1:-}"
repository="${2:-}"
branch="${3:-}"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
[[ "$repository" == */* && -n "$branch" && -n "${GH_TOKEN:-}" ]] || { echo "usage: $0 <family> <owner/repo> <branch> with GH_TOKEN" >&2; exit 2; }
case "$family" in python|go|dart|java|dotnet|php) ;; *) echo "unsupported SDK family: $family" >&2; exit 2;; esac
temp_root="$(cd "${TMPDIR:-/tmp}" && pwd -P)"
work="$(mktemp -d "$temp_root/ghanageo-sdk-sync.XXXXXX")"
cleanup() { rm -rf "$work"; }
trap cleanup EXIT HUP INT TERM
git clone "https://x-access-token:${GH_TOKEN}@github.com/${repository}.git" "$work/repo"
default_branch="$(gh api "repos/$repository" --jq .default_branch)"
git -C "$work/repo" switch -c "$branch"
git -C "$work/repo" rm -r --ignore-unmatch . >/dev/null
mkdir -p "$work/export"
tar -C "$repo_root/sdks/$family" --exclude='.git' --exclude='.venv' --exclude='vendor' --exclude='node_modules' --exclude='target' --exclude='build' --exclude='bin' --exclude='obj' --exclude='artifacts' -cf - . | tar -C "$work/export" -xf -
cp -R "$work/export/." "$work/repo/"
git -C "$work/repo" add -A
git -C "$work/repo" diff --cached --quiet && exit 0
git -C "$work/repo" config user.name github-actions[bot]
git -C "$work/repo" config user.email 41898282+github-actions[bot]@users.noreply.github.com
git -C "$work/repo" commit -m "chore: regenerate GhanaGeo contract exports"
git -C "$work/repo" push origin "$branch"
gh pr create --repo "$repository" --base "$default_branch" --head "$branch" --title "chore: regenerate GhanaGeo contract exports" --body "Automated export from $GITHUB_REPOSITORY@$GITHUB_SHA."
