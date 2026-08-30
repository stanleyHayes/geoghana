#!/bin/sh
set -eu

ruby tools/conformance/generate.rb --check
ruby tools/conformance/validate.rb
ruby -Itools/conformance tools/conformance/test/validate_test.rb
ruby -Itools/conformance tools/conformance/test/matrix_test.rb
(cd tools/conformance/grpc && GOWORK=off GOTOOLCHAIN=local go test ./...)
pnpm --filter @ghanageo/conformance typecheck
pnpm --filter @ghanageo/conformance test
pnpm --filter @ghanageo/client typecheck
pnpm --filter @ghanageo/client test
