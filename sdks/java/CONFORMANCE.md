# Java conformance

- Runner: `dev.ghanageo.conformance.ConformanceRunner`
- Claimed protocols: REST
- Contract runner version: `1.0.0`
- SDK/API versions: `2.0.0-SNAPSHOT` / `v1`
- Evidence: captured and redacted `java.net.http.HttpClient` wire requests and
  responses, with SHA-256 digests in the schema-v2 report
- Expected applicable cases: 41 passed, 0 failed, 0 skipped

The gRPC-only stream and GraphQL-only complexity case are correctly outside
the Java REST artifact's protocol claim. Publication is not conformance.
