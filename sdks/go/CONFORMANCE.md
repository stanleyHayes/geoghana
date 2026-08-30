# Conformance status

The public REST facade is covered by deterministic loopback tests in
`client_test.go`, including all contract paths, anonymous defaults, typed
catalog errors, cancellation, retries and lazy cursor safety. The thin runner
in `conformance/runner.go` exercises that public facade against the shared live
fixture and emits a report accepted by `contracts/conformance/report-schema.json`.

Generated protobuf messages and `GeographyServiceClient` are compiled from the
canonical proto and covered by `go test ./...` / `go vet ./...`.

A multi-protocol report additionally requires a live fixture exposing
the canonical gRPC service, especially the server-streaming
`StreamDatasetChanges` RPC. The repository's current fixture server exposes
REST and GraphQL only. The SDK deliberately does not label synthetic protobuf
construction as live gRPC evidence. Once that fixture exists, the Go runner can
bind public REST calls and generated `GeographyServiceClient` calls into the
shared report schema without changing the SDK facade.
