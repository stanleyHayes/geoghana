# TypeScript SDK release

The six scoped V1 SDKs are published together from a `vX.Y.Z` tag. The same
tag also triggers the separate CLI workflow, which publishes the `ghanageo`
umbrella/CLI package and native binaries.

## Before tagging

1. Run `./scripts/verify-sdk-release.sh X.Y.Z` with package manifests already
   stamped to that version.
2. Confirm the quality and security workflows are green for the exact commit.
3. Confirm the repository has an npm trusted publisher or an `NPM_TOKEN` with
   publish access to the `@ghanageo` organization.
4. Confirm `DATASET_VERSION` and the offline dataset match the release tested
   by the API.

## Release

Push the signed `vX.Y.Z` tag. `sdk-release.yml` regenerates, builds, tests and
packs every package before publishing in dependency order with npm provenance.
The CLI workflow publishes `ghanageo` separately from the same tag.

For a no-publish rehearsal, run the TypeScript SDK workflow manually. It
uploads the exact verified tarballs as a workflow artifact and never reads the
npm token.

## Verification and rollback

Verify every package with `npm view <name>@X.Y.Z dist.integrity version`. npm
versions are immutable: if a release is bad, deprecate it with an explanatory
message, fix forward under a new patch version, and never reuse the tag.
