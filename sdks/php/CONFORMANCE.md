# PHP conformance

The runner calls the public `GhanaGeo\Client` facade over actual REST paths. It
does not call the fixture server's synthetic execute endpoint. Each request is
tagged with its case ID, mapped through the SDK method, and recorded as normalized
request/response digest evidence in the shared report schema.

```bash
ruby tools/conformance/export_cases.rb > sdks/php/.cases.json
php sdks/php/conformance/runner.php sdks/php/.cases.json \
  "$GHANAGEO_CONFORMANCE_URL" sdks/php/php-conformance-report.json
ruby tools/conformance/validate.rb --report sdks/php/php-conformance-report.json
```

The cancellation case uses a deliberately delayed response and an explicit 10
ms `RequestOptions` deadline passed through `DeadlineAwareClientInterface` to
`GuzzleTransport`. A generic client timeout does not count as evidence. This
proves the advertised Guzzle adapter behavior without pretending PSR-18 defines
a universal cancellation API. Evidence digests come from captured, sanitized
PSR request paths, query, header names and body plus actual response status,
content type and body—not from intended case inputs.
Only REST cases are claimed in `supportedProtocols`; gRPC/GraphQL-only cases are
outside the PHP SDK's advertised protocol surface.
