package fairuse

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
)

type fakeRepo struct {
	policy   *identity.FairUsePolicy
	override *identity.FairUseOverride
	err      error
	reads    int
}

func (f *fakeRepo) CurrentPolicy(context.Context, time.Time) (*identity.FairUsePolicy, error) {
	f.reads++
	return f.policy, f.err
}
func (f *fakeRepo) CurrentOverride(context.Context, string, time.Time) (*identity.FairUseOverride, error) {
	f.reads++
	return f.override, f.err
}

func validPolicy(now time.Time) identity.FairUsePolicy {
	p := identity.DefaultFairUsePolicy()
	p.Revision = 2
	p.CreatedAt = now.Add(-time.Hour)
	p.EffectiveFrom = p.CreatedAt
	p.Reason = "operational adjustment"
	p.Anonymous.BurstUnits = 222
	return p
}

func TestResolverFallsBackToLegacyDefaults(t *testing.T) {
	r := NewResolver(&fakeRepo{err: errors.New("offline")}, time.Second)
	got := r.Resolve(context.Background(), identity.Anonymous("1.2.3.4"), false, identity.CostNormal)
	want := identity.AllowanceFor(identity.Anonymous("1.2.3.4"), false)
	if got.Allowance != want || !got.Allowed {
		t.Fatalf("fallback=%+v want=%+v", got, want)
	}
}

func TestResolverRefreshesPolicyAndApplicationOverride(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	p := validPolicy(now)
	repo := &fakeRepo{policy: &p}
	r := NewResolver(repo, time.Minute)
	r.now = func() time.Time { return now }
	anon := r.Resolve(context.Background(), identity.Anonymous("ip"), false, identity.CostNormal)
	if anon.Allowance.BurstUnits != 222 {
		t.Fatalf("policy burst=%d", anon.Allowance.BurstUnits)
	}
	o := identity.FairUseOverride{ID: "ovr-1", ApplicationID: "app-1", Allowance: identity.Allowance{BurstUnits: 3000, RefillPerSecond: 50, Window: time.Hour}, CostCeilings: p.CostCeilings, Enabled: true, Reason: "public health response", ActorID: "admin-1", CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Hour)}
	repo.override = &o
	id := identity.FromKey(&identity.APIKey{ApplicationID: "app-1", Prefix: "key"})
	got := r.Resolve(context.Background(), id, false, identity.CostGeometry)
	if got.Allowance.BurstUnits != 3000 || got.Source != "override:ovr-1" {
		t.Fatalf("override not applied: %+v", got)
	}
}

func TestResolverCacheRefreshAndStaleSafety(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	p := validPolicy(now)
	repo := &fakeRepo{policy: &p}
	r := NewResolver(repo, time.Minute)
	r.now = func() time.Time { return now }
	first := r.Resolve(context.Background(), identity.Anonymous("ip"), false, identity.CostCheap)
	p.Anonymous.BurstUnits = 333
	r.Resolve(context.Background(), identity.Anonymous("ip"), false, identity.CostCheap)
	if repo.reads != 1 {
		t.Fatalf("cache missed: reads=%d", repo.reads)
	}
	now = now.Add(2 * time.Minute)
	repo.err = errors.New("transient")
	stale := r.Resolve(context.Background(), identity.Anonymous("ip"), false, identity.CostCheap)
	if stale.Allowance != first.Allowance {
		t.Fatal("repository failure discarded last valid policy")
	}
	repo.err = nil
	r.Invalidate()
	fresh := r.Resolve(context.Background(), identity.Anonymous("ip"), false, identity.CostCheap)
	if fresh.Allowance.BurstUnits != 333 {
		t.Fatalf("live refresh burst=%d", fresh.Allowance.BurstUnits)
	}
}

func TestExpiredOverrideCannotLeakFromCache(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	p := validPolicy(now)
	o := identity.FairUseOverride{ID: "ovr", ApplicationID: "app", Allowance: identity.Allowance{BurstUnits: 3000, RefillPerSecond: 50, Window: time.Hour}, CostCeilings: p.CostCeilings, Enabled: true, Reason: "temporary import", ActorID: "admin", CreatedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Second)}
	repo := &fakeRepo{policy: &p, override: &o}
	r := NewResolver(repo, time.Hour)
	r.now = func() time.Time { return now }
	id := identity.FromKey(&identity.APIKey{ApplicationID: "app", Prefix: "key"})
	if r.Resolve(context.Background(), id, false, identity.CostCheap).Source != "override:ovr" {
		t.Fatal("active override not used")
	}
	now = now.Add(2 * time.Second)
	if got := r.Resolve(context.Background(), id, false, identity.CostCheap); got.Source == "override:ovr" {
		t.Fatal("expired cached override remained active")
	}
}

func TestResolverPreservesLegacyElevationDuringMigration(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	p := validPolicy(now)
	r := NewResolver(&fakeRepo{policy: &p}, time.Minute)
	r.now = func() time.Time { return now }
	id := identity.FromKey(&identity.APIKey{ApplicationID: "app", Prefix: "key", Elevated: true, ElevatedReason: "legacy grant"})
	got := r.Resolve(context.Background(), id, false, identity.CostNormal)
	if got.Source != "legacy-elevated" || got.Allowance.BurstUnits <= p.Authenticated.BurstUnits {
		t.Fatalf("legacy elevation lost: %+v", got)
	}
}

func TestPolicyCanDisableCostClass(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	p := validPolicy(now)
	p.CostCeilings[identity.CostGeometry] = 0
	r := NewResolver(&fakeRepo{policy: &p}, time.Minute)
	r.now = func() time.Time { return now }
	got := r.Resolve(context.Background(), identity.Anonymous("ip"), false, identity.CostGeometry)
	if got.Allowed {
		t.Fatal("geometry request passed a disabled cost-class ceiling")
	}
}
