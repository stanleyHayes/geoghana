package mongo

import (
	"context"
	"strings"
	"testing"
	"time"

	adminapp "github.com/ghanageo/ghanageo/services/api/internal/app/adminops"
	admindomain "github.com/ghanageo/ghanageo/services/api/internal/domain/adminops"
	ingestdomain "github.com/ghanageo/ghanageo/services/api/internal/domain/ingest"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestImportRunLifecycleIsDurableAndIdempotent(t *testing.T) {
	store := sessionTestStore(t)
	ctx := context.Background()
	if err := Migrate(ctx, store.DB()); err != nil {
		t.Fatal(err)
	}
	repo := NewImportRunRepo(store)
	now := time.Now().UTC().Truncate(time.Millisecond)
	run, err := ingestdomain.NewRun("geonames", strings.Repeat("a", 64), "request-1", now)
	if err != nil {
		t.Fatal(err)
	}
	queued, inserted, err := repo.Queue(ctx, run)
	if err != nil || !inserted {
		t.Fatalf("queue: inserted=%v err=%v", inserted, err)
	}
	if err := repo.Start(ctx, queued.ID, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	record, err := ingestdomain.NewRawRecord(queued, "provider-record-42", strings.Repeat("b", 64),
		ingestdomain.RecordRejected, "unresolved region: private@example.com", now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CommitRecord(ctx, record, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CommitRecord(ctx, record, nil); err != nil {
		t.Fatalf("idempotent replay: %v", err)
	}
	if err := repo.Complete(ctx, queued.ID, now.Add(3*time.Second), 1, 2); err != nil {
		t.Fatal(err)
	}

	var got importRunDoc
	if err := store.db.Collection(ColSourceRuns).FindOne(ctx, bson.M{"_id": queued.ID}).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Status != ingestdomain.StatusSucceeded || got.RecordsProcessed != 1 || len(got.Errors) != 1 {
		t.Fatalf("unexpected durable run: %+v", got)
	}
	if got.Errors[0].Message != "unresolved_region" {
		t.Fatalf("unredacted error: %+v", got.Errors[0])
	}
	if got.ReconciliationConflicts != 1 || got.DuplicateCandidates != 2 || got.DurationMS != 2000 {
		t.Fatalf("summary mismatch: %+v", got)
	}
	var raw bson.M
	if err := store.db.Collection(ColSourceRecords).FindOne(ctx, bson.M{"_id": record.ID}).Decode(&raw); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"payload", "name", "email", "aliases", "coordinate"} {
		if _, exists := raw[forbidden]; exists {
			t.Fatalf("raw record retained forbidden %q: %#v", forbidden, raw)
		}
	}
	if raw["reasonCode"] != "unresolved_region" {
		t.Fatalf("reason was not redacted: %#v", raw)
	}
	if count, _ := store.db.Collection(ColSourceRecords).CountDocuments(ctx, bson.M{}); count != 1 {
		t.Fatalf("idempotent replay created %d source records", count)
	}
	admin := NewAdminOpsRepo(store)
	page, err := admin.SourceRecords(ctx, queued.ID, adminapp.SourceRecordQuery{ReasonCode: "unresolved_region"})
	if err != nil {
		t.Fatal(err)
	}
	if page.DetailAvailability != admindomain.DetailDurable || len(page.Data) != 1 {
		t.Fatalf("unexpected admin record page: %+v", page)
	}
	if page.Data[0].PayloadHash != strings.Repeat("b", 64) || page.Data[0].ExternalRef != "provider-record-42" {
		t.Fatalf("raw-record projection lost drill-down metadata: %+v", page.Data[0])
	}
	detail, err := admin.SourceRecord(ctx, queued.ID, record.ID)
	if err != nil || detail.ID != record.ID {
		t.Fatalf("source record detail = %+v, err=%v", detail, err)
	}
	empty, err := admin.SourceRecords(ctx, queued.ID, adminapp.SourceRecordQuery{ReasonCode: "duplicate_candidate"})
	if err != nil || len(empty.Data) != 0 || empty.DetailAvailability != admindomain.DetailDurable {
		t.Fatalf("honest empty duplicate page = %+v, err=%v", empty, err)
	}
}

func TestImportRecordRollsBackWhenRunIsNotRunning(t *testing.T) {
	store := sessionTestStore(t)
	ctx := context.Background()
	if err := Migrate(ctx, store.DB()); err != nil {
		t.Fatal(err)
	}
	repo := NewImportRunRepo(store)
	now := time.Now().UTC()
	run, _ := ingestdomain.NewRun("geonames", strings.Repeat("c", 64), "request-rollback", now)
	if _, _, err := repo.Queue(ctx, run); err != nil {
		t.Fatal(err)
	}
	record, _ := ingestdomain.NewRawRecord(run, "record-1", strings.Repeat("d", 64), ingestdomain.RecordRejected, "bad value", now)
	if _, err := repo.CommitRecord(ctx, record, nil); err == nil {
		t.Fatal("record committed for a queued run")
	}
	if count, _ := store.db.Collection(ColSourceRecords).CountDocuments(ctx, bson.M{}); count != 0 {
		t.Fatalf("transaction leaked %d source records", count)
	}
}
