# GhanaGeo for Dart

The official null-safe SDK for the GhanaGeo v1 API. It is anonymous by default,
supports real request cancellation, typed catalog errors, bounded retry and
response sizes, lazy cursor streams, privacy-safe optional telemetry, all public
REST geography/dataset operations, and an injectable native gRPC dataset-change
stream.

The canonical OpenAPI, protobuf and shared-conformance digests are pinned in
`contract.lock.json`; verification fails on unreviewed drift.

```dart
final client = GhanaGeoClient();
final regions = await client.regions(limit: 16);
await for (final page in client.placePages(regionId: 'gh-region-ashanti')) {
  print(page.data.map((place) => place.name));
}
```

Call `GhanaGeoClient(apiKey: value)` only for attributed usage. Set
`telemetry: false` to disable metadata-only telemetry. API compatibility
(`GhanaGeoClient.apiVersion`), tested dataset compatibility
(`testedDatasetVersion`), the package version, and offline dataset version are
deliberately separate.

This core package has no Flutter SDK dependency and works in Dart CLI, server,
desktop and web consumers. Flutter apps can add the separately publishable
`ghanageo_flutter` companion under `packages/ghanageo_flutter`; its purpose-built
picker debounces search, rejects stale responses, supports keyboard selection,
and exposes live semantic status.

`offlineRegions` contains the 16 regions for startup and offline selectors. It
does not silently replace live district/place data.

## Verification

```sh
dart pub get
dart run tool/verify.dart
dart run tool/run_conformance.dart
```

The conformance runner refuses to pass without a live fixture URL and reports
real wire outcomes in the shared schema-v2 report, with request/response
evidence digests (including canonical response-body hashes) and
quota/shape/error checks. Registry publication is intentionally separate from
the dry run and requires the release environment's pub.dev credentials.
The verifier checks Dart 3.13.2 and installs an isolated staged publish payload
into a clean consumer rather than linking the source checkout.
