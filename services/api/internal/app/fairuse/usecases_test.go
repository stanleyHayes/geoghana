package fairuse

import (
	"context"
	"testing"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
)

type fakeRepo struct {
	policy                           *identity.FairUsePolicy
	policyEvidence, overrideEvidence *audit.Entry
}

func (r *fakeRepo) CurrentPolicy(context.Context, time.Time) (*identity.FairUsePolicy, error) {
	return r.policy, nil
}
func (r *fakeRepo) CurrentOverride(context.Context, string, time.Time) (*identity.FairUseOverride, error) {
	return nil, nil
}
func (r *fakeRepo) AppendPolicyAudited(_ context.Context, p identity.FairUsePolicy, e audit.Entry) error {
	r.policy = &p
	r.policyEvidence = &e
	return nil
}
func (r *fakeRepo) AppendOverrideAudited(_ context.Context, _ identity.FairUseOverride, e audit.Entry) error {
	r.overrideEvidence = &e
	return nil
}

type fakeAudit struct{ entries []audit.Entry }

func (a *fakeAudit) Append(_ context.Context, e audit.Entry) (audit.Entry, error) {
	a.entries = append(a.entries, e)
	return e, nil
}

type fakeInvalidator struct{ calls int }

func (i *fakeInvalidator) Invalidate() { i.calls++ }

func validPolicy(now time.Time) identity.FairUsePolicy {
	p := identity.DefaultFairUsePolicy()
	p.Revision = 2
	p.Reason = "measured capacity change"
	p.EffectiveFrom = now
	return p
}
func superActor() Actor {
	return Actor{ID: "adm-1", Email: "admin@example.test", Role: account.RoleSuperAdmin, RequestID: "req-1"}
}

func TestAppendPolicyCommitsAuditAndInvalidates(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	base := identity.DefaultFairUsePolicy()
	base.CreatedAt = now.Add(-time.Hour)
	base.EffectiveFrom = base.CreatedAt
	r := &fakeRepo{policy: &base}
	a := &fakeAudit{}
	i := &fakeInvalidator{}
	s := NewService(r, a, i)
	s.now = func() time.Time { return now }
	created, err := s.AppendPolicy(context.Background(), superActor(), validPolicy(now))
	if err != nil {
		t.Fatal(err)
	}
	if created.ActorID != "adm-1" || r.policyEvidence == nil || r.policyEvidence.Reason != "measured capacity change" {
		t.Fatalf("missing immutable evidence: %+v", r.policyEvidence)
	}
	if i.calls != 1 {
		t.Fatalf("invalidations=%d", i.calls)
	}
}

func TestRefusedMutationIsAudited(t *testing.T) {
	now := time.Now().UTC()
	base := identity.DefaultFairUsePolicy()
	base.CreatedAt = now.Add(-time.Hour)
	base.EffectiveFrom = base.CreatedAt
	r := &fakeRepo{policy: &base}
	a := &fakeAudit{}
	i := &fakeInvalidator{}
	s := NewService(r, a, i)
	s.now = func() time.Time { return now }
	actor := superActor()
	actor.Role = account.RoleDeveloperSupport
	if _, err := s.AppendPolicy(context.Background(), actor, validPolicy(now)); err == nil {
		t.Fatal("unauthorized mutation accepted")
	}
	if len(a.entries) != 1 || a.entries[0].Outcome != audit.OutcomeFailed {
		t.Fatalf("refusal evidence=%+v", a.entries)
	}
	if r.policyEvidence != nil || i.calls != 0 {
		t.Fatal("refusal mutated policy or cache")
	}
}

func TestOverrideRequiresExpiryAndRaisesAllowance(t *testing.T) {
	now := time.Now().UTC()
	base := identity.DefaultFairUsePolicy()
	base.CreatedAt = now.Add(-time.Hour)
	base.EffectiveFrom = base.CreatedAt
	s := NewService(&fakeRepo{policy: &base}, &fakeAudit{}, &fakeInvalidator{})
	s.now = func() time.Time { return now }
	o := identity.FairUseOverride{ApplicationID: "app-1", Allowance: base.Authenticated, CostCeilings: base.CostCeilings, Enabled: true, Reason: "temporary reconciliation"}
	if _, err := s.AppendOverride(context.Background(), superActor(), o); err == nil {
		t.Fatal("override without expiry accepted")
	}
	o.ExpiresAt = now.Add(time.Hour)
	if _, err := s.AppendOverride(context.Background(), superActor(), o); err == nil {
		t.Fatal("non-raising override accepted")
	}
}
