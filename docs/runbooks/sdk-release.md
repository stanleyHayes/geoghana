# TypeScript and CLI rehearsal ownership

`sdk-release.yml` and `cli-release.yml` are manual, non-publishing verification utilities. They build the scoped TypeScript packages and native CLI/umbrella artifacts respectively, upload dry-run evidence, and have no tag trigger or registry mutation step.

`v2-sdk-release.yml` is the only tag-driven SDK release owner. Its attested bundle contains all seven npm artifacts. The protected sequential publisher publishes the six scoped packages, waits until npm resolves every exact version, and then publishes the unscoped `ghanageo` umbrella. This removes the former independent-workflow race and prevents duplicate ownership.

Use manual dispatch on either legacy workflow for focused troubleshooting only. Use manual `v2-sdk-release.yml` with `publish=false` for a complete no-mutation rehearsal. See `v2-sdk-release.md` for the release gate, credentials, source exports and rollback policy.
