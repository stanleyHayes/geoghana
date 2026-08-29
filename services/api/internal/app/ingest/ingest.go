// Package ingest defines the ETL contract every source adapter implements.
//
// The pipeline is FETCH → VALIDATE → NORMALIZE → MATCH → RECONCILE (Spec §4.1).
// Each stage is separately testable, and no adapter writes to a canonical
// collection directly: it emits records, and this package decides what happens
// to them.
package ingest

import (
	"context"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
)

// SourceRecord is one row as the adapter understood it, before matching.
type SourceRecord struct {
	// ExternalID is the source's own identifier. It must be stable across
	// runs, because it is what makes reimports idempotent (Spec §22.1).
	ExternalID string
	Name       string
	Aliases    []string
	Type       geography.PlaceType
	Coordinate *geography.Coordinate
	Population *int64
	// RegionHint and DistrictHint are the source's own administrative labels,
	// resolved to canonical ids during matching.
	RegionHint   string
	DistrictHint string
	Provenance   geography.Provenance
}

// Adapter fetches and normalizes one source. Adapters perform NO writes.
type Adapter interface {
	// Name identifies the source in the licensing register.
	Name() string
	// Licence is the exact obligation this source carries. Recording it here
	// keeps attribution attached to the data rather than to a wiki page.
	Licence() Licence
	// Fetch streams normalized records. It must be safe to call repeatedly.
	Fetch(ctx context.Context, emit func(SourceRecord) error) error
}

// Licence travels with every record the adapter produces, so attribution can
// be rendered in API responses and downloads (plan rule R4).
type Licence struct {
	SPDX        string
	Name        string
	URL         string
	Attribution string
	// Redistributable records whether the raw data may be republished. A
	// source that is reference-only must never reach a bulk download.
	Redistributable bool
}

// Result summarises a run for the import-runs screen.
type Result struct {
	Source        string
	StartedAt     time.Time
	FinishedAt    time.Time
	Fetched       int
	Created       int
	Updated       int
	Skipped       int
	Rejected      int
	RejectReasons map[string]int
}

func NewResult(source string) *Result {
	return &Result{Source: source, StartedAt: time.Now(), RejectReasons: map[string]int{}}
}

func (r *Result) Reject(reason string) {
	r.Rejected++
	r.RejectReasons[reason]++
}

// nowFunc is a seam for tests.
var nowFunc = time.Now
