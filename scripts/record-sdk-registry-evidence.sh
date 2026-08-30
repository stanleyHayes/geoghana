#!/usr/bin/env bash
set -euo pipefail
kind="${1:-}" ledger="${2:-}" checkpoint="${3:-}" version="${4:-}" bundle="${5:-}"
[[ -n "$kind" && -f "$ledger" && -n "$checkpoint" && -n "$version" ]] || { echo "usage: $0 <pypi|nuget|pub|maven|go|packagist> <ledger> <checkpoint> <version> <bundle-package-tree-or-repo>" >&2; exit 2; }
work="$(mktemp -d "${TMPDIR:-/tmp}/ghanageo-registry-evidence.XXXXXX")"
trap 'rm -rf "$work"' EXIT HUP INT TERM
case "$kind" in
  pypi)
    curl -fsSL "https://pypi.org/pypi/ghanageo/$version/json" > "$work/remote.json"
    node - "$bundle" "$work/remote.json" "$work/evidence.json" <<'NODE'
const fs=require('fs'),crypto=require('crypto'); const [dir,json,out]=process.argv.slice(2);
const remote=JSON.parse(fs.readFileSync(json)).urls; const files=fs.readdirSync(dir).filter(x=>/\.(whl|tar\.gz)$/.test(x));
const artifacts=files.map(name=>{const sha256=crypto.createHash('sha256').update(fs.readFileSync(`${dir}/${name}`)).digest('hex'); const r=remote.find(x=>x.filename===name); if(!r||r.digests.sha256!==sha256) throw Error(`PyPI digest mismatch: ${name}`); return {name,sha256};});
fs.writeFileSync(out,JSON.stringify({registry:'pypi',artifacts}));
NODE
    ;;
  nuget)
    local_file="$(find "$bundle" -maxdepth 1 -iname '*.nupkg' ! -iname '*.snupkg' -print -quit)"
    curl -fsSL "https://api.nuget.org/v3-flatcontainer/ghanageo/${version,,}/ghanageo.${version,,}.nupkg" > "$work/remote.nupkg"
    local_sha="$(shasum -a 256 "$local_file"|awk '{print $1}')"; remote_sha="$(shasum -a 256 "$work/remote.nupkg"|awk '{print $1}')"
    [[ "$local_sha" == "$remote_sha" ]] || { echo "NuGet nupkg digest mismatch" >&2; exit 1; }
    node -e 'require("fs").writeFileSync(process.argv[1],JSON.stringify({registry:"nuget",package:"GhanaGeo",version:process.argv[2],sha256:process.argv[3]}))' "$work/evidence.json" "$version" "$local_sha"
    ;;
  pub)
    curl -fsSL "https://pub.dev/api/packages/$bundle/versions/$version" > "$work/remote.json"
    node - "$work/remote.json" "$work/evidence.json" "$bundle" "$version" <<'NODE'
const fs=require('fs'),crypto=require('crypto'); const [input,out,name,version]=process.argv.slice(2); const bytes=fs.readFileSync(input); const m=JSON.parse(bytes);
if(m.version!==version) throw Error('pub.dev version metadata mismatch');
// pub.dev exposes package metadata but no immutable archive digest. Bind the canonical API response hash; source tag and compatibility manifest remain the byte provenance boundary.
fs.writeFileSync(out,JSON.stringify({registry:'pub.dev',name,version,metadataSha256:crypto.createHash('sha256').update(bytes).digest('hex'),archiveDigestAvailable:false}));
NODE
    ;;
  maven)
    : > "$work/rows"
    for spec in 'ghanageo-java:client' 'ghanageo-spring-boot-starter:spring-boot-starter'; do
      artifact="${spec%%:*}"; module="${spec#*:}"
      for suffix in pom jar sources.jar javadoc.jar; do
        remote="$work/$artifact-$suffix"; url="https://repo1.maven.org/maven2/dev/ghanageo/$artifact/$version/$artifact-$version.$suffix"
        curl -fsSL "$url" > "$remote"; curl -fsSL "$url.asc" >/dev/null
        remote_sha="$(shasum -a 256 "$remote"|awk '{print $1}')"
        if [[ "$suffix" != pom ]]; then local_file="$bundle/$module/target/$artifact-$version.$suffix"; [[ -f "$local_file" ]] || { echo "missing staged Maven file $local_file" >&2; exit 1; }; local_sha="$(shasum -a 256 "$local_file"|awk '{print $1}')"; [[ "$local_sha" == "$remote_sha" ]] || { echo "Maven digest mismatch: $artifact $suffix" >&2; exit 1; }; fi
        printf '%s\t%s\t%s\n' "$artifact" "$suffix" "$remote_sha" >> "$work/rows"
      done
    done
    node - "$work/rows" "$work/evidence.json" <<'NODE'
const fs=require('fs'); const [rows,out]=process.argv.slice(2); const artifacts=fs.readFileSync(rows,'utf8').trim().split('\n').map(x=>{const [artifact,file,sha256]=x.split('\t');return {artifact,file,sha256,signature:true}}); fs.writeFileSync(out,JSON.stringify({registry:'maven-central',artifacts}));
NODE
    ;;
  go)
    escaped_module="github.com%2Fghanageo%2Fghanageo-go"
    curl -fsSL "https://proxy.golang.org/$escaped_module/@v/v$version.info" > "$work/info.json"
    curl -fsSL "https://proxy.golang.org/$escaped_module/@v/v$version.zip" > "$work/module.zip"
    node - "$work/evidence.json" "$version" "$bundle" "$work/info.json" "$work/module.zip" <<'NODE'
const f=require('fs'),c=require('crypto'); const [out,version,tree,info,zip]=process.argv.slice(2); const hash=p=>c.createHash('sha256').update(f.readFileSync(p)).digest('hex'); f.writeFileSync(out,JSON.stringify({registry:'go-module',repository:'ghanageo/ghanageo-go',version,tree,proxyInfoSha256:hash(info),proxyZipSha256:hash(zip)}));
NODE
    ;;
  packagist)
    curl -fsSL https://repo.packagist.org/p2/ghanageo/ghanageo-php.json > "$work/metadata.json"
    tag_object="$(gh api "repos/$bundle/git/ref/tags/v$version" --jq .object.sha)"; tag_type="$(gh api "repos/$bundle/git/ref/tags/v$version" --jq .object.type)"; [[ "$tag_type" == tag ]] && tag_object="$(gh api "repos/$bundle/git/tags/$tag_object" --jq .object.sha)"
    ruby -rjson -e 'wanted,commit=ARGV; versions=JSON.parse(STDIN.read).fetch("packages",{}).values.flatten; exit(versions.any? { |v| (v["version_normalized"]&.start_with?(wanted) || v["version"]==wanted) && (v.dig("source","reference")==commit || v.dig("dist","reference")==commit) } ? 0 : 1)' "$version" "$tag_object" < "$work/metadata.json" || { echo "Packagist source reference mismatch" >&2; exit 1; }
    node -e 'require("fs").writeFileSync(process.argv[1],JSON.stringify({registry:"packagist",version:process.argv[2],sourceCommit:process.argv[3]}))' "$work/evidence.json" "$version" "$tag_object"
    ;;
  *) echo "unknown registry evidence kind: $kind" >&2; exit 2;;
esac
node scripts/sdk-release-ledger.mjs complete "$ledger" _ "$checkpoint" "$work/evidence.json"
