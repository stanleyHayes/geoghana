# Production restoration drill — 2026-08-29

**Result: passed.** A full backup of the production Atlas database was restored
into an isolated scratch database on the same cluster, verified collection by
collection, and cleaned up. Production was untouched throughout.

## Measured

| Step | Measure |
|---|---|
| Backup (`mongodump --archive`) | **136.5s** |
| Restore (`mongorestore --nsFrom/--nsTo`) | **346.3s** |
| **Recovery time (RTO), full dataset** | **~8 minutes** |
| Archive size | 78,128,444 bytes |
| Archive SHA-256 | `512af1f5d7ab38250eb7aa72040b6e1fdb013e9a2dcaf8fc41099fd6bb45e847` |
| Documents restored | **87,798**, 0 failed |

## Verified

All 18 collections matched source counts exactly:

| Collection | Documents |
|---|---|
| place_redirects | 32,161 |
| roads | 19,728 |
| pois | 19,686 |
| places | 15,941 |
| districts | 261 (**261 with boundary geometry**) |
| regions | 16 |
| dataset_versions | 5 |
| the remaining 11 | 0 — identity collections, empty by design |

Indexes restored intact (6/6 on `districts`, including both 2dsphere). Scratch
database dropped; `ghanageo`, `admin` and `local` remain, and production counts
were re-checked afterwards.

## A near-miss worth recording

The first restore attempt reported success in 4.6 seconds and wrote nothing.
The scratch database name — `ghanageo_restore_drill_<timestamp>` — was 39
bytes against MongoDB's 38-byte limit, and the failure did not surface through
the pipeline being used. Only counting the restored collections caught it.

A drill that is not verified document-by-document is not a drill. Scratch names
are now short by construction.

## RPO

**RPO equals the age of the last manual dump.** This cluster is Atlas **M0
(shared/free)**, which does not offer continuous backup or point-in-time
restore — those begin at M10. Until the cluster is upgraded, the recovery
point is whenever `mongodump` last ran, and there is no way to recover to an
arbitrary moment.

That is the sole reason GEO-22.1 remains open: it is a cluster-tier
constraint, not a gap in tooling or code.

## Reproduce

```bash
# Backup
mongodump --uri "$MONGO_URI" --db ghanageo --archive > backup.archive

# Restore into an isolated scratch database (name must be < 38 bytes)
mongorestore --uri "$MONGO_URI" --archive \
  --nsFrom 'ghanageo.*' --nsTo 'gg_drill_<stamp>.*' < backup.archive

# Verify every collection, then drop the scratch database.
```
