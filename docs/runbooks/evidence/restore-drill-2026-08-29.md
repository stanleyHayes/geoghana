# Restoration drill evidence — 2026-08-29

- Environment: local MongoDB 8.0 replica-set container
- Source database: `ghanageo`
- Restore target: isolated `ghanageo_restore_drill_20260829T154355Z-86080`
- Archive size: 33,082,144 bytes
- Archive SHA-256: `c6f36bba811ac28c13ee5cd24da4c90f7f592d79e71f2b9ded4ad9425df3cf60`
- Outcome: passed; scratch database removed after verification

## Collection comparison

| Collection | Source | Restored |
|---|---:|---:|
| `api_keys` | 27 | 27 |
| `audit_log` | 3 | 3 |
| `dataset_versions` | 1 | 1 |
| `districts` | 261 | 261 |
| `place_redirects` | 17 | 17 |
| `places` | 15,941 | 15,941 |
| `regions` | 16 | 16 |

All seven restored collections retained JSON Schema validators. A follow-up
database listing confirmed that no `ghanageo_restore_drill_*` database remained.

## Scope and remaining production gate

This proves the archive, namespace-remapping, count comparison, validator
preservation and cleanup mechanics against the real local dataset. It does not
prove MongoDB Atlas point-in-time recovery or production RPO/RTO. GEO-22.1 and
the production portion of GEO-22.2 remain open until an Atlas project and
scratch cluster are available and the same evidence is captured there.
