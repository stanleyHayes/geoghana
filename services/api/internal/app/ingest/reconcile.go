package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
	ingestdomain "github.com/ghanageo/ghanageo/services/api/internal/domain/ingest"
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
	Runs      ports.ImportRunRepository
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
	return im.RunWithMetadata(ctx, a, batchSize, "", "")
}

// RunWithMetadata executes an import with a stable file digest and request id.
// Supplying either makes retries resolve to the same durable run.
func (im *Importer) RunWithMetadata(ctx context.Context, a Adapter, batchSize int, payloadHash, requestID string) (*Result, error) {
	res := NewResult(a.Name())
	if batchSize <= 0 {
		batchSize = 500
	}
	var run ingestdomain.Run
	if im.Runs != nil {
		var err error
		run, err = ingestdomain.NewRun(a.Name(), payloadHash, requestID, res.StartedAt)
		if err != nil {
			return res, err
		}
		var inserted bool
		run, inserted, err = im.Runs.Queue(ctx, run)
		if err != nil {
			return res, err
		}
		if !inserted && run.Status == ingestdomain.StatusSucceeded {
			res.Fetched = int(run.RecordsProcessed)
			if run.FinishedAt != nil {
				res.FinishedAt = *run.FinishedAt
			} else {
				res.finish()
			}
			return res, nil
		}
		if err := im.Runs.Start(ctx, run.ID, res.StartedAt); err != nil {
			return res, err
		}
	}

	idx, err := im.buildRegionIndex(ctx)
	if err != nil {
		im.failRun(ctx, run, err)
		return res, fmt.Errorf("build region index: %w", err)
	}
	licence := a.Licence()

	err = a.Fetch(ctx, func(rec SourceRecord) error {
		res.Fetched++
		recordHash := hashSourceRecord(rec)
		externalRef := rec.ExternalID
		if strings.TrimSpace(externalRef) == "" {
			externalRef = fmt.Sprintf("missing:%d", res.Fetched)
		}

		region, ok := idx.resolve(rec.RegionHint)
		if !ok {
			// Refuse to file a place under a region we cannot identify.
			res.Reject("unresolved region: " + rec.RegionHint)
			if err := im.record(ctx, run, externalRef, recordHash, ingestdomain.RecordRejected, "unresolved region", nil); err != nil {
				return err
			}
			return nil
		}

		place, perr := im.toPlace(rec, region, licence)
		if perr != nil {
			res.Reject(perr.Error())
			if err := im.record(ctx, run, externalRef, recordHash, ingestdomain.RecordRejected, perr.Error(), nil); err != nil {
				return err
			}
			return nil
		}

		// Never clobber steward-reviewed work. A reference source may create a
		// record and may refresh a record it owns, but it may not overwrite one
		// a human has reviewed or published (rule R8).
		existing, gerr := im.Places.Get(ctx, place.ID)
		if gerr == nil && existing != nil {
			if existing.VerificationStatus.PromotableToCanonical() {
				res.Skipped++
				if err := im.record(ctx, run, externalRef, recordHash, ingestdomain.RecordSkipped, "", nil); err != nil {
					return err
				}
				return nil
			}
		}

		var created bool
		var uerr error
		if im.Runs != nil {
			created, uerr = im.recordPlace(ctx, run, externalRef, recordHash, place)
		} else {
			created, uerr = im.Places.Upsert(ctx, place)
		}
		if uerr != nil {
			return uerr
		}
		if created {
			res.Created++
		} else {
			res.Updated++
		}
		return nil
	})

	res.finish()
	if err != nil {
		im.failRun(ctx, run, err)
		return res, err
	}
	if im.Runs != nil {
		conflicts := int64(0)
		for reason, count := range res.RejectReasons {
			if strings.HasPrefix(reason, "unresolved region") {
				conflicts += int64(count)
			}
		}
		if completeErr := im.Runs.Complete(ctx, run.ID, res.FinishedAt, conflicts, 0); completeErr != nil {
			return res, completeErr
		}
	}
	return res, err
}

func hashSourceRecord(rec SourceRecord) string {
	payload, _ := json.Marshal(rec)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func (im *Importer) record(ctx context.Context, run ingestdomain.Run, ref, hash string, outcome ingestdomain.RecordOutcome, reason string, place *geography.Place) error {
	if im.Runs == nil {
		return nil
	}
	raw, err := ingestdomain.NewRawRecord(run, ref, hash, outcome, reason, time.Now())
	if err != nil {
		return err
	}
	_, err = im.Runs.CommitRecord(ctx, raw, place)
	return err
}

func (im *Importer) recordPlace(ctx context.Context, run ingestdomain.Run, ref, hash string, place geography.Place) (bool, error) {
	raw, err := ingestdomain.NewRawRecord(run, ref, hash, ingestdomain.RecordUpdated, "", time.Now())
	if err != nil {
		return false, err
	}
	return im.Runs.CommitRecord(ctx, raw, &place)
}

func (im *Importer) failRun(ctx context.Context, run ingestdomain.Run, _ error) {
	if im.Runs == nil || run.ID == "" {
		return
	}
	now := time.Now().UTC()
	_ = im.Runs.Fail(ctx, run.ID, ingestdomain.Error{Code: "import_failed", Message: "import failed", At: now}, now)
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

	id, err := geography.StableID("place", prov.SourceID, rec.ExternalID)
	if err != nil {
		return geography.Place{}, fmt.Errorf("stable id: %w", err)
	}
	p := geography.Place{
		// Deterministic and traceable: the same GeoNames record always
		// produces the same id, which is what makes reimports idempotent and
		// lets anyone trace a record back to its source row.
		ID:             id,
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
