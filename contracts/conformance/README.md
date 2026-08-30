# GhanaGeo SDK conformance contract

This directory is the language-neutral behavior contract for every GhanaGeo
SDK. SDK runners consume the YAML case files and emit reports that conform to
`report-schema.json`; they must not reach into service storage or application
internals.

`operations.generated.yaml` is derived deterministically from the public
Geography, Spatial and Datasets operations in `contracts/openapi/v1.yaml` and
their matching RPCs in `proto/ghanageo/v1/geography.proto`. Curated cases under
`cases/` add stable fixture inputs, semantic expectations, quota costs and the
complete public error catalog. Generated content never overwrites curated
cases.

Run the CI-ready check from the repository root:

```sh
sh tools/conformance/check.sh
```

Regenerate the operation inventory after an intentional contract change:

```sh
ruby tools/conformance/generate.rb
ruby tools/conformance/validate.rb
```

The validator fails when generated inventory drifts, a case is malformed, a
required operation or Appendix B error lacks coverage, a required case is
skipped in a supplied report, or a fixture contains a key-shaped secret.

`tools/conformance/runner-kit.ts` is the thin TypeScript runner contract.
`fixture-server.ts` binds an ephemeral loopback port and `fixture-smoke.ts`
checks runner plumbing twice. This is explicitly a **non-conformance smoke**: it
cannot certify an SDK. The reference runner at
`packages/client/tests/conformance.runner.ts` separately calls actual
`GhanaGeoClient` public methods over the real REST paths and contract GraphQL
selections. Missing facade methods become named `GEO-24.TS` failures rather than
falling back to a synthetic execute endpoint.

To validate an SDK report as well:

```sh
ruby tools/conformance/validate.rb --report path/to/report.json
```

Reports are made byte-stable before CI artifact upload with:

```sh
ruby tools/conformance/normalize_report.rb path/to/report.json > normalized-report.json
```

Normalization redacts API-key and bearer-token shapes from diagnostic messages.
The validator rejects unredacted messages, unknown/duplicate case IDs, and any
required result that is missing, failed or skipped.

The report's contract, runner and evidence digests provide reproducible
**integrity**, not authenticity. Anyone who controls a report can recompute its
digests; the format does not pretend otherwise and is not self-signed. CI
therefore generates the TypeScript report directly from a clean checkout and
uploads that output only as evidence. It never treats a caller-uploaded report
as a trusted gate. Authentic release attestations belong to managed CI identity
and registry provenance, not an embedded repository key.
