# Licensing register

Every source GhanaGeo ingests, and exactly what its licence permits.

**Rule R4:** a source's licence is recorded here *before* its adapter is
written. Attribution travels with the data — into API responses and bulk
downloads — rather than living only in this file.

| Source | Licence | Redistribute? | Status | Adapter |
|---|---|---|---|---|
| **GeoNames** | CC BY 4.0 | ✅ Yes, with attribution | **Ingested** — 15,925 populated places | `internal/adapters/ingest/geonames` |
| Ghana Statistical Service | Official government files; terms to confirm per dataset | ⚠️ Verify before publication | Not started (GEO-4.6) | — |
| OpenStreetMap (Geofabrik Ghana) | ODbL 1.0 | ✅ Yes, with attribution and share-alike | Not started (GEO-4.7) | — |
| Ghana National Household Registry | Government reference data; reuse terms unconfirmed | ⚠️ Staging only | Not started (GEO-4.5) | — |
| Community submissions | Contributor terms, evidence required | ✅ After steward review | Not started | — |
| **GhanaPostGPS** | **Proprietary — Ghana Post owns digital addresses** | ❌ **BLOCKED** | **Never. No licence.** | Port only, no implementation |

---

## GeoNames — CC BY 4.0

- **Licence:** <https://creativecommons.org/licenses/by/4.0/>
- **Source:** <https://download.geonames.org/export/dump/GH.zip>
- **Attribution (rendered with every derived record):**
  *Contains data from GeoNames (https://www.geonames.org), licensed CC BY 4.0.*

**What we take:** populated places only (feature class `P`) — name, alternate
names, coordinates, population and first-order administrative division.

**What we exclude and why:**
- `PPLQ` abandoned and `PPLW` destroyed places, which would pollute search
  with settlements that no longer exist.
- Feature codes with no clean mapping onto our place types. Guessing a type
  propagates into search ranking, so an unmappable record is skipped.
- Any place whose coordinate falls outside Ghana's bounding box. GeoNames is
  generally reliable, but a mis-signed longitude would put a town in the Gulf
  of Guinea and silently corrupt reverse geocoding.
- Any place whose GeoNames region code we cannot resolve to one of Ghana's 16
  regions. Filing a place under the wrong region is worse than not importing it.

**Reconciliation status:** GeoNames is a **reference** source, not an authority
on Ghanaian administrative geography. Records land as `REFERENCE` and are never
promoted to canonical automatically (rule R5). District assignment is
deliberately left empty: GeoNames carries second-order codes, but mapping them
onto Ghana's 261 MMDAs needs reconciliation against GNHR/GSS, which is GEO-4.6.

---

## GhanaPostGPS — blocked

GhanaPostGPS digital addresses are owned by Ghana Post and are **not licensed
to GhanaGeo**. They must never be scraped, stored or redistributed (rule R1).

A port exists at `internal/ports/ghanapost.go` with no implementation. It may
only be implemented after a signed licence or written integration agreement is
filed here and countersigned by the Engineering Lead. Even then, the licence
would permit *lookup*, never *redistribution*, and the CI exclusion test
(GEO-21.7) stays enabled permanently.
