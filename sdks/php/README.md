# GhanaGeo PHP SDK

Official, strict PHP 8.2+ client for GhanaGeo. The SDK is anonymous by default,
implements every public REST operation in the V1 OpenAPI contract, and exposes
immutable DTOs with the contract's exact camel-case fields.

```bash
composer require ghanageo/ghanageo-php
```

```php
use GhanaGeo\Client;
use GuzzleHttp\Client as GuzzleClient;
use GuzzleHttp\Psr7\HttpFactory;

$factory = new HttpFactory();
$ghanaGeo = new Client(new GuzzleClient(['http_errors' => false]), $factory);

foreach ($ghanaGeo->allRegions() as $region) {
    echo $region->name;
}
```

The core depends on PSR-18 and PSR-17 abstractions, not Guzzle. Inject any
compatible client and request factory. The included `GuzzleTransport` adds
explicit per-request deadlines through `RequestOptions`; a plain PSR-18 client
cannot portably receive timeout or cancellation metadata. An API key is optional; pass `apiKey:`
only for attributed access. Telemetry is disabled unless a callback is passed,
and telemetry never receives URLs, headers, request bodies, or keys.

## Operations

`regions`, `region`, `regionDistricts`, `districts`, `district`,
`districtPlaces`, `places`, `place`, `search`, `autocomplete`, `geocode`,
`reverseGeocode`, `nearby`, `boundary`, `datasets`, `datasetDownloads`,
`downloadDatasetArtifact`, `roads`, and `pointsOfInterest` map directly to the
20 public REST operations. `allRegions`, `allDistricts`, `allPlaces`, and
`pages` are lazy generators that reject repeated cursors.

API failures throw `GhanaGeoException` with stable `errorCode`, `statusCode`,
`requestId`, `docs`, and `details`. Safe GETs use bounded exponential backoff.
Response, error, and download bodies are bounded. Dataset downloads optionally
verify SHA-256 and use a same-directory temporary file plus atomic rename.

PSR-18 deliberately has no portable cancellation token. `GuzzleTransport`
implements the SDK's deadline-aware extension, so `search(..., options: new
RequestOptions(timeoutSeconds: 0.5))` terminates through Guzzle's explicit
timeout option. Other transports may implement `DeadlineAwareClientInterface`;
plain PSR-18 clients ignore SDK timeout options and must be configured natively.
This is a deadline, not a claim of portable cooperative cancellation.

When a download destination is supplied, bytes stream into a same-directory
temporary file while size and SHA-256 are checked, followed by atomic rename;
the method returns an empty string. Without a destination it returns the bounded
bytes in memory.

## Laravel

Laravel package discovery registers `GhanaGeoServiceProvider` and the
`GhanaGeo` facade. Publish `ghanageo.php` with the `ghanageo-config` tag. The
provider reads `GHANAGEO_BASE_URL` and optional `GHANAGEO_API_KEY`; no key is
embedded in the package. It reuses existing PSR-18/PSR-17 container bindings.
When Guzzle is installed it supplies safe defaults; otherwise resolving the
client raises a clear instruction to bind those interfaces.

## Compatibility

- SDK: `2.0.0-alpha.1`
- API: `v1`
- Tested dataset: `2026.08.3-ulid`
- PHP: current supported branches `8.2` and newer
- Transport: REST (PSR-18/PSR-7)

Run `composer verify` from a GhanaGeo monorepo checkout. Registry publication is
intentionally separate from the Packagist metadata/archive dry run.
