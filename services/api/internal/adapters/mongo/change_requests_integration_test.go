package mongo

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	domain "github.com/ghanageo/ghanageo/services/api/internal/domain/changerequest"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func moderationFixture(t *testing.T) domain.ChangeRequest {
	t.Helper()
	cr, err := domain.New(domain.Target{Kind: "place", ID: "gh-place-fixture"}, map[string]any{"name": "Revised"},
		map[string]any{"name": "Original", "districtId": "gh-district-1"}, map[string]any{"name": "Revised", "districtId": "gh-district-1"},
		domain.Evidence{SourceRecordIDs: []string{"source-record-1"}, DuplicateCandidateIDs: []string{"duplicate-1"}, ReconciliationConflictIDs: []string{"conflict-1"}}, "submitter-1", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	cr.History[0].RequestID = "submit-request-1"
	return cr
}

func moderationAudit(t *testing.T, action audit.Action, cr domain.ChangeRequest, actorID string) audit.Entry {
	t.Helper()
	e, err := audit.New(audit.Actor{Kind: audit.ActorAdmin, ID: actorID}, action, audit.Target{Kind: "change_request", ID: cr.ID})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestChangeRequestRepositoryIsIdempotentAndSingleWinner(t *testing.T) {
	store := sessionTestStore(t)
	ctx := context.Background()
	if err := Migrate(ctx, store.DB()); err != nil {
		t.Fatal(err)
	}
	repo := NewChangeRequestRepo(store)
	cr := moderationFixture(t)
	created, inserted, err := repo.Create(ctx, cr, "submit-request-1", moderationAudit(t, audit.ActionChangeRequestSubmitted, cr, cr.SubmitterID))
	if err != nil || !inserted {
		t.Fatalf("create inserted=%v err=%v", inserted, err)
	}
	replayed, inserted, err := repo.Create(ctx, moderationFixture(t), "submit-request-1", moderationAudit(t, audit.ActionChangeRequestSubmitted, cr, cr.SubmitterID))
	if err != nil || inserted || replayed.ID != created.ID {
		t.Fatalf("replay inserted=%v id=%s err=%v", inserted, replayed.ID, err)
	}

	const racers = 12
	var winners atomic.Int32
	var wg sync.WaitGroup
	errs := make(chan error, racers)
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			reviewer := fmt.Sprintf("reviewer-%d", i)
			next, transitionErr := created.Transition(domain.StateInReview, reviewer, "", fmt.Sprintf("review-%d", i), time.Now())
			if transitionErr != nil {
				errs <- transitionErr
				return
			}
			err := repo.Transition(ctx, next, created.Version, moderationAudit(t, audit.ActionChangeRequestReviewStarted, next, reviewer))
			if err == nil {
				winners.Add(1)
				return
			}
			if err != domain.ErrConflict {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if winners.Load() != 1 {
		t.Fatalf("transition winners=%d, want 1", winners.Load())
	}
	stored, err := repo.Get(ctx, cr.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Version != 2 || stored.State != domain.StateInReview || len(stored.History) != 2 {
		t.Fatalf("bad persisted lifecycle: %+v", stored)
	}
	if count, _ := store.db.Collection(ColAuditLog).CountDocuments(ctx, bson.M{"targetId": cr.ID}); count != 2 {
		t.Fatalf("audit rows=%d, want submission + winning transition", count)
	}
}

func TestApprovalPreservesSnapshotAndCanonicalRecord(t *testing.T) {
	store := sessionTestStore(t)
	ctx := context.Background()
	if err := Migrate(ctx, store.DB()); err != nil {
		t.Fatal(err)
	}
	repo := NewChangeRequestRepo(store)
	cr := moderationFixture(t)
	if _, _, err := repo.Create(ctx, cr, "approval-submit", moderationAudit(t, audit.ActionChangeRequestSubmitted, cr, cr.SubmitterID)); err != nil {
		t.Fatal(err)
	}
	inReview, _ := cr.Transition(domain.StateInReview, "reviewer", "", "review-start", time.Now())
	if err := repo.Transition(ctx, inReview, cr.Version, moderationAudit(t, audit.ActionChangeRequestReviewStarted, inReview, "reviewer")); err != nil {
		t.Fatal(err)
	}
	approved, _ := inReview.Transition(domain.StateApproved, "reviewer", "verified against evidence", "approve", time.Now())
	if err := repo.Transition(ctx, approved, inReview.Version, moderationAudit(t, audit.ActionChangeRequestApproved, approved, "reviewer")); err != nil {
		t.Fatal(err)
	}
	stored, err := repo.Get(ctx, cr.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Snapshot.Digest != cr.Snapshot.Digest || stored.Snapshot.BeforeJSON != cr.Snapshot.BeforeJSON || stored.Snapshot.AfterJSON != cr.Snapshot.AfterJSON {
		t.Fatal("immutable side-by-side snapshot changed")
	}
	if count, _ := store.db.Collection(ColPlaces).CountDocuments(ctx, bson.M{"_id": cr.Target.ID}); count != 0 {
		t.Fatal("approval mutated canonical geography")
	}
}
