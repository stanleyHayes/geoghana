# GhanaGeo for .NET

The official nullable, typed .NET 8 client for GhanaGeo. Public reads are
anonymous by default; an API key is only sent when you explicitly configure
one. The package targets REST `v1` and was validated against dataset
`2026.08.3-ulid`.

## Install

```bash
dotnet add package GhanaGeo
```

Direct construction:

```csharp
using GhanaGeo;

using var http = new HttpClient();
IGhanaGeoClient client = new GhanaGeoClient(http);
var ashanti = await client.GetRegionAsync("gh-region-ashanti", cancellationToken);
```

ASP.NET Core dependency injection and `IHttpClientFactory`:

```csharp
builder.Services.AddGhanaGeo(options =>
{
    options.ApiKey = builder.Configuration["GhanaGeo:ApiKey"]; // Optional.
    options.TelemetryEnabled = false;
});
```

Every network method accepts a trailing `CancellationToken`. Page iterators are
lazy and reject repeated cursors:

```csharp
await foreach (var page in client.EnumeratePlacePagesAsync(
    new PlaceListOptions(RegionId: "gh-region-ashanti", Limit: 50),
    cancellationToken))
{
    foreach (var place in page.Data) Console.WriteLine(place.Name);
}
```

Dataset artifacts are streamed with a caller-defined byte cap. The file helper
writes beside the destination and atomically replaces it only after the stream
and optional SHA-256 check succeed:

```csharp
await client.DownloadDatasetArtifactToFileAsync(
    "2026.08.3-ulid", "regions", "json", "regions.json",
    new DatasetArtifactOptions(MaximumBytes: 32 * 1024 * 1024, ExpectedSha256: checksum),
    cancellationToken);
```

Safe GETs retry twice by default (maximum five), with capped exponential jitter
and `Retry-After` support. Telemetry callbacks receive operation, attempt,
status, duration, and outcome only—never keys, URLs, coordinates, or bodies.
JSON success and error streams have independent configurable byte limits;
`MaximumJsonResponseBytes` defaults to 8 MiB and
`MaximumErrorResponseBytes` defaults to 1 MiB. Both reject an oversized
`Content-Length` before reading and enforce the same limit while streaming.

## Development

Run `./verify.sh` for formatting, analyzer, tests, deterministic package,
source-symbol, secret, and clean-consumer checks. A running shared fixture is
required only for live conformance:

```bash
dotnet run --project conformance/GhanaGeo.Conformance.csproj -- \
  --root ../.. --base-url http://localhost:8180/v1/ --report conformance-report.json
```

See [CONFORMANCE.md](CONFORMANCE.md) for the evidence contract.
