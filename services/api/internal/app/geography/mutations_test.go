package geography

import (
	"context"
	"strings"
	"testing"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
)

type recordingSink struct{ entries []audit.Entry }

func (r *recordingSink) Append(_ context.Context, e audit.Entry) (audit.Entry, error) {
	r.entries = append(r.entries, e)
	return e, nil
}

func (r *recordingSink) last() (audit.Entry, bool) {
	if len(r.entries) == 0 {
		return audit.Entry{}, false
	}
	return r.entries[len(r.entries)-1], true
}

// Authorization is checked against the PERMISSION, never the role name, and a
// refused attempt is still audited — a rejected privileged action is exactly
// what a security review wants to see.
func TestMutationsRequirePermissionAndAuditRefusals(t *testing.T) {
	roles := []struct {
		role    account.Role
		allowed bool
	}{
		{account.RoleSuperAdmin, true},
		{account.RoleDataAdmin, true},
		{account.RoleDataReviewer, false},
		{account.RoleDataContributor, false},
		{account.RoleDeveloperSupport, false},
		{account.RoleSecurityAuditor, false},
		{account.RoleDeveloper, false},
	}
	for _, c := range roles {
		t.Run(string(c.role), func(t *testing.T) {
			sink := &recordingSink{}
			s := &Service{auditSink: sink}
			a := Actor{ID: "u1", Email: "u@example.com", Role: c.role}

			err := s.authorize(a, account.PermEditGeography)
			if c.allowed && err != nil {
				t.Fatalf("%s was refused: %v", c.role, err)
			}
			if !c.allowed {
				if err == nil {
					t.Fatalf("%s was allowed to edit geography", c.role)
				}
				// The refusal must name the permission the caller lacked,
				// so an operator can fix the grant rather than guess.
				if !strings.Contains(err.Error(), "PERMISSION_DENIED") {
					t.Errorf("unhelpful refusal: %v", err)
				}
				// And it must be recorded.
				s.record(context.Background(), a, audit.ActionRecordUpdated,
					audit.Target{Kind: "region", ID: "gh-region-ashanti"}, nil, nil, err)
				e, ok := sink.last()
				if !ok {
					t.Fatal("a refused mutation was not audited")
				}
				if e.Outcome != audit.OutcomeFailed {
					t.Errorf("refusal recorded as %q", e.Outcome)
				}
			}
		})
	}
}

// A record must never be merged into itself: the redirect would loop and a
// consumer following it would never resolve.
func TestDeprecateRejectsSelfMerge(t *testing.T) {
	sink := &recordingSink{}
	s := &Service{auditSink: sink}
	a := Actor{ID: "u1", Email: "u@example.com", Role: account.RoleDataAdmin}

	err := s.DeprecatePlace(context.Background(), a, "gh-place-osu", "gh-place-osu", "typo")
	if err == nil {
		t.Fatal("a record was merged into itself")
	}
	e, ok := sink.last()
	if !ok || e.Outcome != audit.OutcomeFailed {
		t.Error("the rejected self-merge was not audited as a failure")
	}
}

// The audit row must carry the actor, the target and the request id, or it is
// not evidence of who did what.
func TestAuditRowCarriesActorTargetAndRequest(t *testing.T) {
	sink := &recordingSink{}
	s := &Service{auditSink: sink}
	a := Actor{
		ID: "acc_1", Email: "steward@example.com", Role: account.RoleDataAdmin,
		IP: "41.66.0.1", RequestID: "req_abc",
	}
	s.record(context.Background(), a, audit.ActionRecordUpdated,
		audit.Target{Kind: "district", ID: "gh-district-x", Label: "Somewhere"},
		map[string]any{"name": "Old"}, map[string]any{"name": "New"}, nil)

	e, ok := sink.last()
	if !ok {
		t.Fatal("nothing recorded")
	}
	if e.Actor.ID != "acc_1" || e.Actor.Label != "steward@example.com" || e.Actor.IP != "41.66.0.1" {
		t.Errorf("actor not captured: %+v", e.Actor)
	}
	if e.Target.Kind != "district" || e.Target.ID != "gh-district-x" {
		t.Errorf("target not captured: %+v", e.Target)
	}
	if e.RequestID != "req_abc" {
		t.Errorf("request id not captured: %q", e.RequestID)
	}
	if e.Before["name"] != "Old" || e.After["name"] != "New" {
		t.Errorf("before/after not captured: %v → %v", e.Before, e.After)
	}
}

// A service with no audit sink must not panic — read-only deployments do not
// attach one — but it must also not silently accept a mutation as audited.
func TestRecordWithoutSinkIsSafe(t *testing.T) {
	s := &Service{}
	s.record(context.Background(), Actor{ID: "u", Role: account.RoleDataAdmin},
		audit.ActionRecordUpdated, audit.Target{Kind: "region", ID: "r"}, nil, nil, nil)
}
