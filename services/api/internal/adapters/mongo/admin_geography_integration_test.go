package mongo

import (
	"context"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
)

func adminGeoEvidence(t *testing.T, requestID, kind, id string, action audit.Action) audit.Entry {
	t.Helper()
	e, err := audit.New(audit.Actor{Kind: audit.ActorAdmin, ID: "admin-1"}, action, audit.Target{Kind: kind, ID: id})
	if err != nil {
		t.Fatal(err)
	}
	return e.WithChange(nil, map[string]any{"id": id}).WithRequest(requestID)
}
func adminGeoRegion(t *testing.T, external string) geography.Region {
	t.Helper()
	id, err := geography.StableID("region", "test", external)
	if err != nil {
		t.Fatal(err)
	}
	return geography.Region{ID: id, CountryCode: "GH", Name: external, Status: geography.StatusActive, VerificationStatus: geography.VerificationReviewed, Provenance: geography.Provenance{SourceID: "test", ExternalID: external, RetrievedAt: "2026-08-30", SourcePayloadHash: strings.Repeat("a", 64)}, DatasetVersion: "test"}
}

func TestAdminGeographyMutationIsAtomicAndIdempotent(t *testing.T) {
	store := sessionTestStore(t)
	ctx := context.Background()
	if err := Migrate(ctx, store.DB()); err != nil {
		t.Fatal(err)
	}
	repo := NewAdminGeographyRepo(store)
	first := adminGeoRegion(t, "first")
	e := adminGeoEvidence(t, "actor-key-1", "region", first.ID, audit.ActionRecordUpdated)
	if err := repo.CreateRegion(ctx, first, e); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateRegion(ctx, first, e); err != nil {
		t.Fatalf("idempotent replay: %v", err)
	}
	if n, _ := store.db.Collection(ColRegions).CountDocuments(ctx, bson.M{"_id": first.ID}); n != 1 {
		t.Fatalf("regions=%d", n)
	}
	if n, _ := store.db.Collection(ColAuditLog).CountDocuments(ctx, bson.M{"requestId": "actor-key-1"}); n != 1 {
		t.Fatalf("audit rows=%d", n)
	}
	second := adminGeoRegion(t, "second")
	if err := repo.CreateRegion(ctx, second, adminGeoEvidence(t, "actor-key-1", "region", second.ID, audit.ActionRecordUpdated)); err == nil {
		t.Fatal("same idempotency key accepted for a different mutation")
	}
}

func TestAdminGeographyDeprecationCommitsRedirectAndAuditTogether(t *testing.T) {
	store := sessionTestStore(t)
	ctx := context.Background()
	if err := Migrate(ctx, store.DB()); err != nil {
		t.Fatal(err)
	}
	repo := NewAdminGeographyRepo(store)
	old, survivor := adminGeoRegion(t, "old"), adminGeoRegion(t, "survivor")
	if err := repo.CreateRegion(ctx, old, adminGeoEvidence(t, "create-old", "region", old.ID, audit.ActionRecordUpdated)); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateRegion(ctx, survivor, adminGeoEvidence(t, "create-survivor", "region", survivor.ID, audit.ActionRecordUpdated)); err != nil {
		t.Fatal(err)
	}
	e := adminGeoEvidence(t, "merge-old", "region", old.ID, audit.ActionRecordDeprecated)
	if err := repo.Deprecate(ctx, "region", old.ID, survivor.ID, "duplicate", e); err != nil {
		t.Fatal(err)
	}
	if err := repo.Deprecate(ctx, "region", old.ID, survivor.ID, "duplicate", e); err != nil {
		t.Fatalf("merge replay: %v", err)
	}
	var row bson.M
	if err := store.db.Collection(ColRegions).FindOne(ctx, bson.M{"_id": old.ID}).Decode(&row); err != nil {
		t.Fatal(err)
	}
	if row["status"] != "MERGED" {
		t.Fatalf("status=%v", row["status"])
	}
	if n, _ := store.db.Collection(ColRedirects).CountDocuments(ctx, bson.M{"_id": old.ID, "newId": survivor.ID, "kind": "region"}); n != 1 {
		t.Fatalf("redirects=%d", n)
	}
	if n, _ := store.db.Collection(ColAuditLog).CountDocuments(ctx, bson.M{"requestId": "merge-old"}); n != 1 {
		t.Fatalf("audit rows=%d", n)
	}
}
