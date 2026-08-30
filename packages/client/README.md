# @ghanageo/client

Framework-neutral REST and GraphQL access for GhanaGeo. The client works anonymously by default and forwards `AbortSignal` to every request.

```ts
import { GhanaGeoClient } from "@ghanageo/client";

const client = new GhanaGeoClient();
const results = await client.search("Tema Community 25");
```

The client exposes `apiVersion` and `testedDatasetVersion`, accepts an
`AbortSignal` on every operation, retries safe transient reads with bounded
backoff, and supports lazy cursor pagination:

```ts
for await (const page of client.placePages({ regionId: "gh-region-ashanti", limit: 50 })) {
  console.log(page.data);
}
```

Pass `retry: false` to disable automatic retries or `telemetry: false` to
explicitly disable metadata-only telemetry. Neither feature records request
bodies or authorization headers.

JSON success and error bodies are streamed under `maxJsonBytes`, which defaults
to 5 MiB and is capped at 64 MiB. Dataset artifacts use
`maxArtifactBytes`, which defaults to 100 MiB and is capped at 1 GiB. If a
declared `Content-Length` exceeds the configured limit, the client rejects the
response before reading its body. If an undeclared or inaccurate body crosses
the limit while streaming, the client cancels the reader and throws a
`GhanaGeoError` with code `RESPONSE_TOO_LARGE`.

Buffered downloads remain available and can verify an expected hexadecimal
SHA-256 digest:

```ts
const bytes = await client.downloadDatasetArtifact("2026.08.3-ulid", "places", "json", {
  expectedSha256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
});
```

For large artifacts, `downloadDatasetArtifactTo` writes bounded chunks to a
`WritableStream` or sink object. When `expectedSha256` is supplied, the client
verifies it after streaming and aborts the sink on mismatch. Browser and
generic sinks are not atomic, so bytes may already have been written when a
limit, cancellation, sink, or checksum error occurs. Use a temporary
destination and an environment-specific atomic rename when publication must be
all-or-nothing.
