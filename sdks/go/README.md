# GhanaGeo for Go

Idiomatic Go access to GhanaGeo's public REST API plus generated protobuf and
gRPC types from the canonical `proto/ghanageo/v1/geography.proto` contract.

```go
client, err := ghanageo.New(
    // ghanageo.WithAPIKey(os.Getenv("GHANAGEO_API_KEY")), // optional
)
page, err := client.Regions(ctx, ghanageo.PageOptions{Limit: 16})
```

All network methods accept `context.Context` first. Public reads are anonymous
by default. The client retries safe reads twice for transient transport errors
and HTTP 429/502/503/504, with a configurable maximum of five retries.
Telemetry is off by default. `WithTelemetry(func(TelemetryEvent) { ... })`
adds a local metadata-only observer; observer failures never affect requests.

Lazy pagination:

```go
pages := client.PlacePages(ghanageo.PlaceOptions{RegionID: "gh-region-greater-accra"})
for pages.Next(ctx) {
    page := pages.Page()
    // consume page.Data
}
if err := pages.Err(); err != nil { return err }
```

Errors support both `errors.As` and catalog sentinel matching:

```go
var apiErr *ghanageo.Error
if errors.As(err, &apiErr) { log.Print(apiErr.Code, apiErr.RequestID) }
if errors.Is(err, ghanageo.ErrNotFound) { /* handle missing record */ }
```

Large artifacts should stream to an atomic destination with a size bound and
the checksum published by the dataset catalogue:

```go
err := client.DownloadDatasetArtifactTo(ctx, version, "regions", "json", path,
    ghanageo.DownloadOptions{MaxBytes: 128 << 20, SHA256: download.Checksum})
```

## Compatibility

- Go 1.23 and newer (reviewed every six months; next review February 2027)
- API `v1`
- Tested dataset `2026.08.3-ulid`

Run `./verify.sh` from this directory. Against the shared fixture, produce REST
evidence with `go run ./conformance --base-url http://127.0.0.1:PORT/v1`.
This is a standalone module outside the repository `go.work`; use
`GOWORK=off go test ./...` for direct commands. `verify.sh` sets it itself.
