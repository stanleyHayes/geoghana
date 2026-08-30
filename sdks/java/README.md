# GhanaGeo Java SDK

`dev.ghanageo:ghanageo-java` is the Java 21+ REST client for GhanaGeo. It is
anonymous by default: an API key and local telemetry callback are both opt-in.
The same immutable client exposes strongly typed OpenAPI records through blocking methods and cancellation-aware
`CompletableFuture` methods. A `CancellationToken` propagates through HTTP,
scheduled backoff and streamed downloads. Safe GETs use capped exponential
jitter and honor numeric or HTTP-date `Retry-After`; JSON, streamed
dataset artifacts, and retry delays all have hard configurable ceilings.

```java
try (var ghana = GhanaGeoClient.create()) {
  Models.SearchPage results = ghana.search("Kumasi", Map.of("limit", 5));
  results.data().forEach(hit -> System.out.println(hit.name()));
}
```

Spring Boot applications can add `dev.ghanageo:ghanageo-spring-boot-starter`
and configure `ghanageo.base-url`, `ghanageo.api-key`, and the safety limits.
An application-defined `GhanaGeoClient` bean always wins.

## Verification

Both entry points are version-pinned and checksum verified:

```sh
./mvnw clean verify
./gradlew clean test build
```

To produce real wire evidence through the shared fixture, start
`tools/conformance/fixture-server.ts`, export the cases, then run:

```sh
ruby tools/conformance/export_cases.rb > /tmp/java-cases.json
sdks/java/mvnw install -DskipTests
sdks/java/mvnw -pl conformance exec:java \
  -Dexec.mainClass=dev.ghanageo.conformance.ConformanceRunner \
  -Dexec.args="/tmp/java-cases.json http://127.0.0.1:PORT/v1 /tmp/java-report.json"
ruby tools/conformance/validate.rb --report /tmp/java-report.json
```

The runner sets the fixture case header and invokes only the public typed SDK
facade—there is no synthetic execute endpoint. It covers all 41 REST-applicable
required cases and binds captured, redacted wire requests and responses to
SHA-256 evidence while checking shape, status, error details, quota, dataset,
anonymous auth, cursor, cancellation, orthography, and outside-Ghana semantics.

## Publishing

The Maven model includes Central coordinates, project metadata, SCM, license,
stable strict Javadocs, sources, GPG signing and the Central publishing plugin.
The verifier's file-backed `deploy` is only a credential-free repository-layout
dry run; it does not claim Central acceptance. Actual Central deployment requires
the GhanaGeo namespace token, signing key, and managed-CI provenance.
