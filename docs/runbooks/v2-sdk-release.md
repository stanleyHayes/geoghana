# V2 SDK coordinated release

`v2-sdk-release.yml` is the single release entry point for GhanaGeo V2 SDKs. A tag or a manual dispatch builds all seven npm artifacts plus Python, Go, Dart, Flutter, Java, .NET, and PHP distribution archives. A `v2.X.Y` tag is release intent: after quality and protected-environment gates it publishes every SDK. Manual dispatch defaults to `publish=false` and is the no-mutation rehearsal path.

Run the same rehearsal locally with:

```sh
./scripts/rehearse-v2-sdk-release.sh 2.0.0-rc.1
```

The immutable output directory is `release/v2/<version>/`. Reusing a version fails instead of replacing evidence. `compatibility-manifest.json` records the coordinated release version, each independently versioned SDK/package, exact install command, commit, API target, tested dataset, aggregate contract and conformance digests, every artifact SHA-256, and provenance/signing state. Install documentation is therefore generated from the same package metadata as the artifacts instead of carrying hand-maintained versions. CI also emits a GitHub build-provenance attestation. Rehearsal artifacts are intentionally unsigned and say so explicitly.

## Publishing gate

Publishing requires a V2 tag or a manual dispatch with `publish=true`, plus approval of the single protected `sdk-release` environment. One sequential job owns every mutation and one secrets context. Before any registry mutation, it downloads the rehearsal artifact, verifies its GitHub build-provenance attestation is signed by `.github/workflows/v2-sdk-release.yml`, verifies the commit/version and every manifest checksum, probes all seven npm versions, PyPI, both pub.dev packages, both Maven coordinates, NuGet, Packagist, Go/PHP tags and repository access, and validates every credential/namespace gate. GitHub repository permissions are probed without mutation. OIDC registries cannot authenticate until their publish exchange, so their protected namespace flags are release-owner readiness assertions, not credential-verification claims.

| Registry | Required external setup | Intended authentication/signing |
|---|---|---|
| npm | `@ghanageo` ownership for six scoped packages and `ghanageo` | One trusted-publisher sequence publishes scoped packages, waits for registry visibility, then publishes the umbrella from the same attested bundle. `cli-release.yml` is manual rehearsal only. |
| PyPI | `ghanageo` project ownership | PyPI trusted publisher and `pypa/gh-action-pypi-publish`; no long-lived token |
| Go | `github.com/ghanageo/ghanageo-go` repository and tag ownership | signed Git tag; public Go proxy indexes the module |
| pub.dev | `ghanageo` and `ghanageo_flutter` package ownership | pub.dev automated publishing with GitHub OIDC |
| Maven Central | verified `dev.ghanageo` namespace | Central Portal credential plus an offline/CI GPG signing key; staged profile uses `autoPublish=true` and `waitUntil=published`, followed by a public Central visibility poll |
| NuGet | `GhanaGeo` package ownership | NuGet trusted publishing/OIDC where available, otherwise a protected short-lived API key; repository signature policy required |
| Packagist | `ghanageo/ghanageo-php` ownership | protected Packagist token or GitHub service hook; signed Git tag is the source artifact |

No credentials belong in repository variables, generated manifests, archives, logs, or workflow artifacts. Registry publication must consume the exact downloaded rehearsal artifact and verify its manifest SHA-256 first; it must never rebuild.

## Contract regeneration

`sdk-contract-regeneration.yml` runs on matching changes to `main` or manual dispatch. It creates a run-specific branch, regenerates the language-neutral inventory, Go protobufs and every SDK contract lock, verifies them, commits only when a diff exists, pushes with `GITHUB_TOKEN`, and opens a PR. Contract drift is a reviewed source change, never an implicit release-time rewrite.

The regeneration workflow always opens the monorepo PR and then iterates the six slugs in `SDK_EXTERNAL_REPOSITORIES` (`python`, `go`, `dart`, `java`, `dotnet`, `php`). `SDK_EXTERNAL_REPOSITORIES_TOKEN` must have contents and pull-request access to each repository; the workflow exports each regenerated SDK root, creates a run-specific branch, pushes it and opens a matching PR. Python now has a generated and verified `contract.lock.json` like the other SDKs.

Release publication uses `SDK_RELEASE_GITHUB_TOKEN` plus `SOURCE_GIT_SIGNING_KEY` to replace the Go and PHP external repository trees with the exact verified source archives, create signed annotated SDK-version tags, push, and compare the remote tag's tree digest with the exported tree. Repository identities are immutable manifest/code constants: `github.com/ghanageo/ghanageo-go` and `github.com/ghanageo/ghanageo-php`; operators configure credentials, not mutable destination slugs. Source mutations occur late, after binary/package registries, and only then dispatch Go release automation or refresh Packagist.

After each artifact becomes publicly visible, the workflow records ecosystem evidence in `release-ledger.json` and replaces `release-ledger-<version>`. npm has seven independent checkpoints containing registry SHA-1 and SHA-512 integrity; reruns verify already-published tarballs and continue with missing packages. PyPI records wheel/sdist SHA-256, NuGet re-downloads and checks the nupkg SHA-256, Maven records POM/JAR/source/Javadoc hashes and requires signatures while comparing staged binaries, and Go records the verified source tree plus proxy metadata/archive hashes. PHP source, Go source, Go dispatch and Packagist are separate checkpoints; existing signed tags are accepted only after their remote tree matches the attested export. Packagist evidence binds its source reference to that tag commit.

pub.dev exposes version metadata but no immutable package-archive digest. The ledger therefore hashes the canonical version API response and records `archiveDigestAvailable=false`; the attested release archive, compatibility manifest, ordered core/Flutter publication, and exact version metadata remain the provenance boundary. GitHub release creation is independently checkpointed and idempotent: an existing asset is downloaded and its SHA-256 must match the attested umbrella tarball. It remains the final step after every registry confirmation.

## Failure handling

- A failed verifier produces no final release directory.
- An existing version is immutable; fix forward under a new version.
- A stable coordinated version rejects any SDK whose own package/runtime metadata is a prerelease or `SNAPSHOT`; prerelease coordinated rehearsals may contain prerelease SDKs. Package metadata and install-time dependency versions are checked against the staged artifacts.
- Java is a stricter exception to source snapshots: the rehearsal copies the Maven reactor into a temporary directory, replaces every parent/runtime `2.0.0-SNAPSHOT` marker with the explicit coordinated version, rebuilds there, and rejects any staged `SNAPSHOT`. The source POMs and `Version.java` are hashed before and after staging and are never mutated. The manifest and Maven install command are read back from that staged archive.
- A missing registry owner, protected environment, OIDC trust, signing identity, or live conformance endpoint is an external blocker. Record it and continue validating other registries.
- V2 publishing owns all seven npm artifacts in strict dependency order. `cli-release.yml` has no tag trigger or publish steps and remains a manual build/rehearsal utility, eliminating the prior umbrella race.
- Dart publishing is strictly serialized: `ghanageo` core publishes first, pub.dev must report the exact core version, and only then may `ghanageo_flutter` publish.
- Every third-party action in the V2 release, protected publisher, regeneration, and CLI rehearsal workflows is pinned to an immutable commit SHA; the adjacent comment records the reviewed major line.
