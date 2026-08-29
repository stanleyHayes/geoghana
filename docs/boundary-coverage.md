# District boundary coverage

**248 of 261 districts have boundary geometry. All 16 regions do.**

This is not a bug in the matcher and it is not fixable by improving name
matching. It is a real gap between Ghana's current administrative structure
and the best openly-licensed boundary data available, and this page records
exactly which districts are affected and why, so nobody re-investigates it
from scratch.

## The source

geoBoundaries `gbOpen/GHA/ADM2`, CC BY 4.0, representing **2019**. 260
features. Ghana currently has **261** MMDAs.

The year is the whole story. Ghana reorganised its districts after this data
was published: districts were created, split and renamed. Matching 2026
records against 2019 shapes therefore leaves genuine residue.

## The 13 districts without geometry

### Absent from the source entirely (6)

No shape exists to attach. Nothing can fix these except newer data.

| District | Region | Note |
|---|---|---|
| Guan | Oti | Inaugurated **8 October 2021**, two years after the source |
| Dormaa Central | Bono | Not present under any name |
| Assin Central Municipal | Central | Not present under any name |
| Akuapim North Municipal | Eastern | Not present under any name |
| Akuapim South | Eastern | Not present under any name |
| Akyemansa | Eastern | Not present under any name |

### Split or merged since 2019 — deliberately not auto-resolved (3)

The source shape does not correspond one-to-one with our record. Attaching
either half would make the other silently wrong.

| District | Source shapes | Why not matched |
|---|---|---|
| Atwima Nwabiagya Municipal | `Atwima Nwabiagya North`, `Atwima Nwabiagya South` | Two plausible candidates. Spec §4.2 says never resolve this automatically |
| Sekyere Afram Plains | `Sekyere Afram Plains North` | Only the northern half exists in the source |
| Awutu Senya West | `Awutu Senya` | The source predates the East/West split |

### Renamed — ranked correctly, held for review (4)

The right shape is identifiable and ranks first, but scores below the
auto-apply threshold. Rule **R8**: a source does not overwrite canonical data
merely because it arrived. A steward applies these.

| District | Source shape | Score |
|---|---|---|
| Akrofuom | `Adansi Akrofuom` | 0.53 |
| Lower Manya Krobo Municipal | `Lower Manya` | below threshold |
| Upper Manya Krobo | `Upper Manya` | below threshold |
| Bolgatanga East | — | see below |

## Why matching is gated on region

`Bolgatanga East` is in Upper East. The closest national name match in the
source is **`Ga East`**, in Greater Accra — about 700km away, differing by one
word. A national name comparison can reach it.

Candidates are therefore filtered to the region the boundary's own shape sits
inside before any name comparison happens. The guard is geometric, not
textual, so a cross-region match is structurally impossible rather than merely
improbable. `InRegion` in `internal/app/ingest/boundaries.go`.

## What this affects

- **Reverse geocoding** falls back to nearest-locality proximity in these 13
  districts rather than polygon containment.
- **`/boundaries/{id}`** returns a documented "no geometry yet" error naming
  the record, not a 404 — the district exists, its shape does not.
- **Bulk GeoJSON downloads** carry `"geometry": null` for these features,
  which is valid GeoJSON and honest. Inventing a shape would be worse.

## Closing the gap

Needs a source newer than 2019 — the Ghana Statistical Service ADM2 release
(GEO-4.6) or an OpenStreetMap extract (GEO-4.7). Until one is ingested, 248 is
the correct number and `ghanageo-admin data validate` asserts it.
