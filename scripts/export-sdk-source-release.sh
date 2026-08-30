#!/usr/bin/env bash
set -euo pipefail
family="${1:-}"
archive="${2:-}"
version="${3:-}"
repository="${4:-}"
[[ "$family" == go || "$family" == php ]] || { echo "family must be go or php" >&2; exit 2; }
[[ -f "$archive" && -n "$version" && "$repository" == */* ]] || { echo "usage: $0 <go|php> <archive> <version> <owner/repo>" >&2; exit 2; }
[[ -n "${GH_TOKEN:-}" && -n "${SOURCE_GIT_SIGNING_KEY:-}" ]] || { echo "GH_TOKEN and SOURCE_GIT_SIGNING_KEY are required" >&2; exit 2; }
temp_root="$(cd "${TMPDIR:-/tmp}" && pwd -P)"
work="$(mktemp -d "$temp_root/ghanageo-source-release.XXXXXX")"
cleanup() { rm -rf "$work"; }
trap cleanup EXIT HUP INT TERM
git clone "https://x-access-token:${GH_TOKEN}@github.com/${repository}.git" "$work/repo"
default_branch="$(gh api "repos/$repository" --jq .default_branch)"
git -C "$work/repo" rm -r --ignore-unmatch . >/dev/null
mkdir -p "$work/export"
if [[ "$family" == go ]]; then
  tar -xzf "$archive" -C "$work/export"
  cp -R "$work/export/sdks/go/." "$work/repo/"
  mkdir -p "$work/repo/proto/ghanageo/v1"
  cp -R "$work/export/proto/ghanageo/v1/." "$work/repo/proto/ghanageo/v1/"
else
  unzip -q "$archive" -d "$work/export"
  cp -R "$work/export/." "$work/repo/"
fi
git -C "$work/repo" add -A
expected_tree="$(git -C "$work/repo" write-tree)"
tag="v$version"
if [[ "${VERIFY_ONLY:-false}" == true ]]; then
  tag_type="$(gh api "repos/$repository/git/ref/tags/$tag" --jq .object.type)"
  tag_object="$(gh api "repos/$repository/git/ref/tags/$tag" --jq .object.sha)"
  if [[ "$tag_type" == tag ]]; then tag_object="$(gh api "repos/$repository/git/tags/$tag_object" --jq .object.sha)"; fi
  remote_tree="$(gh api "repos/$repository/git/commits/$tag_object" --jq .tree.sha)"
  [[ "$remote_tree" == "$expected_tree" ]] || { echo "$family existing tag does not match verified source artifact" >&2; exit 1; }
  printf '%s\n' "$remote_tree"
  exit 0
fi
git -C "$work/repo" config user.name ghanageo-release
git -C "$work/repo" config user.email releases@ghanageo.gov.gh
printf '%s\n' "$SOURCE_GIT_SIGNING_KEY" > "$work/signing-key"
chmod 600 "$work/signing-key"
git -C "$work/repo" config gpg.format ssh
git -C "$work/repo" config user.signingkey "$work/signing-key"
git -C "$work/repo" commit -S -m "release: $version" >/dev/null
tree="$(git -C "$work/repo" rev-parse 'HEAD^{tree}')"
git -C "$work/repo" tag -s -a "$tag" -m "GhanaGeo $family SDK $version; exported tree $tree"
git -C "$work/repo" push origin "HEAD:$default_branch" "$tag"
tag_type="$(gh api "repos/$repository/git/ref/tags/$tag" --jq .object.type)"
tag_object="$(gh api "repos/$repository/git/ref/tags/$tag" --jq .object.sha)"
if [[ "$tag_type" == tag ]]; then tag_object="$(gh api "repos/$repository/git/tags/$tag_object" --jq .object.sha)"; fi
remote_tree="$(gh api "repos/$repository/git/commits/$tag_object" --jq .tree.sha)"
[[ "$remote_tree" == "$tree" ]] || { echo "$family tag/tree verification failed" >&2; exit 1; }
printf '%s\n' "$tree"
