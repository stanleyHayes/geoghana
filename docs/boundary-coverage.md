# District boundary coverage

**261 of 261 districts have boundary geometry. All 16 regions do.**

This page previously recorded a 13-district gap and argued it could not be
closed without an official Ghana Statistical Service release. That was half
right: geoBoundaries genuinely cannot supply those districts, but
OpenStreetMap can, and it did.

## Two sources, in order

| Source | Licence | Represents | Supplies |
|---|---|---|---|
| geoBoundaries `gbOpen/GHA/ADM2` | CC BY 4.0 | **2019** | 248 districts, 16 regions |
| OpenStreetMap `admin_level=6` | ODbL 1.0 | current | the remaining 13 |

geoBoundaries is used first because it is a curated administrative release.
OSM fills only what is missing — a district that already has a boundary is
never overwritten, because the existing one has been through the region-gated
matcher and possibly a steward, and a second source arriving is not grounds to
replace it (rule R8).

## Why OSM had what geoBoundaries did not

The source year is the whole story. Ghana reorganised its districts after 2019:

- **Guan District** was inaugurated on **8 October 2021**, two years after the
  geoBoundaries release. No amount of matching could have found it there.
- **Atwima Nwabiagya** appears in the 2019 data split as North and South;
  our record is the un-split Municipal, so neither half could be attached
  without making the other silently wrong.
- Several districts were renamed (`Adansi Akrofuom` → `Akrofuom`,
  `Lower Manya` → `Lower Manya Krobo Municipal`).

All 13 matched OSM at an exact score of 1.00 once names were compared with
administrative suffixes normalised on both sides.

## Region gating, and a bug it hid

Boundary names are only compared within the region the shape sits inside. A
national comparison lets `Bolgatanga East` (Upper East) score against
`Ga East` (Greater Accra) — two districts about 700km apart whose names differ
by one word.

The first implementation resolved that region with `$geoIntersects` against
the district's **own polygon**. A district boundary shares edges with its
neighbours, so the query matched several regions and returned an arbitrary
one; two districts were gated into the wrong region and then matched nothing
there. Containment now uses an **area-weighted interior point**, which belongs
to exactly one region. That took the fill from 11 of 13 to 13 of 13.

## Downstream effects

With complete coverage:

- **Reverse geocoding** resolves by polygon containment everywhere, rather
  than falling back to nearest-locality proximity in 13 districts.
- **99.6% of points of interest** (19,601 of 19,686) and **99.5% of roads**
  (19,627 of 19,728) resolve to a district. The remainder sit just outside
  every polygon — offshore, or across a land border — and are left unassigned
  rather than snapped to the nearest district.
- **Bulk downloads** carry geometry for every district.

## Attribution

Both licences require it, and ODbL is share-alike, so attribution is carried
on the records themselves rather than in a footer: the domain validators
refuse an unattributed row, the Mongo schema requires one, every API response
includes the notice, and every export file carries it at file and feature
level.

Rebuild coverage with:

```bash
ghanageo-admin data boundaries --level ADM2 --file <geoBoundaries.geojson> --apply
ghanageo-admin data osm-boundaries --file <ghana-latest.osm.pbf> --apply
```
