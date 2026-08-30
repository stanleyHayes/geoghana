# GhanaGeo SDK Charter

**Status:** Approved V2.0 behavior contract
**Canonical API target:** `v1`
**Reference implementation:** `@ghanageo/client` (TypeScript), subordinate to the published contracts

This charter defines the behavior every official GhanaGeo SDK must expose. A
language may use idiomatic spelling, but it must preserve these semantics and
pass the shared conformance suite through its public facade.

## Public naming and surface

- Public methods use the language's conventional verb and casing while keeping
  operation meaning recognizable: list regions, get a district, search,
  geocode, reverse geocode, nearby, boundary and dataset access.
- Generated wire models remain generated. The hand-written facade may normalize
  transport details but must not invent a competing data model.
- Stable methods are never renamed without a documented deprecation period.
- REST and GraphQL results normalize to the same public shapes. An SDK only
  advertises gRPC when its public facade makes native gRPC calls.

## Authentication and privacy

- Anonymous public reads are the default. No package, example, fixture, build
  artifact or fallback configuration contains an API key.
- A key is sent only when the caller explicitly supplies one. SDKs never log or
  emit authorization headers or request bodies.
- Telemetry is local, metadata-only and optional. Callers can disable it with
  `telemetry: false`; disabling telemetry never changes request behavior.

## Errors

- Non-success responses throw/return one typed GhanaGeo error carrying HTTP or
  transport status, canonical catalog code, message, request ID and structured
  details when supplied.
- Error codes retain their canonical uppercase values. Unknown codes remain
  accessible rather than being coerced to a misleading known value.
- Cancellation is distinct from an API failure and preserves the host
  language's standard cancellation error.

## Cancellation, deadlines and retries

- Every network operation accepts the language's standard cancellation token.
  Cancellation must terminate the in-flight request and any retry delay.
- Automatic retry is bounded: the default is two retries and implementations
  must cap configuration at five. Exponential backoff is capped and honors a
  valid `Retry-After` response.
- Only safe reads may retry automatically, and only for transport failures or
  `429`, `502`, `503` and `504`. GraphQL POSTs and future writes are never
  replayed implicitly.

## Pagination

- Page methods expose `data`, `datasetVersion` and the opaque `nextCursor`.
- SDKs provide a lazy iterator over pages (and may additionally provide an item
  iterator). Iterators request the next page only after the consumer advances.
- Cursors are never parsed or manufactured by an SDK. A repeated cursor stops
  iteration with a typed client error instead of creating an infinite loop.

## Versions and compatibility

- Every network-client facade exposes target `apiVersion` and
  `testedDatasetVersion`; its registry manifest remains authoritative for SDK
  SemVer. Model, framework, offline-data and protocol support artifacts expose
  their own SemVer through their registry manifest and inherit API/dataset
  compatibility from their client or generated contract bundle. The dataset
  version is compatibility evidence, not a request pin and not the version
  returned by live responses.
- Contract bundles and generators are pinned and checksummed. Additive contract
  changes regenerate models; breaking facade changes require the SDK's next
  major version.
- Release artifacts must pass a clean-consumer install/build, shared
  conformance, secret scan and dependency/provenance checks.

## Idiomatic language mappings

All rows implement the common rules above, including anonymous construction,
no embedded key, two safe-read retries capped at five, capped `Retry-After`,
metadata-only telemetry with an explicit off switch, and facade API/dataset
version metadata.

| SDK | Naming and typed error | Cancellation | Lazy pagination | Configuration and versions |
|---|---|---|---|---|
| TypeScript | lower camel case such as `regionDistricts`; `GhanaGeoError` | `AbortSignal` | `AsyncGenerator<Page<T>>` | constructor options `retry`, `telemetry`, optional `apiKey`; static and instance `apiVersion` / `testedDatasetVersion` |
| Python | snake case such as `region_districts`; `GhanaGeoError` | task cancellation plus an optional timeout/cancel scope supported by the HTTP runtime | async iterator yielding typed pages | keyword-only `retry`, `telemetry`, optional `api_key`; module/client `api_version` / `tested_dataset_version` |
| Go | exported Pascal case such as `RegionDistricts`; typed `*ghanageo.Error` usable with `errors.As` | `context.Context` is the first network-method argument | iterator with `Next(ctx)` and `Page()`/`Err()` | functional options for retry/telemetry/key; exported API and tested-dataset constants |
| Dart | lower camel case such as `regionDistricts`; `GhanaGeoException` | cancellable request token accepted by every async operation | `Stream<Page<T>>` | named constructor options for retry/telemetry/key; static API and tested-dataset constants |
| Java | lower camel case such as `regionDistricts`; `GhanaGeoException` with catalog fields | `CompletableFuture.cancel` and an explicit cancellation token for blocking adapters | `Flow.Publisher<Page<T>>` (plus iterable page helper where blocking APIs are offered) | builder options for retry/telemetry/key; public API and tested-dataset constants |
| C#/.NET | Pascal case such as `RegionDistrictsAsync`; `GhanaGeoException` | trailing `CancellationToken` on every async operation | `IAsyncEnumerable<Page<T>>` | options object/DI configuration for retry/telemetry/key; public API and tested-dataset constants |
| PHP | lower camel case such as `regionDistricts`; `GhanaGeoException` | optional cancellation token supported by the selected PSR-18 async adapter; synchronous calls honor configured deadlines | `Generator<Page>` | named constructor/config options for retry/telemetry/key; public API and tested-dataset constants |

Language SDKs may provide additional item iterators or blocking conveniences,
but the table's public behavior is required. Test-only HTTP and sleep injection
seams never weaken anonymous-by-default behavior.
