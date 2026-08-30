#!/bin/sh
set -eu

cd "$(dirname "$0")"
export GOWORK=off

test "$(protoc --version)" = "libprotoc 33.2" || { echo "protoc 33.2 required" >&2; exit 1; }
test "$(protoc-gen-go --version)" = "protoc-gen-go v1.36.11" || { echo "protoc-gen-go v1.36.11 required" >&2; exit 1; }
test "$(protoc-gen-go-grpc --version)" = "protoc-gen-go-grpc 1.6.2" || { echo "protoc-gen-go-grpc 1.6.2 required" >&2; exit 1; }

gofmt_output=$(gofmt -l .)
test -z "$gofmt_output" || { echo "unformatted files: $gofmt_output" >&2; exit 1; }
go test ./...
go vet ./...
go build ./...

generated=$(mktemp -d "${TMPDIR:-/tmp}/ghanageo-go-generated.XXXXXX")
trap 'rm -rf "$generated"' EXIT HUP INT TERM
protoc -I ../../proto \
  --go_out="$generated" --go_opt=module=github.com/ghanageo/ghanageo-go \
  --go_opt=Mghanageo/v1/geography.proto=github.com/ghanageo/ghanageo-go/proto/ghanageo/v1 \
  --go-grpc_out="$generated" --go-grpc_opt=module=github.com/ghanageo/ghanageo-go \
  --go-grpc_opt=Mghanageo/v1/geography.proto=github.com/ghanageo/ghanageo-go/proto/ghanageo/v1 \
  ../../proto/ghanageo/v1/geography.proto
cmp proto/ghanageo/v1/geography.pb.go "$generated/proto/ghanageo/v1/geography.pb.go"
cmp proto/ghanageo/v1/geography_grpc.pb.go "$generated/proto/ghanageo/v1/geography_grpc.pb.go"

repo_root=$(cd ../.. && pwd)
ruby -r "$repo_root/tools/conformance/lib.rb" -e '
  files = Dir[File.join(ARGV.fetch(0), "**", "*")].select { |path| File.file?(path) }
  offending = files.find { |path| GhanaGeo::Conformance.contains_secret?(File.binread(path)) }
  abort "secret-shaped content in #{offending}" if offending
' "$(pwd)"

consumer=$(mktemp -d "${TMPDIR:-/tmp}/ghanageo-go-consumer.XXXXXX")
trap 'rm -rf "$consumer" "$generated"' EXIT HUP INT TERM
(
  cd "$consumer"
  go mod init example.invalid/consumer >/dev/null
  go mod edit -replace github.com/ghanageo/ghanageo-go="$(cd "$repo_root/sdks/go" && pwd)"
  go get github.com/ghanageo/ghanageo-go@v0.0.0 >/dev/null
  printf '%s\n' 'package main' 'import ("context"; g "github.com/ghanageo/ghanageo-go")' 'func main(){ c,_:=g.New(); _,_=c.Regions(context.Background(),g.PageOptions{Limit:1}) }' > main.go
  go build .
)

echo "Go SDK verification passed"
