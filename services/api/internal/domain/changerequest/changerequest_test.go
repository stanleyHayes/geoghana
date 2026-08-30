package changerequest

import (
	"errors"
	"testing"
	"time"
)

func validRequest(t *testing.T) ChangeRequest {
	t.Helper()
	r, err := New(Target{Kind: "place", ID: "gh-place-1"}, map[string]any{"name": "New"},
		map[string]any{"name": "Old"}, map[string]any{"name": "New"}, Evidence{DuplicateCandidateIDs: []string{"dup-1"}}, "contributor", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestLifecycleAndImmutableSnapshot(t *testing.T) {
	r := validRequest(t)
	r.Snapshot.Before()["name"] = "tampered"
	if got := r.Snapshot.Before()["name"]; got != "Old" {
		t.Fatalf("snapshot mutated: %v", got)
	}
	now := time.Now()
	inReview, err := r.Transition(StateInReview, "reviewer", "", "req-1", now)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := inReview.Transition(StateChangesRequested, "reviewer", "cite the gazette", "req-2", now)
	if err != nil {
		t.Fatal(err)
	}
	resubmitted, err := changes.Revise("contributor", map[string]any{"name": "Newest"}, map[string]any{"name": "Newest"}, Evidence{References: []string{"gazette:42"}}, "evidence added", "req-3", now)
	if err != nil {
		t.Fatal(err)
	}
	if resubmitted.Version != 4 || len(resubmitted.History) != 4 || len(resubmitted.Revisions) != 2 || resubmitted.ReviewerID != "" {
		t.Fatalf("bad lifecycle: %+v", resubmitted)
	}
}

func TestIllegalAndSelfReviewTransitionsAreRejected(t *testing.T) {
	r := validRequest(t)
	if _, err := r.Transition(StateApproved, "reviewer", "", "", time.Now()); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("submitted -> approved: %v", err)
	}
	if _, err := r.Transition(StateInReview, "contributor", "", "", time.Now()); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("self review: %v", err)
	}
	inReview, _ := r.Transition(StateInReview, "reviewer", "", "", time.Now())
	if _, err := inReview.Transition(StateRejected, "reviewer", "", "", time.Now()); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty rejection: %v", err)
	}
}

func TestPatchMustProduceDisplayedAfterSnapshot(t *testing.T) {
	_, err := New(Target{Kind: "place", ID: "gh-place-1"}, map[string]any{"name": "Proposed"}, map[string]any{"name": "Old"}, map[string]any{"name": "Something else"}, Evidence{References: []string{"source"}}, "submitter", time.Now())
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("inconsistent diff accepted: %v", err)
	}
}
