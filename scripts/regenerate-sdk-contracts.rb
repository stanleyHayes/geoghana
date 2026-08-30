#!/usr/bin/env ruby
# frozen_string_literal: true

require "digest"
require "json"

root = File.expand_path("..", __dir__)
Dir.chdir(root)
system("ruby", "tools/conformance/generate.rb") or abort "conformance generation failed"
digest = ->(path) { Digest::SHA256.file(path).hexdigest }

%w[sdks/java/contract.lock sdks/php/contract.lock].each do |lock_path|
  # macOS still ships Ruby 2.6 on some release workstations, before
  # Enumerable#filter_map was introduced. Keep regeneration portable.
  paths = File.readlines(lock_path).map { |line| line.match(/^[a-f0-9]{64}\s+(.+)$/)&.captures&.first }.compact
  abort "#{lock_path} has no contract paths" if paths.empty?
  File.write(lock_path, paths.map { |path| "#{digest.call(path)}  #{path}\n" }.join)
end

dotnet_path = "sdks/dotnet/contract.lock.json"
dotnet = JSON.parse(File.read(dotnet_path))
dotnet["contracts"] = dotnet.fetch("contracts").to_h { |path, _value| [path, digest.call(path)] }
File.write(dotnet_path, JSON.pretty_generate(dotnet) + "\n")
system("python3", "sdks/python/scripts/check_contract_lock.py", "--write") or abort "Python contract lock generation failed"

dart_path = "sdks/dart/contract.lock.json"
dart = JSON.parse(File.read(dart_path))
%w[openapi protobuf].each { |name| dart.fetch(name)["sha256"] = digest.call(dart.fetch(name).fetch("path")) }
export = IO.popen(%w[ruby tools/conformance/export_cases.rb], &:read)
abort "case export failed" unless $?.success?
dart.fetch("conformance")["digest"] = JSON.parse(export).fetch("contractDigest")
File.write(dart_path, JSON.pretty_generate(dart) + "\n")

proto = "proto/ghanageo/v1/geography.proto"
proto_digest = digest.call(proto)
go_lock = "sdks/go/proto.lock"
File.write(go_lock, File.read(go_lock).sub(/^sha256 = .+$/, "sha256 = #{proto_digest}"))
go_generate = "sdks/go/generate.sh"
File.write(go_generate, File.read(go_generate).sub(/^expected=.+$/, "expected=#{proto_digest}"))
system("sh", go_generate) or abort "Go protobuf generation failed"
puts "regenerated conformance inventory, SDK locks, and Go protobuf exports"
