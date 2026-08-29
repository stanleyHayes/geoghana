package ingest

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
	"github.com/ghanageo/ghanageo/services/api/internal/ports"
)

// Deduplication of places that different sources describe independently.
//
// The seed shipped 16 regional capitals; GeoNames then supplied its own record
// for each of the same 16 places. Both are correct, and neither is wrong to
// exist — but a caller searching "Kumasi" should get ONE answer, not two
// identical-looking ones, and a caller who stored gh-place-kumasi must not
// find it silently outranked by a record they have never seen.
//
// Spec §4.2 is explicit about the danger: two places may legitimately share a
// name in different districts, and Ghana has many — GeoNames alone carries
// four villages called Kumasi. Merging on name alone would destroy real places.
// A merge therefore requires name AND region AND compatible type, and anything
// short of that becomes a review item.

// MergeCandidate is one proposed merge, applied or not.
type MergeCandidate struct {
	SurvivorID   string
	SurvivorName string
	MergedID     string
	MergedName   string
	Region       string
	Reason       string
	// Enrichment the merged record contributes to the survivor.
	GainsCoordinate bool
	GainsAliases    int
	GainsDistrict   bool
}

type DedupeResult struct {
	Examined  int
	Merged    []MergeCandidate
	Ambiguous []string
	DryRun    bool
}

func (r DedupeResult) String() string {
	mode := "applied"
	if r.DryRun {
		mode = "dry run"
	}
	return fmt.Sprintf("%s: examined %d name groups · %d merges · %d ambiguous",
		mode, r.Examined, len(r.Merged), len(r.Ambiguous))
}

// PlaceGrouper finds places that share a normalized name.
type PlaceGrouper interface {
	GroupsByNormalizedName(ctx context.Context, minSize int) (map[string][]geography.Place, error)
}

// Deduper merges cross-source duplicates.
type Deduper struct {
	Places    ports.PlaceRepository
	Grouper   PlaceGrouper
	Redirects ports.RedirectRepository
}

// capitalLike types describe the same kind of settlement closely enough that a
// difference between them is a classification detail, not a different place.
var capitalLike = map[geography.PlaceType]bool{
	geography.PlaceRegionalCapital: true,
	geography.PlaceCity:            true,
	geography.PlaceTown:            true,
}

func compatibleTypes(a, b geography.PlaceType) bool {
	if a == b {
		return true
	}
	return capitalLike[a] && capitalLike[b]
}

// Run finds and optionally applies merges.
func (d *Deduper) Run(ctx context.Context, apply bool) (*DedupeResult, error) {
	res := &DedupeResult{DryRun: !apply}

	groups, err := d.Grouper.GroupsByNormalizedName(ctx, 2)
	if err != nil {
		return res, err
	}
	res.Examined = len(groups)

	names := make([]string, 0, len(groups))
	for n := range groups {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, name := range names {
		places := groups[name]

		// Partition by region: two places with the same name in DIFFERENT
		// regions are different places, and Spec §4.2 forbids merging them.
		byRegion := map[string][]geography.Place{}
		for _, p := range places {
			byRegion[p.RegionID] = append(byRegion[p.RegionID], p)
		}

		for regionID, sameRegion := range byRegion {
			if len(sameRegion) < 2 {
				continue
			}

			// Within one region, only merge records whose types are compatible.
			// Four villages called Kumasi in Ashanti are four villages.
			var mergeable []geography.Place
			for _, p := range sameRegion {
				if capitalLike[p.Type] {
					mergeable = append(mergeable, p)
				}
			}
			if len(mergeable) < 2 {
				continue
			}
			if len(mergeable) > 2 {
				res.Ambiguous = append(res.Ambiguous, fmt.Sprintf(
					"%q in region %s has %d capital-like records; a steward should decide",
					name, regionID, len(mergeable)))
				continue
			}

			a, b := mergeable[0], mergeable[1]
			if !compatibleTypes(a.Type, b.Type) {
				res.Ambiguous = append(res.Ambiguous, fmt.Sprintf(
					"%q in region %s: incompatible types %s and %s", name, regionID, a.Type, b.Type))
				continue
			}

			survivor, merged := chooseSurvivor(a, b)
			cand := MergeCandidate{
				SurvivorID: survivor.ID, SurvivorName: survivor.Name,
				MergedID: merged.ID, MergedName: merged.Name,
				Region: survivor.RegionName,
				Reason: "same name, same region, compatible type",
			}

			enriched := enrich(survivor, merged, &cand)

			if apply {
				if _, err := d.Places.Upsert(ctx, enriched); err != nil {
					return res, fmt.Errorf("enrich %s: %w", survivor.ID, err)
				}
				// The merged record is DEPRECATED, never deleted, and a
				// redirect is written so a caller holding the old id gets 410
				// with mergedInto rather than a bare 404 (Spec §18, rule R7).
				merged.Status = geography.StatusMerged
				if _, err := d.Places.Upsert(ctx, merged); err != nil {
					return res, fmt.Errorf("deprecate %s: %w", merged.ID, err)
				}
				if err := d.Redirects.Put(ctx, geography.Redirect{
					OldID:    merged.ID,
					NewID:    survivor.ID,
					Reason:   cand.Reason,
					MergedAt: time.Now().UTC().Format(time.RFC3339),
				}); err != nil {
					return res, fmt.Errorf("redirect %s: %w", merged.ID, err)
				}
			}
			res.Merged = append(res.Merged, cand)
		}
	}
	return res, nil
}

// chooseSurvivor decides which id callers keep using.
//
// The FIRST-PUBLISHED id wins, not the richest record. A consumer who stored
// gh-place-kumasi must keep resolving it; silently promoting a newer GeoNames
// id would break them for the sake of tidiness. The richer record's data is
// merged into the survivor instead, so nothing is lost.
func chooseSurvivor(a, b geography.Place) (survivor, merged geography.Place) {
	aSeed := a.Provenance.SourceID == "seed-bootstrap"
	bSeed := b.Provenance.SourceID == "seed-bootstrap"
	switch {
	case aSeed && !bSeed:
		return a, b
	case bSeed && !aSeed:
		return b, a
	}
	// Neither or both are seed records: prefer the one with a coordinate, then
	// the lexically smaller id so the outcome is deterministic across runs.
	if (a.Centroid != nil) != (b.Centroid != nil) {
		if a.Centroid != nil {
			return a, b
		}
		return b, a
	}
	if a.ID < b.ID {
		return a, b
	}
	return b, a
}

// enrich copies what the merged record knows and the survivor does not.
// It never overwrites a value the survivor already has.
func enrich(survivor, merged geography.Place, cand *MergeCandidate) geography.Place {
	out := survivor

	if out.Centroid == nil && merged.Centroid != nil {
		out.Centroid = merged.Centroid
		cand.GainsCoordinate = true
	}
	if out.DistrictID == "" && merged.DistrictID != "" {
		out.DistrictID = merged.DistrictID
		out.DistrictName = merged.DistrictName
		cand.GainsDistrict = true
	}
	if out.Population == nil && merged.Population != nil {
		out.Population = merged.Population
	}
	if merged.Type == geography.PlaceRegionalCapital {
		out.Type = geography.PlaceRegionalCapital
	}

	// Aliases union. The merged record's NAME becomes an alias too when it
	// differs, so searching the old spelling still finds the survivor.
	have := map[string]bool{}
	for _, a := range out.Aliases {
		have[a.NormalizedValue] = true
	}
	before := len(out.Aliases)
	for _, a := range merged.Aliases {
		if !have[a.NormalizedValue] {
			out.Aliases = append(out.Aliases, a)
			have[a.NormalizedValue] = true
		}
	}
	if merged.Name != out.Name && !have[merged.NormalizedName] {
		out.Aliases = append(out.Aliases, geography.Alias{
			Value:           merged.Name,
			NormalizedValue: merged.NormalizedName,
			AliasType:       "merged-name",
		})
	}
	cand.GainsAliases = len(out.Aliases) - before

	// Provenance records that this record now rests on two sources.
	if out.Provenance.Notes == "" {
		out.Provenance.Notes = "merged with " + merged.Provenance.SourceID
	} else {
		out.Provenance.Notes += "; merged with " + merged.Provenance.SourceID
	}
	return out
}
