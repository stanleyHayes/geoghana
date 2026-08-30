# V2 SDK matrix evidence — 2026-08-30

## Result

The strict aggregate validator accepted the complete 13-report matrix:

```text
complete SDK matrix valid: 13 reports, 109 distinct case/protocol evidence pairs
```

Every report used contract digest
`0b19aa446e1b744ea966a20f637f0e391cb84bfbcc4667666fb3644f34fdaf04`
and observed dataset version `2026.08.3-ulid`. All required results passed; no
report contained a failed or skipped result.

| Retained report | Runtime | Protocol | Passed | Report SHA-256 |
|---|---|---:|---:|---|
| [TypeScript](v2-sdk-matrix-reports/typescript.json) | Node 26.5.0 | REST + GraphQL | 42 | `5f33f2c20507e2fcaa7cf1d98845dee68037af8f3dabf8a1ef9843dbc3d12b47` |
| [Go REST](v2-sdk-matrix-reports/go-rest.json) | Go 1.26.6 | REST | 41 | `6f72e77ae1819e6312d743191a837a11cd118fd176373c9e72e2fcd2585b798e` |
| [Go protobuf](v2-sdk-matrix-reports/go-grpc.json) | Go 1.26.6 | gRPC | 33 | `a470295862abb5a68035648b2dddaf835bbaf44f46f0de6bcd5293d6cfe2ec08` |
| [Python 3.12](v2-sdk-matrix-reports/python-3.12.json) | 3.12.13 | REST | 41 | `35f7f7a6d15a9db7faba729c38c1cdc34851106166625c6058de836fd3a2fd3c` |
| [Python 3.13](v2-sdk-matrix-reports/python-3.13.json) | 3.13.14 | REST | 41 | `61771f1736709da32ae8fd06420b75d40da61cfc4f32d9da7bc0a7ede67a5ccd` |
| [Python 3.14](v2-sdk-matrix-reports/python-3.14.json) | 3.14.6 | REST | 41 | `9b3d20c6c9b6e71ef6f6573cdf2cfa3f3e2e5dc0c54800e2cb399f51a63f943d` |
| [Dart](v2-sdk-matrix-reports/dart.json) | 3.13.2 | REST | 41 | `e3e7f49085e21112b9237378193a8aaa2d1a7d82a64a6d01bf96f95ad474bbc6` |
| [Java](v2-sdk-matrix-reports/java.json) | Java source/target 21 | REST | 41 | `461cc7457aca5ec67418141937a4a9f904ddf4f3d79d0b106301137a8f7dcea5` |
| [.NET](v2-sdk-matrix-reports/dotnet.json) | SDK 8.0.403 | REST | 41 | `863dc4a197f50f3fc8d8018e2baf2b5c9732fd063f05a0f8960468cf0a2bdd0c` |
| [PHP 8.2](v2-sdk-matrix-reports/php-8.2.json) | 8.2 | REST | 41 | `794f20a8fb6eca4cd77c81fe3e1ca50f838a51334f465c500970f675c43eab63` |
| [PHP 8.3](v2-sdk-matrix-reports/php-8.3.json) | 8.3 | REST | 41 | `794f20a8fb6eca4cd77c81fe3e1ca50f838a51334f465c500970f675c43eab63` |
| [PHP 8.4](v2-sdk-matrix-reports/php-8.4.json) | 8.4 | REST | 41 | `794f20a8fb6eca4cd77c81fe3e1ca50f838a51334f465c500970f675c43eab63` |
| [PHP 8.5](v2-sdk-matrix-reports/php-8.5.json) | 8.5.0 | REST | 41 | `794f20a8fb6eca4cd77c81fe3e1ca50f838a51334f465c500970f675c43eab63` |

Identical PHP report hashes are expected: the four runtimes observed identical
canonical wire requests, responses, and normalized evidence.

## Toolchain provenance

- Flutter `3.47.2`, framework revision `d3b14c876900e553bc736ca19295fc09e3853e8e`, bundled Dart `3.13.2`.
  The official macOS arm64 archive was selected from Flutter's official release
  manifest and verified before extraction: SHA-256
  `f456fd6733053d9301828a2e702d6cbec872923126809aa8c48eb0a696d6cc01`.
- Python runtimes were installed into a task-local directory by uv `0.11.28`;
  no global interpreter was changed.
- PHP 8.2–8.4 ran in isolated images derived from official immutable bases:
  `php@sha256:6ca4b01d84082465358c5d541a2bef0edd9d0be494802aeb40f2d7c7a8d73adb`,
  `php@sha256:177529735599a8244b2c903522f029839dce1c2ac4be122fdc00ada4b45a20e4`, and
  `php@sha256:9cc9310a457019cd6b682109eb3c5dd8bf73498e7d3b9ee5c33d0d0b83d0faf3`.
  PHP 8.5 used the installed 8.5.0 runtime. Composer `2.10.3` was downloaded
  task-locally after verifying the official installer signature; its PHAR
  SHA-256 was `7a2d379d5b8ffdaa028580ef26494c36d2feef4b178d3dd1473a4dbc5e17c8d6`.
- Other tools: pnpm `11.11.0`, Docker Engine `28.0.1`.

## Verification commands

The language release gates, deterministic live fixture runners, and strict
per-report validators passed for all report-producing SDKs. The Dart core gate
was rerun after its conformance lock was synchronized and passed completely.

`make sdk-matrix-verify REPORT_DIR="$PWD/docs/runbooks/evidence/v2-sdk-matrix-reports"`
passed all SDK release gates and regenerated/validated the retained matrix.
`make docs-sdk-check` passed the full nine-source drift check plus TypeScript,
React, Python, Go, Dart, Java, .NET, PHP, curl, web production build, link
reachability, and secret-hygiene checks.

The canonical Flutter companion check also passed: contract and toolchain
checks, formatting, analysis, five widget tests, the isolated staged consumer,
and publication dry-run. The dry-run emitted only its explicitly accepted
local-core override hint (`0 warnings and 1 hint`).

The retained directory is 256 KiB. The canonical secret scanner passed, and a
separate scan confirmed that no report records a localhost URL, temporary path,
or fixture address.
