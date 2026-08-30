# .NET conformance

`conformance/GhanaGeo.Conformance.csproj` invokes the public `GhanaGeoClient`
facade against the shared live fixture. It exports the canonical cases through
`tools/conformance/export_cases.rb`, selects REST cases, sends
`x-conformance-case`, checks expected outcomes, statuses, required response and
typed-error fields, anonymous behavior, cancellation, and dataset-version
presence, and emits schema-version 2 evidence with request/response digests.

The runner never reports an unexecuted case as passed. Network failures,
contract-export failures, facade mapping errors, response mismatches, and
cancellation failures produce a non-zero exit. The report contains digests,
not authorization values or response bodies.

Run from `sdks/dotnet` while the deterministic fixture is listening:

```bash
dotnet run --project conformance/GhanaGeo.Conformance.csproj -c Release -- \
  --root ../.. --base-url http://localhost:8180/v1/ \
  --report artifacts/conformance-report.json
```
