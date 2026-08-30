#!/bin/sh
set -eu

cd "$(dirname "$0")"
expected=93f2448982cec282cf260e0ea1e52151b88c6ca1148dc8eb57c8ba47404ff153
actual=$(shasum -a 256 ../../proto/ghanageo/v1/geography.proto | awk '{print $1}')
test "$actual" = "$expected" || {
  echo "canonical proto checksum changed; review the contract and update proto.lock" >&2
  exit 1
}

protoc -I ../../proto \
  --go_out=. --go_opt=module=github.com/ghanageo/ghanageo-go \
  --go_opt=Mghanageo/v1/geography.proto=github.com/ghanageo/ghanageo-go/proto/ghanageo/v1 \
  --go-grpc_out=. --go-grpc_opt=module=github.com/ghanageo/ghanageo-go \
  --go-grpc_opt=Mghanageo/v1/geography.proto=github.com/ghanageo/ghanageo-go/proto/ghanageo/v1 \
  ../../proto/ghanageo/v1/geography.proto
