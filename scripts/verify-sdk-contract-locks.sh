#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$repo_root"
ruby tools/conformance/generate.rb --check
shasum -a 256 -c sdks/java/contract.lock
shasum -a 256 -c sdks/php/contract.lock
ruby -rjson -rdigest -e '
  dotnet = JSON.parse(File.read("sdks/dotnet/contract.lock.json")).fetch("contracts")
  dart = JSON.parse(File.read("sdks/dart/contract.lock.json"))
  locks = dotnet.merge(
    dart.fetch("openapi").fetch("path") => dart.fetch("openapi").fetch("sha256"),
    dart.fetch("protobuf").fetch("path") => dart.fetch("protobuf").fetch("sha256")
  )
  locks.each do |path, expected|
    actual = Digest::SHA256.file(path).hexdigest
    abort "#{path} drifted" unless actual == expected
  end
'
python3 sdks/python/scripts/check_contract_lock.py
echo "all SDK contract locks match exportable roots"
