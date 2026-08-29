package ingest

import (
	"context"
	"fmt"
	"strings"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/normalize"
	"github.com/ghanageo/ghanageo/services/api/internal/ports"
)

// Importer turns adapter records into canonical places.
//
// It enforces the rules that keep an import safe to re-run and safe to trust:
// no source overwrites steward-reviewed work (R8), every record keeps its
// provenance (R2), and nothing is promoted to canonical automatically (R5).
type Importer struct {
	Regions   ports.RegionRepository
	Districts ports.DistrictRepository
	Places    ports.PlaceRepository
	// DatasetVersion stamps every written record.
	DatasetVersion string
}

// regionIndex resolves a source's region label to a canonical region id.
type regionIndex struct {
	byNormalizedName map[string]geography.Region
}

func (im *Importer) buildRegionIndex(ctx context.Context) (*regionIndex, error) {
	page, err := im.Regions.List(ctx, ports.ListParams{Limit: 100})
	if err != nil {
		return nil, err
	}
	idx := &regionIndex{byNormalizedName: make(map[string]geography.Region, len(page.Data))}
	for _, r := range page.Data {
		idx.byNormalizedName[normalize.Name(r.Name)] = r
	}
	return idx, nil
}

func (idx *regionIndex) resolve(hint string) (geography.Region, bool) {
	r, ok := idx.byNormalizedName[normalize.Name(hint)]
	return r, ok
}

// Run executes a full import.
//
// Idempotency comes from the deterministic id: the same source record always
// produces the same place id, so a second run updates rather than duplicates.
func (im *Importer) Run(ctx context.Context, a Adapter, batchSize int) (*Result, error) {
	res := NewResult(a.Name())
	if batchSize <= 0 {
		batchSize = 500
	}

	idx, err := im.buildRegionIndex(ctx)
	if err != nil {
		return res, fmt.Errorf("build region index: %w", err)
	}
	licence := a.Licence()

	err = a.Fetch(ctx, func(rec SourceRecord) error {
		res.Fetched++

		region, ok := idx.resolve(rec.RegionHint)
		if !ok {
			// Refuse to file a place under a region we cannot identify.
			res.Reject("unresolved region: " + rec.RegionHint)
			return nil
		}

		place, perr := im.toPlace(rec, region, licence)
		if perr != nil {
			res.Reject(perr.Error())
			return nil
		}

		// Never clobber steward-reviewed work. A reference source may create a
		// record and may refresh a record it owns, but it may not overwrite one
		// a human has reviewed or published (rule R8).
		existing, gerr := im.Places.Get(ctx, place.ID)
		if gerr == nil && existing != nil {
			if existing.VerificationStatus.PromotableToCanonical() {
				res.Skipped++
				return nil
			}
		}

		created, uerr := im.Places.Upsert(ctx, place)
		if uerr != nil {
			res.Reject("write failed: " + uerr.Error())
			return nil
		}
		if created {
			res.Created++
		} else {
			res.Updated++
		}
		return nil
	})

	res.finish()
	return res, err
}

func (r *Result) finish() { r.FinishedAt = nowFunc() }

// toPlace builds a canonical place from a source record.
func (im *Importer) toPlace(
	rec SourceRecord, region geography.Region, licence Licence,
) (geography.Place, error) {
	if strings.TrimSpace(rec.ExternalID) == "" {
		return geography.Place{}, fmt.Errorf("missing external id")
	}

	aliases := make([]geography.Alias, 0, len(rec.Aliases))
	for _, a := range rec.Aliases {
		aliases = append(aliases, geography.Alias{
			Value:           a,
			NormalizedValue: normalize.Name(a),
			AliasType:       "alternate",
		})
	}

	prov := rec.Provenance
	if prov.Notes == "" {
		prov.Notes = licence.Attribution
	}

	p := geography.Place{
		// Deterministic and traceable: the same GeoNames record always
		// produces the same id, which is what makes reimports idempotent and
		// lets anyone trace a record back to its source row.
		ID:             deterministicID(prov.SourceID, rec.ExternalID),
		Name:           rec.Name,
		NormalizedName: normalize.Name(rec.Name),
		Type:           rec.Type,
		RegionID:       region.ID,
		RegionName:     region.Name,
		Aliases:        aliases,
		Centroid:       rec.Coordinate,
		Population:     rec.Population,
		Status:         geography.StatusActive,
		// A reference source never lands as canonical (rule R5).
		VerificationStatus: geography.VerificationReference,
		Provenance:         prov,
		DatasetVersion:     im.DatasetVersion,
	}
	if err := p.Validate(); err != nil {
		return geography.Place{}, err
	}
	return p, nil
}

func deterministicID(source, externalID string) string {
	return fmt.Sprintf("gh-place-%s-%s", shortSource(source), externalID)
}

func shortSource(s string) string {
	switch s {
	case "geonames":
		return "gn"
	case "openstreetmap", "osm":
		return "osm"
	case "gss":
		return "gss"
	default:
		return s
	}
}
