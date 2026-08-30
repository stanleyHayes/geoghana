package ingest

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
	ingestdomain "github.com/ghanageo/ghanageo/services/api/internal/domain/ingest"
	"github.com/ghanageo/ghanageo/services/api/internal/ports"
)

type durableTestRegions struct{}

func (durableTestRegions) List(context.Context, ports.ListParams) (ports.Page[geography.Region], error) {
	return ports.Page[geography.Region]{}, nil
}
func (durableTestRegions) Get(context.Context, string) (*geography.Region, error) {
	return nil, errors.New("unused")
}
func (durableTestRegions) Upsert(context.Context, geography.Region) (bool, error) {
	return false, errors.New("unused")
}
func (durableTestRegions) Count(context.Context) (int64, error) { return 0, nil }

type durableTestAdapter struct{ fetchErr error }

func (durableTestAdapter) Name() string     { return "test-source" }
func (durableTestAdapter) Licence() Licence { return Licence{} }
func (a durableTestAdapter) Fetch(_ context.Context, emit func(SourceRecord) error) error {
	if err := emit(SourceRecord{ExternalID: "external-1", Name: "Private Name", RegionHint: "Unknown"}); err != nil {
		return err
	}
	return a.fetchErr
}

type durableTestRuns struct {
	run       ingestdomain.Run
	records   []ingestdomain.RawRecord
	completed bool
	failed    bool
	conflicts int64
}

func (r *durableTestRuns) Queue(_ context.Context, run ingestdomain.Run) (ingestdomain.Run, bool, error) {
	r.run = run
	return run, true, nil
}
func (r *durableTestRuns) Start(_ context.Context, _ string, at time.Time) error {
	r.run.Status = ingestdomain.StatusRunning
	r.run.StartedAt = &at
	return nil
}
func (r *durableTestRuns) CommitRecord(_ context.Context, raw ingestdomain.RawRecord, _ *geography.Place) (bool, error) {
	r.records = append(r.records, raw)
	return false, nil
}
func (r *durableTestRuns) Complete(_ context.Context, _ string, _ time.Time, conflicts, _ int64) error {
	r.completed = true
	r.conflicts = conflicts
	return nil
}
func (r *durableTestRuns) Fail(_ context.Context, _ string, _ ingestdomain.Error, _ time.Time) error {
	r.failed = true
	return nil
}

func TestImporterPersistsLifecycleAndPrivacySafeRecord(t *testing.T) {
	runs := &durableTestRuns{}
	im := Importer{Regions: durableTestRegions{}, Runs: runs, DatasetVersion: "test"}
	result, err := im.RunWithMetadata(context.Background(), durableTestAdapter{}, 10, strings.Repeat("a", 64), "request-1")
	if err != nil {
		t.Fatal(err)
	}
	if !runs.completed || runs.failed || runs.conflicts != 1 {
		t.Fatalf("lifecycle: %+v", runs)
	}
	if result.Rejected != 1 || len(runs.records) != 1 {
		t.Fatalf("result=%+v records=%d", result, len(runs.records))
	}
	record := runs.records[0]
	if record.Outcome != ingestdomain.RecordRejected || record.ReasonCode != "unresolved_region" {
		t.Fatalf("record=%+v", record)
	}
	if strings.Contains(record.ReasonCode, "Private") || len(record.PayloadHash) != 64 {
		t.Fatalf("unsafe record=%+v", record)
	}
}

func TestImporterMarksDurableRunFailed(t *testing.T) {
	runs := &durableTestRuns{}
	im := Importer{Regions: durableTestRegions{}, Runs: runs, DatasetVersion: "test"}
	_, err := im.RunWithMetadata(context.Background(), durableTestAdapter{fetchErr: errors.New("provider unavailable")}, 10, strings.Repeat("b", 64), "request-2")
	if err == nil || !runs.failed || runs.completed {
		t.Fatalf("err=%v lifecycle=%+v", err, runs)
	}
}
