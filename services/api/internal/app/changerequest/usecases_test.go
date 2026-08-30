package changerequest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	domain "github.com/ghanageo/ghanageo/services/api/internal/domain/changerequest"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
)

type fakeRepo struct {
	value     domain.ChangeRequest
	requestID string
}

func (f *fakeRepo) Create(_ context.Context, cr domain.ChangeRequest, requestID string, _ audit.Entry) (domain.ChangeRequest, bool, error) {
	if f.requestID == requestID && requestID != "" {
		return f.value, false, nil
	}
	f.value, f.requestID = cr, requestID
	return cr, true, nil
}
func (f *fakeRepo) Get(_ context.Context, _ string) (domain.ChangeRequest, error) {
	if f.value.ID == "" {
		return domain.ChangeRequest{}, domain.ErrNotFound
	}
	return f.value, nil
}
func (f *fakeRepo) List(_ context.Context, q Filter) (Page, error) {
	if q.SubmitterID != "" && q.SubmitterID != f.value.SubmitterID {
		return Page{}, nil
	}
	return Page{Data: []domain.ChangeRequest{f.value}}, nil
}
func (f *fakeRepo) Transition(_ context.Context, cr domain.ChangeRequest, version int64, _ audit.Entry) error {
	if f.value.Version != version {
		return domain.ErrConflict
	}
	f.value = cr
	return nil
}
func (f *fakeRepo) Revise(ctx context.Context, cr domain.ChangeRequest, version int64, e audit.Entry) error {
	return f.Transition(ctx, cr, version, e)
}
func (f *fakeRepo) AddComment(ctx context.Context, cr domain.ChangeRequest, version int64, e audit.Entry) error {
	return f.Transition(ctx, cr, version, e)
}

func actor(id string, role account.Role) Actor {
	return Actor{ID: id, Email: id + "@example.test", Role: role, RequestID: "request-1"}
}
func command() SubmitCommand {
	return SubmitCommand{Target: domain.Target{Kind: "place", ID: "gh-place-1"}, ProposedPatch: map[string]any{"name": "New"}, Before: map[string]any{"name": "Old"}, After: map[string]any{"name": "New"}, Evidence: domain.Evidence{ReconciliationConflictIDs: []string{"conflict-1"}}}
}

func TestPermissionsOwnershipAndModeration(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo).WithClock(func() time.Time { return time.Unix(100, 0).UTC() })
	contributor := actor("contributor", account.RoleDataContributor)
	cr, err := svc.Submit(context.Background(), contributor, command())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Transition(context.Background(), contributor, cr.ID, cr.Version, domain.StateInReview, ""); apierr.From(err).Code != apierr.PermissionDenied {
		t.Fatalf("contributor reviewed: %v", err)
	}
	reviewer := actor("reviewer", account.RoleDataReviewer)
	inReview, err := svc.Transition(context.Background(), reviewer, cr.ID, cr.Version, domain.StateInReview, "")
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := svc.Transition(context.Background(), reviewer, cr.ID, cr.Version, domain.StateInReview, "")
	if err != nil || replayed.Version != inReview.Version {
		t.Fatalf("idempotent review retry: %+v err=%v", replayed, err)
	}
	reviewer.RequestID = "comment-1"
	commented, err := svc.AddReviewerComment(context.Background(), reviewer, cr.ID, inReview.Version, "checked source")
	if err != nil {
		t.Fatal(err)
	}
	commentReplay, err := svc.AddReviewerComment(context.Background(), reviewer, cr.ID, inReview.Version, "checked source")
	if err != nil || commentReplay.Version != commented.Version {
		t.Fatalf("idempotent comment retry: %+v err=%v", commentReplay, err)
	}
	reviewer.RequestID = "approve-1"
	approved, err := svc.Transition(context.Background(), reviewer, cr.ID, commented.Version, domain.StateApproved, "verified")
	if err != nil {
		t.Fatal(err)
	}
	if approved.State != domain.StateApproved {
		t.Fatalf("state=%s", approved.State)
	}
}

func TestSubmitIsIdempotentAndOptimisticConflictIsVisible(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	a := actor("contributor", account.RoleDataContributor)
	first, err := svc.Submit(context.Background(), a, command())
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Submit(context.Background(), a, command())
	if err != nil || second.ID != first.ID {
		t.Fatalf("idempotent submit: first=%s second=%s err=%v", first.ID, second.ID, err)
	}
	reviewer := actor("reviewer", account.RoleDataReviewer)
	if _, err := svc.Transition(context.Background(), reviewer, first.ID, first.Version+1, domain.StateInReview, ""); apierr.From(err).Code != apierr.Conflict {
		t.Fatalf("stale transition: %v", err)
	}
}

func TestContributorListIsForcedToOwnRequests(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	_, _ = svc.Submit(context.Background(), actor("alice", account.RoleDataContributor), command())
	page, err := svc.List(context.Background(), actor("bob", account.RoleDataContributor), Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Data) != 0 {
		t.Fatal("contributor saw another submitter's request")
	}
	if _, err := svc.Get(context.Background(), actor("bob", account.RoleDataContributor), repo.value.ID); apierr.From(err).Code != apierr.PermissionDenied {
		t.Fatalf("get ownership: %v", err)
	}
	if !errors.Is(domain.ErrConflict, domain.ErrConflict) {
		t.Fatal("sentinel")
	}
}
